package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// A person's phones all ring; one answers. The others have to stop, and the
// only thing both clients take cleanly for that is a discarded call - a
// phoneCallAccepted reaching a phone that is still ringing goes down iOS's
// fallback branch. So the other devices are told the call is over, busy,
// and the device that answered is not.
func TestWhenOneDeviceAnswersTheOthersStopRinging(t *testing.T) {
	s := newStand(t)

	if _, err := s.as(bob).PhoneAcceptCall(&mtproto.TLPhoneAcceptCall{Peer: s.peer(), GB: []byte("g_b"), Protocol: protocol()}); err != nil {
		t.Fatalf("bob cannot accept: %v", err)
	}

	if len(s.sync.notMe) != 1 {
		t.Fatalf("%d pushes to the other devices, expected one", len(s.sync.notMe))
	}
	push := s.sync.notMe[0]
	if push.GetUserId() != bob || push.GetPermAuthKeyId() != deviceOf(bob) {
		t.Fatalf("addressed to %d except device %d", push.GetUserId(), push.GetPermAuthKeyId())
	}
	updates := push.GetUpdates()
	update := updates.GetUpdate()
	if update == nil && len(updates.GetUpdates()) == 1 {
		update = updates.GetUpdates()[0]
	}
	pc := update.GetPhoneCall()
	if pc.GetPredicateName() != mtproto.Predicate_phoneCallDiscarded || pc.GetId() != s.call.Id {
		t.Fatalf("the other devices were sent %s for call %d", pc.GetPredicateName(), pc.GetId())
	}
	if pc.GetReason().GetPredicateName() != mtproto.Predicate_phoneCallDiscardReasonBusy {
		t.Errorf("the reason is %s", pc.GetReason().GetPredicateName())
	}
}
