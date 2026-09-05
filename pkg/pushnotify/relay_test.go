package pushnotify

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/devices"
	"github.com/teamgram/teamgram-server/pkg/fcm"
	"github.com/teamgram/teamgram-server/pkg/pushrelay"
)

type fakeRegistry struct {
	mu        sync.Mutex
	forgotten []string
}

func (f *fakeRegistry) ListByUser(context.Context, int64) ([]devices.DeviceDO, error) {
	return nil, nil
}

func (f *fakeRegistry) Forget(_ context.Context, tokenType int32, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forgotten = append(f.forgotten, token)
	return nil
}

func secretForTest() string {
	key := make([]byte, 256)
	for i := range key {
		key[i] = byte(i*3 + 1)
	}
	return hex.EncodeToString(key)
}

// A notifier talking to a relay that records what it was sent and answers
// as told.
func notifierForTest(t *testing.T, status int) (*Notifier, *fakeRegistry, *[]map[string]any) {
	t.Helper()
	var got []map[string]any
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		got = append(got, body)
		mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	registry := &fakeRegistry{}
	n := &Notifier{registry: registry, title: "ice9", body: "New message"}
	n.relay.Store(&pushrelay.Client{URL: srv.URL, Key: "k"})
	return n, registry, &got
}

func TestAnIPhoneIsHandedASealedEnvelopeAndNothingElse(t *testing.T) {
	n, _, got := notifierForTest(t, 200)
	secret := secretForTest()
	n.send(context.Background(), devices.DeviceDO{TokenType: devices.TokenTypeAPNs, Token: "tok", AppSandbox: true, Secret: secret, UserId: 7, AuthKeyId: 9},
		3, "", int32(mtproto.PEER_CHAT), 120099, 42)
	if len(*got) != 1 {
		t.Fatalf("the relay saw %d pushes", len(*got))
	}
	sent := (*got)[0]
	for field := range sent {
		switch field {
		case "platform", "token", "sandbox", "badge", "from_id", "p":
		default:
			t.Errorf("a field the relay does not take, and that could carry words: %s", field)
		}
	}
	if sent["platform"] != "apns" || sent["token"] != "tok" || sent["sandbox"] != true || sent["badge"] != float64(3) {
		t.Fatalf("sent %v", sent)
	}
	inside, err := fcm.OpenEnvelope(secret, sent["p"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if inside["chat_id"] != "120099" || inside["msg_id"] != "42" {
		t.Fatalf("the envelope does not name the chat: %v", inside)
	}
}

func TestAnAndroidIsHandedItsOwnShape(t *testing.T) {
	n, _, got := notifierForTest(t, 200)
	secret := secretForTest()
	n.send(context.Background(), devices.DeviceDO{TokenType: devices.TokenTypeFCM, Token: "tok2", Secret: secret, UserId: 7, AuthKeyId: 9},
		1, "5", int32(mtproto.PEER_USER), 5, 1)
	sent := (*got)[0]
	if sent["platform"] != "fcm" || sent["from_id"] != "5" {
		t.Fatalf("sent %v", sent)
	}
	inside, err := fcm.OpenEnvelope(secret, sent["p"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if inside["custom"].(map[string]any)["from_id"] != "5" {
		t.Fatalf("not the shape the app reads: %v", inside)
	}
}

func TestADeadTokenIsForgottenAndAFailureIsNot(t *testing.T) {
	n, registry, _ := notifierForTest(t, 410)
	n.send(context.Background(), devices.DeviceDO{TokenType: devices.TokenTypeAPNs, Token: "dead", UserId: 7, AuthKeyId: 9}, 0, "", 0, 0, 0)
	if len(registry.forgotten) != 1 || registry.forgotten[0] != "dead" {
		t.Fatalf("410 did not forget the token: %v", registry.forgotten)
	}
	n, registry, _ = notifierForTest(t, 502)
	n.send(context.Background(), devices.DeviceDO{TokenType: devices.TokenTypeAPNs, Token: "alive", UserId: 7, AuthKeyId: 9}, 0, "", 0, 0, 0)
	if len(registry.forgotten) != 0 {
		t.Fatalf("a relay failure forgot a token: %v", registry.forgotten)
	}
}

func TestWithoutAKeyNothingIsSentAndNothingIsEnabled(t *testing.T) {
	n := &Notifier{registry: &fakeRegistry{}, title: "ice9", body: "New message"}
	if n.Enabled() {
		t.Fatal("enabled with no relay key")
	}
}
