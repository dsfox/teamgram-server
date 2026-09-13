package calls

import (
	"errors"
	"testing"
	"time"
)

// A call is two people and nobody else. Every rule below is about who may move
// it and in what order - the server carries the key exchange, it never performs
// it, and it must never let a third party touch a call they are not in.

const (
	caller = int64(1001)
	callee = int64(2002)
	other  = int64(3003)
)

var t0 = time.Unix(1_700_000_000, 0)

func placed(t *testing.T) *Call {
	t.Helper()
	c, err := Request(caller, callee, []byte("g_a_hash"), t0)
	if err != nil {
		t.Fatalf("cannot place a call: %v", err)
	}
	return c
}

func accepted(t *testing.T) *Call {
	t.Helper()
	c := placed(t)
	if err := c.Accept(callee, []byte("g_b"), t0); err != nil {
		t.Fatalf("cannot accept: %v", err)
	}
	return c
}

func TestACallStartsRingingAtTheOneCalled(t *testing.T) {
	c := placed(t)
	if c.State != Waiting {
		t.Errorf("a placed call should be waiting, got %v", c.State)
	}
	if c.Admin != caller || c.Participant != callee {
		t.Error("the call must remember who placed it and who was called")
	}
}

func TestCallingYourselfIsRefused(t *testing.T) {
	if _, err := Request(caller, caller, []byte("g_a_hash"), t0); err == nil {
		t.Error("a call to oneself has no other side")
	}
}

func TestOnlyTheOneCalledMayAccept(t *testing.T) {
	for who, name := range map[int64]string{caller: "the caller", other: "a stranger"} {
		c := placed(t)
		if err := c.Accept(who, []byte("g_b"), t0); !errors.Is(err, ErrWrongParty) {
			t.Errorf("%s must not be able to accept the call, got %v", name, err)
		}
	}
}

func TestOnlyTheCallerMayConfirm(t *testing.T) {
	for who, name := range map[int64]string{callee: "the one called", other: "a stranger"} {
		c := accepted(t)
		if err := c.Confirm(who, []byte("g_a"), 42, t0); !errors.Is(err, ErrWrongParty) {
			t.Errorf("%s must not be able to confirm the call, got %v", name, err)
		}
	}
}

func TestConfirmBeforeAcceptIsRefused(t *testing.T) {
	c := placed(t)
	if err := c.Confirm(caller, []byte("g_a"), 42, t0); !errors.Is(err, ErrWrongState) {
		t.Errorf("there is no g_b to confirm against yet, got %v", err)
	}
}

func TestAcceptingTwiceIsRefused(t *testing.T) {
	c := accepted(t)
	if err := c.Accept(callee, []byte("g_b again"), t0); !errors.Is(err, ErrWrongState) {
		t.Errorf("a second acceptance would replace the key material, got %v", err)
	}
}

func TestEitherPartyMayHangUpButAStrangerMayNot(t *testing.T) {
	for _, who := range []int64{caller, callee} {
		c := accepted(t)
		if err := c.Discard(who, t0); err != nil {
			t.Errorf("%d is in the call and must be able to hang up: %v", who, err)
		}
		if c.State != Discarded {
			t.Error("hanging up must end the call")
		}
	}
	c := accepted(t)
	if err := c.Discard(other, t0); !errors.Is(err, ErrWrongParty) {
		t.Errorf("a stranger must not be able to hang up someone else's call, got %v", err)
	}
}

func TestADiscardedCallIsFinal(t *testing.T) {
	c := placed(t)
	if err := c.Discard(callee, t0); err != nil {
		t.Fatalf("cannot discard: %v", err)
	}
	if err := c.Accept(callee, []byte("g_b"), t0); !errors.Is(err, ErrWrongState) {
		t.Errorf("a call that ended cannot be answered, got %v", err)
	}
	if err := c.Confirm(caller, []byte("g_a"), 42, t0); !errors.Is(err, ErrWrongState) {
		t.Errorf("a call that ended cannot be confirmed, got %v", err)
	}
}

// This is the guard on sendSignalingData: a candidate blob goes to the other
// leg of the call and nowhere else.
func TestSignallingGoesToTheOtherLegAndOnlyBetweenTheTwo(t *testing.T) {
	c := accepted(t)
	if to, err := c.Other(caller); err != nil || to != callee {
		t.Errorf("from the caller it must reach the one called, got %d %v", to, err)
	}
	if to, err := c.Other(callee); err != nil || to != caller {
		t.Errorf("from the one called it must reach the caller, got %d %v", to, err)
	}
	if _, err := c.Other(other); !errors.Is(err, ErrWrongParty) {
		t.Errorf("a stranger has no other leg here, got %v", err)
	}
}

// The server is a courier for the key exchange, not a party to it.
func TestTheKeyExchangeIsCarriedNotComputed(t *testing.T) {
	c := accepted(t)
	if err := c.Confirm(caller, []byte("g_a"), 42, t0); err != nil {
		t.Fatalf("cannot confirm: %v", err)
	}
	if string(c.GAHash) != "g_a_hash" || string(c.GB) != "g_b" || string(c.GA) != "g_a" {
		t.Error("the blobs must survive untouched - the phones derive the key from them")
	}
	if c.KeyFingerprint != 42 {
		t.Error("the fingerprint the two sides compare must be carried as given")
	}
	if c.State != Active {
		t.Errorf("after confirmation the call is live, got %v", c.State)
	}
}

func TestAnUnansweredCallExpires(t *testing.T) {
	c := placed(t)
	if c.Expired(t0.Add(30 * time.Second)) {
		t.Error("half a minute of ringing is not an expiry")
	}
	late := t0.Add(RingingFor + time.Second)
	if !c.Expired(late) {
		t.Error("a call nobody answered must not ring for ever")
	}
	if err := c.Accept(callee, []byte("g_b"), late); !errors.Is(err, ErrWrongState) {
		t.Errorf("answering after the ringing gave up must be refused, got %v", err)
	}
}

func TestAnActiveCallDoesNotExpireWhileItIsSpoken(t *testing.T) {
	c := accepted(t)
	if err := c.Confirm(caller, []byte("g_a"), 42, t0); err != nil {
		t.Fatalf("cannot confirm: %v", err)
	}
	if c.Expired(t0.Add(RingingFor + time.Hour)) {
		t.Error("a call in progress is not an unanswered one")
	}
}
