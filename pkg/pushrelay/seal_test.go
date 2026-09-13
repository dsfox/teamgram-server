package pushrelay

import (
	"encoding/hex"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/fcm"
)

func testSecret() string {
	key := make([]byte, 256)
	for i := range key {
		key[i] = byte(i)
	}
	return hex.EncodeToString(key)
}

func TestSealForAppleNamesTheChatAndNothingElse(t *testing.T) {
	secret := testSecret()
	p, err := SealForApple(secret, "ice9", "New message", 3, int32(mtproto.PEER_CHAT), 120099, 42)
	if err != nil {
		t.Fatal(err)
	}
	inside, err := fcm.OpenEnvelope(secret, p)
	if err != nil {
		t.Fatal(err)
	}
	if inside["chat_id"] != "120099" || inside["msg_id"] != "42" {
		t.Fatalf("the envelope does not name the chat and the message: %v", inside)
	}
	aps := inside["aps"].(map[string]any)
	if aps["alert"].(map[string]any)["body"] != "New message" {
		t.Fatalf("the words inside are not the constant ones: %v", aps)
	}
	if _, there := inside["from_id"]; there {
		t.Fatalf("a group message names a sender at the top level: %v", inside)
	}
}

func TestSealForApplePicksTheKeyByPeer(t *testing.T) {
	secret := testSecret()
	for kind, key := range map[int32]string{
		int32(mtproto.PEER_USER):    "from_id",
		int32(mtproto.PEER_CHAT):    "chat_id",
		int32(mtproto.PEER_CHANNEL): "channel_id",
	} {
		p, err := SealForApple(secret, "ice9", "New message", 0, kind, 7, 1)
		if err != nil {
			t.Fatal(err)
		}
		inside, _ := fcm.OpenEnvelope(secret, p)
		if inside[key] != "7" {
			t.Errorf("peer %d: %s is %v", kind, key, inside[key])
		}
	}
}

func TestSealForGoogleIsTheShapeTheAppReads(t *testing.T) {
	secret := testSecret()
	p, err := SealForGoogle(secret, 2, "42")
	if err != nil {
		t.Fatal(err)
	}
	inside, err := fcm.OpenEnvelope(secret, p)
	if err != nil {
		t.Fatal(err)
	}
	if inside["custom"].(map[string]any)["from_id"] != "42" || inside["loc_key"] != "" {
		t.Fatalf("not the shape the app reads: %v", inside)
	}
}

// A secret that cannot key anything is an error here, and the caller sends
// the alert alone: the push still says a message came.
func TestASecretThatIsNotOneIsRefused(t *testing.T) {
	if _, err := SealForApple("not hex", "ice9", "New message", 0, 0, 1, 1); err == nil {
		t.Error("an envelope was sealed with a key that is not one")
	}
	if _, err := SealForGoogle("not hex", 0, "1"); err == nil {
		t.Error("an envelope was sealed with a key that is not one")
	}
}

// The call envelopes: for an iPhone the TL update itself, base64url under
// "updates", because the app reports the call to CallKit from the push and
// creates the session from that update without asking the server; for an
// Android the wake-up shape its listener reads, with the account it is for.
// Neither carries a name or a word.
func TestSealForAppleCallCarriesTheUpdateAndNothingElse(t *testing.T) {
	secret := testSecret()
	p, err := SealForAppleCall(secret, []byte{0x74, 0xae, 0x8f, 0x1b, 1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	inside, err := fcm.OpenEnvelope(secret, p)
	if err != nil {
		t.Fatal(err)
	}
	if inside["updates"] != "dK6PGwECAw" {
		t.Fatalf("the update is not there as base64url without padding: %v", inside)
	}
	if len(inside) != 1 {
		t.Fatalf("the envelope carries more than the update: %v", inside)
	}
}

func TestSealForGoogleCallIsTheWakeUpTheAppReads(t *testing.T) {
	secret := testSecret()
	p, err := SealForGoogleCall(secret, 1002, 1001, 777)
	if err != nil {
		t.Fatal(err)
	}
	inside, err := fcm.OpenEnvelope(secret, p)
	if err != nil {
		t.Fatal(err)
	}
	custom, _ := inside["custom"].(map[string]any)
	if inside["user_id"] != "1002" || inside["loc_key"] != "PHONE_CALL_REQUEST" || custom["from_id"] != "1001" || custom["call_id"] != "777" {
		t.Fatalf("not the shape the app reads: %v", inside)
	}
}
