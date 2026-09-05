package fcm

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// A key of the size a device registers, filled with something that is not all
// zeroes: a wrong offset in the derivation would be invisible against zeroes.
func testKey() string {
	key := make([]byte, 256)
	for i := range key {
		key[i] = byte(i*7 + 3)
	}
	return hex.EncodeToString(key)
}

// openLikeTheClient is OpenEnvelope with the test failing on its behalf.
func openLikeTheClient(t *testing.T, secretHex, envelope string) map[string]any {
	t.Helper()
	payload, err := OpenEnvelope(secretHex, envelope)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestTheClientCanOpenTheEnvelope(t *testing.T) {
	secret := testKey()
	envelope, err := Envelope(secret, map[string]any{
		"badge":   3,
		"custom":  map[string]any{"from_id": "42"},
		"loc_key": "",
	})
	if err != nil {
		t.Fatalf("cannot build the envelope: %v", err)
	}

	payload := openLikeTheClient(t, secret, envelope)
	if payload["badge"].(float64) != 3 {
		t.Errorf("the badge came out as %v", payload["badge"])
	}
	if from := payload["custom"].(map[string]any)["from_id"]; from != "42" {
		t.Errorf("from_id came out as %v", from)
	}
}

func TestEachEnvelopeDiffers(t *testing.T) {
	secret := testKey()
	same := map[string]any{"badge": 1, "loc_key": ""}

	first, err := Envelope(secret, same)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Envelope(secret, same)
	if err != nil {
		t.Fatal(err)
	}
	// The padding is random, so two identical notifications must not be
	// identical on the wire - otherwise anyone watching learns when the same
	// thing happened twice.
	if first == second {
		t.Error("the same notification encrypted to the same bytes twice")
	}
}

func TestAShortKeyIsRefusedRatherThanSentGarbled(t *testing.T) {
	if _, err := Envelope(hex.EncodeToString(make([]byte, 32)), map[string]any{}); err == nil {
		t.Error("a key too short to derive from was accepted")
	}
	if _, err := Envelope("not hex at all", map[string]any{}); err == nil {
		t.Error("a key that is not hex was accepted")
	}
}

// sealedForTest is the envelope the server seals with the device's key before
// handing the notification on; here it is sealed in place.
func sealedForTest(t *testing.T) string {
	t.Helper()
	envelope, err := Envelope(testKey(), map[string]any{"badge": 1, "custom": map[string]any{"from_id": "42"}, "loc_key": ""})
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func TestWithoutAKeyTheBannerIsSentInstead(t *testing.T) {
	s := &Sender{projectId: "p"}

	withKey, err := s.compose("token", Notify{Badge: 1, Envelope: sealedForTest(t)})
	if err != nil {
		t.Fatal(err)
	}
	message := withKey["message"].(map[string]any)
	if _, drawn := message["notification"]; drawn {
		t.Error("a message with a key still asks Firebase to draw it, so the app will not run")
	}
	if _, carried := message["data"].(map[string]any)["p"]; !carried {
		t.Error("the envelope is missing, so the app has nothing to open")
	}

	without, err := s.compose("token", Notify{Title: "ice9", Body: "New message"})
	if err != nil {
		t.Fatal(err)
	}
	fallback := without["message"].(map[string]any)
	if _, drawn := fallback["notification"]; !drawn {
		t.Error("a device with no key was sent nothing it can show")
	}
}

func TestTheTextIsNowhereInTheMessage(t *testing.T) {
	s := &Sender{projectId: "p"}
	message, err := s.compose("token", Notify{
		Badge: 1, FromId: "42", Envelope: sealedForTest(t),
		Title: "ice9", Body: "New message",
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	// Whatever else changes here, nothing a person wrote may travel to Google -
	// and neither may the name of whoever wrote it.
	for _, forbidden := range []string{"New message", "ice9"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("%q is in the message sent to Google", forbidden)
		}
	}
}
