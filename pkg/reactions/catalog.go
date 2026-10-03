package reactions

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/teamgram/proto/mtproto"
)

// Where the image keeps the pictures (Dockerfile: teamgramd/ becomes /app/),
// and where the file service reads documents from (rewrite-configs.py sets
// Minio.Dir to it on every server we build).
const (
	DefaultDir   = "/app/reactions"
	DefaultFiles = "/app/data/files"
)

// The date every reaction document carries. Clients do not read it; a fixed
// one keeps the answer the same from one start to the next.
const documentDate = 1790985600

// Catalog is the set of reactions this server offers, as
// tools/reaction_assets.py wrote it into teamgramd/reactions/.
//
// A client draws a reaction only from the documents the server lists for it,
// and iOS leaves out any reaction missing one of them. So the list is built
// from the same manifest that names the files, and Place puts those files
// where the file service hands documents out; neither can name a picture the
// other does not have.
type Catalog struct {
	dir     string
	hash    int32
	entries []catalogEntry
	byEmoji map[string]bool
}

type catalogFile struct {
	File       string `json:"file"`
	Mime       string `json:"mime"`
	Size       int64  `json:"size"`
	Sha256     string `json:"sha256"`
	DocumentId int64  `json:"document_id"`
}

type catalogEntry struct {
	Emoji     string      `json:"emoji"`
	Title     string      `json:"title"`
	Icon      catalogFile `json:"icon"`
	Animation catalogFile `json:"animation"`
}

func Load(dir string) (*Catalog, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Hash      int32          `json:"hash"`
		Reactions []catalogEntry `json:"reactions"`
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("manifest.json in %s: %w", dir, err)
	}
	if len(manifest.Reactions) == 0 || manifest.Hash == 0 {
		return nil, fmt.Errorf("manifest.json in %s names no reactions or has no hash", dir)
	}
	c := &Catalog{dir: dir, hash: manifest.Hash, entries: manifest.Reactions, byEmoji: map[string]bool{}}
	for _, entry := range c.entries {
		for _, file := range []catalogFile{entry.Icon, entry.Animation} {
			if sum, err := hex.DecodeString(file.Sha256); err != nil || len(sum) != sha256.Size || file.DocumentId == 0 {
				return nil, fmt.Errorf("manifest.json in %s: %s has no checksum or no document id", dir, file.File)
			}
		}
		c.byEmoji[entry.Emoji] = true
	}
	return c, nil
}

// Hash is what messages.getAvailableReactions answers with. Never zero: every
// client cached the empty list the server used to give under zero.
func (c *Catalog) Hash() int32 {
	return c.hash
}

func (c *Catalog) Offers(emoji string) bool {
	return c.byEmoji[emoji]
}

// Reactions is the set in order, as getTopReactions and the default list name it.
func (c *Catalog) Reactions() []*mtproto.Reaction {
	list := make([]*mtproto.Reaction, 0, len(c.entries))
	for _, entry := range c.entries {
		list = append(list, mtproto.FromReaction(entry.Emoji))
	}
	return list
}

// Available is the list messages.getAvailableReactions answers with.
//
// Telegram has five animations and two icons for each reaction; we have one
// animation and one still, so the animation stands in every animated place.
// Each of the five is required by the schema and by iOS, which drops a
// reaction lacking one; the last two only make it appear in the menu.
func (c *Catalog) Available() []*mtproto.AvailableReaction {
	list := make([]*mtproto.AvailableReaction, 0, len(c.entries))
	for _, entry := range c.entries {
		icon := entry.Icon.document(entry.Emoji)
		animation := entry.Animation.document(entry.Emoji)
		list = append(list, mtproto.MakeTLAvailableReaction(&mtproto.AvailableReaction{
			Reaction:          entry.Emoji,
			Title:             entry.Title,
			StaticIcon:        icon,
			AppearAnimation:   animation,
			SelectAnimation:   animation,
			ActivateAnimation: animation,
			EffectAnimation:   animation,
			AroundAnimation:   animation,
			CenterIcon:        animation,
		}).To_AvailableReaction())
	}
	return list
}

// Place puts every picture where the file service reads documents from,
// <files>/documents/<document id>.dat, and says how many it had to write.
// A file already there with the right bytes is left alone, so this runs at
// every start; a source whose bytes are not the manifest's is refused rather
// than served.
func (c *Catalog) Place(files string) (int, error) {
	target := filepath.Join(files, "documents")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return 0, err
	}
	written := 0
	for _, entry := range c.entries {
		for _, file := range []catalogFile{entry.Icon, entry.Animation} {
			path := filepath.Join(target, fmt.Sprintf("%d.dat", file.DocumentId))
			if current, err := os.ReadFile(path); err == nil && file.matches(current) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(c.dir, file.File))
			if err != nil {
				return written, err
			}
			if !file.matches(data) {
				return written, fmt.Errorf("%s is not the file the manifest names", file.File)
			}
			partial := path + ".partial"
			if err = os.WriteFile(partial, data, 0o644); err != nil {
				return written, err
			}
			if err = os.Rename(partial, path); err != nil {
				return written, err
			}
			written++
		}
	}
	return written, nil
}

func (f catalogFile) matches(data []byte) bool {
	sum := sha256.Sum256(data)
	return int64(len(data)) == f.Size && hex.EncodeToString(sum[:]) == f.Sha256
}

// The access hash carries the storage type in its high half, as the file
// service writes it (dfs.uploadDocumentFileV2) and reads it back when it hands
// the file out; the low half is the file's own, the same on every server.
func (f catalogFile) accessHash() int64 {
	storage := mtproto.CRC32_storage_filePartial
	if f.Mime == "image/webp" {
		storage = mtproto.CRC32_storage_fileWebp
	}
	sum, _ := hex.DecodeString(f.Sha256)
	return int64(int32(storage))<<32 | int64(binary.BigEndian.Uint32(sum[:4]))
}

// A file name is what makes iOS play an x-tgsticker document at all
// (TelegramMediaFile.isAnimatedSticker); the sticker attribute names the
// emoji it stands for, as Telegram's own reaction documents do.
func (f catalogFile) document(emoji string) *mtproto.Document {
	return mtproto.MakeTLDocument(&mtproto.Document{
		Id:            f.DocumentId,
		AccessHash:    f.accessHash(),
		FileReference: []byte{},
		Date:          documentDate,
		MimeType:      f.Mime,
		Size2:         f.Size,
		Size2_INT32:   int32(f.Size),
		Size2_INT64:   f.Size,
		DcId:          1,
		Attributes: []*mtproto.DocumentAttribute{
			mtproto.MakeTLDocumentAttributeImageSize(&mtproto.DocumentAttribute{W: 512, H: 512}).To_DocumentAttribute(),
			mtproto.MakeTLDocumentAttributeSticker(&mtproto.DocumentAttribute{
				Alt:        emoji,
				Stickerset: mtproto.MakeTLInputStickerSetEmpty(nil).To_InputStickerSet(),
			}).To_DocumentAttribute(),
			mtproto.MakeTLDocumentAttributeFilename(&mtproto.DocumentAttribute{FileName: f.File}).To_DocumentAttribute(),
		},
	}).To_Document()
}
