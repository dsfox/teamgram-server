package core

import (
	"context"
	"math/rand"

	"github.com/teamgram/proto/mtproto"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
)

// tellInviter gives whoever minted the invitation the person who just came in
// on it as a contact, and - unless they turned "contact joined" off - the
// "X joined ice9" message. The inviter's phone sent the number to the server
// once, while nobody held it, and the server keeps no numbers of people who
// are not here, so without this the inviter could not find the person they
// had invited. The invitation is the one record that says who wants to know.
// The person is signed up either way: whatever goes wrong here is one log line.
func (c *AuthorizationCore) tellInviter(ctx context.Context, inviterId, userId int64, phone string) {
	if inviterId == 0 || inviterId == userId {
		return
	}
	users, err := c.svcCtx.Dao.UserClient.UserGetMutableUsersV2(ctx, &userpb.TLUserGetMutableUsersV2{
		Id:      []int64{inviterId, userId},
		Privacy: true,
		HasTo:   true,
		To:      []int64{inviterId},
	})
	if err != nil {
		c.Logger.Errorf("auth.signUp - cannot read %d and the inviter %d: %v", userId, inviterId, err)
		return
	}
	inviter, ok := users.GetImmutableUser(inviterId)
	if !ok || inviter.Deleted() {
		c.Logger.Infof("auth.signUp - %d came in on the invitation of %d, who is gone", userId, inviterId)
		return
	}
	newcomer, ok := users.GetImmutableUser(userId)
	if !ok {
		c.Logger.Errorf("auth.signUp - %d is not among the users just read", userId)
		return
	}

	if _, err := c.svcCtx.Dao.UserClient.UserAddContact(ctx, &userpb.TLUserAddContact{
		UserId:                   inviterId,
		AddPhonePrivacyException: mtproto.BoolFalse,
		Id:                       userId,
		FirstName:                newcomer.FirstName(),
		LastName:                 newcomer.LastName(),
		Phone:                    phone,
	}); err != nil {
		c.Logger.Errorf("auth.signUp - cannot add %d to the contacts of the inviter %d: %v", userId, inviterId, err)
		return
	}

	told, _ := c.svcCtx.Dao.UserClient.UserGetContactSignUpNotification(ctx, &userpb.TLUserGetContactSignUpNotification{
		UserId: inviterId,
	})
	if !mtproto.FromBool(told) {
		c.Logger.Infof("auth.signUp - %d is a contact of the inviter %d, who asked not to be told", userId, inviterId)
		return
	}
	if _, err := c.svcCtx.Dao.MsgClient.MsgPushUserMessage(ctx, &msgpb.TLMsgPushUserMessage{
		UserId:    userId,
		AuthKeyId: 0,
		PeerType:  mtproto.PEER_USER,
		PeerId:    inviterId,
		PushType:  1,
		Message: msgpb.MakeTLOutboxMessage(&msgpb.OutboxMessage{
			NoWebpage:    false,
			Background:   false,
			RandomId:     rand.Int63(),
			Message:      mtproto.MakeContactSignUpMessage(userId, inviterId),
			ScheduleDate: nil,
		}).To_OutboxMessage(),
	}); err != nil {
		c.Logger.Errorf("auth.signUp - %d is a contact of the inviter %d, who was not told: %v", userId, inviterId, err)
		return
	}
	c.Logger.Infof("auth.signUp - the inviter %d has %d as a contact and was told", inviterId, userId)
}
