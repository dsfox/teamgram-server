package invite

import (
	"context"
	"encoding/json"
	"strings"
)

// The list of invitations that have been minted and not yet used.
//
// The store this runs on has no way to walk its keys, and the tool used to fake
// one by asking about every five-digit code in turn. Codes became six digits and
// the walk kept asking about five, so `--list` answered "no invitations
// outstanding" straight after minting one - which is worse than not having the
// command at all: it says the door is shut when it is open.
const outstandingKey = invitationPrefix + "outstanding"

// RememberOutstanding notes that a code exists, so it can be listed later.
func RememberOutstanding(ctx context.Context, store Store, code string) error {
	codes, _ := outstandingCodes(ctx, store)
	for _, existing := range codes {
		if existing == code {
			return nil
		}
	}
	codes = append(codes, code)
	return writeOutstanding(ctx, store, codes)
}

// Outstanding returns the codes that are still good, and forgets the rest.
//
// Each one is asked about rather than trusted: an invitation is used once and
// then gone, and nothing tells this list about it. Checking here keeps the list
// honest without the verifying path having to know it exists.
func Outstanding(ctx context.Context, store Store) []Outstanding_ {
	codes, _ := outstandingCodes(ctx, store)

	var live []Outstanding_
	var stillThere []string
	for _, code := range codes {
		value, err := store.GetCtx(ctx, InvitationKey(code))
		if err != nil || value == "" {
			continue
		}
		live = append(live, Outstanding_{Code: code, Invitation: Decode(value)})
		stillThere = append(stillThere, code)
	}

	if len(stillThere) != len(codes) {
		_ = writeOutstanding(ctx, store, stillThere)
	}
	return live
}

// Outstanding_ is one live invitation: the code and what it stands for.
type Outstanding_ struct {
	Code       string
	Invitation Invitation
}

func outstandingCodes(ctx context.Context, store Store) ([]string, error) {
	value, err := store.GetCtx(ctx, outstandingKey)
	if err != nil || strings.TrimSpace(value) == "" {
		return nil, err
	}
	var codes []string
	if err := json.Unmarshal([]byte(value), &codes); err != nil {
		return nil, err
	}
	return codes, nil
}

func writeOutstanding(ctx context.Context, store Store, codes []string) error {
	if len(codes) == 0 {
		_, err := store.DelCtx(ctx, outstandingKey)
		return err
	}
	body, err := json.Marshal(codes)
	if err != nil {
		return err
	}
	return store.SetCtx(ctx, outstandingKey, string(body))
}
