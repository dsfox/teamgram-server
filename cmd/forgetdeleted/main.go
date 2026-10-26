// Command forgetdeleted removes what deleted messages left on the server (#220).
//
// Until #220 a deleted message was only marked (deleted = 1): its row kept the
// words or the ciphertext, user_pts_updates kept a copy for getDifference to
// replay, reactions to it stayed, and so did the file it carried. The server
// removes all of that when a message is deleted now (pkg/deletion), except the
// file. This command does the rest, for what older servers left and for files:
//
//   - copies in user_pts_updates of a message the person's box no longer
//     holds lose their words, file and entities (the rows stay: a gap in pts
//     breaks every phone's catch-up);
//   - reactions to a message nobody's box holds any more go;
//   - rows marked deleted go;
//   - a document or photo nothing in the database names any more goes, with
//     its files, once it is older than FreshFor - an upload a message has not
//     named yet is not abandoned;
//
// A file on disk that no row describes is left alone: the server puts files
// there itself without one - the reaction pictures (pkg/reactions Place) -
// and a deleted message's file always has its row.
//
// "Names" is measured, not listed: every text and json column of every table
// is read for long numbers, and every integer column whose name says photo,
// document, file, thumb, music, wallpaper or sticker is read for its values.
// A table added later is covered without anybody remembering this command.
//
// Without -apply it only counts. It exits non-zero unless, afterwards, no
// marked row and no readable copy of a gone message is left.
//
//	docker exec ice9-teamgram /app/bin/forgetdeleted          # counts
//	docker exec ice9-teamgram /app/bin/forgetdeleted -apply   # does it
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/pkg/deletion"
)

// FreshFor is how old an upload has to be before nothing naming it means it
// was abandoned. An album is uploaded first and sent after; two hours is far
// past any of that.
const FreshFor = 2 * time.Hour

// A copy in user_pts_updates of a message, still holding something readable,
// whose row in the person's box is gone or marked.
const goneCopy = "json_unquote(json_extract(u.update_data, '$.predicate_name')) in ('updateNewMessage', 'updateEditMessage') " +
	"and (coalesce(json_unquote(json_extract(u.update_data, '$.message_MESSAGE.message')), '') <> '' " +
	"or json_contains_path(u.update_data, 'one', '$.message_MESSAGE.media') " +
	"or json_contains_path(u.update_data, 'one', '$.message_MESSAGE.entities')) " +
	"and not exists (select 1 from messages m where m.user_id = u.user_id " +
	"and m.user_message_box_id = cast(json_extract(u.update_data, '$.message_MESSAGE.id') as signed) and m.deleted = 0)"

// A reaction to a message no box holds any more.
const goneReaction = "not exists (select 1 from messages m where m.dialog_message_id = r.dialog_message_id and m.deleted = 0)"

// The tables that describe files; what names a file is looked for everywhere else.
var fileTables = map[string]bool{"documents": true, "photos": true, "photo_sizes": true, "video_sizes": true}

var longNumber = regexp.MustCompile(`-?\d{12,20}`)

type counts struct {
	Marked    int64 `db:"marked"`
	Copies    int64 `db:"copies"`
	Reactions int64 `db:"reactions"`
}

func count(ctx context.Context, db *sqlx.DB) (c counts, err error) {
	err = db.QueryRowPartial(ctx, &c, "select "+
		"(select count(*) from messages where deleted = 1) as marked, "+
		"(select count(*) from user_pts_updates u where "+goneCopy+") as copies, "+
		"(select count(*) from message_reactions r where "+goneReaction+") as reactions")
	return
}

type column struct {
	Table  string `db:"table_name"`
	Column string `db:"column_name"`
	Type   string `db:"data_type"`
}

// named reads every id the database mentions outside the file tables.
func named(ctx context.Context, db *sqlx.DB) (map[int64]bool, error) {
	var columns []column
	if err := db.QueryRowsPartial(ctx, &columns, "select table_name as table_name, column_name as column_name, data_type as data_type "+
		"from information_schema.columns where table_schema = database() and ("+
		"data_type in ('json', 'text', 'tinytext', 'mediumtext', 'longtext', 'varchar', 'char') or "+
		"(data_type = 'bigint' and column_name regexp 'photo|document|file|thumb|music|wallpaper|sticker'))"); err != nil {
		return nil, fmt.Errorf("cannot list the columns: %w", err)
	}
	ids := map[int64]bool{}
	for _, c := range columns {
		if fileTables[c.Table] {
			continue
		}
		// What a deleted message or a copy of a gone one names is not a use:
		// those go in this run, so the dry run counts what -apply frees.
		where := ""
		switch c.Table {
		case "messages":
			where = " and u.deleted = 0"
		case "user_pts_updates":
			where = " and not (" + goneCopy + ")"
		}
		var values []string
		query := fmt.Sprintf("select cast(u.`%s` as char) from `%s` u where u.`%s` is not null%s", c.Column, c.Table, c.Column, where)
		if err := db.QueryRowsPartial(ctx, &values, query); err != nil && err != sqlx.ErrNotFound {
			return nil, fmt.Errorf("cannot read %s.%s: %w", c.Table, c.Column, err)
		}
		for _, v := range values {
			for _, found := range longNumber.FindAllString(v, -1) {
				if id, err := strconv.ParseInt(found, 10, 64); err == nil && id != 0 {
					ids[id] = true
				}
			}
		}
	}
	return ids, nil
}

type document struct {
	Id      int64  `db:"document_id"`
	Path    string `db:"file_path"`
	Size    int64  `db:"file_size"`
	Thumb   int64  `db:"thumb_id"`
	Video   int64  `db:"video_thumb_id"`
	Created int64  `db:"date2"`
}

type photo struct {
	Id        int64 `db:"photo_id"`
	SizeId    int64 `db:"size_id"`
	VideoSize int64 `db:"video_size_id"`
	Created   int64 `db:"date2"`
}

type sizeFile struct {
	Path string `db:"file_path"`
}

type files struct {
	dir   string
	apply bool
	gone  int
	bytes int64
	seen  map[string]bool
}

// remove takes a file away, if it is there, and counts it once.
func (f *files) remove(bucket, path string) {
	name := filepath.Join(f.dir, bucket, filepath.Clean("/"+path))
	if f.seen[name] {
		return
	}
	info, err := os.Stat(name)
	if err != nil {
		return
	}
	f.seen[name] = true
	f.gone++
	f.bytes += info.Size()
	if f.apply {
		if err := os.Remove(name); err != nil {
			fmt.Printf("cannot remove %s: %v\n", name, err)
		}
	}
}

func main() {
	apply := flag.Bool("apply", false, "do it; without it, only count")
	dir := flag.String("files", "/app/data/files", "the directory the dfs service keeps files in")
	flag.Parse()

	dsn := os.Getenv("FORGET_MYSQL_DSN")
	if dsn == "" {
		dsn = "teamgram:" + os.Getenv("MYSQL_PASSWORD") + "@tcp(mysql:3306)/teamgram?charset=utf8mb4&parseTime=true"
	}
	db := sqlx.NewMySQL(&sqlx.Config{DSN: dsn, Active: 2, Idle: 2})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	before, err := count(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot count: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("before: %d marked row(s), %d readable copie(s) of gone messages, %d reaction(s) to gone messages\n",
		before.Marked, before.Copies, before.Reactions)

	if *apply {
		exec := func(q string, args ...interface{}) (int64, error) {
			r, err := db.Exec(ctx, q, args...)
			if err != nil {
				return 0, err
			}
			return r.RowsAffected()
		}
		blanked, err := exec("update user_pts_updates u set u.update_data = " +
			strings.ReplaceAll(deletion.Blanked, "update_data", "u.update_data") + " where " + goneCopy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "the copies stay: %v\n", err)
			os.Exit(1)
		}
		reactions, err := exec("delete r from message_reactions r where " + goneReaction)
		if err != nil {
			fmt.Fprintf(os.Stderr, "the reactions stay: %v\n", err)
			os.Exit(1)
		}
		rows, err := exec("delete from messages where deleted = 1")
		if err != nil {
			fmt.Fprintf(os.Stderr, "the marked rows stay: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("blanked %d copie(s), removed %d reaction(s) and %d row(s)\n", blanked, reactions, rows)
	}

	// Files. Read after the rows went, so what only a deleted message named is free.
	ids, err := named(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	old := time.Now().Add(-FreshFor).Unix()
	var documents []document
	if err = db.QueryRowsPartial(ctx, &documents,
		"select document_id, file_path, file_size, thumb_id, video_thumb_id, date2 from documents"); err != nil && err != sqlx.ErrNotFound {
		fmt.Fprintf(os.Stderr, "cannot read documents: %v\n", err)
		os.Exit(1)
	}
	var photos []photo
	if err = db.QueryRowsPartial(ctx, &photos,
		"select photo_id, size_id, video_size_id, date2 from photos"); err != nil && err != sqlx.ErrNotFound {
		fmt.Fprintf(os.Stderr, "cannot read photos: %v\n", err)
		os.Exit(1)
	}

	disk := &files{dir: *dir, apply: *apply, seen: map[string]bool{}}
	keptDocuments := map[string]bool{}
	thumbs := map[int64]bool{}
	var freeDocuments []document
	for _, d := range documents {
		if d.Created < old && !ids[d.Id] {
			freeDocuments = append(freeDocuments, d)
			continue
		}
		keptDocuments[d.Path] = true
		keptDocuments[fmt.Sprintf("%d.dat", d.Id)] = true
		thumbs[d.Id], thumbs[d.Thumb], thumbs[d.Video] = true, true, true
	}
	keptPhotos := map[int64]bool{}
	var freePhotos []photo
	for _, p := range photos {
		if p.Created < old && !ids[p.Id] && !ids[p.SizeId] && !thumbs[p.Id] {
			freePhotos = append(freePhotos, p)
			continue
		}
		keptPhotos[p.Id], keptPhotos[p.SizeId] = true, true
	}

	var documentBytes int64
	for _, d := range freeDocuments {
		documentBytes += d.Size
		disk.remove("documents", d.Path)
		disk.remove("photos", fmt.Sprintf("m/%d.dat", d.Id))
		if *apply {
			if _, err := db.Exec(ctx, "delete from documents where document_id = ?", d.Id); err != nil {
				fmt.Printf("document %d stays: %v\n", d.Id, err)
			}
		}
	}
	for _, p := range freePhotos {
		// The sizes are keyed by the photo's own id; size_id is 0 on every row
		// this server wrote, and is read too in case an upstream path sets it.
		keys := []int64{p.Id}
		if p.SizeId != 0 && p.SizeId != p.Id {
			keys = append(keys, p.SizeId)
		}
		videoKeys := []int64{p.Id}
		if p.VideoSize != 0 && p.VideoSize != p.Id {
			videoKeys = append(videoKeys, p.VideoSize)
		}
		in, videoIn := sqlx.InInt64List(keys), sqlx.InInt64List(videoKeys)
		var sizes []sizeFile
		_ = db.QueryRowsPartial(ctx, &sizes, "select file_path from photo_sizes where photo_size_id in ("+in+")")
		for _, s := range sizes {
			disk.remove("photos", s.Path)
		}
		disk.remove("photos", fmt.Sprintf("0/%d.dat", p.Id))
		var videos []sizeFile
		_ = db.QueryRowsPartial(ctx, &videos, "select file_path from video_sizes where video_size_id in ("+videoIn+")")
		for _, v := range videos {
			disk.remove("videos", v.Path)
		}
		if *apply {
			for _, q := range []string{
				"delete from photo_sizes where photo_size_id in (" + in + ")",
				"delete from video_sizes where video_size_id in (" + videoIn + ")",
				"delete from photos where photo_id = " + strconv.FormatInt(p.Id, 10),
			} {
				if _, err := db.Exec(ctx, q); err != nil {
					fmt.Printf("photo %d stays: %v\n", p.Id, err)
				}
			}
		}
	}
	fmt.Printf("documents nothing names: %d (%.1f MB), photos nothing names: %d\n",
		len(freeDocuments), float64(documentBytes)/1048576, len(freePhotos))

	verb := "would remove"
	if *apply {
		verb = "removed"
	}
	fmt.Printf("files: %s %d (%.1f MB)\n", verb, disk.gone, float64(disk.bytes)/1048576)

	if !*apply {
		fmt.Println("nothing changed: run with -apply to remove them")
		return
	}
	after, err := count(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot count again: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("after: %d marked row(s), %d readable copie(s) of gone messages, %d reaction(s) to gone messages\n",
		after.Marked, after.Copies, after.Reactions)
	if after.Marked != 0 || after.Copies != 0 || after.Reactions != 0 {
		os.Exit(1)
	}
}
