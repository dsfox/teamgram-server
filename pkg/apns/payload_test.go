package apns

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/fcm"
	"github.com/teamgram/teamgram-server/pkg/pushrelay"
)

// A key of the size a device registers, not all zeroes: a wrong offset in the
// derivation would be invisible against zeroes.
func testKey() string {
	key := make([]byte, 256)
	for i := range key {
		key[i] = byte(i*7 + 3)
	}
	return hex.EncodeToString(key)
}

// sealed is what the server hands the relay: the envelope, already sealed
// with the phone's secret by pushrelay.SealForApple.
func sealed(t *testing.T, secret string, peerType int32, peerId int64, msgId int32) string {
	t.Helper()
	envelope, err := pushrelay.SealForApple(secret, "ice9", "New message", 3, peerType, peerId, msgId)
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func sent(t *testing.T, n Notify) map[string]any {
	t.Helper()
	raw, err := buildPayload(n).MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// The payload as the extension reads it: the alert stays as the fallback,
// mutable-content is what makes the extension run, and p is the envelope in
// the shape upstream's extension was written to read - the alert again, the
// chat's id under the key for its kind, the message's id - and no text,
// because the text never leaves the device (#42).
func TestPayloadWithSecretWakesTheExtension(t *testing.T) {
	secret := testKey()
	got := sent(t, Notify{Title: "ice9", Body: "New message", Badge: 3, FromId: "136908607",
		Envelope: sealed(t, secret, int32(mtproto.PEER_USER), 136908607, 790)})

	aps := got["aps"].(map[string]any)
	if aps["mutable-content"] != float64(1) {
		t.Fatalf("mutable-content is %v, the extension would never run", aps["mutable-content"])
	}
	if aps["alert"].(map[string]any)["body"] != "New message" {
		t.Fatalf("the fallback alert changed: %v", aps["alert"])
	}
	envelope, ok := got["p"].(string)
	if !ok || envelope == "" {
		t.Fatal("no envelope: the extension would have nothing to open")
	}
	inside, err := fcm.OpenEnvelope(secret, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if inside["from_id"] != "136908607" || inside["msg_id"] != "790" {
		t.Fatalf("the envelope does not name the chat and the message: %v", inside)
	}
	if inside["aps"].(map[string]any)["alert"].(map[string]any)["body"] != "New message" {
		t.Fatalf("the envelope's own alert is not the fallback: %v", inside)
	}
	for _, key := range []string{"text", "message", "sender", "title", "body"} {
		if _, there := inside[key]; there {
			t.Fatalf("the envelope carries %q, which must never leave the device", key)
		}
	}
}

// A group's id goes under chat_id, a channel's under channel_id: the key is
// what tells the extension which kind of chat to open.
func TestTheEnvelopeNamesTheChatByItsKind(t *testing.T) {
	secret := testKey()
	for kind, key := range map[int32]string{
		int32(mtproto.PEER_CHAT):    "chat_id",
		int32(mtproto.PEER_CHANNEL): "channel_id",
	} {
		got := sent(t, Notify{Title: "ice9", Body: "New message", Envelope: sealed(t, secret, kind, 120062, 5)})
		inside, err := fcm.OpenEnvelope(secret, got["p"].(string))
		if err != nil {
			t.Fatal(err)
		}
		if inside[key] != "120062" {
			t.Fatalf("a chat of kind %d is not under %s: %v", kind, key, inside)
		}
		if _, there := inside["from_id"]; there {
			t.Fatalf("a chat of kind %d also says from_id, and the extension would open a person: %v", kind, inside)
		}
	}
}

// Without a key there is nothing the extension could open, so the payload is
// exactly what it has always been.
func TestPayloadWithoutSecretIsTodays(t *testing.T) {
	got := sent(t, Notify{Title: "ice9", Body: "New message", Badge: 1, FromId: "1"})
	if _, there := got["p"]; there {
		t.Fatal("an envelope without a key to open it")
	}
	if _, there := got["aps"].(map[string]any)["mutable-content"]; there {
		t.Fatal("mutable-content with nothing for the extension to do")
	}
	if got["from_id"] != "1" {
		t.Fatalf("from_id went missing: %v", got)
	}
}
