package pushrelay

import (
	"encoding/base64"
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

// SealForAppleCall is the envelope an iPhone opens on a VoIP push (#14): the
// TL update of the call itself, base64url without padding under "updates" -
// the app reports the call to CallKit from it and creates the session from
// it, and asks the server for nothing. The update names the caller; it is
// sealed with the phone's own secret, so the relay carries a name it cannot
// read.
func SealForAppleCall(secretHex string, updates []byte) (string, error) {
	return fcm.Envelope(secretHex, map[string]any{
		"updates": base64.RawURLEncoding.EncodeToString(updates),
	})
}

// SealForGoogleCall is the wake-up an Android reads (#14): which account, that
// it is a call, and from whom. The app draws nothing from it; it reconnects
// and rings on the update it then receives.
func SealForGoogleCall(secretHex string, userId, fromId, callId int64) (string, error) {
	return fcm.Envelope(secretHex, map[string]any{
		"user_id": strconv.FormatInt(userId, 10),
		"loc_key": "PHONE_CALL_REQUEST",
		"custom": map[string]any{
			"from_id": strconv.FormatInt(fromId, 10),
			"call_id": strconv.FormatInt(callId, 10),
		},
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
