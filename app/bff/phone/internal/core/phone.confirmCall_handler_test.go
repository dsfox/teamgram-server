package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// "Hide my IP": the P2P privacy setting, off by default. When either of the
// two chose it, the call is confirmed with p2p_allowed off and relay
// connections only - a STUN entry would hand the peer the very address the
// setting hides. Both directions matter: my IP is hidden from you when I say
// so, and yours from me when you do.
func TestHidingTheIPConfirmsWithRelayOnly(t *testing.T) {
	for _, hides := range []int64{bob, alice} {
		t.Run(map[int64]string{bob: "the callee hides", alice: "the caller hides"}[hides], func(t *testing.T) {
			s := newStand(t)
			s.users.hidesIP[hides] = true
			if _, err := s.as(bob).PhoneAcceptCall(&mtproto.TLPhoneAcceptCall{Peer: s.peer(), GB: []byte("g_b"), Protocol: protocol()}); err != nil {
				t.Fatal(err)
			}
			reply, err := s.as(alice).PhoneConfirmCall(&mtproto.TLPhoneConfirmCall{Peer: s.peer(), GA: []byte("g_a"), KeyFingerprint: 1, Protocol: protocol()})
			if err != nil {
				t.Fatalf("cannot confirm: %v", err)
			}
			for who, pc := range map[string]*mtproto.PhoneCall{"the reply": reply.GetPhoneCall(), "the push": s.lastCall(t, bob)} {
				if pc.GetP2PAllowed() {
					t.Errorf("%s allows p2p", who)
				}
				for _, conn := range pc.GetConnections() {
					if conn.GetStun() || !conn.GetTurn() {
						t.Errorf("%s hands out a direct path: %v", who, conn)
					}
				}
				if len(pc.GetConnections()) == 0 {
					t.Errorf("%s hands out nothing", who)
				}
			}
		})
	}
}
