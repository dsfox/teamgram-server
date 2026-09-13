package calls

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestACallCanBeFoundByItsIdAndHash(t *testing.T) {
	r := NewRegistry()
	c, err := r.Place(caller, callee, []byte("g_a_hash"), t0)
	if err != nil {
		t.Fatalf("cannot place: %v", err)
	}
	found, err := r.Get(c.Id, c.AccessHash, t0)
	if err != nil {
		t.Fatalf("the call it just placed is not there: %v", err)
	}
	if found.Admin != caller || found.Participant != callee {
		t.Error("a different call came back")
	}
}

// The access hash is the capability: knowing an id is not knowing a call.
func TestAGuessedIdWithoutTheHashFindsNothing(t *testing.T) {
	r := NewRegistry()
	c, _ := r.Place(caller, callee, []byte("g_a_hash"), t0)
	if _, err := r.Get(c.Id, c.AccessHash+1, t0); !errors.Is(err, ErrNoCall) {
		t.Errorf("a wrong hash must not open the call, got %v", err)
	}
}

func TestAnUnknownCallIsNotFound(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Get(12345, 67890, t0); !errors.Is(err, ErrNoCall) {
		t.Errorf("nothing was placed, got %v", err)
	}
}

func TestSomeoneAlreadyInACallIsBusy(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Place(caller, callee, []byte("g_a_hash"), t0); err != nil {
		t.Fatalf("cannot place the first call: %v", err)
	}
	if _, err := r.Place(caller, other, []byte("g_a_hash"), t0); !errors.Is(err, ErrBusy) {
		t.Errorf("the caller is already in a call, got %v", err)
	}
	if _, err := r.Place(other, callee, []byte("g_a_hash"), t0); !errors.Is(err, ErrBusy) {
		t.Errorf("the one called is already ringing, got %v", err)
	}
	if _, err := r.Place(other, int64(4004), []byte("g_a_hash"), t0); err != nil {
		t.Errorf("two people not in any call may talk: %v", err)
	}
}

func TestAFinishedCallIsForgotten(t *testing.T) {
	r := NewRegistry()
	c, _ := r.Place(caller, callee, []byte("g_a_hash"), t0)
	if err := c.Discard(caller, t0); err != nil {
		t.Fatalf("cannot discard: %v", err)
	}
	r.Sweep(t0)
	if _, err := r.Get(c.Id, c.AccessHash, t0); !errors.Is(err, ErrNoCall) {
		t.Errorf("a call that ended should not be kept, got %v", err)
	}
	if _, err := r.Place(caller, callee, []byte("g_a_hash"), t0); err != nil {
		t.Errorf("with the old call gone they may call again: %v", err)
	}
}

func TestRingingThatGaveUpIsSweptAway(t *testing.T) {
	r := NewRegistry()
	c, _ := r.Place(caller, callee, []byte("g_a_hash"), t0)
	late := t0.Add(RingingFor + time.Second)
	if n := r.Sweep(late); n != 1 {
		t.Errorf("the unanswered call should have been swept, swept %d", n)
	}
	if _, err := r.Get(c.Id, c.AccessHash, late); !errors.Is(err, ErrNoCall) {
		t.Error("a call that gave up ringing is gone")
	}
	if _, err := r.Place(caller, callee, []byte("g_a_hash"), late); err != nil {
		t.Errorf("neither of them is busy any more: %v", err)
	}
}

// Placing a call is not a single-threaded act; the registry is shared.
func TestTheRegistryIsSafeToShare(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a, b := int64(10_000+i*2), int64(10_001+i*2)
			if c, err := r.Place(a, b, []byte("g_a_hash"), t0); err == nil {
				_, _ = r.Get(c.Id, c.AccessHash, t0)
			}
			r.Sweep(t0)
		}(i)
	}
	wg.Wait()
}

// A phone that was not connected when the call was placed asks for its
// difference once it wakes; the registry has to say whether a call is still
// ringing for that person - and only while it rings.
func TestARingingCallIsFoundByTheOneItRingsFor(t *testing.T) {
	r := NewRegistry()
	c, _ := r.Place(caller, callee, []byte("g_a_hash"), t0)

	if got := r.RingingFor(callee, t0); got != c {
		t.Fatalf("the callee's ringing call is %v, expected %v", got, c)
	}
	if got := r.RingingFor(caller, t0); got != nil {
		t.Errorf("the caller is not being rung, got %v", got)
	}
	if got := r.RingingFor(other, t0); got != nil {
		t.Errorf("a stranger has no ringing call, got %v", got)
	}
	if got := r.RingingFor(callee, t0.Add(RingingFor+time.Second)); got != nil {
		t.Errorf("a call that gave up still rings: %v", got)
	}

	c2, _ := r.Place(caller, callee, []byte("g_a_hash"), t0)
	if err := c2.Accept(callee, []byte("g_b"), t0); err != nil {
		t.Fatal(err)
	}
	if got := r.RingingFor(callee, t0); got != nil {
		t.Errorf("an answered call still rings: %v", got)
	}
}
