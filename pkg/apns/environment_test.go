package apns

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Apple keys can be bound to a single environment: production or sandbox. A key
// from the wrong environment yields BadEnvironmentKeyInToken, and notifications
// silently stop arriving — the only trace is a line in the server log.
//
// The test answers "which environments does our key serve" by sending a
// deliberately invalid device token to both.
//
//	BadDeviceToken           — the environment works, only the made-up token was rejected;
//	BadEnvironmentKeyInToken — the key does not fit this environment.
//
// Without a key the test is skipped.
func TestKeyEnvironments(t *testing.T) {
	keyPath, keyId := findKey(t)

	sender, err := New(Config{
		KeyPath: keyPath,
		KeyId:   envOrDefault("APNS_KEY_ID", keyId),
		TeamId:  envOrDefault("APNS_TEAM_ID", "C3DL5896VG"),
		Topic:   envOrDefault("APNS_TOPIC", "app.twobytes.ios"),
	})
	if err != nil {
		t.Fatalf("cannot read the key: %v", err)
	}
	t.Logf("key: %s", filepath.Base(keyPath))

	const fakeToken = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

	for _, c := range []struct {
		name     string
		sandbox  bool
		required bool
	}{
		// We hand out the app through TestFlight, and such builds work with the
		// production environment only. Without it nobody gets notifications.
		{"production (TestFlight builds)", false, true},
		// The sandbox matters only for builds installed on a device directly. A
		// key bound to one environment does not serve it, and that is not a fault.
		{"sandbox (direct device builds)", true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := sender.Send(context.Background(), fakeToken, Notify{
				Title: "check", Body: "check", Badge: -1, Sandbox: c.sandbox,
			})

			switch {
			case errors.Is(err, ErrTokenGone):
				t.Log("environment works: the key was accepted, only the made-up token was rejected")
			case err == nil:
				t.Error("the made-up token was accepted, which must not happen")
			case c.required:
				t.Errorf("environment unavailable although it is required: %v", err)
			default:
				t.Skipf("environment unavailable, and our distribution does not need it: %v", err)
			}
		})
	}
}

// findKey looks for the key in secrets. Apple names the file
// AuthKey_<identifier>.p8, so the identifier comes from there — keeping it
// separately in the code would mean two sources of truth.
func findKey(t *testing.T) (path, keyId string) {
	t.Helper()

	matches, _ := filepath.Glob("../../../secrets/AuthKey_*.p8")
	switch len(matches) {
	case 0:
		t.Skip("no key in secrets, nothing to check")
	case 1:
	default:
		t.Skipf("several keys found (%v), unclear which one to check", matches)
	}

	name := filepath.Base(matches[0])
	keyId = strings.TrimSuffix(strings.TrimPrefix(name, "AuthKey_"), ".p8")

	return matches[0], keyId
}

func envOrDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}

	return fallback
}
