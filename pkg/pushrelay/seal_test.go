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
