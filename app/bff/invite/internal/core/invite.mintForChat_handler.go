package core

import (
	"github.com/teamgram/proto/mtproto"
	chatpb "github.com/teamgram/teamgram-server/app/service/biz/chat/chat"
	"github.com/teamgram/teamgram-server/pkg/code/invite"
)

// InviteMintForChat mints a code that leads into a group: whoever signs up
// with it is put into the group by the server, from the person asking (#164).
// Only a member may ask, and the number must have no account - the picker
// shows a number that is on ice9 as a person to add, not as somebody to invite.
//
// invite.mintForChat chat_id:long phone:string = invite.Minted;
func (c *InviteCore) InviteMintForChat(in *mtproto.TLInviteMintForChat) (*mtproto.Invite_Minted, error) {
	phone := invite.Digits(in.GetPhone())
	if phone == "" {
		return nil, mtproto.ErrPhoneNumberInvalid
	}

	chat, err := c.svcCtx.ChatClient.ChatGetMutableChat(c.ctx, &chatpb.TLChatGetMutableChat{ChatId: in.GetChatId()})
	if err != nil || chat == nil {
		c.Logger.Infof("invite.mintForChat - %d asked for chat %d, which is not there: %v", c.MD.UserId, in.GetChatId(), err)
		return nil, mtproto.ErrChatIdInvalid
	}
	me, ok := chat.GetImmutableChatParticipant(c.MD.UserId)
	if !ok || !me.IsChatMemberStateNormal() {
		c.Logger.Infof("invite.mintForChat - %d is not in chat %d", c.MD.UserId, in.GetChatId())
		return nil, mtproto.ErrUserNotParticipant
	}

	return c.mint(phone, in.GetChatId())
}
