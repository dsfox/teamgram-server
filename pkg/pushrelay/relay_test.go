package pushrelay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/teamgram/teamgram-server/pkg/apns"
)

type recording struct {
	got []Push
	err error
}

func (r *recording) Send(_ context.Context, p Push) error {
	r.got = append(r.got, p)
	return r.err
}

func decodeJSON(t *testing.T, r io.Reader, into any) {
	t.Helper()
	if err := json.NewDecoder(r).Decode(into); err != nil {
		t.Fatal(err)
	}
}

func relayForTest(t *testing.T, apple, google Forwarder) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(NewHandler(NewMemoryStore(), apple, google, NewQuotas(), time.Now))
	t.Cleanup(srv.Close)
	res, err := http.Post(srv.URL+"/v1/servers", "application/json", strings.NewReader(`{"address":"1.2.3.4:10443"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("registration answered %d", res.StatusCode)
	}
	var reg struct {
		ServerId string `json:"server_id"`
		Key      string `json:"key"`
	}
	decodeJSON(t, res.Body, &reg)
	if reg.ServerId == "" || len(reg.Key) != 64 {
		t.Fatalf("registration gave %+v", reg)
	}
	return srv, reg.Key
}

func push(t *testing.T, url, key, body string) int {
	t.Helper()
	req, _ := http.NewRequest("POST", url+"/v1/push", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

func TestAPushGoesToTheRightForwarder(t *testing.T) {
	apple, google := &recording{}, &recording{}
	srv, key := relayForTest(t, apple, google)
	if code := push(t, srv.URL, key, `{"platform":"apns","token":"tok","sandbox":true,"badge":1,"p":"QUJD"}`); code != 200 {
		t.Fatalf("apple push answered %d", code)
	}
	if code := push(t, srv.URL, key, `{"platform":"fcm","token":"tok2","badge":0,"from_id":"5"}`); code != 200 {
		t.Fatalf("google push answered %d", code)
	}
	if len(apple.got) != 1 || len(google.got) != 1 || apple.got[0].P != "QUJD" || !apple.got[0].Sandbox || google.got[0].FromId != "5" {
		t.Fatalf("apple %+v, google %+v", apple.got, google.got)
	}
}

func TestWordsAreRefused(t *testing.T) {
	apple := &recording{}
	srv, key := relayForTest(t, apple, &recording{})
	for _, body := range []string{
		`{"platform":"apns","token":"tok","body":"buy now"}`,
		`{"platform":"apns","token":"tok","title":"x"}`,
		`{"platform":"apns","token":"tok","message":"x"}`,
	} {
		if code := push(t, srv.URL, key, body); code != 400 {
			t.Errorf("%s got %d, not 400", body, code)
		}
	}
	if len(apple.got) != 0 {
		t.Fatal("a push with words was forwarded")
	}
}

func TestADeadTokenIs410(t *testing.T) {
	srv, key := relayForTest(t, &recording{err: apns.ErrTokenGone}, &recording{})
	if code := push(t, srv.URL, key, `{"platform":"apns","token":"tok"}`); code != 410 {
		t.Fatalf("got %d", code)
	}
}

func TestAnUnknownKeyIs401(t *testing.T) {
	srv, _ := relayForTest(t, &recording{}, &recording{})
	if code := push(t, srv.URL, "nope", `{"platform":"apns","token":"tok"}`); code != 401 {
		t.Fatalf("got %d", code)
	}
}

func TestAPlatformWithoutAForwarderIs503(t *testing.T) {
	srv, key := relayForTest(t, &recording{}, nil)
	if code := push(t, srv.URL, key, `{"platform":"fcm","token":"tok"}`); code != 503 {
		t.Fatalf("got %d", code)
	}
}

func TestATokenOverQuotaIs429(t *testing.T) {
	apple := &recording{}
	srv, key := relayForTest(t, apple, &recording{})
	for i := 0; i < 30; i++ {
		if code := push(t, srv.URL, key, `{"platform":"apns","token":"same"}`); code != 200 {
			t.Fatalf("push %d got %d", i, code)
		}
	}
	if code := push(t, srv.URL, key, `{"platform":"apns","token":"same"}`); code != 429 {
		t.Fatalf("the 31st push to one token in a minute got %d, not 429", code)
	}
	if code := push(t, srv.URL, key, `{"platform":"apns","token":"another"}`); code != 200 {
		t.Fatalf("another token was refused with %d", code)
	}
}

func TestRegistrationIsTenADayPerIP(t *testing.T) {
	srv := httptest.NewServer(NewHandler(NewMemoryStore(), &recording{}, &recording{}, NewQuotas(), time.Now))
	defer srv.Close()
	for i := 0; i < 10; i++ {
		res, _ := http.Post(srv.URL+"/v1/servers", "application/json", strings.NewReader(`{"address":"x"}`))
		if res.StatusCode != 200 {
			t.Fatalf("registration %d got %d", i, res.StatusCode)
		}
	}
	res, _ := http.Post(srv.URL+"/v1/servers", "application/json", strings.NewReader(`{"address":"x"}`))
	if res.StatusCode != 429 {
		t.Fatalf("the 11th registration got %d", res.StatusCode)
	}
}

func TestABlockedServerIs403(t *testing.T) {
	store := NewMemoryStore()
	srv := httptest.NewServer(NewHandler(store, &recording{}, &recording{}, NewQuotas(), time.Now))
	defer srv.Close()
	res, _ := http.Post(srv.URL+"/v1/servers", "application/json", strings.NewReader(`{"address":"x"}`))
	var reg struct {
		ServerId string `json:"server_id"`
		Key      string `json:"key"`
	}
	decodeJSON(t, res.Body, &reg)
	if err := store.Block(reg.ServerId); err != nil {
		t.Fatal(err)
	}
	if code := push(t, srv.URL, reg.Key, `{"platform":"apns","token":"tok"}`); code != 403 {
		t.Fatalf("got %d", code)
	}
}
