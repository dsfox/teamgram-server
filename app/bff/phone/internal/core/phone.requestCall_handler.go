package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

// PhoneRequestCall places a call and makes the other phone ring.
//
// phone.requestCall video:flags.0?true user_id:InputUser random_id:int
//
//	g_a_hash:bytes protocol:PhoneCallProtocol = phone.PhoneCall;
//
// The caller has committed to a secret by sending only the hash of g_a; g_a
// itself follows at confirmCall, once g_b is back. The server carries those
// blobs and never opens them - the media key is the two phones' business.
func (c *PhoneCore) PhoneRequestCall(in *mtproto.TLPhoneRequestCall) (*mtproto.Phone_PhoneCall, error) {
	now := time.Now()
	callee := in.GetUserId().GetUserId()

	call, err := c.svcCtx.Registry.Place(c.MD.UserId, callee, in.GetGAHash(), now)
	if err != nil {
		// Busy, or a call to oneself. Mapping these onto the errors the client
		// draws as "busy" comes with the rest of the lifecycle.
		c.Logger.Errorf("phone.requestCall - %d cannot call %d: %v", c.MD.UserId, callee, err)
		return nil, err
	}
	// Nobody else can find the call before the ring below goes out.
	call.Protocol = in.GetProtocol()
	call.Video = in.GetVideo()

	waiting := c.waiting(call, now)

	// The whole reason the server is in this at all: the other phone has to
	// hear about the call. Everything after this is the two of them talking.
	c.ring(callee, waiting, now)

	return mtproto.MakeTLPhonePhoneCall(&mtproto.Phone_PhoneCall{
		PhoneCall: waiting,
		Users:     []*mtproto.User{},
	}).To_Phone_PhoneCall(), nil
}

// waiting is the call as the caller sees it before anyone picks up: the same
// object at requestCall and again, with receive_date, once the callee's phone
// says it is ringing.
func (c *PhoneCore) waiting(call *calls.Call, now time.Time) *mtproto.PhoneCall {
	pc := &mtproto.PhoneCall{
		Id:            call.Id,
		AccessHash:    call.AccessHash,
		Date:          int32(call.Created.Unix()),
		AdminId:       call.Admin,
		ParticipantId: call.Participant,
		Protocol:      call.Protocol,
		Video:         call.Video,
	}
	if !call.Received.IsZero() {
		pc.ReceiveDate = mtproto.MakeFlagsInt32(int32(call.Received.Unix()))
	}
	return mtproto.MakeTLPhoneCallWaiting(pc).To_PhoneCall()
}

// ring tells one person's devices about a call. A failure here is logged, not
// returned: the caller's own leg is already set up, and a phone that missed the
// update will still see the call when it next syncs.
func (c *PhoneCore) ring(userId int64, pc *mtproto.PhoneCall, now time.Time) {
	if _, err := c.svcCtx.SyncClient.SyncPushUpdates(c.ctx, &sync.TLSyncPushUpdates{
		UserId:  userId,
		Updates: c.updatesFor(pc, now),
	}); err != nil {
		c.Logger.Errorf("phone: could not ring %d: %v", userId, err)
	}
}

// updatesFor wraps one phone-call update the way the clients expect it.
func (c *PhoneCore) updatesFor(pc *mtproto.PhoneCall, now time.Time) *mtproto.Updates {
	return mtproto.MakeTLUpdateShort(&mtproto.Updates{
		Update: mtproto.MakeTLUpdatePhoneCall(&mtproto.Update{PhoneCall: pc}).To_Update(),
		Date:   int32(now.Unix()),
	}).To_Updates()
}
