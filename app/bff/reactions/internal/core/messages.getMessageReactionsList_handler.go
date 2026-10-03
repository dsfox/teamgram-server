package core

import (
	"sort"
	"strconv"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/service/biz/message/message"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
	"github.com/teamgram/teamgram-server/pkg/reactions"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

const listPage = 50

// MessagesGetMessageReactionsList
// messages.getMessageReactionsList
//
// Who reacted to a message, newest first, optionally with one reaction only.
// The offset is how many have been handed out already.
func (c *ReactionsCore) MessagesGetMessageReactionsList(in *mtproto.TLMessagesGetMessageReactionsList) (*mtproto.Messages_MessageReactionsList, error) {
	me := c.MD.UserId
	box, err := c.svcCtx.MessageClient.MessageGetUserMessage(c.ctx, &message.TLMessageGetUserMessage{UserId: me, Id: in.Id})
	if err != nil || box == nil || !samePeer(mtproto.FromInputPeer2(me, in.Peer), box) {
		c.Logger.Errorf("messages.getMessageReactionsList - no message %d with %v in the box of %d: %v", in.Id, in.Peer, me, err)
		return nil, mtproto.ErrMessageIdInvalid
	}
	rows, err := c.svcCtx.Store.Of(c.ctx, box.DialogMessageId)
	if err != nil {
		c.Logger.Errorf("messages.getMessageReactionsList - reactions to %d: %v", box.DialogMessageId, err)
		return nil, mtproto.ErrInternalServerError
	}

	only := in.Reaction_FLAGREACTION.ToString()
	if only == "" {
		only = in.Reaction_FLAGSTRING.GetValue()
	}
	picked := make([]reactions.MessageReactionsDO, 0, len(rows))
	for _, row := range rows {
		if only == "" || row.Reaction == only {
			picked = append(picked, row)
		}
	}
	sort.Slice(picked, func(i, j int) bool {
		if picked[i].ReactedAt != picked[j].ReactedAt {
			return picked[i].ReactedAt > picked[j].ReactedAt
		}
		return picked[i].UserId < picked[j].UserId
	})

	from, _ := strconv.Atoi(in.Offset.GetValue())
	if from < 0 || from > len(picked) {
		from = len(picked)
	}
	limit := int(in.Limit)
	if limit <= 0 || limit > listPage {
		limit = listPage
	}
	page := picked[from:min(from+limit, len(picked))]

	list := make([]*mtproto.MessagePeerReaction, 0, len(page))
	ids := make([]int64, 0, len(page))
	for _, row := range page {
		list = append(list, reactions.PeerReaction(row, me))
		ids = append(ids, row.UserId)
	}
	var users []*mtproto.User
	if len(ids) > 0 {
		found, err := c.svcCtx.UserClient.UserGetMutableUsers(c.ctx, &userpb.TLUserGetMutableUsers{Id: append(ids, me), To: []int64{me}})
		if err != nil {
			c.Logger.Errorf("messages.getMessageReactionsList - the people %v: %v", ids, err)
		}
		users = found.GetUserListByIdList(me, ids...)
	}
	if users == nil {
		users = []*mtproto.User{}
	}

	var next *wrapperspb.StringValue
	if end := from + len(page); end < len(picked) {
		next = &wrapperspb.StringValue{Value: strconv.Itoa(end)}
	}
	return mtproto.MakeTLMessagesMessageReactionsList(&mtproto.Messages_MessageReactionsList{
		Count:      int32(len(picked)),
		Reactions:  list,
		Chats:      []*mtproto.Chat{},
		Users:      users,
		NextOffset: next,
	}).To_Messages_MessageReactionsList(), nil
}
