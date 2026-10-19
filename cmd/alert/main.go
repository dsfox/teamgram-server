// Command alert tells the owner about a server problem, in the owner's chat
// with ice9 (777000).
//
// The health check finds trouble, but a finding sitting in a log file on the
// server is only useful to someone who goes and looks. This puts it where the
// owner reads: a message from the service account, the way a login code
// arrives. The ordinary notification path then wakes the phone through the
// push relay, which is where the Apple and Google keys are (#167).
//
// It used to talk to Apple itself, with a key this container stopped having
// when the keys moved to the relay - and from then on answered "no Apple key
// configured" to a cron job that went on counting the alerts as sent (#226).
//
// The words are kept on the server in the clear, in the service chat, as the
// login codes are: they are the server's words about itself, not anybody's
// conversation (the owner's word of 19 October).
//
// The owner is named explicitly by ALERT_USER_ID - an alert must never reach
// whoever happens to be signed in - and an id that names no account is an
// error said out loud: ALERT_USER_ID named an account that no longer existed,
// and nothing said so.
//
// Usage: alert "text of the problem"
package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/proto/mtproto"
	msgclient "github.com/teamgram/teamgram-server/app/messenger/msg/msg/client"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"

	"github.com/zeromicro/go-zero/zrpc"
)

// ServiceUserId is the account the server speaks as.
const ServiceUserId = 777000

func parseOwners(raw string) []int64 {
	var owners []int64
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil && id != 0 {
			owners = append(owners, id)
		}
	}

	return owners
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// One account, as much of it as says whether it is there.
type userRow struct {
	Id      int64 `db:"id"`
	Deleted bool  `db:"deleted"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: alert \"text of the problem\"")
		os.Exit(2)
	}
	text := strings.Join(os.Args[1:], " ")

	// A comma separated list: the owner may have more than one account.
	owners := parseOwners(os.Getenv("ALERT_USER_ID"))
	if len(owners) == 0 {
		fmt.Fprintln(os.Stderr, "ALERT_USER_ID is not set: nobody to notify")
		os.Exit(2)
	}

	dsn := os.Getenv("ALERT_MYSQL_DSN")
	if dsn == "" {
		dsn = "teamgram:" + os.Getenv("MYSQL_PASSWORD") + "@tcp(mysql:3306)/teamgram?charset=utf8mb4&parseTime=true"
	}
	db := sqlx.NewMySQL(&sqlx.Config{DSN: dsn, Active: 2, Idle: 2})

	// The message service, where the login codes go too: it stores the
	// message in both boxes and hands the update to sync, which wakes the
	// phone. Inside the container, as the configs in etc2 name it.
	msg := msgclient.NewMsgClient(zrpc.MustNewClient(zrpc.RpcClientConf{
		Endpoints: []string{envOr("ALERT_MSG_ENDPOINT", "127.0.0.1:20030")},
		NonBlock:  true,
		Timeout:   15000,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sent := 0
	for _, owner := range owners {
		var row userRow
		err := db.QueryRowPartial(ctx, &row, "select id, deleted from users where id = ?", owner)
		if err != nil || row.Deleted {
			fmt.Fprintf(os.Stderr, "ALERT_USER_ID names %d, and there is no such account here (%v)\n", owner, err)
			continue
		}
		message := mtproto.MakeTLMessage(&mtproto.Message{
			Out:     true,
			Date:    int32(time.Now().Unix()),
			FromId:  mtproto.MakePeerUser(ServiceUserId),
			PeerId:  mtproto.MakeTLPeerUser(&mtproto.Peer{UserId: owner}).To_Peer(),
			Message: "Server problem: " + text,
		}).To_Message()
		_, err = msg.MsgPushUserMessage(ctx, &msgpb.TLMsgPushUserMessage{
			UserId:    ServiceUserId,
			AuthKeyId: 0,
			PeerType:  mtproto.PEER_USER,
			PeerId:    owner,
			PushType:  1,
			Message: msgpb.MakeTLOutboxMessage(&msgpb.OutboxMessage{
				RandomId: rand.Int63(),
				Message:  message,
			}).To_OutboxMessage(),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot write to %d: %v\n", owner, err)
			continue
		}
		sent++
	}

	if sent == 0 {
		fmt.Fprintln(os.Stderr, "the alert reached nobody")
		os.Exit(1)
	}
	fmt.Printf("alert written to the chat with ice9 of %d owner(s)\n", sent)
}
