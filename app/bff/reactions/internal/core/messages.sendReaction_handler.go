package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	chatpb "github.com/teamgram/teamgram-server/app/service/biz/chat/chat"
	"github.com/teamgram/teamgram-server/app/service/biz/message/message"
)

// MessagesSendReaction
// messages.sendReaction
//
// One reaction per person per message: a new one replaces the old, none takes
// it back. Everybody else in the conversation is told at once, each with the
// number the message has in their own box and with their own reaction marked;
// the sender's other phones are told too, and the sender gets the same in the
// answer.
//
// What is not in the set, or comes from somebody no longer in the group,
// changes nothing and is answered with how things stand. An answer rather than
// a refusal: a client that is refused retries.
func (c *ReactionsCore) MessagesSendReaction(in *mtproto.TLMessagesSendReaction) (*mtproto.Updates, error) {
	me := c.MD.UserId
	box, err := c.svcCtx.MessageClient.MessageGetUserMessage(c.ctx, &message.TLMessageGetUserMessage{
		UserId: me,
		Id:     in.MsgId,
	})
	if err != nil || box == nil || !samePeer(mtproto.FromInputPeer2(me, in.Peer), box) {
		c.Logger.Errorf("messages.sendReaction - no message %d with %v in the box of %d: %v", in.MsgId, in.Peer, me, err)
		return nil, mtproto.ErrMessageIdInvalid
	}

	members, err := c.membersOf(box)
	if err != nil {
		c.Logger.Errorf("messages.sendReaction - members of %v: %v", peerOf(box), err)
		return nil, err
	}

	emoji := chosen(in)
	changed := true
	switch {
	case !contains(members, me):
		changed = false
		c.Logger.Infof("messages.sendReaction - %d is not in %v any more, nothing changes", me, peerOf(box))
	case emoji == "":
		err = c.svcCtx.Store.Remove(c.ctx, box.DialogMessageId, me)
	case c.svcCtx.Catalog != nil && c.svcCtx.Catalog.Offers(emoji):
		err = c.svcCtx.Store.Set(c.ctx, box.DialogMessageId, me, emoji, int32(time.Now().Unix()))
	default:
		changed = false
		c.Logger.Infof("messages.sendReaction - %q is not offered, nothing changes", emoji)
	}
	if err != nil {
		c.Logger.Errorf("messages.sendReaction - storing the reaction of %d to %d: %v", me, box.DialogMessageId, err)
		return nil, mtproto.ErrInternalServerError
	}

	rows, err := c.svcCtx.Store.Of(c.ctx, box.DialogMessageId)
	if err != nil {
		c.Logger.Errorf("messages.sendReaction - reading the reactions to %d: %v", box.DialogMessageId, err)
		return nil, mtproto.ErrInternalServerError
	}

	answer := mtproto.MakeUpdatesByUpdates(reactionsUpdate(box, rows))
	if !changed {
		return answer, nil
	}

	copies, err := c.svcCtx.MessageClient.MessageGetUserMessageListByDataIdUserIdList(c.ctx,
		&message.TLMessageGetUserMessageListByDataIdUserIdList{
			Id:         box.DialogMessageId,
			UserIdList: members,
		})
	if err != nil {
		c.Logger.Errorf("messages.sendReaction - the copies of %d: %v", box.DialogMessageId, err)
	}
	told := 0
	for _, held := range copies.GetDatas() {
		if held.UserId == me {
			continue
		}
		if _, err = c.svcCtx.SyncClient.SyncPushUpdates(c.ctx, &sync.TLSyncPushUpdates{
			UserId:  held.UserId,
			Updates: mtproto.MakeUpdatesByUpdates(reactionsUpdate(held, rows)),
		}); err != nil {
			c.Logger.Errorf("messages.sendReaction - telling %d: %v", held.UserId, err)
			continue
		}
		told++
	}
	if _, err = c.svcCtx.SyncClient.SyncUpdatesNotMe(c.ctx, &sync.TLSyncUpdatesNotMe{
		UserId:        me,
		PermAuthKeyId: c.MD.PermAuthKeyId,
		Updates:       answer,
	}); err != nil {
		c.Logger.Errorf("messages.sendReaction - telling the other phones of %d: %v", me, err)
	}
	c.Logger.Infof("messages.sendReaction - %d gave %q to %d, %d reactions on it, %d others told",
		me, emoji, box.DialogMessageId, len(rows), told)
	return answer, nil
}

// The reaction a client asks for. Older layers send a string; newer ones a
// list, of which a person with one reaction has at most one. An empty list,
// reactionEmpty or nothing at all takes the reaction back.
func chosen(in *mtproto.TLMessagesSendReaction) string {
	for _, reaction := range in.Reaction_FLAGVECTORREACTION {
		if reaction.GetPredicateName() == mtproto.Predicate_reactionEmoji {
			return reaction.Emoticon
		}
		if reaction.GetPredicateName() != mtproto.Predicate_reactionEmpty {
			// A custom or paid reaction: a value the set can never hold.
			return reaction.ToString()
		}
	}
	return in.Reaction_FLAGSTRING.GetValue()
}

// Everybody whose box holds a copy of this message and may see its reactions:
// the two people of a chat between two, or the group's members.
func (c *ReactionsCore) membersOf(box *mtproto.MessageBox) ([]int64, error) {
	if box.DialogId1 < 0 {
		participants, err := c.svcCtx.ChatClient.ChatGetChatParticipantIdList(c.ctx, &chatpb.TLChatGetChatParticipantIdList{
			ChatId: box.DialogId2,
		})
		if err != nil {
			return nil, err
		}
		return participants.GetDatas(), nil
	}
	if box.DialogId1 == box.DialogId2 {
		return []int64{box.DialogId1}, nil
	}
	return []int64{box.DialogId1, box.DialogId2}, nil
}

func contains(list []int64, id int64) bool {
	for _, v := range list {
		if v == id {
			return true
		}
	}
	return false
}
