package pushrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrTokenGone is the relay's 410: the phone is no longer where this token
// pointed, and the server should forget it.
var ErrTokenGone = errors.New("pushrelay: token is gone")

// ErrRefused is the relay saying no to this server - an unknown key, a
// blocked server, a quota - as opposed to Apple or Google saying no.
var ErrRefused = errors.New("pushrelay: refused")

// Client is a server's side of the wire.
type Client struct {
	URL  string
	Key  string
	HTTP *http.Client
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// Send hands one push to the relay.
func (c *Client) Send(ctx context.Context, p Push) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.URL, "/")+"/v1/push", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)
	res, err := c.http().Do(req)
	if err != nil {
		return fmt.Errorf("pushrelay: %w", err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	switch res.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusGone:
		return ErrTokenGone
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ErrRefused, res.Status)
	default:
		return fmt.Errorf("pushrelay: %s", res.Status)
	}
}

// Register asks the relay for a key, once per server.
func Register(ctx context.Context, url, address string) (id, key string, err error) {
	body, _ := json.Marshal(map[string]string{"address": address})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(url, "/")+"/v1/servers", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", "", fmt.Errorf("pushrelay: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("pushrelay: registration answered %s", res.Status)
	}
	var answer struct {
		ServerId string `json:"server_id"`
		Key      string `json:"key"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&answer); err != nil {
		return "", "", fmt.Errorf("pushrelay: %w", err)
	}
	if answer.ServerId == "" || answer.Key == "" {
		return "", "", errors.New("pushrelay: registration answered without an id or a key")
	}
	return answer.ServerId, answer.Key, nil
}
