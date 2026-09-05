// Package pushrelay is the wire between a server and the relay that holds the
// Apple and Google keys (#167). A notification crosses it as six fields and no
// words: the relay's own title and body are the only text a phone ever sees,
// so a caller cannot put a name, a message, an advertisement or a link into a
// push, whatever it sends.
package pushrelay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	PlatformApple  = "apns"
	PlatformGoogle = "fcm"
)

// Push is one notification as the relay accepts it. Exactly these fields:
// anything else is refused, because a field is a place words could travel.
type Push struct {
	Platform string `json:"platform"`
	Token    string `json:"token"`
	Sandbox  bool   `json:"sandbox"`
	Badge    int    `json:"badge"`
	// Who wrote, as a number that means something only on the server that
	// sent it - for a phone with no secret, whose tap has to open a chat.
	FromId string `json:"from_id,omitempty"`
	// The envelope the phone's own secret opens, sealed by the server. The
	// relay forwards it and cannot read it.
	P string `json:"p,omitempty"`
}

var ErrBadPush = errors.New("pushrelay: not a push")

// Decode reads a push strictly: an unknown field is an error, not noise.
func Decode(r io.Reader) (Push, error) {
	dec := json.NewDecoder(io.LimitReader(r, 8192))
	dec.DisallowUnknownFields()
	var p Push
	if err := dec.Decode(&p); err != nil {
		return Push{}, fmt.Errorf("%w: %v", ErrBadPush, err)
	}
	if p.Platform != PlatformApple && p.Platform != PlatformGoogle {
		return Push{}, fmt.Errorf("%w: platform %q", ErrBadPush, p.Platform)
	}
	if p.Token == "" || len(p.Token) > 512 || len(p.P) > 4096 || len(p.FromId) > 32 {
		return Push{}, fmt.Errorf("%w: a field is missing or too long", ErrBadPush)
	}
	return p, nil
}
