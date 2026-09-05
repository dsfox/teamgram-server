package pushrelay

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/teamgram/teamgram-server/pkg/apns"
	"github.com/teamgram/teamgram-server/pkg/fcm"
)

// Forwarder hands one push to Apple or to Google.
type Forwarder interface {
	Send(ctx context.Context, p Push) error
}

// The quotas: what one server, and one token, may do.
const (
	perServerMinute = 600
	perServerDay    = 100000
	perTokenMinute  = 30
)

type handler struct {
	store  Store
	apple  Forwarder
	google Forwarder
	quotas *Quotas
	now    func() time.Time
}

// NewHandler is the relay's HTTP face: registration and one push at a time.
func NewHandler(store Store, apple, google Forwarder, q *Quotas, now func() time.Time) http.Handler {
	h := &handler{store: store, apple: apple, google: google, quotas: q, now: now}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/servers", h.register)
	mux.HandleFunc("POST /v1/push", h.push)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok\n") })
	return mux
}

func (h *handler) register(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1024))
	dec.DisallowUnknownFields()
	var body struct {
		Address string `json:"address"`
	}
	if err := dec.Decode(&body); err != nil || body.Address == "" || len(body.Address) > 128 {
		http.Error(w, "a registration is {\"address\": \"host:port\"}", http.StatusBadRequest)
		return
	}
	ip := callerIP(r)
	id, key, err := h.store.Register(ip, body.Address, h.now())
	if errors.Is(err, ErrTooMany) {
		http.Error(w, "too many registrations from this address today", http.StatusTooManyRequests)
		return
	}
	if err != nil {
		log.Printf("registration failed: %v", err)
		http.Error(w, "cannot register now", http.StatusInternalServerError)
		return
	}
	log.Printf("server registered: %s for %s from %s", id, body.Address, ip)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"server_id": id, "key": key})
}

func (h *handler) push(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if key == "" || key == r.Header.Get("Authorization") {
		http.Error(w, "a push carries Authorization: Bearer <key>", http.StatusUnauthorized)
		return
	}
	server, known, err := h.store.Lookup(KeyHash(key))
	if err != nil {
		http.Error(w, "cannot look the key up now", http.StatusInternalServerError)
		return
	}
	if !known {
		http.Error(w, "unknown key", http.StatusUnauthorized)
		return
	}
	if server.Blocked {
		http.Error(w, "this server is blocked", http.StatusForbidden)
		return
	}
	p, err := Decode(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := h.now()
	tokenKey := "t:" + server.Id + ":" + KeyHash(p.Token)[:16]
	if !h.quotas.Allow("s:"+server.Id, perServerMinute, now) ||
		!h.quotas.AllowDay("s:"+server.Id, perServerDay, now) ||
		!h.quotas.Allow(tokenKey, perTokenMinute, now) {
		h.say(server.Id, p, http.StatusTooManyRequests)
		http.Error(w, "over quota", http.StatusTooManyRequests)
		return
	}
	forwarder := h.apple
	if p.Platform == PlatformGoogle {
		forwarder = h.google
	}
	if forwarder == nil {
		h.say(server.Id, p, http.StatusServiceUnavailable)
		http.Error(w, "this platform is not configured here", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	err = forwarder.Send(ctx, p)
	status := http.StatusOK
	switch {
	case err == nil:
	case errors.Is(err, apns.ErrTokenGone), errors.Is(err, fcm.ErrTokenGone):
		status = http.StatusGone
	default:
		status = http.StatusBadGateway
	}
	h.say(server.Id, p, status)
	if status != http.StatusOK {
		http.Error(w, http.StatusText(status), status)
		return
	}
	io.WriteString(w, "sent\n")
}

// say is the one log line per push: who, which platform, a shadow of the
// token, and the answer. Never anything from the body.
func (h *handler) say(serverId string, p Push, status int) {
	log.Printf("push %s %s %s %d", serverId, p.Platform, KeyHash(p.Token)[:8], status)
}

// callerIP is the first hop nginx reports, or the peer itself.
func callerIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
