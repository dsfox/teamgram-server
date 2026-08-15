package invite

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/teamgram/teamgram-server/pkg/code/attempt"
)

// Six words are 2^66, which no person will ever type through and a machine does
// not type at all. The only thing that makes that number matter is charging for
// every attempt, so the wait doubles: the tenth guess costs more than the first
// nine together, and somebody who mistyped their own phrase twice waits two
// seconds and never notices.
func TestTheWaitDoublesWithEachWrongPhrase(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	phone := "79995550001"

	waits := []int{}
	// A fresh code hash each time: three attempts on one hash are spent by
	// design, and the fourth would be refused for that instead of counted.
	for i := 1; i <= freeMisses+6; i++ {
		// Pretend the previous wait has been served, or the next attempt is
		// refused for owing time rather than counted.
		delete(store.data, waitKey(phone))
		if err := v.VerifySmsCode(ctx, attempt.Attempt{
			PhoneNumber: phone, CodeHash: "hash" + strconv.Itoa(i),
			Code: "wrong", Generated: "48210",
		}); err == nil {
			t.Fatal("a wrong code was accepted")
		}
		if i <= freeMisses {
			if store.data[waitKey(phone)] != "" {
				t.Fatalf("miss %d was charged for, and the first %d are free", i, freeMisses)
			}
			continue
		}
		until, _ := strconv.ParseInt(store.data[waitKey(phone)], 10, 64)
		waits = append(waits, int(until-time.Now().Unix()))
	}

	for i := 1; i < len(waits); i++ {
		if waits[i] < waits[i-1]*2 {
			t.Fatalf("the wait did not double: %v", waits)
		}
	}
	if waits[len(waits)-1] < 30 {
		t.Fatalf("the ninth wrong phrase cost only %ds: %v", waits[len(waits)-1], waits)
	}
}

// And the wait is a refusal, not a delay somebody can talk over: while it is
// owed, even the right phrase is not looked at.
func TestWhatIsOwedIsRefusedBeforeAnythingIsCompared(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	phone := "79995550002"

	store.data[waitKey(phone)] = strconv.FormatInt(time.Now().Unix()+30, 10)
	err := v.VerifySmsCode(ctx, attempt.Attempt{
		PhoneNumber: phone, CodeHash: "hash", Code: "48210", Generated: "48210",
	})
	if err == nil {
		t.Fatal("a number that owes time was let in")
	}
	if !strings.Contains(err.Error(), "FLOOD_WAIT") {
		t.Fatalf("refused, but not with a wait: %v", err)
	}
}

// The wait lives in the store, which survives a restart of the server. A
// counter held in memory is an invitation to knock the server over between
// guesses, and that is cheaper than guessing.
func TestTheWaitIsNotHeldInMemory(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	phone := "79995550003"

	for i := 0; i <= freeMisses; i++ {
		delete(store.data, waitKey(phone))
		_ = v.VerifySmsCode(ctx, attempt.Attempt{
			PhoneNumber: phone, CodeHash: "hash" + strconv.Itoa(i),
			Code: "wrong", Generated: "48210",
		})
	}

	// A new verifier is a restarted server; the store is what it comes back to.
	restarted := &verifier{store: store}
	if restarted.waiting(ctx, phone) <= 0 {
		t.Fatal("the wait was forgotten when the server restarted")
	}
}

// And it is forgiven the moment somebody proves the number is theirs, so a
// person who finally remembers their phrase is not made to sit out a punishment
// meant for whoever was guessing.
func TestGettingItRightClearsWhatIsOwed(t *testing.T) {
	v, store := newTestVerifier()
	ctx := context.Background()
	phone := "79995550004"

	for i := 0; i <= freeMisses; i++ {
		delete(store.data, waitKey(phone))
		_ = v.VerifySmsCode(ctx, attempt.Attempt{
			PhoneNumber: phone, CodeHash: "miss" + strconv.Itoa(i),
			Code: "wrong", Generated: "48210",
		})
	}
	delete(store.data, waitKey(phone))

	if err := v.VerifySmsCode(ctx, attempt.Attempt{
		PhoneNumber: phone, CodeHash: "hash2", Code: "48210", Generated: "48210",
	}); err != nil {
		t.Fatalf("the right code was refused: %v", err)
	}
	if v.waiting(ctx, phone) != 0 || store.data[failuresKey(phone)] != "" {
		t.Fatal("what was owed survived a correct sign-in")
	}
}
