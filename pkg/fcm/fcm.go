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
	"strconv"
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
}

func (s *Sender) Send(ctx context.Context, deviceToken string, n Notify) error {
	// The badge is Android's "notification count", and it is a string here where
	// Apple takes a number - one of several small differences that make a shared
	// payload type not worth it.
	message := map[string]any{
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
			"data": map[string]any{
				// The client wakes and fetches; it never reads the text from here.
				"badge": strconv.Itoa(n.Badge),
				// Who it is from, so a tap opens that conversation rather than
				// whatever the app opens by default.
				"from_id": n.FromId,
			},
		},
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
