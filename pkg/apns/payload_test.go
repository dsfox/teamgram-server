package apns

import (
	"encoding/hex"
	"encoding/json"
	"testing"

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
// mutable-content is what makes the extension run, and p is the envelope the
// FCM path already builds - from_id and no text, because the text never
// leaves the device (#42).
func TestPayloadWithSecretWakesTheExtension(t *testing.T) {
	secret := testKey()
	got := sent(t, Notify{Title: "ice9", Body: "New message", Badge: 3, FromId: "136908607", Secret: secret})

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
	if inside["custom"].(map[string]any)["from_id"] != "136908607" {
		t.Fatalf("the envelope does not say who it is from: %v", inside)
	}
	for _, key := range []string{"text", "message", "sender", "title", "body"} {
		if _, there := inside[key]; there {
			t.Fatalf("the envelope carries %q, which must never leave the device", key)
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

// A secret that cannot key anything does not stop the push: the alert alone
// still says a message came, which is what an old build shows anyway.
func TestABrokenSecretStillSendsTheAlert(t *testing.T) {
	got := sent(t, Notify{Title: "ice9", Body: "New message", Badge: 1, FromId: "1", Secret: "not hex"})
	if _, there := got["p"]; there {
		t.Fatal("an envelope was built from a key that is not one")
	}
	if got["aps"].(map[string]any)["alert"].(map[string]any)["body"] != "New message" {
		t.Fatal("the alert went missing with the envelope")
	}
}
