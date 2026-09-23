package apns

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
)

const (
	ourToken     = "0011223344556677889900112233445566778899001122334455667788990011"
	foreignToken = "ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100"
)

// A token Apple issued for another app never works on our topic: a simulator
// build carries upstream's bundle id, and its token came back
// DeviceTokenNotForTopic on every message, each one a failed delivery on the
// server and a line in the alert (#203). Such a token is gone for us.
//
// But the same answer is what every token gets when it is the topic that is
// wrong, and forgetting on that would wipe every iPhone's registration at once.
// So a token is judged foreign only once this sender has delivered something:
// that proves the topic, and until then the answer stays an error that says so.
func TestAForeignTokenIsGoneOnceTheTopicHasDelivered(t *testing.T) {
	sender := senderAgainst(t)
	ctx := context.Background()

	err := sender.Send(ctx, foreignToken, Notify{Badge: -1})
	if err == nil || errors.Is(err, ErrTokenGone) {
		t.Fatalf("before anything was delivered the topic itself is in doubt, and the token was judged: %v", err)
	}

	if err := sender.Send(ctx, ourToken, Notify{Badge: -1}); err != nil {
		t.Fatalf("a token of our own app was refused: %v", err)
	}

	if err := sender.Send(ctx, foreignToken, Notify{Badge: -1}); !errors.Is(err, ErrTokenGone) {
		t.Fatalf("a token of another app, after the topic delivered, must be forgotten: %v", err)
	}
}

// senderAgainst is a Sender whose Apple answers ourToken and refuses anything
// else as issued for another app.
func senderAgainst(t *testing.T) *Sender {
	t.Helper()
	apple := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimPrefix(r.URL.Path, "/3/device/") == ourToken {
			w.Header().Set("apns-id", "delivered")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"reason":"`+apns2.ReasonDeviceTokenNotForTopic+`"}`)
	}))
	apple.EnableHTTP2 = true
	apple.StartTLS()
	t.Cleanup(apple.Close)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	client := apns2.NewTokenClient(&token.Token{AuthKey: key, KeyID: "KEY1234567", TeamID: "TEAM123456"})
	client.HTTPClient = apple.Client()
	client.Host = apple.URL
	return &Sender{topic: "app.twobytes.ios", production: client, sandbox: client}
}
