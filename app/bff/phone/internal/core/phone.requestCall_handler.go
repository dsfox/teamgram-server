package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	"github.com/teamgram/teamgram-server/pkg/calls"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
		return nil, rpcError(err, mtproto.ErrCallOccupyFailed)
	}
	// Nobody else can find the call before the ring below goes out.
	call.Protocol = in.GetProtocol()
	call.Video = in.GetVideo()
	call.AdminKey, call.AdminServer = c.MD.PermAuthKeyId, c.MD.ServerId

	// The whole reason the server is in this at all: the other phone has to
	// hear about the call. Everything after this is the two of them talking.
	// The devices connected now hear it over their session and are marked
	// so; the rest are woken by a push, and one that comes back while the
	// call still rings is rung then, once (RecallRinging).
	call.MarkRung(c.connectedDevices(callee)...)
	c.ring(callee, c.requested(call), now)
	c.pushCall(call, now)

	return mtproto.MakeTLPhonePhoneCall(&mtproto.Phone_PhoneCall{
		PhoneCall: c.waiting(call, now),
		Users:     []*mtproto.User{},
	}).To_Phone_PhoneCall(), nil
}

// requested is the call as the callee first sees it. Both phones start
// ringing on this constructor and on no other - a waiting call with no session
// behind it is dropped by either client - and g_a_hash travels in it: the
// callee checks g_a against it at the end of the exchange.
func (c *PhoneCore) requested(call *calls.Call) *mtproto.PhoneCall {
	return mtproto.MakeTLPhoneCallRequested(&mtproto.PhoneCall{
		Id:            call.Id,
		AccessHash:    call.AccessHash,
		Date:          int32(call.Created.Unix()),
		AdminId:       call.Admin,
		ParticipantId: call.Participant,
		GAHash:        call.GAHash,
		Protocol:      call.Protocol,
		Video:         call.Video,
	}).To_PhoneCall()
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

// ring tells every device of a person about a call. A failure here is
// logged, not returned: the caller's own leg is already set up, and a phone
// that missed the update will still see the call when it next syncs.
func (c *PhoneCore) ring(userId int64, pc *mtproto.PhoneCall, now time.Time) {
	c.tell(userId, 0, "", c.updatesFor(pc, now))
}

// tell delivers updates to a person - to one device of theirs when it is
// known, which is how everything after the answer travels. The push by person
// goes through the status list, and a phone woken by the call a moment ago is
// not in it yet; naming the device's session server lets sync deliver without
// that list, straight to the server holding the session. Seen live: without
// the server the confirmed call never reached the phone that had answered.
func (c *PhoneCore) tell(userId, permAuthKeyId int64, server string, updates *mtproto.Updates) {
	var err error
	if permAuthKeyId != 0 {
		push := &sync.TLSyncUpdatesMe{UserId: userId, PermAuthKeyId: permAuthKeyId, Updates: updates}
		if server != "" {
			push.ServerId = wrapperspb.String(server)
		}
		_, err = c.svcCtx.SyncClient.SyncUpdatesMe(c.ctx, push)
	} else {
		_, err = c.svcCtx.SyncClient.SyncPushUpdates(c.ctx, &sync.TLSyncPushUpdates{
			UserId:  userId,
			Updates: updates,
		})
	}
	if err != nil {
		c.Logger.Errorf("phone: could not reach %d (device %d): %v", userId, permAuthKeyId, err)
	}
}

// updatesFor wraps one phone-call update the way the clients expect it.
func (c *PhoneCore) updatesFor(pc *mtproto.PhoneCall, now time.Time) *mtproto.Updates {
	return mtproto.MakeTLUpdateShort(&mtproto.Updates{
		Update: mtproto.MakeTLUpdatePhoneCall(&mtproto.Update{PhoneCall: pc}).To_Update(),
		Date:   int32(now.Unix()),
	}).To_Updates()
}
