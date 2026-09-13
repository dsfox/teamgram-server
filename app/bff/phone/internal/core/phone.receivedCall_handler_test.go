package core

import (
	"errors"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

// The callee's phone says "it is ringing here" - and both phones give up the
// call when this is refused: Android stops its service, iOS marks it missed.
// The caller is told, so its screen goes from waiting to ringing.
func TestTheCalleeSaysItIsRingingAndTheCallerHears(t *testing.T) {
	s := newStand(t)

	ok, err := s.as(bob).PhoneReceivedCall(&mtproto.TLPhoneReceivedCall{Peer: s.peer()})
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !mtproto.FromBool(ok) {
		t.Fatal("answered false")
	}
	if s.call.Received.IsZero() {
		t.Error("the call does not remember that it rang")
	}

	if len(s.sync.pushes) != 1 {
		t.Fatalf("%d pushes went out, expected one to the caller", len(s.sync.pushes))
	}
	push := s.sync.pushes[0]
	if push.GetUserId() != alice {
		t.Fatalf("told %d, expected the caller %d", push.GetUserId(), alice)
	}
	update := onlyUpdate(t, push)
	if update.GetPredicateName() != mtproto.Predicate_updatePhoneCall {
		t.Fatalf("the caller got %s", update.GetPredicateName())
	}
	pc := update.GetPhoneCall()
	if pc.GetPredicateName() != mtproto.Predicate_phoneCallWaiting {
		t.Fatalf("the caller got %s, expected phoneCallWaiting", pc.GetPredicateName())
	}
	if pc.GetId() != s.call.Id || pc.GetAccessHash() != s.call.AccessHash {
		t.Errorf("under call %d/%d, expected %d/%d", pc.GetId(), pc.GetAccessHash(), s.call.Id, s.call.AccessHash)
	}
	if pc.GetReceiveDate() == nil || pc.GetReceiveDate().GetValue() == 0 {
		t.Error("receive_date is not set, so the caller's screen stays at waiting")
	}
	if pc.GetProtocol() == nil {
		t.Error("the waiting call carries no protocol, which the client parser requires")
	}
}

func TestOnlyTheCalleeMaySayItIsRinging(t *testing.T) {
	for who, name := range map[int64]string{alice: "the caller", carol: "a stranger"} {
		t.Run(name, func(t *testing.T) {
			s := newStand(t)
			_, err := s.as(who).PhoneReceivedCall(&mtproto.TLPhoneReceivedCall{Peer: s.peer()})
			if !errors.Is(err, calls.ErrWrongParty) {
				t.Fatalf("%s was answered with %v", name, err)
			}
			if len(s.sync.pushes) != 0 {
				t.Fatalf("%d pushes went out", len(s.sync.pushes))
			}
		})
	}
}
