package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
	"github.com/teamgram/teamgram-server/app/service/status/status"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

// pushLayer is the layer the iPhone parses the pushed call at: its
// Serialization.currentLayer() (clients/ios, TelegramCore/State/
// Serialization.swift). The user object's constructor changes between layers,
// and the push cannot ask which layer the phone speaks - so it speaks this
// one, and follows the client when the client moves.
const pushLayer = 228

// connectedDevices is every device of a person the session layer will hand
// the ring to: the ones the status service lists, expired or not, because
// sync pushes to all of them and the session queue holds it for a device
// that is between connections.
func (c *PhoneCore) connectedDevices(userId int64) []int64 {
	list, err := c.svcCtx.Sessions.StatusGetUserOnlineSessions(c.ctx, &status.TLStatusGetUserOnlineSessions{UserId: userId})
	if err != nil {
		c.Logger.Errorf("phone: cannot list the sessions of %d, so every device counts as away: %v", userId, err)
		return nil
	}
	keys := make([]int64, 0, len(list.GetUserSessions()))
	for _, sess := range list.GetUserSessions() {
		keys = append(keys, sess.GetPermAuthKeyId())
	}
	return keys
}

// pushCall wakes the callee's phones that are not connected. The iPhone gets
// the call itself: the same update the session carries, with the caller as
// the callee sees them, in one Updates container it parses on its own.
func (c *PhoneCore) pushCall(call *calls.Call, now time.Time) {
	users, err := c.svcCtx.Users.UserGetMutableUsers(c.ctx, &userpb.TLUserGetMutableUsers{Id: []int64{call.Admin, call.Participant}})
	if err != nil {
		// The Android only needs the wake-up; the iPhone will show the call
		// without a name. Better than no ring, and said.
		c.Logger.Errorf("phone: cannot fetch the caller %d for the push of call %d: %v", call.Admin, call.Id, err)
	}
	container := mtproto.MakeTLUpdates(&mtproto.Updates{
		Updates: []*mtproto.Update{c.updateFor(call)},
		Users:   users.GetUserListByIdList(call.Participant, call.Admin),
		Chats:   []*mtproto.Chat{},
		Date:    int32(now.Unix()),
		Seq:     0,
	}).To_Updates()

	buf := mtproto.NewEncodeBuf(1024)
	if err := container.Encode(buf, pushLayer); err != nil {
		c.Logger.Errorf("phone: cannot encode the push of call %d: %v", call.Id, err)
		return
	}
	c.svcCtx.Ringer.IncomingCall(c.ctx, call.Participant, call.Admin, call.Id, buf.GetBuf())
}

// updateFor is the ring as one update: what the session carries and what the
// push carries, the same object.
func (c *PhoneCore) updateFor(call *calls.Call) *mtproto.Update {
	return mtproto.MakeTLUpdatePhoneCall(&mtproto.Update{PhoneCall: c.requested(call)}).To_Update()
}

// RecallRinging rings one device of a person for the call still ringing for
// them, if that device was not rung yet - called when the device asks for
// its difference, which is what a phone woken by a push does first. Once per
// device: an Android told twice answers "busy".
func (c *PhoneCore) RecallRinging(permAuthKeyId int64) {
	now := time.Now()
	call, first := c.svcCtx.Registry.RingOnce(c.MD.UserId, permAuthKeyId, now)
	if !first {
		return
	}
	c.Logger.Infof("phone: device %d of %d came back while call %d rings, ringing it", permAuthKeyId, c.MD.UserId, call.Id)
	// The device is the one asking, so its session server is in the metadata:
	// named, so the ring does not depend on a status list it is not yet in.
	c.tell(c.MD.UserId, permAuthKeyId, c.MD.ServerId, mtproto.MakeTLUpdateShort(&mtproto.Updates{
		Update: c.updateFor(call),
		Date:   int32(now.Unix()),
	}).To_Updates())
}
