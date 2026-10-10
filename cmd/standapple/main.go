// Command standapple is Apple as the stand sees it (#188).
//
// The push scenarios register tokens they make up, and the stand's relay sent
// them to Apple, which answered that no such phone exists. The server then did
// what it must with a gone token - forgot it, every row of it - and the next
// step of the scenario had nobody to wake: three scenarios measured nothing.
//
// So the stand's relay is pointed here (APNS_SANDBOX_HOST, APNS_PRODUCTION_HOST
// in deploy/docker-compose.yml). A made-up token is answered as delivered; a
// real one - the simulator's, in tests/simulator/notification_extension_walk.py
// - is handed to Apple as it came and Apple's answer handed back. Nothing about
// this is in the relay or the server, which only learn where Apple is.
//
//	sandbox    STANDAPPLE_SANDBOX (default :8401)    -> api.sandbox.push.apple.com
//	production STANDAPPLE_PRODUCTION (default :8402) -> api.push.apple.com
package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	appleSandbox    = "https://api.sandbox.push.apple.com"
	appleProduction = "https://api.push.apple.com"
)

// What travels to Apple besides the body: the key's token and the
// notification's own headers.
var forwarded = []string{
	"authorization", "content-type",
	"apns-topic", "apns-push-type", "apns-priority", "apns-expiration", "apns-collapse-id", "apns-id",
}

func main() {
	sandbox := &front{apple: appleSandbox, client: &http.Client{Timeout: 30 * time.Second}}
	production := &front{apple: appleProduction, client: &http.Client{Timeout: 30 * time.Second}}
	go serve(envOr("STANDAPPLE_SANDBOX", ":8401"), "sandbox", sandbox)
	serve(envOr("STANDAPPLE_PRODUCTION", ":8402"), "production", production)
}

func serve(address, name string, f *front) {
	log.Printf("%s: listening on %s, real tokens go to %s", name, address, f.apple)
	log.Fatal(http.ListenAndServe(address, f))
}

type front struct {
	apple  string
	client *http.Client
}

func (f *front) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/3/device/")
	if token == r.URL.Path || token == "" {
		http.NotFound(w, r)
		return
	}
	shown := token
	if len(shown) > 8 {
		shown = shown[:8]
	}
	if madeUp(token) {
		w.Header().Set("apns-id", "standapple")
		w.WriteHeader(http.StatusOK)
		log.Printf("made-up token %s: accepted", shown)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := http.NewRequestWithContext(r.Context(), r.Method, f.apple+r.URL.Path, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, name := range forwarded {
		if value := r.Header.Get(name); value != "" {
			out.Header.Set(name, value)
		}
	}
	answer, err := f.client.Do(out)
	if err != nil {
		log.Printf("token %s: Apple did not answer: %v", shown, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer answer.Body.Close()
	said, _ := io.ReadAll(answer.Body)
	if id := answer.Header.Get("apns-id"); id != "" {
		w.Header().Set("apns-id", id)
	}
	if kind := answer.Header.Get("content-type"); kind != "" {
		w.Header().Set("content-type", kind)
	}
	w.WriteHeader(answer.StatusCode)
	_, _ = w.Write(said)
	log.Printf("token %s: Apple said %d %s", shown, answer.StatusCode, strings.TrimSpace(string(said)))
}

// madeUp says whether a token is one a scenario wrote rather than one Apple
// issued: the scenarios spell a word in hex and pad it with zeros, so it opens
// into printable letters and then nothing. Apple's are 32 random bytes, which
// are all printable about once in 10^14.
func madeUp(token string) bool {
	raw, err := hex.DecodeString(token)
	if err != nil || len(raw) == 0 || raw[0] == 0 {
		return false
	}
	padding := false
	for _, b := range raw {
		switch {
		case b == 0:
			padding = true
		case padding || b < 0x20 || b > 0x7e:
			return false
		}
	}
	return true
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
