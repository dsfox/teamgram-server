// Package fcm sends notifications to Android devices through Firebase Cloud
// Messaging, the way pkg/apns does for Apple.
//
// It speaks the HTTP v1 API directly rather than through the Firebase SDK: the
// whole exchange is one signed POST, and the SDK would pull in a great deal of
// machinery to build it.
package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ErrTokenGone means the device is no longer reachable: the app was removed, or
// the token was replaced. The caller should forget it rather than retry.
var ErrTokenGone = errors.New("fcm: token is no longer registered")

const scope = "https://www.googleapis.com/auth/firebase.messaging"

// Config is what it takes to send: a service account key and the project it
// belongs to.
type Config struct {
	ServiceAccountPath string
	ProjectId          string
}

// ConfigFromEnv reads the settings, and reports whether both are present.
// Without them the server runs exactly as before, minus Android notifications.
func ConfigFromEnv() (Config, bool) {
	c := Config{
		ServiceAccountPath: os.Getenv("FCM_SERVICE_ACCOUNT"),
		ProjectId:          os.Getenv("FCM_PROJECT_ID"),
	}
	return c, c.ServiceAccountPath != "" && c.ProjectId != ""
}

// Sender holds the credentials and refreshes the access token by itself.
type Sender struct {
	projectId string
	client    *http.Client
}

func New(ctx context.Context, c Config) (*Sender, error) {
	key, err := os.ReadFile(c.ServiceAccountPath)
	if err != nil {
		return nil, fmt.Errorf("fcm: cannot read the service account %s: %w", c.ServiceAccountPath, err)
	}

	creds, err := google.CredentialsFromJSON(ctx, key, scope)
	if err != nil {
		return nil, fmt.Errorf("fcm: the service account is not usable: %w", err)
	}

	return &Sender{
		projectId: c.ProjectId,
		// The token source refreshes on its own; the timeout is ours, so a slow
		// Google does not hold up the goroutine sending the notification.
		client: &http.Client{
			Transport: &oauth2.Transport{Source: creds.TokenSource},
			Timeout:   10 * time.Second,
		},
	}, nil
}

// Notify is what the phone shows. Deliberately the same shape as the Apple one,
// and deliberately without the message text: putting it here would hand what
// people write to Google.
type Notify struct {
	Title string
	Body  string
	Badge int
	// Who the message is from, so that a tap opens that conversation.
	FromId string
	// The device's push key, hex, as it registered it. With one the app is
	// woken and draws the notification itself; without one Firebase draws a
	// bare banner and the app never runs (#94).
	Secret string
}

// compose builds the message Google is asked to deliver.
//
// With the device's key it is data only, which is what makes the app itself
// run: Firebase hands a data-only message straight to the app, and draws
// nothing of its own. The app then wakes its connection, fetches the message
// and shows a notification with the sender's real name - and, in an encrypted
// conversation, text that only that device could have read.
//
// Without a key there is nothing the app could open, so the old banner is sent
// instead: "New message", drawn by Firebase, better than silence.
func (s *Sender) compose(deviceToken string, n Notify) (map[string]any, error) {
	if n.Secret == "" {
		return map[string]any{
			"message": map[string]any{
				"token": deviceToken,
				"notification": map[string]any{
					"title": n.Title,
					"body":  n.Body,
				},
				"android": map[string]any{
					"priority": "high",
					"notification": map[string]any{
						"notification_count": n.Badge,
						"sound":              "default",
					},
				},
			},
		}, nil
	}

	// No loc_key the client knows: it is not being told what to draw, only that
	// something arrived. Everything it shows afterwards it fetches itself.
	envelope, err := Envelope(n.Secret, map[string]any{
		"badge":   n.Badge,
		"custom":  map[string]any{"from_id": n.FromId},
		"loc_key": "",
	})
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"message": map[string]any{
			"token": deviceToken,
			// High priority is what buys the app a moment of network while the
			// phone is dozing; without it a data-only message can wait hours.
			"android": map[string]any{"priority": "high"},
			"data":    map[string]any{"p": envelope},
		},
	}, nil
}

func (s *Sender) Send(ctx context.Context, deviceToken string, n Notify) error {
	message, err := s.compose(deviceToken, n)
	if err != nil {
		return err
	}

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("fcm: cannot build the message: %w", err)
	}

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", s.projectId)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("fcm: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	answer, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

	// 404 is the device saying it is gone; 400 with UNREGISTERED means the same
	// thing said differently. Everything else is worth retrying and worth a log.
	if resp.StatusCode == http.StatusNotFound || bytes.Contains(answer, []byte("UNREGISTERED")) {
		return ErrTokenGone
	}
	return fmt.Errorf("fcm: %s: %s", resp.Status, answer)
}
