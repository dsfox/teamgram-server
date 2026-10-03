package core

import (
	"github.com/teamgram/proto/mtproto"
	chatpb "github.com/teamgram/teamgram-server/app/service/biz/chat/chat"
)

// MessagesSetChatAvailableReactions
// messages.setChatAvailableReactions
//
// Which reactions a group allows. The chat service keeps it and decides who
// may change it; the group itself comes back so the phone redraws it.
func (c *ReactionsCore) MessagesSetChatAvailableReactions(in *mtproto.TLMessagesSetChatAvailableReactions) (*mtproto.Updates, error) {
	me := c.MD.UserId
	peer := mtproto.FromInputPeer2(me, in.Peer)
	if peer.PeerType != mtproto.PEER_CHAT {
		c.Logger.Errorf("messages.setChatAvailableReactions - %v is not a group", in.Peer)
		return nil, mtproto.ErrPeerIdInvalid
	}
	kind, list := in.AvailableReactions_CHATREACTIONS.ToChatReactions()
	if in.AvailableReactions_CHATREACTIONS == nil {
		kind, list = mtproto.ChatReactionsTypeSome, in.AvailableReactions_VECTORSTRING
	}
	chat, err := c.svcCtx.ChatClient.ChatSetChatAvailableReactions(c.ctx, &chatpb.TLChatSetChatAvailableReactions{
		SelfId:                 me,
		ChatId:                 peer.PeerId,
		AvailableReactionsType: kind,
		AvailableReactions:     list,
	})
	if err != nil {
		c.Logger.Errorf("messages.setChatAvailableReactions - %v: %v", in, err)
		return nil, err
	}
	return mtproto.MakeUpdatesByUpdatesUsersChats(nil, []*mtproto.Chat{chat.ToUnsafeChat(me)}), nil
}
