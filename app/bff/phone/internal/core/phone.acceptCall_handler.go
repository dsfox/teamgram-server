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

	// Every other phone of the callee was ringing too. A discarded call is
	// the one thing both clients take cleanly for "answered elsewhere"; a
	// phoneCallAccepted reaching a phone that still rings goes down iOS's
	// fallback branch. Busy: this person is on this call, on another device.
	c.tellOthers(call.Participant, c.MD.PermAuthKeyId, c.updatesFor(mtproto.MakeTLPhoneCallDiscarded(&mtproto.PhoneCall{
		Id:     call.Id,
		Reason: mtproto.MakeTLPhoneCallDiscardReasonBusy(nil).To_PhoneCallDiscardReason(),
		Video:  call.Video,
	}).To_PhoneCall(), now))

	// The callee is answered with the call as the callee sees it: still
	// waiting, for the caller's confirm. phoneCallAccepted is the caller's
	// object; iOS answered with it reads "failed" and hangs up within a
	// moment (CallSessionManager.swift:1626) - the first real Android ->
	// iPhone call ended that way.
	return mtproto.MakeTLPhonePhoneCall(&mtproto.Phone_PhoneCall{
		PhoneCall: c.waiting(call, now),
		Users:     []*mtproto.User{},
	}).To_Phone_PhoneCall(), nil
}
