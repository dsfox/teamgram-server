package pushrelay

import (
	"context"
	"log"

	"github.com/teamgram/teamgram-server/pkg/apns"
	"github.com/teamgram/teamgram-server/pkg/fcm"
)

// AppleForwarder hands a push to Apple with the relay's own words.
type AppleForwarder struct {
	Sender      *apns.Sender
	Title, Body string
}

func (f AppleForwarder) Send(ctx context.Context, p Push) error {
	return f.Sender.Send(ctx, p.Token, apns.Notify{
		Title: f.Title, Body: f.Body, Badge: p.Badge, Sandbox: p.Sandbox, FromId: p.FromId, Envelope: p.P,
	})
}

// GoogleForwarder hands a push to Google with the relay's own words.
type GoogleForwarder struct {
	Sender      *fcm.Sender
	Title, Body string
}

func (f GoogleForwarder) Send(ctx context.Context, p Push) error {
	return f.Sender.Send(ctx, p.Token, fcm.Notify{
		Title: f.Title, Body: f.Body, Badge: p.Badge, FromId: p.FromId, Envelope: p.P,
	})
}

// LogOnly is a relay without keys: it says what it would send and sends
// nothing. What the local stand runs.
type LogOnly struct{}

func (LogOnly) Send(_ context.Context, p Push) error {
	log.Printf("would send %s %s", p.Platform, KeyHash(p.Token)[:8])
	return nil
}
