// Package calls builds what one phone needs in order to reach the other.
//
// The server is the matchmaker, not the wire. It hands out STUN so the two
// devices can discover their own public addresses and punch a hole to each
// other, and it offers a relay only as the fallback for the one case a direct
// path cannot win - a symmetric NAT on both sides. Media never passes through
// us on the direct path, and on the relayed one it passes as ciphertext the
// relay cannot read: the call key is agreed between the two devices and never
// leaves them.
//
// When the caller asked to hide their IP, no direct candidate is offered at
// all. Offering STUN "just in case" would hand the peer the very address the
// setting exists to hide, so with no relay configured that call is refused
// rather than quietly leaked. (#14)
package calls

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"strconv"
	"time"

	"github.com/teamgram/proto/mtproto"
)

// Server is one STUN or TURN endpoint, as handed to both phones.
type Server struct {
	Id     int64
	Host   string // IPv4 address
	HostV6 string // IPv6 address; empty when the endpoint has none
	Port   int32
	Stun   bool
	Turn   bool
	Tcp    bool
}

// Credentials open a relay, and only for a while.
type Credentials struct {
	Username string
	Password string
}

// ErrNoRelay says the call asked for the relay-only path and there is no relay
// to give it. The honest answer is to fail, not to fall back to a direct
// candidate the caller asked us not to reveal.
var ErrNoRelay = errors.New("calls: peer-to-peer refused and no relay is configured")

// TurnCredentials mints the short-lived pair a TURN server accepts. This is the
// standard TURN REST scheme: the username is the moment the pair stops working,
// and the password is its HMAC, so the relay can check it without being told
// anything in advance and a leaked pair dies on its own.
//
// SHA-1 here is the scheme's, not a choice of ours - TURN's long-term
// credential mechanism specifies HMAC-SHA1.
func TurnCredentials(secret string, ttl time.Duration, now time.Time) Credentials {
	username := strconv.FormatInt(now.Add(ttl).Unix(), 10)
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(username))
	return Credentials{
		Username: username,
		Password: base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	}
}

// Connections lists what the phone should try, in the order we want it tried:
// every STUN endpoint first, because a direct path is the point, and the relays
// after it as the fallback. With peer-to-peer refused the STUN half is dropped
// entirely and only relays remain.
func Connections(servers []Server, p2pAllowed bool, secret string, ttl time.Duration, now time.Time) ([]*mtproto.PhoneConnection, error) {
	out := make([]*mtproto.PhoneConnection, 0, len(servers))
	servers = withAnAddress(servers)

	if p2pAllowed {
		for _, s := range servers {
			if s.Stun {
				// STUN asks nothing of the phone; handing it credentials would
				// only put a reusable pair on the wire for no reason.
				out = append(out, descriptor(s, true, false, Credentials{}))
			}
		}
	}

	// No secret, no relay: credentials minted from nothing are credentials
	// the relay refuses, and an engine waits on a relay it was handed.
	if secret != "" {
		creds := TurnCredentials(secret, ttl, now)
		for _, s := range servers {
			if s.Turn {
				out = append(out, descriptor(s, false, true, creds))
			}
		}
	}

	if len(out) == 0 {
		return nil, ErrNoRelay
	}
	return out, nil
}

// withAnAddress drops endpoints that have none: a server installed without
// CALLS_HOST in its environment reads an empty host out of its config, and an
// entry with an empty ip handed to the phones is worse than no entry.
func withAnAddress(servers []Server) []Server {
	out := servers[:0:0]
	for _, s := range servers {
		if s.Host != "" || s.HostV6 != "" {
			out = append(out, s)
		}
	}
	return out
}

func descriptor(s Server, stun, turn bool, creds Credentials) *mtproto.PhoneConnection {
	return mtproto.MakeTLPhoneConnectionWebrtc(&mtproto.PhoneConnection{
		Id:       s.Id,
		Ip:       s.Host,
		Ipv6:     s.HostV6,
		Port:     s.Port,
		Tcp:      s.Tcp,
		Stun:     stun,
		Turn:     turn,
		Username: creds.Username,
		Password: creds.Password,
	}).To_PhoneConnection()
}
