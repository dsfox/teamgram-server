package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
)

// PhoneReceivedCall is the callee's phone saying it is ringing.
//
// phone.receivedCall peer:InputPhoneCall = Bool;
//
// Both phones give the call up when this is refused - Android stops its
// service, iOS marks the call missed - so it has to succeed for anyone the
// state machine lets in. The caller is shown the same waiting call again, now
// with receive_date, and its screen goes from waiting to ringing.
func (c *PhoneCore) PhoneReceivedCall(in *mtproto.TLPhoneReceivedCall) (*mtproto.Bool, error) {
	now := time.Now()
	call, err := c.find(in.GetPeer(), now)
	if err != nil {
		return nil, rpcError(err, mtproto.ErrCallAlreadyAccepted)
	}
	if err = call.Receive(c.MD.UserId, now); err != nil {
		c.Logger.Errorf("phone.receivedCall - %d cannot acknowledge %d: %v", c.MD.UserId, call.Id, err)
		return nil, rpcError(err, mtproto.ErrCallAlreadyAccepted)
	}

	c.ring(call.Admin, c.waiting(call, now), now)

	return mtproto.BoolTrue, nil
}
