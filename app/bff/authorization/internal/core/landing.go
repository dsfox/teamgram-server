package core

import (
	"context"
	"math/rand"

	"github.com/teamgram/proto/mtproto"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"
	chatpb "github.com/teamgram/teamgram-server/app/service/biz/chat/chat"
)

// landInChat puts the account that just signed up into the group its
// invitation was minted from, from the person who minted it - the same path
// and the same service message as the "Add" button, so every member's device
// sees the newcomer and lets them into the conversation (#164). The person
// is signed up either way: whatever goes wrong here is one log line.
func (c *AuthorizationCore) landInChat(ctx context.Context, chatId, inviterId, userId int64) {
	existing, err := c.svcCtx.Dao.ChatClient.ChatGetMutableChat(ctx, &chatpb.TLChatGetMutableChat{ChatId: chatId})
	if err != nil || existing == nil {
		c.Logger.Errorf("auth.signUp - %d was invited into chat %d, which is not there: %v", userId, chatId, err)
		return
	}
	// The inviter may have left since minting; then the addition is the
	// creator's, the way an admin's addition is.
	fromId := inviterId
	if me, ok := existing.GetImmutableChatParticipant(inviterId); !ok || !me.IsChatMemberStateNormal() {
		fromId = existing.Creator()
		inviterId = 0
	}

	updated, err := c.svcCtx.Dao.ChatClient.ChatAddChatUser(ctx, &chatpb.TLChatAddChatUser{
		ChatId:    chatId,
		InviterId: inviterId,
		UserId:    userId,
		IsBot:     false,
	})
	if err != nil || updated == nil {
		c.Logger.Errorf("auth.signUp - %d was invited into chat %d and cannot be put in: %v", userId, chatId, err)
		return
	}

	if _, err := c.svcCtx.Dao.MsgClient.MsgSendMessageV2(ctx, &msgpb.TLMsgSendMessageV2{
		UserId:    fromId,
		AuthKeyId: 0,
		PeerType:  mtproto.PEER_CHAT,
		PeerId:    chatId,
		Message: []*msgpb.OutboxMessage{
			msgpb.MakeTLOutboxMessage(&msgpb.OutboxMessage{
				NoWebpage:    true,
				Background:   false,
				RandomId:     rand.Int63(),
				Message:      updated.MakeMessageService(fromId, mtproto.MakeMessageActionChatAddUser(userId)),
				ScheduleDate: nil,
			}).To_OutboxMessage(),
		},
	}); err != nil {
		c.Logger.Errorf("auth.signUp - %d is in chat %d, but the group was not told: %v", userId, chatId, err)
		return
	}
	c.Logger.Infof("auth.signUp - %d landed in chat %d on %d's invitation", userId, chatId, fromId)
}
