package core

import (
	"bytes"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

// The devices connected when the call is placed hear it over their session;
// the rest hear it from a push. The iPhone's push carries the call itself:
// the same update, and the caller as the callee sees them, in one TL Updates
// container the app parses at its own layer.
func TestPlacingACallPushesTheCallItselfToThePhones(t *testing.T) {
	s := newStand(t)
	s.svcCtx.Registry = calls.NewRegistry()
	s.sessions.permKeys[bob] = []int64{501, 502}

	reply, err := s.as(alice).PhoneRequestCall(&mtproto.TLPhoneRequestCall{
		UserId: inputUser(bob), RandomId: 7, GAHash: []byte("g_a_hash"), Protocol: protocol(), Video: true,
	})
	if err != nil {
		t.Fatalf("cannot place the call: %v", err)
	}
	call := s.svcCtx.Registry.RingingFor(bob, nowish())
	if call == nil {
		t.Fatal("the call is not ringing for bob")
	}
	if !call.WasRung(501) || !call.WasRung(502) || call.WasRung(503) {
		t.Errorf("the connected devices were not marked as rung: 501=%v 502=%v 503=%v", call.WasRung(501), call.WasRung(502), call.WasRung(503))
	}

	if len(s.ringer.rings) != 1 {
		t.Fatalf("%d pushes were asked for, expected one", len(s.ringer.rings))
	}
	r := s.ringer.rings[0]
	if r.callee != bob || r.caller != alice || r.callId != reply.GetPhoneCall().GetId() {
		t.Fatalf("pushed callee %d caller %d call %d", r.callee, r.caller, r.callId)
	}

	decoded := &mtproto.Updates{}
	if err := decoded.Decode(mtproto.NewDecodeBuf(r.updates)); err != nil {
		t.Fatalf("the iPhone cannot decode the blob: %v", err)
	}
	if decoded.GetPredicateName() != mtproto.Predicate_updates || len(decoded.GetUpdates()) != 1 {
		t.Fatalf("the blob is %s with %d updates", decoded.GetPredicateName(), len(decoded.GetUpdates()))
	}
	pc := decoded.GetUpdates()[0].GetPhoneCall()
	if decoded.GetUpdates()[0].GetPredicateName() != mtproto.Predicate_updatePhoneCall || pc.GetPredicateName() != mtproto.Predicate_phoneCallRequested {
		t.Fatalf("the blob carries %s / %s", decoded.GetUpdates()[0].GetPredicateName(), pc.GetPredicateName())
	}
	if pc.GetId() != call.Id || pc.GetAccessHash() != call.AccessHash || !bytes.Equal(pc.GetGAHash(), []byte("g_a_hash")) || !pc.GetVideo() {
		t.Errorf("the pushed call is %v", pc)
	}
	if len(decoded.GetUsers()) != 1 || decoded.GetUsers()[0].GetId() != alice || decoded.GetUsers()[0].GetFirstName().GetValue() != "Alice" {
		t.Errorf("the caller is not in the blob as the callee sees them: %v", decoded.GetUsers())
	}
	if len(r.updates) > 3000 {
		t.Errorf("the blob is %d bytes; sealed and base64'd it has to stay under the relay's 4096", len(r.updates))
	}
}

// A device that comes back while the call rings asks for its difference. If
// it was not among the connected ones it is rung now, over its own session,
// and only once; one that already heard the call is left alone - twice is
// "busy" on an Android.
func TestADeviceThatComesBackIsRungOnceOverItsSession(t *testing.T) {
	s := newStand(t)
	s.call.MarkRung(501)

	s.as(bob).RecallRinging(502)
	s.as(bob).RecallRinging(502)
	s.as(bob).RecallRinging(501)
	s.as(alice).RecallRinging(601)

	if len(s.sync.pushes) != 1 {
		t.Fatalf("%d pushes went out, expected exactly one to device 502", len(s.sync.pushes))
	}
	if len(s.sync.me) != 1 || s.sync.me[0].GetUserId() != bob || s.sync.me[0].GetPermAuthKeyId() != 502 {
		t.Fatalf("the ring went to %v", s.sync.me)
	}
	if s.sync.me[0].GetServerId().GetValue() != serverOf(bob) {
		t.Fatalf("the ring does not name the device's session server: %v", s.sync.me[0].GetServerId())
	}
	update := onlyUpdate(t, s.sync.pushes[0])
	if update.GetPhoneCall().GetPredicateName() != mtproto.Predicate_phoneCallRequested || update.GetPhoneCall().GetId() != s.call.Id {
		t.Fatalf("device 502 was sent %s for call %d", update.GetPhoneCall().GetPredicateName(), update.GetPhoneCall().GetId())
	}
	if !s.call.WasRung(502) {
		t.Error("device 502 is not marked as rung")
	}
}
