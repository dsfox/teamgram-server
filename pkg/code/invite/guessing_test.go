package invite

import (
	"context"
	"strings"
	"testing"

	"github.com/teamgram/teamgram-server/pkg/code/attempt"
)

// The hole this closes, measured before it was written: thirty-two guesses
// against one number went through in two seconds and nothing stopped them,
// because three tries are counted per code request and a fresh connection
// brings a fresh request. A hundred million codes at that rate is ten weeks in
// one thread.
func TestGuessingOneNumberRunsOutNoMatterHowManyRequests(t *testing.T) {
	v, _ := newTestVerifier()
	ctx := context.Background()
	const phone = "+79990012345"

	accepted := 0
	for i := 0; i < maxFailuresPerPhone*3; i++ {
		// A new code hash every time: that is what a new connection gets, and
		// what made the per-request counter worthless.
		err := v.VerifySmsCode(ctx, attempt.Attempt{
			CodeHash:    "hash-" + strings.Repeat("x", i),
			Code:        "00000000",
			PhoneNumber: phone, PhoneRegistered: true,
		})
		if err == nil {
			t.Fatal("a wrong code was accepted")
		}
		if isWait(err) {
			break
		}
		accepted++
	}

	if accepted > maxFailuresPerPhone {
		t.Fatalf("%d guesses went through before the number stopped answering, "+
			"expected at most %d", accepted, maxFailuresPerPhone)
	}
}

// The refusal has to say that waiting is what fixes it. Answering "invalid
// code" leaves a person retyping a code that is right.
func TestTheRefusalSaysToWait(t *testing.T) {
	v, _ := newTestVerifier()
	ctx := context.Background()
	const phone = "+79990012345"

	var last error
	for i := 0; i <= maxFailuresPerPhone; i++ {
		last = v.VerifySmsCode(ctx, attempt.Attempt{
			CodeHash: "hash-" + strings.Repeat("x", i), Code: "00000000",
			PhoneNumber: phone, PhoneRegistered: true,
		})
	}

	if !isWait(last) {
		t.Fatalf("after %d wrong codes the answer was %v, not a wait",
			maxFailuresPerPhone+1, last)
	}
}

// Guessing at one number must not shut another one out.
func TestOneNumbersGuessesDoNotLockAnother(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()

	for i := 0; i <= maxFailuresPerPhone*2; i++ {
		_ = v.VerifySmsCode(ctx, attempt.Attempt{
			CodeHash: "hash-" + strings.Repeat("x", i), Code: "00000000",
			PhoneNumber: "+79990012345", PhoneRegistered: true,
		})
	}

	code, _ := MintRecoveryCode(ctx, store, "+79995550001", true)
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-other", Code: code,
		PhoneNumber: "+79995550001", PhoneRegistered: true,
	}); err != nil {
		t.Fatalf("a different number was locked out by somebody else's guessing: %v", err)
	}
}

// Getting in clears the count, or a person who mistypes a few times and then
// gets it right carries those failures into their next sign-in.
func TestGettingInForgivesTheMisses(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	const phone = "+79990012345"

	for i := 0; i < maxFailuresPerPhone-1; i++ {
		_ = v.VerifySmsCode(ctx, attempt.Attempt{
			CodeHash: "hash-" + strings.Repeat("x", i), Code: "00000000",
			PhoneNumber: phone, PhoneRegistered: true,
		})
	}

	code, _ := MintRecoveryCode(ctx, store, phone, true)
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-right", Code: code, PhoneNumber: phone, PhoneRegistered: true,
	}); err != nil {
		t.Fatalf("the right code after a few misses was refused: %v", err)
	}

	if value, _ := store.GetCtx(ctx, failuresKey(phone)); value != "" {
		t.Errorf("the misses were remembered after getting in: %q", value)
	}
}

// Ten wrong codes an hour against a hundred million of them is the whole
// argument, so it is worth failing loudly if either number moves.
func TestGuessingTakesLongerThanAnybodyHas(t *testing.T) {
	const space = 100_000_000 // eight digits

	yearsToHalf := float64(space) / 2 / float64(maxFailuresPerPhone) *
		float64(failureWindow) / 3600 / 24 / 365

	if yearsToHalf < 100 {
		t.Fatalf("half the codes fall in %.0f years at %d tries per %d seconds, "+
			"which is not long enough", yearsToHalf, maxFailuresPerPhone, failureWindow)
	}
}

func isWait(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FLOOD_WAIT")
}
