package core

import (
	"bytes"
	"errors"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

func inputUser(id int64) *mtproto.InputUser {
	return mtproto.MakeTLInputUser(&mtproto.InputUser{UserId: id, AccessHash: 0}).To_InputUser()
}

func protocol() *mtproto.PhoneCallProtocol {
	return mtproto.MakeTLPhoneCallProtocol(&mtproto.PhoneCallProtocol{
		UdpP2P: true, UdpReflector: true, MinLayer: 65, MaxLayer: 92, LibraryVersions: []string{"4.0.0"},
	}).To_PhoneCallProtocol()
}

// lastCall is the phone call inside the newest push to one person.
func (s *stand) lastCall(t *testing.T, to int64) *mtproto.PhoneCall {
	t.Helper()
	for i := len(s.sync.pushes) - 1; i >= 0; i-- {
		if s.sync.pushes[i].GetUserId() == to {
			update := onlyUpdate(t, s.sync.pushes[i])
			if update.GetPredicateName() != mtproto.Predicate_updatePhoneCall {
				t.Fatalf("%d was last sent %s, not a phone call", to, update.GetPredicateName())
			}
			return update.GetPhoneCall()
		}
	}
	t.Fatalf("nothing was ever pushed to %d", to)
	return nil
}

// The whole of a call, as the server sees it: the key exchange carried across
// unread, each step told to exactly the other phone, the connections handed
// out with STUN first.
func TestACallFromRingToHangUp(t *testing.T) {
	s := newStand(t)
	// The stand comes with a call already placed; this test places its own.
	s.svcCtx.Registry = calls.NewRegistry()
	s.sync.pushes = nil

	// alice calls bob: bob rings, alice gets the waiting call back.
	reply, err := s.as(alice).PhoneRequestCall(&mtproto.TLPhoneRequestCall{
		UserId: inputUser(bob), RandomId: 7, GAHash: []byte("g_a_hash"), Protocol: protocol(),
	})
	if err != nil {
		t.Fatalf("cannot place the call: %v", err)
	}
	waiting := reply.GetPhoneCall()
	if waiting.GetPredicateName() != mtproto.Predicate_phoneCallWaiting {
		t.Fatalf("the caller was handed %s", waiting.GetPredicateName())
	}
	peer := mtproto.MakeTLInputPhoneCall(&mtproto.InputPhoneCall{
		Id: waiting.GetId(), AccessHash: waiting.GetAccessHash(),
	}).To_InputPhoneCall()
	// Both phones start ringing only on phoneCallRequested - a waiting call
	// with no session behind it is dropped on the floor by either client -
	// and the callee needs g_a_hash from it to check g_a later.
	rang := s.lastCall(t, bob)
	if rang.GetPredicateName() != mtproto.Predicate_phoneCallRequested || rang.GetId() != waiting.GetId() {
		t.Fatalf("bob was sent %s for call %d", rang.GetPredicateName(), rang.GetId())
	}
	if rang.GetAdminId() != alice || rang.GetParticipantId() != bob {
		t.Errorf("bob's ring says %d calls %d", rang.GetAdminId(), rang.GetParticipantId())
	}
	if !bytes.Equal(rang.GetGAHash(), []byte("g_a_hash")) || rang.GetProtocol() == nil || rang.GetAccessHash() != waiting.GetAccessHash() {
		t.Errorf("bob's ring carries g_a_hash %q, protocol %v, access hash %d", rang.GetGAHash(), rang.GetProtocol(), rang.GetAccessHash())
	}

	// While it rings, neither of them can be called or call.
	if _, err := s.as(carol).PhoneRequestCall(&mtproto.TLPhoneRequestCall{
		UserId: inputUser(bob), GAHash: []byte("x"), Protocol: protocol(),
	}); !errors.Is(err, mtproto.ErrCallOccupyFailed) {
		t.Errorf("a second call to a ringing phone was answered with %v", err)
	}

	// bob answers with g_b: alice gets it.
	if _, err := s.as(bob).PhoneAcceptCall(&mtproto.TLPhoneAcceptCall{
		Peer: peer, GB: []byte("g_b"), Protocol: protocol(),
	}); err != nil {
		t.Fatalf("bob cannot accept: %v", err)
	}
	accepted := s.lastCall(t, alice)
	if accepted.GetPredicateName() != mtproto.Predicate_phoneCallAccepted || !bytes.Equal(accepted.GetGB(), []byte("g_b")) {
		t.Fatalf("alice was sent %s with g_b %q", accepted.GetPredicateName(), accepted.GetGB())
	}
	// From here on the two devices in the call are known, and what they
	// trade goes to them by key: a push by user looks the session up in the
	// status list, which a phone woken a moment ago is not yet in - seen live,
	// the confirmed call never reached the callee that way.
	if n := len(s.sync.me); n != 1 || toDevice(s.sync.me[0]) != [2]int64{alice, deviceOf(alice)} {
		t.Fatalf("the answer was not addressed to the caller's device: %d addressed pushes, %v", n, s.sync.me)
	}

	// alice confirms with g_a and the fingerprint: bob gets both, and both
	// get the connections, STUN before any relay.
	confirmed, err := s.as(alice).PhoneConfirmCall(&mtproto.TLPhoneConfirmCall{
		Peer: peer, GA: []byte("g_a"), KeyFingerprint: 0x0badcafe, Protocol: protocol(),
	})
	if err != nil {
		t.Fatalf("alice cannot confirm: %v", err)
	}
	active := s.lastCall(t, bob)
	for who, pc := range map[string]*mtproto.PhoneCall{"alice's reply": confirmed.GetPhoneCall(), "bob's push": active} {
		if pc.GetPredicateName() != mtproto.Predicate_phoneCall {
			t.Fatalf("%s is %s, expected phoneCall", who, pc.GetPredicateName())
		}
		if !bytes.Equal(pc.GetGAOrB(), []byte("g_a")) || pc.GetKeyFingerprint() != 0x0badcafe {
			t.Errorf("%s carries g_a %q and fingerprint %x", who, pc.GetGAOrB(), pc.GetKeyFingerprint())
		}
		// The flag both engines read before they gather a single host or
		// server-reflexive candidate: without it they wait for a relay, and
		// with none configured the call fails after a timeout - seen live.
		if !pc.GetP2PAllowed() {
			t.Errorf("%s does not allow p2p, so the phones would never try a direct path", who)
		}
		conns := pc.GetConnections()
		if len(conns) != 2 || !conns[0].GetStun() || !conns[1].GetTurn() {
			t.Fatalf("%s lists %d connections, wanted STUN then relay: %v", who, len(conns), conns)
		}
		if conns[0].GetIpv6() == "" || conns[1].GetUsername() == "" || conns[1].GetPassword() == "" {
			t.Errorf("%s: STUN without IPv6 or relay without credentials", who)
		}
	}

	if n := len(s.sync.me); n != 2 || toDevice(s.sync.me[1]) != [2]int64{bob, deviceOf(bob)} {
		t.Fatalf("the confirmed call was not addressed to the device that answered: %v", s.sync.me)
	}

	// Candidates go device to device, both ways.
	if _, err := s.as(alice).PhoneSendSignalingData(signalling(peer, []byte("from alice"))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.as(bob).PhoneSendSignalingData(signalling(peer, []byte("from bob"))); err != nil {
		t.Fatal(err)
	}
	if n := len(s.sync.me); n != 4 || toDevice(s.sync.me[2]) != [2]int64{bob, deviceOf(bob)} || toDevice(s.sync.me[3]) != [2]int64{alice, deviceOf(alice)} {
		t.Fatalf("candidates were not addressed to the two devices: %v", s.sync.me)
	}

	// bob hangs up: alice is told, and the call is gone.
	if _, err := s.as(bob).PhoneDiscardCall(&mtproto.TLPhoneDiscardCall{
		Peer: peer, Duration: 42, Reason: mtproto.MakeTLPhoneCallDiscardReasonHangup(nil).To_PhoneCallDiscardReason(),
	}); err != nil {
		t.Fatalf("bob cannot hang up: %v", err)
	}
	ended := s.lastCall(t, alice)
	if ended.GetPredicateName() != mtproto.Predicate_phoneCallDiscarded || ended.GetId() != waiting.GetId() {
		t.Fatalf("alice was sent %s for call %d", ended.GetPredicateName(), ended.GetId())
	}
	if _, err := s.as(alice).PhoneSendSignalingData(signalling(peer, []byte("late"))); !errors.Is(err, mtproto.ErrCallPeerInvalid) {
		t.Errorf("the call is still findable after hang-up: %v", err)
	}

	// And nobody outside the two was ever told anything.
	for _, push := range s.sync.pushes {
		if push.GetUserId() != alice && push.GetUserId() != bob {
			t.Errorf("%d was told about a call they are not in", push.GetUserId())
		}
	}
}

// The state machine's refusals reach the handlers as the same errors, so a
// client sees the right one - and nothing is pushed on a refusal.
func TestTheHandlersRefuseWhatTheCallRefuses(t *testing.T) {
	s := newStand(t)

	if _, err := s.as(alice).PhoneAcceptCall(&mtproto.TLPhoneAcceptCall{Peer: s.peer(), GB: []byte("g_b"), Protocol: protocol()}); !errors.Is(err, mtproto.ErrCallPeerInvalid) {
		t.Errorf("the caller accepting their own call: %v", err)
	}
	if _, err := s.as(alice).PhoneConfirmCall(&mtproto.TLPhoneConfirmCall{Peer: s.peer(), GA: []byte("g_a"), Protocol: protocol()}); !errors.Is(err, mtproto.ErrCallPeerInvalid) {
		t.Errorf("confirming before the answer: %v", err)
	}
	if _, err := s.as(carol).PhoneDiscardCall(&mtproto.TLPhoneDiscardCall{Peer: s.peer()}); !errors.Is(err, mtproto.ErrCallPeerInvalid) {
		t.Errorf("a stranger hanging up: %v", err)
	}
	if len(s.sync.pushes) != 0 {
		t.Fatalf("%d pushes went out on refusals", len(s.sync.pushes))
	}
}
