// Command pushpayload prints the APNs payload the server would send about one
// message, for a walk to hand to a simulator with `xcrun simctl push` (#42).
//
// The simulator cannot be reached through Apple, and the server does not log
// what it sends - the envelope is keyed by the device's secret. So the walk
// reads the secret and the message off the server's database and asks this
// for the payload, built by the very code the notifier uses.
//
// Usage: pushpayload -secret <hex> -peer <user|group|channel> -peer-id <id> -msg-id <id> [-badge n]
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/apns"
)

// kinds are the peer types by the words a walk uses for them.
var kinds = map[string]int32{
	"user":    int32(mtproto.PEER_USER),
	"group":   int32(mtproto.PEER_CHAT),
	"channel": int32(mtproto.PEER_CHANNEL),
}

func main() {
	secret := flag.String("secret", "", "the device's push secret, hex as the database holds it")
	peer := flag.String("peer", "user", "user, group or channel")
	peerId := flag.Int64("peer-id", 0, "the chat's id")
	msgId := flag.Int("msg-id", 0, "the message's id in the recipient's box")
	badge := flag.Int("badge", 1, "the unread count")
	flag.Parse()
	peerType, known := kinds[*peer]
	if *secret == "" || *peerId == 0 || *msgId == 0 || !known {
		flag.Usage()
		os.Exit(2)
	}
	fromId := ""
	if peerType == int32(mtproto.PEER_USER) {
		fromId = fmt.Sprint(*peerId)
	}
	out, err := apns.PayloadJSON(apns.Notify{
		Title:    "ice9",
		Body:     "New message",
		Badge:    *badge,
		FromId:   fromId,
		PeerType: peerType,
		PeerId:   *peerId,
		MsgId:    int32(*msgId),
		Secret:   *secret,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(out)
}
