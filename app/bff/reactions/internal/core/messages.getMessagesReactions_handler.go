package core

import (
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/service/biz/message/message"
)

// MessagesGetMessagesReactions
// messages.getMessagesReactions
//
// Both clients ask this for the messages on the screen, every so often while a
// chat is open: it is how a phone that was away when somebody reacted comes to
// see it. Every message asked about is answered, the ones without reactions
// with an empty list, so a reaction taken back while the phone was away goes
// from it too.
func (c *ReactionsCore) MessagesGetMessagesReactions(in *mtproto.TLMessagesGetMessagesReactions) (*mtproto.Updates, error) {
	me := c.MD.UserId
	asked := mtproto.FromInputPeer2(me, in.Peer)
	updates := make([]*mtproto.Update, 0, len(in.Id))
	if len(in.Id) > 0 {
		boxes, err := c.svcCtx.MessageClient.MessageGetUserMessageList(c.ctx, &message.TLMessageGetUserMessageList{
			UserId: me,
			IdList: in.Id,
		})
		if err != nil {
			c.Logger.Errorf("messages.getMessagesReactions - the messages %v of %d: %v", in.Id, me, err)
		}
		for _, box := range boxes.GetDatas() {
			if !samePeer(asked, box) {
				continue
			}
			rows, err := c.svcCtx.Store.Of(c.ctx, box.DialogMessageId)
			if err != nil {
				c.Logger.Errorf("messages.getMessagesReactions - reactions to %d: %v", box.DialogMessageId, err)
				continue
			}
			updates = append(updates, reactionsUpdate(box, rows))
		}
	}
	return mtproto.MakeUpdatesByUpdates(updates...), nil
}
