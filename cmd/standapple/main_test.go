package main

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The tokens the scenarios register, spelt the way they spell them.
func scenarioToken(word string) string {
	spelt := hex.EncodeToString([]byte(word))
	spelt += strings.Repeat("0", 64)
	return spelt[:64]
}

func TestMadeUpTokensAreTold(t *testing.T) {
	for _, word := range []string{"badge1791631761123", "bg1791631761", "1791631761123", "again17916317611234567890abcdef"} {
		if !madeUp(scenarioToken(word)) {
			t.Errorf("%q, as a scenario spells it, was taken for a real token", word)
		}
	}
	for i := 0; i < 1000; i++ {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			t.Fatal(err)
		}
		if madeUp(hex.EncodeToString(raw)) {
			t.Fatalf("a random token %x was taken for a made-up one", raw)
		}
	}
	for _, token := range []string{"", "zz", "00" + scenarioToken("late")[2:], "41004200"} {
		if madeUp(token) {
			t.Errorf("%q was taken for a made-up token", token)
		}
	}
}

func TestARealTokenGoesToAppleAndBack(t *testing.T) {
	var got *http.Request
	var body string
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		read, _ := io.ReadAll(r.Body)
		body = string(read)
		w.Header().Set("apns-id", "from-apple")
		w.WriteHeader(http.StatusGone)
		_, _ = io.WriteString(w, `{"reason":"Unregistered"}`)
	}))
	defer apple.Close()
	stand := httptest.NewServer(&front{apple: apple.URL, client: apple.Client()})
	defer stand.Close()

	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	issued := hex.EncodeToString(raw)
	request, _ := http.NewRequest(http.MethodPost, stand.URL+"/3/device/"+issued, strings.NewReader(`{"aps":{}}`))
	request.Header.Set("authorization", "bearer key")
	request.Header.Set("apns-topic", "app.twobytes.ios")
	answer, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	said, _ := io.ReadAll(answer.Body)
	if got == nil || got.URL.Path != "/3/device/"+issued || body != `{"aps":{}}` ||
		got.Header.Get("authorization") != "bearer key" || got.Header.Get("apns-topic") != "app.twobytes.ios" {
		t.Fatalf("Apple was not handed the notification as it came: %v %q", got, body)
	}
	if answer.StatusCode != http.StatusGone || answer.Header.Get("apns-id") != "from-apple" || !strings.Contains(string(said), "Unregistered") {
		t.Fatalf("Apple's answer did not come back as it was: %d %q %q", answer.StatusCode, answer.Header.Get("apns-id"), said)
	}
}

func TestAMadeUpTokenNeverReachesApple(t *testing.T) {
	reached := false
	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer apple.Close()
	stand := httptest.NewServer(&front{apple: apple.URL, client: apple.Client()})
	defer stand.Close()

	answer, err := http.Post(stand.URL+"/3/device/"+scenarioToken("badge1791631761"), "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if answer.StatusCode != http.StatusOK || answer.Header.Get("apns-id") == "" || reached {
		t.Fatalf("a made-up token: status %d, apns-id %q, reached Apple %v", answer.StatusCode, answer.Header.Get("apns-id"), reached)
	}
}
