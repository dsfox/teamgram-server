package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// The connectivity metric (plan, Part 6): after a call each phone is asked
// for its stats log - need_debug on the hang-up - and what it says about the
// route goes to the server's log, one line per phone. The phone is answered
// true whatever the log looks like: it is data, and it is not resent.
func TestAHangUpAsksForTheStatsLogAndTheLogIsTaken(t *testing.T) {
	s := newStand(t)
	reply, err := s.as(bob).PhoneDiscardCall(&mtproto.TLPhoneDiscardCall{Peer: s.peer()})
	if err != nil {
		t.Fatal(err)
	}
	ended := reply.GetUpdate().GetPhoneCall()
	if !ended.GetNeedDebug() {
		t.Error("the one who hung up is not asked for the stats log")
	}
	if pushed := s.lastCall(t, alice); !pushed.GetNeedDebug() {
		t.Error("the other phone is not asked for the stats log")
	}
	pushesAfterHangUp := len(s.sync.pushes)

	for name, log := range map[string]string{
		"a stats log": `{"network":[{"t":"1","c":1,"local":"p2p","remote":"p2p","network":{"local":{"type":"host"},"remote":{"type":"srflx"}}}]}`,
		"not a log":   `route changed`,
		"nothing":     ``,
	} {
		ok, err := s.as(alice).PhoneSaveCallDebug(&mtproto.TLPhoneSaveCallDebug{
			Peer:  s.peer(),
			Debug: mtproto.MakeTLDataJSON(&mtproto.DataJSON{Data: log}).To_DataJSON(),
		})
		if err != nil || !mtproto.FromBool(ok) {
			t.Errorf("%s: answered %v, %v", name, ok, err)
		}
	}
	if len(s.sync.pushes) != pushesAfterHangUp {
		t.Errorf("taking the logs pushed %d updates", len(s.sync.pushes)-pushesAfterHangUp)
	}
}
