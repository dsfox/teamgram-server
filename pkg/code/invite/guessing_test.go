package invite

import (
	"context"
	"strconv"
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

	code, _ := MintRecoveryPhrase(ctx, store, "+79995550001", true)
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-other", Code: code,
		PhoneNumber: "+79995550001", PhoneRegistered: true,
	}); err != nil {
		t.Fatalf("a different number was locked out by somebody else's guessing: %v", err)
	}
}

// Getting in clears the count, or a person who mistypes a few times and then
// gets it right carries those failures into their next sign-in.
//
// Nine misses now owe time before the tenth attempt is looked at, right or
// wrong - that is the whole of what makes guessing expensive, and it is what
// the site promises. So the wait is served here first. It used to be that the
// right code was taken at once however many had come before it, which is a
// door that costs nothing to knock on.
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

	// Served. A test cannot sit out half a minute, and what is being checked
	// here is the forgiving, not the waiting - that is TestTheWaitDoubles.
	_, _ = store.DelCtx(ctx, waitKey(phone))

	code, _ := MintRecoveryPhrase(ctx, store, phone, true)
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-right", Code: code, PhoneNumber: phone, PhoneRegistered: true,
	}); err != nil {
		t.Fatalf("the right code after a few misses was refused: %v", err)
	}

	if value, _ := store.GetCtx(ctx, failuresKey(phone)); value != "" {
		t.Errorf("the misses were remembered after getting in: %q", value)
	}
}

// Ten wrong tries an hour against every phrase there is, so it is worth failing
// loudly if either number moves. The digits this replaced left a hundred million
// possibilities and about six hundred years; six words leave rather more.
func TestGuessingTakesLongerThanAnybodyHas(t *testing.T) {
	space := 1.0
	for i := 0; i < phraseWords; i++ {
		space *= float64(len(wordlist()))
	}

	yearsToHalf := space / 2 / float64(maxFailuresPerPhone) *
		float64(failureWindow) / 3600 / 24 / 365

	if yearsToHalf < 1e6 {
		t.Fatalf("half the phrases fall in %.3g years at %d tries per %d seconds, "+
			"which is not long enough", yearsToHalf, maxFailuresPerPhone, failureWindow)
	}
}

// Somebody working through many numbers from one place trips nothing per
// number and everything per address.
func TestSprayingManyNumbersFromOnePlaceRunsOut(t *testing.T) {
	v, _ := newTestVerifier()
	ctx := context.Background()
	const addr = "203.0.113.9"

	accepted := 0
	for i := 0; i < maxFailuresPerAddr*2; i++ {
		// A different number every time, so the per-number count never bites.
		err := v.VerifySmsCode(ctx, attempt.Attempt{
			CodeHash:    "hash-" + strconv.Itoa(i),
			Code:        "wrong wrong wrong wrong wrong wrong",
			PhoneNumber: "+7999" + strconv.Itoa(1000000+i),
			ClientAddr:  addr, PhoneRegistered: true,
		})
		if isWait(err) {
			break
		}
		accepted++
	}

	if accepted > maxFailuresPerAddr {
		t.Fatalf("%d numbers were tried from one address before it stopped, "+
			"expected at most %d", accepted, maxFailuresPerAddr)
	}
}

// And somebody guessing from one place must not shut out a person coming from
// somewhere else - an address is not a person, and whole regions share one.
func TestOneAddressGuessingDoesNotLockOutAnother(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()

	for i := 0; i < maxFailuresPerAddr*2; i++ {
		_ = v.VerifySmsCode(ctx, attempt.Attempt{
			CodeHash: "hash-" + strconv.Itoa(i), Code: "wrong phrase entirely here now",
			PhoneNumber: "+7999" + strconv.Itoa(1000000+i),
			ClientAddr:  "203.0.113.9", PhoneRegistered: true,
		})
	}

	phrase, _ := MintRecoveryPhrase(ctx, store, "+79995550001", true)
	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		CodeHash: "hash-elsewhere", Code: phrase,
		PhoneNumber: "+79995550001", ClientAddr: "198.51.100.4", PhoneRegistered: true,
	}); err != nil {
		t.Fatalf("somebody at another address was refused: %v", err)
	}
}

func isWait(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FLOOD_WAIT")
}
