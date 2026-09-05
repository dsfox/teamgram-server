package pushrelay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendCarriesTheSixFieldsAndMaps410(t *testing.T) {
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(401)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(410)
	}))
	defer srv.Close()
	c := &Client{URL: srv.URL, Key: "k"}
	err := c.Send(context.Background(), Push{Platform: PlatformApple, Token: "t", Sandbox: true, Badge: 2, FromId: "5", P: "QQ"})
	if !errors.Is(err, ErrTokenGone) {
		t.Fatalf("410 did not become ErrTokenGone: %v", err)
	}
	for field := range sent {
		switch field {
		case "platform", "token", "sandbox", "badge", "from_id", "p":
		default:
			t.Errorf("the client sent a field the relay does not take: %s", field)
		}
	}
	if sent["token"] != "t" || sent["p"] != "QQ" || sent["sandbox"] != true {
		t.Fatalf("sent %v", sent)
	}
}

func TestSendTellsARefusalFromAFailure(t *testing.T) {
	for status, want := range map[int]error{401: ErrRefused, 403: ErrRefused, 429: ErrRefused, 502: nil} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
		err := (&Client{URL: srv.URL, Key: "k"}).Send(context.Background(), Push{Platform: PlatformGoogle, Token: "t"})
		srv.Close()
		if err == nil {
			t.Fatalf("%d was taken as sent", status)
		}
		if want != nil && !errors.Is(err, want) {
			t.Errorf("%d: %v is not %v", status, err, want)
		}
	}
}

func TestRegisterReadsTheAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/servers" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"server_id":"abc","key":"secret"}`))
	}))
	defer srv.Close()
	id, key, err := Register(context.Background(), srv.URL, "1.2.3.4:10443")
	if err != nil || id != "abc" || key != "secret" {
		t.Fatalf("got %q %q %v", id, key, err)
	}
}
