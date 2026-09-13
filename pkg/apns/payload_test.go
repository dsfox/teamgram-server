package apns

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/sideshow/apns2"
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/fcm"
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

// sealed is what the server hands on: the envelope, already sealed with the
// phone's secret - here in the shape pushrelay.SealForApple seals it (that
// package imports this one, so the shape is repeated rather than imported).
func sealed(t *testing.T, secret string, peerType int32, peerId int64, msgId int32) string {
	t.Helper()
	key := map[int32]string{int32(mtproto.PEER_CHAT): "chat_id", int32(mtproto.PEER_CHANNEL): "channel_id"}[peerType]
	if key == "" {
		key = "from_id"
	}
	envelope, err := fcm.Envelope(secret, map[string]any{
		"aps":    map[string]any{"alert": map[string]any{"title": "ice9", "body": "New message"}, "sound": "default", "badge": 3},
		key:      strconv.FormatInt(peerId, 10),
		"msg_id": strconv.FormatInt(int64(msgId), 10),
	})
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func sent(t *testing.T, n Notify) map[string]any {
	t.Helper()
	raw, err := json.Marshal(buildPayload(n))
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

// A badge-only push (#173): the number on the icon and nothing else - no
// banner, no sound, nothing for the extension to open. What a phone gets
// when the person read the chat somewhere else.
func TestASilentPayloadIsTheBadgeAlone(t *testing.T) {
	got := sent(t, Notify{Title: "ice9", Body: "New message", Badge: 0, Silent: true})
	aps := got["aps"].(map[string]any)
	if aps["badge"] != float64(0) {
		t.Fatalf("the badge is %v, not 0", aps["badge"])
	}
	for _, key := range []string{"alert", "sound", "mutable-content"} {
		if _, there := aps[key]; there {
			t.Fatalf("a silent push carries %q", key)
		}
	}
	if _, there := got["p"]; there {
		t.Fatal("a silent push carries an envelope")
	}
}

// A call is a VoIP push: the envelope alone, no alert and no badge, because
// the app reports the call to CallKit itself the moment it opens the envelope;
// on the VoIP topic, with the voip push type, and never held for later - a
// call delivered ten minutes on is a phone ringing for nobody.
func TestACallIsAVoipPushCarryingOnlyTheEnvelope(t *testing.T) {
	got := sent(t, Notify{Call: true, Envelope: "QUJD", Title: "ice9", Body: "New message", Badge: 3})
	if got["p"] != "QUJD" {
		t.Fatalf("the envelope is not there: %v", got)
	}
	if _, there := got["aps"]; there {
		t.Fatalf("a call push carries aps, so it would be drawn as a banner: %v", got)
	}

	s := &Sender{topic: "app.twobytes.ios"}
	n := s.notification("tok", Notify{Call: true, Envelope: "QUJD"})
	if n.PushType != apns2.PushTypeVOIP || n.Topic != "app.twobytes.ios.voip" {
		t.Fatalf("sent as %q on %q", n.PushType, n.Topic)
	}
	if !n.Expiration.IsZero() {
		t.Fatalf("a call push may be held until %v", n.Expiration)
	}
	if n.Priority != apns2.PriorityHigh {
		t.Fatalf("priority %d", n.Priority)
	}

	message := s.notification("tok", Notify{Title: "ice9", Body: "New message"})
	if message.PushType != apns2.PushTypeAlert || message.Topic != "app.twobytes.ios" || message.Expiration.IsZero() {
		t.Fatalf("a message push changed: %q on %q until %v", message.PushType, message.Topic, message.Expiration)
	}
}
