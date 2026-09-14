package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"
	"github.com/teamgram/teamgram-server/pkg/calls"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
		return nil, rpcError(err, mtproto.ErrCallAlreadyDeclined)
	}
	otherLeg, err := call.Other(c.MD.UserId)
	if err != nil {
		return nil, rpcError(err, mtproto.ErrCallAlreadyDeclined)
	}
	spoken := call.State == calls.Active
	if err = call.Discard(c.MD.UserId, now); err != nil {
		c.Logger.Errorf("phone.discardCall - %d cannot discard %d: %v", c.MD.UserId, call.Id, err)
		return nil, rpcError(err, mtproto.ErrCallAlreadyDeclined)
	}

	// The reason as the phone said it, with one correction: a caller who gives
	// up before anyone answered has made a missed call, whatever button they
	// pressed - that is the entry the callee should find.
	reason := in.GetReason()
	if !spoken && c.MD.UserId == call.Admin {
		reason = mtproto.MakeTLPhoneCallDiscardReasonMissed(nil).To_PhoneCallDiscardReason()
	}
	var duration *wrapperspb.Int32Value
	if spoken {
		duration = mtproto.MakeFlagsInt32(in.GetDuration())
	}

	// A video call is one that was placed as video, whatever the phone that
	// hangs up says (iOS says nothing).
	call.Video = call.Video || in.GetVideo()
	discarded := c.discarded(call, reason, duration)

	// The other phone has to stop ringing, or stop talking: the device in the
	// call by its key, and every device of theirs besides, in case more than
	// one was ringing.
	if otherKey, otherServer := call.DeviceOf(otherLeg); otherKey != 0 {
		c.tell(otherLeg, otherKey, otherServer, c.updatesFor(discarded, now))
	}
	c.ring(otherLeg, discarded, now)

	c.leaveEntry(call, reason, duration, call.Video, now)

	return c.updatesFor(discarded, now), nil
}

// leaveEntry writes the call into the chat of the two, from the caller: the
// service message with messageActionPhoneCall that both phones build their
// calls list from (the phone-calls search filter), and that reads "missed
// call" or "outgoing call, 0:42" in the chat. Without it the list stayed
// empty on both phones after the first real calls. A failure is logged, not
// returned: the call is over either way.
func (c *PhoneCore) leaveEntry(call *calls.Call, reason *mtproto.PhoneCallDiscardReason, duration *wrapperspb.Int32Value, video bool, now time.Time) {
	seconds := int32(0)
	if duration != nil {
		seconds = duration.GetValue()
	}
	_, err := c.svcCtx.Msg.MsgSendMessageV2(c.ctx, &msgpb.TLMsgSendMessageV2{
		UserId:    call.Admin,
		AuthKeyId: call.AdminKey,
		PeerType:  mtproto.PEER_USER,
		PeerId:    call.Participant,
		Message: []*msgpb.OutboxMessage{
			msgpb.MakeTLOutboxMessage(&msgpb.OutboxMessage{
				NoWebpage: true,
				RandomId:  call.Id,
				Message: mtproto.MakeTLMessageService(&mtproto.Message{
					Out:    true,
					FromId: mtproto.MakePeerUser(call.Admin),
					PeerId: mtproto.MakePeerUser(call.Participant),
					Date:   int32(now.Unix()),
					Action: mtproto.MakeMessageActionPhoneCall(video, call.Id, reason, seconds),
				}).To_Message(),
			}).To_OutboxMessage(),
		},
	})
	if err != nil {
		c.Logger.Errorf("phone.discardCall - call %d left no entry in the chat: %v", call.Id, err)
	}
}
