package devices

import "testing"

// A call rings an iPhone through its VoIP token and a message never may: a
// VoIP push that carries no call gets the app killed by iOS. So the VoIP kind
// is known, and kept out of Reachable(), which is what message pushes use.
func TestAVoipTokenIsForCallsAndNotForMessages(t *testing.T) {
	voip := DeviceDO{TokenType: TokenTypeAPNsVoIP, Token: "abc"}
	if !voip.IsVoIP() {
		t.Error("a type-9 token is not recognised as VoIP")
	}
	if voip.IsAPNs() || voip.Reachable() {
		t.Error("a VoIP token counts as a place to send a message")
	}

	for _, d := range []DeviceDO{
		{TokenType: TokenTypeAPNs, Token: "abc"},
		{TokenType: TokenTypeFCM, Token: "abc"},
		{TokenType: TokenTypeAPNsVoIP, Token: ""},
	} {
		if d.IsVoIP() {
			t.Errorf("%+v passes for a VoIP token", d)
		}
	}
}
