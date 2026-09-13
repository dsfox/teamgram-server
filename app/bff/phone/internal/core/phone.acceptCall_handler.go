package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
)

// PhoneAcceptCall answers a ringing call with g_b.
//
// phone.acceptCall peer:InputPhoneCall g_b:bytes protocol:PhoneCallProtocol
//
//	= phone.PhoneCall;
//
// Only the person who was called may answer, and only while it is still
// ringing; the state machine says so, not this handler.
func (c *PhoneCore) PhoneAcceptCall(in *mtproto.TLPhoneAcceptCall) (*mtproto.Phone_PhoneCall, error) {
	now := time.Now()
	call, err := c.find(in.GetPeer(), now)
	if err != nil {
		return nil, rpcError(err, mtproto.ErrCallAlreadyAccepted)
	}
	if err = call.Accept(c.MD.UserId, in.GetGB(), now); err != nil {
		c.Logger.Errorf("phone.acceptCall - %d cannot accept %d: %v", c.MD.UserId, call.Id, err)
		return nil, rpcError(err, mtproto.ErrCallAlreadyAccepted)
	}
	// The device that answered is the callee's leg from here on.
	call.ParticipantKey, call.ParticipantServer = c.MD.PermAuthKeyId, c.MD.ServerId

	accepted := mtproto.MakeTLPhoneCallAccepted(&mtproto.PhoneCall{
		Id:            call.Id,
		AccessHash:    call.AccessHash,
		Date:          int32(now.Unix()),
		AdminId:       call.Admin,
		ParticipantId: call.Participant,
		GB:            call.GB,
		Protocol:      in.GetProtocol(),
	}).To_PhoneCall()

	// The caller is waiting on g_b to finish the exchange.
	c.tell(call.Admin, call.AdminKey, call.AdminServer, c.updatesFor(accepted, now))

	return mtproto.MakeTLPhonePhoneCall(&mtproto.Phone_PhoneCall{
		PhoneCall: accepted,
		Users:     []*mtproto.User{},
	}).To_Phone_PhoneCall(), nil
}
