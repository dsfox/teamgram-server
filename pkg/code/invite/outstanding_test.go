package invite

import (
	"context"
	"errors"
	"testing"
)

// The list has to answer with what is actually there.
//
// It used to walk every five-digit code while codes were six digits, so it said
// "no invitations outstanding" one second after minting one - and a tool that
// reports the opposite of the truth sends whoever reads it looking in the wrong
// place. It did.
func TestOutstandingListsWhatWasMinted(t *testing.T) {
	ctx := context.Background()
	store := &mapStore{data: map[string]string{}}

	code := "847360"
	if err := store.SetexCtx(ctx, InvitationKey(code), Encode(Invitation{Phone: "+79055767127", Note: "restart"}), 3600); err != nil {
		t.Fatalf("cannot store the invitation: %v", err)
	}
	if err := RememberOutstanding(ctx, store, code); err != nil {
		t.Fatalf("cannot remember it: %v", err)
	}

	live := Outstanding(ctx, store)
	if len(live) != 1 || live[0].Code != code {
		t.Fatalf("expected the minted code, got %+v", live)
	}
	if live[0].Invitation.Phone != "+79055767127" {
		t.Fatalf("expected the number it was minted for, got %q", live[0].Invitation.Phone)
	}
}

// A list that could not be read is not an empty list. Treating it as one wrote
// the new code over it, and every other live invitation dropped out of --list
// while it still opened the door.
func TestAnUnreadableListIsNotOverwritten(t *testing.T) {
	ctx := context.Background()
	store := &mapStore{data: map[string]string{}}
	_ = RememberOutstanding(ctx, store, "111111")

	flaky := &listUnreadable{mapStore: store}
	if err := RememberOutstanding(ctx, flaky, "222222"); err == nil {
		t.Fatal("remembering a code over a list nobody could read reported success")
	}
	codes, _ := outstandingCodes(ctx, store)
	if len(codes) != 1 || codes[0] != "111111" {
		t.Fatalf("the list was written over: %v", codes)
	}
}

type listUnreadable struct {
	*mapStore
}

func (s *listUnreadable) GetCtx(ctx context.Context, key string) (string, error) {
	if key == outstandingKey {
		return "", errors.New("the store is not answering")
	}
	return s.mapStore.GetCtx(ctx, key)
}

// A used invitation disappears from the store and must disappear from the list,
// without the verifying path having to say so.
func TestOutstandingForgetsWhatWasUsed(t *testing.T) {
	ctx := context.Background()
	store := &mapStore{data: map[string]string{}}

	_ = store.SetexCtx(ctx, InvitationKey("111111"), Encode(Invitation{Note: "used"}), 3600)
	_ = RememberOutstanding(ctx, store, "111111")
	_ = RememberOutstanding(ctx, store, "222222")

	live := Outstanding(ctx, store)
	if len(live) != 1 || live[0].Code != "111111" {
		t.Fatalf("expected only the one that is still there, got %+v", live)
	}
	if again := Outstanding(ctx, store); len(again) != 1 {
		t.Fatalf("the forgotten one came back: %+v", again)
	}
}
