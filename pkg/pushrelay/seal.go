package pushrelay

import (
	"strconv"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/fcm"
)

// SealForApple is the envelope an iPhone's notification extension opens: the
// alert again, so the extension starts from the same words the phone would
// show, and the ids as decimal strings at the top level - the shape upstream's
// extension reads (#42). Sealed with the phone's own registered secret, so the
// relay that carries it cannot open it.
func SealForApple(secretHex, title, body string, badge int, peerType int32, peerId int64, msgId int32) (string, error) {
	inside := map[string]any{
		"aps": map[string]any{
			"alert": map[string]any{"title": title, "body": body},
			"sound": "default",
			"badge": badge,
		},
		peerKey(peerType): strconv.FormatInt(peerId, 10),
		"msg_id":          strconv.FormatInt(int64(msgId), 10),
	}
	return fcm.Envelope(secretHex, inside)
}

// SealForGoogle is the Android equivalent, in the shape the app reads. No
// loc_key the client knows: it is not being told what to draw, only that
// something arrived; everything it shows afterwards it fetches itself.
func SealForGoogle(secretHex string, badge int, fromId string) (string, error) {
	return fcm.Envelope(secretHex, map[string]any{
		"badge":   badge,
		"custom":  map[string]any{"from_id": fromId},
		"loc_key": "",
	})
}

// peerKey is the key the extension reads a chat's id under.
func peerKey(peerType int32) string {
	switch peerType {
	case int32(mtproto.PEER_CHAT):
		return "chat_id"
	case int32(mtproto.PEER_CHANNEL):
		return "channel_id"
	default:
		return "from_id"
	}
}
