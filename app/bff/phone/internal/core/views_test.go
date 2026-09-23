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

// Both phones run versions[0] of the confirmed call's protocol, so the server
// has to put one version there that both can run. It used to hand back the
// caller's whole list: an Android caller's begins with "10.0.0", which its own
// text comparison reads as older than 2.7.7, and it switched its camera off
// on every video call it placed (#186).
func TestTheConfirmedCallCarriesOneVersionBothPhonesRun(t *testing.T) {
	android := []string{"10.0.0", "11.0.0", "12.0.0", "13.0.0", "2.4.4", "2.7.7", "5.0.0", "7.0.0", "8.0.0", "9.0.0"}
	iphone := []string{"9.0.0", "8.0.0", "7.0.0", "5.0.0", "2.7.7", "14.0.0", "13.0.0", "12.0.0", "11.0.0", "10.0.0"}
	cases := []struct {
		name           string
		caller, callee []string
		want           []string
	}{
		{"android calls an iphone", android, iphone, []string{"9.0.0"}},
		{"iphone calls an android", iphone, android, []string{"9.0.0"}},
		{"iphone calls an iphone", iphone, iphone, []string{"9.0.0"}},
		{"nothing shared under the ceiling", []string{"10.0.0", "11.0.0"}, []string{"12.0.0", "11.0.0"}, []string{"11.0.0"}},
		{"nothing shared at all", []string{"4.0.0"}, []string{"5.0.0"}, []string{"4.0.0"}},
	}
	for _, tc := range cases {
		s := newStand(t)
		s.call.Protocol = versions(tc.caller)
		if err := s.call.Receive(bob, nowish()); err != nil {
			t.Fatal(err)
		}
		if err := s.call.Accept(bob, []byte("g_b"), nowish()); err != nil {
			t.Fatal(err)
		}
		s.call.ParticipantProtocol = versions(tc.callee)
		if err := s.call.Confirm(alice, []byte("g_a"), 0x0badcafe, nowish()); err != nil {
			t.Fatal(err)
		}
		active := s.as(alice).active(s.call, nil, true, nowish())
		if got := active.GetProtocol().GetLibraryVersions(); !equalStrings(got, tc.want) {
			t.Errorf("%s: the confirmed call carries %v, want %v", tc.name, got, tc.want)
		}
		if got := s.call.Protocol.GetLibraryVersions(); !equalStrings(got, tc.caller) {
			t.Errorf("%s: the caller's own protocol was changed to %v", tc.name, got)
		}
		if p := active.GetProtocol(); p.GetMinLayer() != 65 || p.GetMaxLayer() != 92 || !p.GetUdpP2P() || !p.GetUdpReflector() {
			t.Errorf("%s: the rest of the protocol was lost: %v", tc.name, p)
		}
	}
}

func versions(list []string) *mtproto.PhoneCallProtocol {
	return mtproto.MakeTLPhoneCallProtocol(&mtproto.PhoneCallProtocol{
		UdpP2P: true, UdpReflector: true, MinLayer: 65, MaxLayer: 92, LibraryVersions: list,
	}).To_PhoneCallProtocol()
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
