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
//
// A lasting invitation (#214) has no seconds: it is written without an expiry,
// and only for a number.
func Mint(ctx context.Context, store Store, seconds int, inv Invitation) (string, error) {
	if inv.Lasting && Digits(inv.Phone) == "" {
		return "", fmt.Errorf("a lasting invitation has to name the number it opens")
	}
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
		var err error
		if inv.Lasting {
			err = store.SetCtx(ctx, key, Encode(inv))
		} else {
			err = store.SetexCtx(ctx, key, Encode(inv), seconds)
		}
		if err != nil {
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

// RevokeFor forgets every outstanding invitation for this number but keep, and
// says how many went. Minting the reviewer's code again used to leave every
// earlier one live until its own time ran out - six of them on 27 September,
// each handed to Apple once and each still opening the account (#214).
func RevokeFor(ctx context.Context, store Store, phone, keep string) (int, error) {
	gone := 0
	for _, item := range Outstanding(ctx, store) {
		if item.Code == keep || Digits(item.Invitation.Phone) != Digits(phone) {
			continue
		}
		if err := Revoke(ctx, store, item.Code); err != nil {
			return gone, err
		}
		gone++
	}
	return gone, nil
}
