package invite

import (
	"context"
	"strconv"
	"testing"

	"github.com/teamgram/teamgram-server/pkg/code/attempt"
)

// The hole this closes, stated as a test: entering 12345 for somebody else's
// registered number used to return their session. It was measured against the
// live server before it was fixed, and the number in the log was a real account.
func TestTheConstantNoLongerOpensAnything(t *testing.T) {
	v, _ := newTestVerifier()

	if err := v.VerifySmsCode(context.Background(), attempt.Attempt{
		CodeHash: "hash-1", Code: "12345", Generated: "48210",
		PhoneNumber: "+79990012345", PhoneRegistered: true,
	}); err == nil {
		t.Fatal("12345 was accepted; that is the hole, not a feature")
	}
}

func TestTheGeneratedCodeIsWhatWorks(t *testing.T) {
	v, _ := newTestVerifier()

	if err := v.VerifySmsCode(context.Background(), attempt.Attempt{
		CodeHash: "hash-2", Code: "48210", Generated: "48210",
		PhoneNumber: "+79990012345", PhoneRegistered: true,
	}); err != nil {
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
		if err := v.VerifySmsCode(ctx, attempt.Attempt{
			CodeHash: "hash-3", Code: "00000", Generated: "48210",
		}); err == nil {
			t.Fatal("a wrong code was accepted")
		}
	}

	// The right code, one try too late.
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-3", Code: "48210", Generated: "48210",
	}); err == nil {
		t.Fatal("after three wrong guesses the code still worked")
	}
}

// A phone with no session cannot be sent anything, so the owner mints an
// invitation. It works once.
func TestAnInvitationWorksOnce(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	store.data[InvitationKey("70314")] = Encode(Invitation{Note: "for Natalya"})

	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-4", Code: "70314", PhoneNumber: "+79995550001",
	}); err != nil {
		t.Fatalf("a minted invitation was refused: %v", err)
	}
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-5", Code: "70314", PhoneNumber: "+79995550002",
	}); err == nil {
		t.Fatal("the same invitation was accepted twice")
	}
}

// The second hole, found by measuring the first fix: an invitation was tied to
// nobody, so anyone holding one could type a number that already had an account
// and be let into it. Measured against the live server - a code minted for a
// stranger opened an account that had just been created.
func TestAnInvitationIsNotAKeyToSomebodyElsesAccount(t *testing.T) {
	v, store := newTestVerifier()
	store.data[InvitationKey("70314")] = Encode(Invitation{Note: "for somebody new"})

	if err := v.VerifySmsCode(context.Background(), attempt.Attempt{
		CodeHash: "hash-6", Code: "70314",
		PhoneNumber: "+79990012345", PhoneRegistered: true,
	}); err == nil {
		t.Fatal("an unbound invitation opened an account that already existed")
	}

	// And it must still be there afterwards: refusing somebody must not burn
	// the invitation, or knowing a code would be enough to cancel it.
	if store.data[InvitationKey("70314")] == "" {
		t.Fatal("a refused attempt spent the invitation")
	}
}

// The way back in after a lost phone: the owner mints an invitation naming that
// number. It opens that account and no other.
func TestAnInvitationForANumberOpensThatNumber(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	store.data[InvitationKey("70314")] = Encode(Invitation{
		Phone: "+7 999 001-23-45", Note: "Dmitry, new phone"})

	// Somebody else's number, even one with no account: not this invitation.
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-7", Code: "70314", PhoneNumber: "+79995550001",
	}); err == nil {
		t.Fatal("an invitation minted for one number opened another")
	}

	// The number it names, spelled the way the client sends it.
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-8", Code: "70314",
		PhoneNumber: "+79990012345", PhoneRegistered: true,
	}); err != nil {
		t.Fatalf("the owner could not get back into their own account: %v", err)
	}
}

func TestAnInventedInvitationIsRefused(t *testing.T) {
	v, _ := newTestVerifier()

	if err := v.VerifySmsCode(context.Background(), attempt.Attempt{
		CodeHash: "hash-9", Code: "11111", PhoneNumber: "+79995550001",
	}); err == nil {
		t.Fatal("a code nobody minted was accepted")
	}
}

// An empty code is not "no code entered yet", it is an attempt with nothing in
// it, and the server must say so rather than compare it against an empty
// Generated and let it through.
func TestAnEmptyCodeIsRefused(t *testing.T) {
	v, _ := newTestVerifier()

	if err := v.VerifySmsCode(context.Background(), attempt.Attempt{
		CodeHash: "hash-10", Code: "", PhoneNumber: "+79995550001",
	}); err == nil {
		t.Fatal("an empty code was accepted")
	}
}

// A store that is not answering must not become a way in.
func TestAFailingStoreDoesNotOpenTheDoor(t *testing.T) {
	v := &verifier{store: brokenStore{}}
	ctx := context.Background()

	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-11", Code: "12345", Generated: "48210",
	}); err == nil {
		t.Fatal("a wrong code was accepted while the store was failing")
	}
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-12", Code: "48210", Generated: "48210",
	}); err != nil {
		t.Fatalf("the right code was refused while the store was failing: %v", err)
	}
}

// Invitations written before they carried a number at all: a plain note. They
// must read as unbound rather than as a code for a number spelled "for Natalya".
func TestAnOldInvitationIsReadAsUnbound(t *testing.T) {
	inv := Decode("for Natalya")

	if inv.Phone != "" {
		t.Errorf("a plain note became a phone binding: %q", inv.Phone)
	}
	if inv.Note != "for Natalya" {
		t.Errorf("the note was lost: %q", inv.Note)
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

func (m *mapStore) SetCtx(_ context.Context, key, value string) error {
	m.data[key] = value
	return nil
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

func (brokenStore) SetCtx(context.Context, string, string) error {
	return errMissing{}
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

// memoRecorder remembers the one redemption it was told about (#47).
type memoRecorder struct {
	code, phone string
	at          int32
}

func (m *memoRecorder) Redeemed(_ context.Context, code, phone string, at int32) error {
	m.code, m.phone, m.at = code, phone, at
	return nil
}

func TestSpendingAnInvitationIsRecorded(t *testing.T) {
	store := &mapStore{data: map[string]string{}}
	code, _ := Mint(context.Background(), store, 3600, Invitation{Phone: "+79991234567"})
	memo := &memoRecorder{}
	v := New(nil, store).WithRecorder(memo)

	if !v.useInvitation(context.Background(), attempt.Attempt{Code: code, PhoneNumber: "+79991234567"}) {
		t.Fatal("the invitation was not spent")
	}
	if memo.code != code || memo.phone != "+79991234567" || memo.at == 0 {
		t.Fatalf("recorded %+v", memo)
	}
}
