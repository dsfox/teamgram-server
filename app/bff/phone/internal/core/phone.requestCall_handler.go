package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
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

	waiting := mtproto.MakeTLPhoneCallWaiting(&mtproto.PhoneCall{
		Id:            call.Id,
		AccessHash:    call.AccessHash,
		Date:          int32(now.Unix()),
		AdminId:       call.Admin,
		ParticipantId: call.Participant,
		Protocol:      in.GetProtocol(),
		Video:         in.GetVideo(),
	}).To_PhoneCall()

	// The whole reason the server is in this at all: the other phone has to
	// hear about the call. Everything after this is the two of them talking.
	c.ring(callee, waiting, now)

	return mtproto.MakeTLPhonePhoneCall(&mtproto.Phone_PhoneCall{
		PhoneCall: waiting,
		Users:     []*mtproto.User{},
	}).To_Phone_PhoneCall(), nil
}

// ring tells one person's devices about a call. A failure here is logged, not
// returned: the caller's own leg is already set up, and a phone that missed the
// update will still see the call when it next syncs.
func (c *PhoneCore) ring(userId int64, pc *mtproto.PhoneCall, now time.Time) {
	updates := mtproto.MakeTLUpdateShort(&mtproto.Updates{
		Update: mtproto.MakeTLUpdatePhoneCall(&mtproto.Update{PhoneCall: pc}).To_Update(),
		Date:   int32(now.Unix()),
	}).To_Updates()

	if _, err := c.svcCtx.SyncClient.SyncPushUpdates(c.ctx, &sync.TLSyncPushUpdates{
		UserId:  userId,
		Updates: updates,
	}); err != nil {
		c.Logger.Errorf("phone: could not ring %d: %v", userId, err)
	}
}
