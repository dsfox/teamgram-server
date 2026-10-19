// Command forgetsaved empties Saved Messages on the server, the way a person
// clearing the chat would, and leaves no copy of the words behind (#284).
//
// The server has refused every write into Saved Messages since 16 October
// (#271): a note there was the one thing kept in the clear. What was written
// before stayed - rows in messages, a dialog pointing at them, and a copy of
// each in user_pts_updates, the log getDifference replays. The owner's word of
// 19 October: remove them, through the server's own path, after a backup.
//
// For each account with a Saved Messages dialog or message:
//   - msg.deleteHistory, as messages.deleteHistory calls it: the rows are
//     marked deleted and every session of the account is told
//     (updateDeleteMessages), so phones drop their copies;
//   - dialog.deleteDialog, which also drops the cached dialog;
//   - its saved_dialogs rows, which only point at those messages.
//
// Then, for everybody: the rows deleteHistory marked are removed for good (a
// row marked deleted still holds the words, #220), and every updateNewMessage
// of a message in Saved Messages in user_pts_updates has its words and media
// taken out. Those rows are blanked rather than deleted: getDifference reads
// pts in order, and a gap would be a hole in every phone's sequence.
//
// Without -apply it only counts. It exits non-zero unless, afterwards, nothing
// readable is left anywhere it looked.
//
// Run in the server's container:
//
//	docker exec ice9-teamgram /app/bin/forgetsaved           # counts
//	docker exec ice9-teamgram /app/bin/forgetsaved -apply    # does it
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/proto/mtproto"
	msgclient "github.com/teamgram/teamgram-server/app/messenger/msg/msg/client"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"
	dialogclient "github.com/teamgram/teamgram-server/app/service/biz/dialog/client"
	dialogpb "github.com/teamgram/teamgram-server/app/service/biz/dialog/dialog"

	"github.com/zeromicro/go-zero/zrpc"
)

// A message in Saved Messages: written by the account, to the account.
const savedMessage = "user_id = peer_id and sender_user_id = user_id and peer_type = 2"

// An update in the pts log that carries a message in Saved Messages.
const savedUpdate = "json_unquote(json_extract(update_data, '$.predicate_name')) = 'updateNewMessage' " +
	"and json_extract(update_data, '$.message_MESSAGE.peer_id.user_id') = user_id " +
	"and json_extract(update_data, '$.message_MESSAGE.from_id.user_id') = user_id"

// The same, still holding words or a file.
const readableUpdate = savedUpdate + " and (json_unquote(json_extract(update_data, '$.message_MESSAGE.message')) <> '' " +
	"or json_contains_path(update_data, 'one', '$.message_MESSAGE.media'))"

type counts struct {
	Accounts int64 `db:"accounts"`
	Rows     int64 `db:"message_rows"`
	Dialogs  int64 `db:"dialogs"`
	Saved    int64 `db:"saved_rows"`
	Updates  int64 `db:"updates"`
}

func count(ctx context.Context, db *sqlx.DB) (c counts, err error) {
	err = db.QueryRowPartial(ctx, &c, "select "+
		"(select count(distinct user_id) from messages where "+savedMessage+") as accounts, "+
		"(select count(*) from messages where "+savedMessage+") as message_rows, "+
		"(select count(*) from dialogs where peer_type = 2 and peer_id = user_id) as dialogs, "+
		"(select count(*) from saved_dialogs) as saved_rows, "+
		"(select count(*) from user_pts_updates where "+readableUpdate+") as updates")
	return
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func main() {
	apply := flag.Bool("apply", false, "do it; without it, only count")
	flag.Parse()

	dsn := os.Getenv("FORGET_MYSQL_DSN")
	if dsn == "" {
		dsn = "teamgram:" + os.Getenv("MYSQL_PASSWORD") + "@tcp(mysql:3306)/teamgram?charset=utf8mb4&parseTime=true"
	}
	db := sqlx.NewMySQL(&sqlx.Config{DSN: dsn, Active: 2, Idle: 2})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	before, err := count(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot count: %v\n", err)
		os.Exit(2)
	}
	fmt.Printf("before: %d message row(s) of %d account(s), %d dialog(s), %d saved_dialogs row(s), %d readable update(s)\n",
		before.Rows, before.Accounts, before.Dialogs, before.Saved, before.Updates)
	if !*apply {
		fmt.Println("nothing changed: run with -apply to remove them")
		return
	}

	// Inside the container, as the configs in etc2 name them.
	msg := msgclient.NewMsgClient(zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{envOr("FORGET_MSG_ENDPOINT", "127.0.0.1:20030")},
		NonBlock:  true,
		Timeout:   15000,
	}))
	dialogs := dialogclient.NewDialogClient(zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{envOr("FORGET_BIZ_ENDPOINT", "127.0.0.1:20020")},
		NonBlock:  true,
		Timeout:   15000,
	}))

	var owners []int64
	if err = db.QueryRowsPartial(ctx, &owners,
		"select user_id from messages where "+savedMessage+" and deleted = 0 "+
			"union select user_id from dialogs where peer_type = 2 and peer_id = user_id "+
			"union select user_id from saved_dialogs"); err != nil && err != sqlx.ErrNotFound {
		fmt.Fprintf(os.Stderr, "cannot list the accounts: %v\n", err)
		os.Exit(2)
	}

	failed := 0
	for _, owner := range owners {
		cleared, err := msg.MsgDeleteHistory(ctx, &msgpb.TLMsgDeleteHistory{
			UserId:   owner,
			PeerType: mtproto.PEER_USER,
			PeerId:   owner,
		})
		if err != nil {
			fmt.Printf("%d: history not cleared: %v\n", owner, err)
			failed++
			continue
		}
		if _, err = dialogs.DialogDeleteDialog(ctx, &dialogpb.TLDialogDeleteDialog{
			UserId:   owner,
			PeerType: mtproto.PEER_USER,
			PeerId:   owner,
		}); err != nil {
			fmt.Printf("%d: dialog stays: %v\n", owner, err)
			failed++
			continue
		}
		if _, err = db.Exec(ctx, "delete from saved_dialogs where user_id = ?", owner); err != nil {
			fmt.Printf("%d: saved_dialogs stay: %v\n", owner, err)
			failed++
			continue
		}
		fmt.Printf("%d: cleared, %d update(s) to its sessions\n", owner, cleared.GetPtsCount())
	}

	purged, err := db.Exec(ctx, "delete from messages where "+savedMessage+" and deleted = 1")
	if err != nil {
		fmt.Fprintf(os.Stderr, "the marked rows stay: %v\n", err)
		os.Exit(1)
	}
	gone, _ := purged.RowsAffected()

	blanked, err := db.Exec(ctx, "update user_pts_updates set update_data = json_remove("+
		"json_set(update_data, '$.message_MESSAGE.message', ''), "+
		"'$.message_MESSAGE.media', '$.message_MESSAGE.entities') where "+readableUpdate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "the copies in user_pts_updates stay: %v\n", err)
		os.Exit(1)
	}
	emptied, _ := blanked.RowsAffected()

	after, err := count(ctx, db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot count again: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("removed %d row(s), blanked %d update(s), %d account(s) failed\n", gone, emptied, failed)
	fmt.Printf("after: %d message row(s), %d dialog(s), %d saved_dialogs row(s), %d readable update(s)\n",
		after.Rows, after.Dialogs, after.Saved, after.Updates)
	if failed > 0 || after.Rows != 0 || after.Dialogs != 0 || after.Saved != 0 || after.Updates != 0 {
		os.Exit(1)
	}
}
