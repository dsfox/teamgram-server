package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// Every object a phone is shown about a call is built from the call by one
// set of views, so no view can forget a field the way two of them did (the
// video flag, on the accepted and the confirmed call). What every view
// carries is checked here, once, for all of them.
func TestEveryViewOfACallCarriesWhatThePhonesRead(t *testing.T) {
	s := newStand(t)
	s.call.Video = true
	if err := s.call.Receive(bob, nowish()); err != nil {
		t.Fatal(err)
	}
	if err := s.call.Accept(bob, []byte("g_b"), nowish()); err != nil {
		t.Fatal(err)
	}
	s.call.ParticipantProtocol = protocol()
	if err := s.call.Confirm(alice, []byte("g_a"), 0x0badcafe, nowish()); err != nil {
		t.Fatal(err)
	}
	c := s.as(alice)
	connections, err := c.connectionsFor(true, nowish())
	if err != nil {
		t.Fatal(err)
	}

	views := map[string]*mtproto.PhoneCall{
		"requested": c.requested(s.call),
		"waiting":   c.waiting(s.call, nowish()),
		"accepted":  c.accepted(s.call),
		"active":    c.active(s.call, connections, true, nowish()),
	}
	for name, pc := range views {
		if pc.GetId() != s.call.Id || pc.GetAccessHash() != s.call.AccessHash {
			t.Errorf("%s: id/hash %d/%d", name, pc.GetId(), pc.GetAccessHash())
		}
		if pc.GetAdminId() != alice || pc.GetParticipantId() != bob {
			t.Errorf("%s: %d calls %d", name, pc.GetAdminId(), pc.GetParticipantId())
		}
		if pc.GetProtocol() == nil {
			t.Errorf("%s: no protocol, which the client parser requires", name)
		}
		if !pc.GetVideo() {
			t.Errorf("%s: the video flag is gone", name)
		}
		if pc.GetDate() == 0 {
			t.Errorf("%s: no date", name)
		}
	}
	if string(views["requested"].GetGAHash()) != "g_a_hash" {
		t.Error("requested: no g_a_hash")
	}
	if views["waiting"].GetReceiveDate() == nil {
		t.Error("waiting: no receive_date once the callee's phone rang")
	}
	if string(views["accepted"].GetGB()) != "g_b" {
		t.Error("accepted: no g_b")
	}
	if a := views["active"]; string(a.GetGAOrB()) != "g_a" || a.GetKeyFingerprint() != 0x0badcafe || len(a.GetConnections()) == 0 || !a.GetP2PAllowed() || a.GetStartDate() == 0 {
		t.Errorf("active: %v", a)
	}

	ended := c.discarded(s.call, mtproto.MakeTLPhoneCallDiscardReasonHangup(nil).To_PhoneCallDiscardReason(), mtproto.MakeFlagsInt32(9))
	if ended.GetPredicateName() != mtproto.Predicate_phoneCallDiscarded || ended.GetId() != s.call.Id || !ended.GetVideo() || !ended.GetNeedDebug() || ended.GetDuration().GetValue() != 9 {
		t.Errorf("discarded: %v", ended)
	}
}
