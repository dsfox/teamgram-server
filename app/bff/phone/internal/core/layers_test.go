package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// Every object a call pushes has to encode at the layer each phone speaks -
// iOS 228, Android 229 - or the session layer drops the push without a word
// and the phone waits for ever. Seen live: the callee got the signalling
// blobs and the hang-up, and never the confirmed call.
func TestEveryCallUpdateEncodesAtBothPhonesLayers(t *testing.T) {
	s := newStand(t)
	if err := s.call.Accept(bob, []byte("g_b"), nowish()); err != nil {
		t.Fatal(err)
	}
	if err := s.call.Confirm(alice, []byte("g_a"), 0x0badcafe, nowish()); err != nil {
		t.Fatal(err)
	}
	c := s.as(alice)
	connections, err := c.connectionsFor(true, nowish())
	if err != nil {
		t.Fatal(err)
	}
	active := mtproto.MakeTLPhoneCall(&mtproto.PhoneCall{
		Id: s.call.Id, AccessHash: s.call.AccessHash, Date: 1, AdminId: alice, ParticipantId: bob,
		GAOrB: s.call.GA, KeyFingerprint: s.call.KeyFingerprint, Protocol: protocol(),
		Connections: connections, StartDate: 1,
	}).To_PhoneCall()

	objects := map[string]*mtproto.Updates{
		"requested":  c.updatesFor(c.requested(s.call), nowish()),
		"waiting":    c.updatesFor(c.waiting(s.call, nowish()), nowish()),
		"active":     c.updatesFor(active, nowish()),
		"signalling": mtproto.MakeTLUpdateShort(&mtproto.Updates{Update: mtproto.MakeTLUpdatePhoneCallSignalingData(&mtproto.Update{PhoneCallId: 1, Data_FLAGBYTES: []byte("x")}).To_Update(), Date: 1}).To_Updates(),
	}
	for _, layer := range []int32{228, 229} {
		for name, u := range objects {
			buf := mtproto.NewEncodeBuf(2048)
			if err := u.Encode(buf, layer); err != nil {
				t.Errorf("layer %d: %s does not encode: %v", layer, name, err)
			} else if len(buf.GetBuf()) < 8 {
				t.Errorf("layer %d: %s encoded to %d bytes", layer, name, len(buf.GetBuf()))
			}
		}
	}
}
