package invite

import (
	"context"
	"strings"
	"testing"

	"github.com/teamgram/teamgram-server/pkg/code/attempt"
)

// The whole point, stated once: a phone is gone, nothing can be delivered
// anywhere, and the person still gets back into their own account.
func TestALostPhoneGetsBackInWithTheRecoveryCode(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	const phone = "+79990012345"

	code, err := MintRecoveryCode(ctx, store, phone, true)
	if err != nil {
		t.Fatalf("no code was minted: %v", err)
	}

	if err = v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-1", Code: code, PhoneNumber: phone, PhoneRegistered: true,
	}); err != nil {
		t.Fatalf("the owner's own recovery code was refused: %v", err)
	}
}

// Once. A code that survived being used would survive being read over
// somebody's shoulder, and the person would never know.
func TestARecoveryCodeWorksOnce(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	const phone = "+79990012345"

	code, _ := MintRecoveryCode(ctx, store, phone, true)
	_ = v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-1", Code: code, PhoneNumber: phone, PhoneRegistered: true})

	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-2", Code: code, PhoneNumber: phone, PhoneRegistered: true,
	}); err == nil {
		t.Fatal("the same recovery code was accepted twice")
	}
}

// It is one account's code, not a master key: the hole that an invitation tied
// to nobody turned out to be, and the same mistake is available here.
func TestARecoveryCodeOpensOnlyItsOwnAccount(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()

	code, _ := MintRecoveryCode(ctx, store, "+79990012345", true)

	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-1", Code: code,
		PhoneNumber: "+79995550001", PhoneRegistered: true,
	}); err == nil {
		t.Fatal("one account's recovery code opened another account")
	}
}

// Reading the store must not be the same as knowing the code. Eight digits is
// a hundred million, which any fast hash gives up in seconds - so what is kept
// has to be slow to test, and it must not be the code itself.
func TestTheStoreDoesNotHoldTheCode(t *testing.T) {
	store := &mapStore{data: map[string]string{}}
	ctx := context.Background()

	code, _ := MintRecoveryCode(ctx, store, "+79990012345", true)

	for key, value := range store.data {
		if strings.Contains(value, code) {
			t.Fatalf("the code is readable in the store under %q", key)
		}
	}
	if len(code) != recoveryDigits {
		t.Errorf("the code is %d digits, expected %d", len(code), recoveryDigits)
	}
}

// A second code would quietly turn the paper the first one is written on into
// a worthless one.
func TestAnAccountIsNotGivenASecondCode(t *testing.T) {
	store := &mapStore{data: map[string]string{}}
	ctx := context.Background()
	const phone = "+79990012345"

	if HasRecoveryCode(ctx, store, phone) {
		t.Fatal("an account with no code was said to have one")
	}

	if _, err := MintRecoveryCode(ctx, store, phone, true); err != nil {
		t.Fatal(err)
	}

	if !HasRecoveryCode(ctx, store, phone) {
		t.Fatal("a minted code was not noticed, so a second one would replace it")
	}
}

// Changing the number must take the way back along with it.
func TestTheCodeFollowsAChangedNumber(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	const from, to = "+79990012345", "+79995550001"

	code, _ := MintRecoveryCode(ctx, store, from, true)
	MoveRecoveryCode(ctx, store, from, to)

	if HasRecoveryCode(ctx, store, from) {
		t.Error("the code stayed on the number the account left")
	}
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-1", Code: code, PhoneNumber: to, PhoneRegistered: true,
	}); err != nil {
		t.Fatalf("the code did not follow the account to its new number: %v", err)
	}
}

// A number nobody minted for must not be openable by guessing wildly.
func TestAnAccountWithoutACodeIsNotOpenedByAnything(t *testing.T) {
	v, _ := newTestVerifier()

	if err := v.VerifySmsCode(context.Background(), attempt.Attempt{
		CodeHash: "hash-1", Code: "12345678",
		PhoneNumber: "+79990012345", PhoneRegistered: true,
	}); err == nil {
		t.Fatal("an account with no recovery code was opened")
	}
}

// A code minted by hand may never reach the person it was for. Until the server
// has handed one over itself, the account is not covered - or somebody ends up
// with a way back that nobody knows and nothing ever says so.
func TestAHandMintedCodeDoesNotCountAsDelivered(t *testing.T) {
	store := &mapStore{data: map[string]string{}}
	ctx := context.Background()
	const phone = "+79990012345"

	if _, err := MintRecoveryCode(ctx, store, phone, false); err != nil {
		t.Fatal(err)
	}

	if !HasRecoveryCode(ctx, store, phone) {
		t.Error("a code was minted and the store says there is none")
	}
	if HasDeliveredRecoveryCode(ctx, store, phone) {
		t.Fatal("a code nobody was told about counted as delivered, so the " +
			"server will never hand one over")
	}
}

// It still opens the account, of course - that is what it was minted for.
func TestAHandMintedCodeStillWorks(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	const phone = "+79990012345"

	code, _ := MintRecoveryCode(ctx, store, phone, false)

	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-1", Code: code, PhoneNumber: phone, PhoneRegistered: true,
	}); err != nil {
		t.Fatalf("a code minted by hand was refused: %v", err)
	}
}

// Codes stored before this carried anything but the hash were all minted and
// delivered by the server, and must not now look undelivered - that would hand
// everybody a second code and void the first.
func TestAnOldStoredCodeCountsAsDelivered(t *testing.T) {
	stored, ok := readRecovery("$2a$10$abcdefghijklmnopqrstuv")

	if !ok {
		t.Fatal("a stored code read as nothing")
	}
	if !stored.Delivered {
		t.Error("a code from before this flag existed read as undelivered")
	}
}
