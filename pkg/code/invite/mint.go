package invite

import (
	"context"
	"fmt"
	"time"
)

// Mint writes an invitation nobody has used yet and returns its code.
//
// The code is as long as every other sign-in code, because the clients draw
// exactly as many boxes as the server declares - a shorter one cannot be typed.
// seconds is how long it is good for.
//
// It lived in cmd/invite until #47, when a phone gained a way to ask for one
// too; the CLI now calls this.
func Mint(ctx context.Context, store Store, seconds int, inv Invitation) (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		code := Code()
		key := InvitationKey(code)

		// Taken already: try another rather than overwrite somebody's
		// invitation. A missing key is an empty string, not an error.
		if existing, err := store.GetCtx(ctx, key); err == nil && existing != "" {
			continue
		}

		if inv.Note == "" {
			inv.Note = "minted " + time.Now().Format(time.RFC3339)
		}
		if err := store.SetexCtx(ctx, key, Encode(inv), seconds); err != nil {
			return "", fmt.Errorf("cannot write the invitation: %w", err)
		}
		// Noted, so that --list can find it: the store cannot walk its keys.
		// The code is returned alongside the error - it exists and works.
		if err := RememberOutstanding(ctx, store, code); err != nil {
			return code, fmt.Errorf("the invitation was minted but not listed: %w", err)
		}
		return code, nil
	}
	return "", fmt.Errorf("could not find a free code after twenty tries")
}

// Revoke forgets an invitation before anybody used it.
func Revoke(ctx context.Context, store Store, code string) error {
	if _, err := store.DelCtx(ctx, InvitationKey(code)); err != nil {
		return fmt.Errorf("cannot revoke the invitation: %w", err)
	}
	return nil
}
