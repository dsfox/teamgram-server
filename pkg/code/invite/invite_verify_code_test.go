package invite

import (
	"context"
	"strconv"
	"testing"
)

// The hole this closes, stated as a test: entering 12345 for somebody else's
// registered number used to return their session. It was measured against the
// live server before it was fixed, and the number in the log was a real account.
func TestTheConstantNoLongerOpensAnything(t *testing.T) {
	v, _ := newTestVerifier()
	realCode := "48210"

	if err := v.VerifySmsCode(context.Background(), "hash-1", "12345", realCode); err == nil {
		t.Fatal("12345 was accepted; that is the hole, not a feature")
	}
}

func TestTheGeneratedCodeIsWhatWorks(t *testing.T) {
	v, _ := newTestVerifier()

	if err := v.VerifySmsCode(context.Background(), "hash-2", "48210", "48210"); err != nil {
		t.Fatalf("the code the server generated was refused: %v", err)
	}
}

// Five digits is a hundred thousand guesses, which is not many. Three tries and
// the attempt is spent - the client has to ask for a new code, and that asking
// is visible.
func TestGuessingRunsOut(t *testing.T) {
	v, _ := newTestVerifier()
	ctx := context.Background()

	for i := 0; i < maxAttempts; i++ {
		if err := v.VerifySmsCode(ctx, "hash-3", "00000", "48210"); err == nil {
			t.Fatal("a wrong code was accepted")
		}
	}

	// The right code, one try too late.
	if err := v.VerifySmsCode(ctx, "hash-3", "48210", "48210"); err == nil {
		t.Fatal("after three wrong guesses the code still worked")
	}
}

// A phone with no session cannot be sent anything, so the owner mints an
// invitation. It works once.
func TestAnInvitationWorksOnce(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	store.data[InvitationKey("70314")] = "for Natalya"

	if err := v.VerifySmsCode(ctx, "hash-4", "70314", ""); err != nil {
		t.Fatalf("a minted invitation was refused: %v", err)
	}
	if err := v.VerifySmsCode(ctx, "hash-5", "70314", ""); err == nil {
		t.Fatal("the same invitation was accepted twice")
	}
}

func TestAnInventedInvitationIsRefused(t *testing.T) {
	v, _ := newTestVerifier()

	if err := v.VerifySmsCode(context.Background(), "hash-6", "11111", ""); err == nil {
		t.Fatal("a code nobody minted was accepted")
	}
}

// An empty code is not "no code entered yet", it is an attempt with nothing in
// it, and the server must say so rather than compare it against an empty
// extraData and let it through.
func TestAnEmptyCodeIsRefused(t *testing.T) {
	v, _ := newTestVerifier()

	if err := v.VerifySmsCode(context.Background(), "hash-7", "", ""); err == nil {
		t.Fatal("an empty code was accepted")
	}
}

// A store that is not answering must not become a way in.
func TestAFailingStoreDoesNotOpenTheDoor(t *testing.T) {
	v := &verifier{store: brokenStore{}}

	if err := v.VerifySmsCode(context.Background(), "hash-8", "12345", "48210"); err == nil {
		t.Fatal("a wrong code was accepted while the store was failing")
	}
	if err := v.VerifySmsCode(context.Background(), "hash-9", "48210", "48210"); err != nil {
		t.Fatalf("the right code was refused while the store was failing: %v", err)
	}
}

func newTestVerifier() (*verifier, *mapStore) {
	store := &mapStore{data: map[string]string{}}
	return &verifier{store: store}, store
}

// mapStore is the smallest thing that behaves like the store: enough to count
// attempts and spend invitations.
type mapStore struct {
	data map[string]string
}

// A missing key is an empty string and no error, which is what the store this
// stands in for does. The first version of this fake returned an error instead,
// and that convenience hid a bug: the tool that mints invitations decided every
// code was taken and could not mint any.
func (m *mapStore) GetCtx(_ context.Context, key string) (string, error) {
	return m.data[key], nil
}

func (m *mapStore) SetexCtx(_ context.Context, key, value string, _ int) error {
	m.data[key] = value
	return nil
}

func (m *mapStore) IncrCtx(_ context.Context, key string) (int64, error) {
	n, _ := strconv.ParseInt(m.data[key], 10, 64)
	n++
	m.data[key] = strconv.FormatInt(n, 10)
	return n, nil
}

func (m *mapStore) DelCtx(_ context.Context, keys ...string) (int, error) {
	deleted := 0
	for _, key := range keys {
		if _, ok := m.data[key]; ok {
			delete(m.data, key)
			deleted++
		}
	}
	return deleted, nil
}

type errMissing struct{}

func (errMissing) Error() string { return "not found" }

type brokenStore struct{}

func (brokenStore) GetCtx(context.Context, string) (string, error) {
	return "", errMissing{}
}

func (brokenStore) SetexCtx(context.Context, string, string, int) error {
	return errMissing{}
}

func (brokenStore) IncrCtx(context.Context, string) (int64, error) {
	return 0, errMissing{}
}

func (brokenStore) DelCtx(context.Context, ...string) (int, error) {
	return 0, errMissing{}
}
