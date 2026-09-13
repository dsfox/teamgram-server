package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
)

// PhoneDiscardCall hangs up. Either side may; nobody else may.
//
// phone.discardCall video:flags.0?true peer:InputPhoneCall duration:int
//
//	reason:PhoneCallDiscardReason connection_id:long = Updates;
func (c *PhoneCore) PhoneDiscardCall(in *mtproto.TLPhoneDiscardCall) (*mtproto.Updates, error) {
	now := time.Now()
	call, err := c.find(in.GetPeer(), now)
	if err != nil {
		return nil, err
	}
	otherLeg, err := call.Other(c.MD.UserId)
	if err != nil {
		return nil, err
	}
	if err = call.Discard(c.MD.UserId, now); err != nil {
		c.Logger.Errorf("phone.discardCall - %d cannot discard %d: %v", c.MD.UserId, call.Id, err)
		return nil, err
	}

	discarded := mtproto.MakeTLPhoneCallDiscarded(&mtproto.PhoneCall{
		Id:       call.Id,
		Reason:   in.GetReason(),
		Duration: mtproto.MakeFlagsInt32(in.GetDuration()),
		Video:    in.GetVideo(),
	}).To_PhoneCall()

	// The other phone has to stop ringing, or stop talking.
	c.ring(otherLeg, discarded, now)

	return c.updatesFor(discarded, now), nil
}
