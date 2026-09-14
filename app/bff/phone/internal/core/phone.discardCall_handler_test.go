package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// The calls list on both phones is the chat's service messages with a
// messageActionPhoneCall, found by the phone-calls filter; without one the
// list stays empty - seen on both phones after the first real calls. Every
// call that ends leaves one, from the caller, in the chat of the two: the
// reason as the phone said it, the duration once it was spoken, and "missed"
// when the caller gave up before anyone answered.
func TestAFinishedCallLeavesItsEntryInTheChat(t *testing.T) {
	s := newStand(t)
	if _, err := s.as(bob).PhoneAcceptCall(&mtproto.TLPhoneAcceptCall{Peer: s.peer(), GB: []byte("g_b"), Protocol: protocol()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.as(alice).PhoneConfirmCall(&mtproto.TLPhoneConfirmCall{Peer: s.peer(), GA: []byte("g_a"), KeyFingerprint: 1, Protocol: protocol()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.as(bob).PhoneDiscardCall(&mtproto.TLPhoneDiscardCall{
		Peer: s.peer(), Duration: 42, Reason: mtproto.MakeTLPhoneCallDiscardReasonHangup(nil).To_PhoneCallDiscardReason(),
	}); err != nil {
		t.Fatal(err)
	}

	if len(s.msg.sent) != 1 {
		t.Fatalf("%d messages were sent, expected the call's one", len(s.msg.sent))
	}
	sent := s.msg.sent[0]
	if sent.GetUserId() != alice || sent.GetAuthKeyId() != deviceOf(alice) || sent.GetPeerType() != mtproto.PEER_USER || sent.GetPeerId() != bob {
		t.Fatalf("sent as %d (device %d) to peer %d/%d, expected from the caller to the callee", sent.GetUserId(), sent.GetAuthKeyId(), sent.GetPeerType(), sent.GetPeerId())
	}
	if len(sent.GetMessage()) != 1 {
		t.Fatalf("%d messages in the send", len(sent.GetMessage()))
	}
	m := sent.GetMessage()[0].GetMessage()
	if m.GetPredicateName() != mtproto.Predicate_messageService || !m.GetOut() || m.GetFromId().GetUserId() != alice || m.GetPeerId().GetUserId() != bob {
		t.Fatalf("the entry is %s out=%v from %v to %v", m.GetPredicateName(), m.GetOut(), m.GetFromId(), m.GetPeerId())
	}
	action := m.GetAction()
	if action.GetPredicateName() != mtproto.Predicate_messageActionPhoneCall || action.GetCallId() != s.call.Id {
		t.Fatalf("the action is %s for call %d", action.GetPredicateName(), action.GetCallId())
	}
	if action.GetReason().GetPredicateName() != mtproto.Predicate_phoneCallDiscardReasonHangup || action.GetDuration().GetValue() != 42 {
		t.Errorf("reason %s, duration %v", action.GetReason().GetPredicateName(), action.GetDuration())
	}
}

func TestACallNobodyAnsweredIsAMissedCall(t *testing.T) {
	s := newStand(t)
	if _, err := s.as(alice).PhoneDiscardCall(&mtproto.TLPhoneDiscardCall{
		Peer: s.peer(), Reason: mtproto.MakeTLPhoneCallDiscardReasonHangup(nil).To_PhoneCallDiscardReason(),
	}); err != nil {
		t.Fatal(err)
	}
	if len(s.msg.sent) != 1 {
		t.Fatalf("%d messages were sent", len(s.msg.sent))
	}
	action := s.msg.sent[0].GetMessage()[0].GetMessage().GetAction()
	if action.GetReason().GetPredicateName() != mtproto.Predicate_phoneCallDiscardReasonMissed {
		t.Errorf("the caller giving up before an answer is %s, expected missed", action.GetReason().GetPredicateName())
	}
	if action.GetDuration() != nil {
		t.Errorf("a call never spoken has a duration: %v", action.GetDuration())
	}
}

func TestADeclinedCallKeepsThePhonesReason(t *testing.T) {
	s := newStand(t)
	if _, err := s.as(bob).PhoneDiscardCall(&mtproto.TLPhoneDiscardCall{
		Peer: s.peer(), Reason: mtproto.MakeTLPhoneCallDiscardReasonBusy(nil).To_PhoneCallDiscardReason(),
	}); err != nil {
		t.Fatal(err)
	}
	action := s.msg.sent[0].GetMessage()[0].GetMessage().GetAction()
	if action.GetReason().GetPredicateName() != mtproto.Predicate_phoneCallDiscardReasonBusy {
		t.Errorf("a decline became %s", action.GetReason().GetPredicateName())
	}
}
