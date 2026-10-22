package core

import (
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

// PhoneSaveCallDebug takes a phone's stats log after a call and keeps two
// things: whether the media went device to device or through the relay, and
// what the phone did with sound (#288). Log lines per phone, nothing stored - the health
// check and a scenario read the line. Answered true whatever the log looks
// like: it is data, and the phone does not resend it.
//
// phone.saveCallDebug peer:InputPhoneCall debug:DataJSON = Bool;
func (c *PhoneCore) PhoneSaveCallDebug(in *mtproto.TLPhoneSaveCallDebug) (*mtproto.Bool, error) {
	data := in.GetDebug().GetData()
	if route, ok := calls.RouteOf([]byte(data)); ok {
		relayed := "direct"
		if route.Relayed() {
			relayed = "relayed"
		}
		c.Logger.Infof("call route: call %d, user %d: %s (%s)", in.GetPeer().GetId(), c.MD.UserId, relayed, route)
	} else {
		c.Logger.Infof("call route: call %d, user %d: no connected route in %d bytes of log", in.GetPeer().GetId(), c.MD.UserId, len(data))
	}
	// What the phone did with sound (#288): who was not heard is in here,
	// along with the client that said it.
	if audio, ok := calls.AudioOf([]byte(data)); ok {
		c.Logger.Infof("call audio: call %d, user %d, client %q layer %d: %s", in.GetPeer().GetId(), c.MD.UserId, c.MD.Client, c.MD.Layer, audio)
	}
	return mtproto.BoolTrue, nil
}
