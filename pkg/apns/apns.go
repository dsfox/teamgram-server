// Package apns delivers notifications to Apple devices.
//
// Important property: a notification carries neither the message text nor the
// sender name - only "New message", and, since #42, the same encrypted
// envelope the Android push carries: the device's registered key opens it and
// finds who wrote, nothing more. Showing the text would require handing it to
// Apple, even if encrypted, so the phone fetches and opens the message itself,
// in the notification extension the app ships, and draws the words on its own
// screen. Apple only ever sees that an event happened, and from whom the app
// should fetch. See docs/03-push-notifications.md.
package apns

import (
	"context"
	"errors"
	"fmt"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/fcm"
	"golang.org/x/net/http2"
	"net/http"
	"os"
	"strconv"
	"time"
)

// ErrTokenGone means Apple reported the token no longer exists: the app was
// removed from the device. Such a token must be forgotten; retrying is useless.
var ErrTokenGone = errors.New("apns: device token is no longer valid")

// ErrWrongEnvironment means the key was issued for a different environment.
// Apple keys can be bound to one of them: production or sandbox. It gets its own
// error because the response code alone does not explain the cause, and the only
// cure is a new key.
var ErrWrongEnvironment = errors.New("apns: key does not serve this environment")

// reasonBadEnvironmentKey is Apple's answer when the key belongs to another
// environment. The library has no constant for it: environment-bound keys
// appeared later.
const reasonBadEnvironmentKey = "BadEnvironmentKeyInToken"

// Config holds what the Apple developer portal issues. The key never expires and
// one key covers every app of the team.
type Config struct {
	KeyPath string // .p8 file with the private key
	KeyId   string // key identifier, 10 characters
	TeamId  string // team identifier, 10 characters
	Topic   string // app identifier, same as the bundle id
}

// ConfigFromEnv reads the settings from the environment. The second value tells
// whether they are set at all: without a key the server runs exactly as before,
// it just silently sends no notifications.
func ConfigFromEnv() (Config, bool) {
	c := Config{
		KeyPath: os.Getenv("APNS_KEY_PATH"),
		KeyId:   os.Getenv("APNS_KEY_ID"),
		TeamId:  os.Getenv("APNS_TEAM_ID"),
		Topic:   os.Getenv("APNS_TOPIC"),
	}

	return c, c.KeyPath != "" && c.KeyId != "" && c.TeamId != "" && c.Topic != ""
}

// Sender delivers notifications. It keeps two connections: production and
// sandbox. They must not be mixed — a token issued to a device in one
// environment is invalid in the other, so the caller picks the environment from
// the device record.
type Sender struct {
	topic      string
	production *apns2.Client
	sandbox    *apns2.Client
}

func New(c Config) (*Sender, error) {
	authKey, err := token.AuthKeyFromFile(c.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("apns: cannot read key %s: %w", c.KeyPath, err)
	}

	tok := &token.Token{
		AuthKey: authKey,
		KeyID:   c.KeyId,
		TeamID:  c.TeamId,
	}

	return &Sender{
		topic:      c.Topic,
		production: newClient(tok).Production(),
		sandbox:    newClient(tok).Development(),
	}, nil
}

// newClient repeats the apns2 defaults and adds a limit on how long an idle
// connection lives. Without it the goroutine count grows: connections to Apple
// pile up and are never closed (sideshow/apns2#238).
func newClient(tok *token.Token) *apns2.Client {
	transport := &http2.Transport{
		ReadIdleTimeout: apns2.ReadIdleTimeout,
	}

	client := apns2.NewTokenClient(tok)
	client.HTTPClient = &http.Client{
		Transport: transport,
		Timeout:   apns2.HTTPClientTimeout,
	}

	return client
}

// Notify describes a notification about a new event for a single device.
//
// Badge is the unread count; iOS puts it on the icon as is. A negative value
// means "leave the badge alone".
type Notify struct {
	Title   string
	Body    string
	Badge   int
	Sandbox bool
	// Who the message is from, as the client expects to read it: a decimal
	// string under `from_id`. Without it a tap has nowhere to go - the app opens
	// whatever it opens by default, which is not the conversation somebody was
	// told about.
	FromId string
	// The chat the message is in and the message itself, for the envelope
	// (#42): the extension polls the difference and then looks this message
	// up by id to read its words, under `from_id`, `chat_id` or `channel_id`
	// by the kind of peer - the keys the extension was written to read.
	PeerType int32
	PeerId   int64
	MsgId    int32
	// The device's registered push secret, hex as the database holds it. With
	// it the push carries the envelope the notification extension opens (#42);
	// without it the alert alone, which is what a build without the extension
	// shows either way.
	Secret string
}

// peerKey is the key the extension reads a chat's id under.
func peerKey(peerType int32) string {
	switch peerType {
	case int32(mtproto.PEER_CHAT):
		return "chat_id"
	case int32(mtproto.PEER_CHANNEL):
		return "channel_id"
	default:
		return "from_id"
	}
}

// buildPayload is the payload as sent, on its own so a test can read it.
//
// The alert is the fallback: what the phone shows when the extension does not
// answer in time, or is not there. mutable-content is what makes the extension
// run at all, and p is the envelope it opens - the same one the FCM path
// builds, with from_id and no text. An envelope that cannot be built does not
// stop the push: the alert still says a message came.
func buildPayload(n Notify) *payload.Payload {
	p := payload.NewPayload().
		AlertTitle(n.Title).
		AlertBody(n.Body).
		Sound("default")
	if n.Badge >= 0 {
		p = p.Badge(n.Badge)
	}
	if n.FromId != "" {
		p = p.Custom("from_id", n.FromId)
	}
	if n.Secret != "" {
		// The shape upstream's extension reads: the alert again, so the
		// extension starts from the same words the phone would show, and the
		// ids as decimal strings at the top level.
		inside := map[string]any{
			"aps": map[string]any{
				"alert": map[string]any{"title": n.Title, "body": n.Body},
				"sound": "default",
				"badge": n.Badge,
			},
			peerKey(n.PeerType): strconv.FormatInt(n.PeerId, 10),
			"msg_id":            strconv.FormatInt(int64(n.MsgId), 10),
		}
		envelope, err := fcm.Envelope(n.Secret, inside)
		if err == nil {
			p = p.MutableContent().Custom("p", envelope)
		}
	}
	return p
}

// PayloadJSON is the payload exactly as it would go to Apple, for a walk that
// hands it to a simulator instead: `xcrun simctl push` takes the same JSON, so
// the extension is measured on what the server really sends rather than on a
// copy of the algorithm kept beside the test (#42).
func PayloadJSON(n Notify) (string, error) {
	raw, err := buildPayload(n).MarshalJSON()
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// Send delivers the notification. It returns ErrTokenGone when the token should
// be forgotten.
func (s *Sender) Send(ctx context.Context, deviceToken string, n Notify) error {
	p := buildPayload(n)

	client := s.production
	if n.Sandbox {
		client = s.sandbox
	}

	res, err := client.PushWithContext(ctx, &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       s.topic,
		Payload:     p,
		Priority:    apns2.PriorityHigh,
		PushType:    apns2.PushTypeAlert,
		// A notification about an unread message goes stale: if the phone stayed
		// off for a day, showing it at power-on is pointless — the person will
		// see the conversation anyway.
		Expiration: time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		return fmt.Errorf("apns: delivery failed: %w", err)
	}

	switch res.Reason {
	case apns2.ReasonUnregistered, apns2.ReasonBadDeviceToken, apns2.ReasonExpiredToken:
		return ErrTokenGone
	case reasonBadEnvironmentKey, apns2.ReasonBadCertificateEnvironment:
		env := "production"
		if n.Sandbox {
			env = "sandbox"
		}
		return fmt.Errorf("%w: the device expects notifications through %s, "+
			"an Apple key for that environment is required", ErrWrongEnvironment, env)
	}

	if !res.Sent() {
		return fmt.Errorf("apns: rejected %d %s", res.StatusCode, res.Reason)
	}

	return nil
}
