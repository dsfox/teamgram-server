package core

import (
	"github.com/teamgram/proto/mtproto"
)

// What the rest of the reaction methods answer. None of it is offered - there
// are no unread reactions, no recent ones kept, no paid ones - but a phone may
// still ask, and an error is something it retries; an answer it accepts.

// MessagesSetDefaultReaction
// messages.setDefaultReaction
func (c *ReactionsCore) MessagesSetDefaultReaction(in *mtproto.TLMessagesSetDefaultReaction) (*mtproto.Bool, error) {
	return mtproto.BoolTrue, nil
}

// MessagesGetUnreadReactions
// messages.getUnreadReactions
func (c *ReactionsCore) MessagesGetUnreadReactions(in *mtproto.TLMessagesGetUnreadReactions) (*mtproto.Messages_Messages, error) {
	return mtproto.MakeTLMessagesMessages(&mtproto.Messages_Messages{
		Messages: []*mtproto.Message{},
		Chats:    []*mtproto.Chat{},
		Users:    []*mtproto.User{},
	}).To_Messages_Messages(), nil
}

// MessagesReadReactions
// messages.readReactions
func (c *ReactionsCore) MessagesReadReactions(in *mtproto.TLMessagesReadReactions) (*mtproto.Messages_AffectedHistory, error) {
	return mtproto.MakeTLMessagesAffectedHistory(&mtproto.Messages_AffectedHistory{
		Pts:      c.svcCtx.IdgenClient.CurrentPtsId(c.ctx, c.MD.UserId),
		PtsCount: 0,
		Offset:   0,
	}).To_Messages_AffectedHistory(), nil
}

// MessagesReportReaction
// messages.reportReaction
func (c *ReactionsCore) MessagesReportReaction(in *mtproto.TLMessagesReportReaction) (*mtproto.Bool, error) {
	c.Logger.Infof("messages.reportReaction - %d reports %v on message %d", c.MD.UserId, in.ReactionPeer, in.Id)
	return mtproto.BoolTrue, nil
}

// MessagesClearRecentReactions
// messages.clearRecentReactions
func (c *ReactionsCore) MessagesClearRecentReactions(in *mtproto.TLMessagesClearRecentReactions) (*mtproto.Bool, error) {
	return mtproto.BoolTrue, nil
}

// MessagesSendPaidReaction
// messages.sendPaidReaction
func (c *ReactionsCore) MessagesSendPaidReaction(in *mtproto.TLMessagesSendPaidReaction) (*mtproto.Updates, error) {
	return mtproto.MakeUpdatesByUpdates(), nil
}

// MessagesTogglePaidReactionPrivacy
// messages.togglePaidReactionPrivacy
func (c *ReactionsCore) MessagesTogglePaidReactionPrivacy(in *mtproto.TLMessagesTogglePaidReactionPrivacy) (*mtproto.Bool, error) {
	return mtproto.BoolTrue, nil
}

// MessagesGetPaidReactionPrivacy
// messages.getPaidReactionPrivacy
func (c *ReactionsCore) MessagesGetPaidReactionPrivacy(in *mtproto.TLMessagesGetPaidReactionPrivacy) (*mtproto.Updates, error) {
	return mtproto.MakeUpdatesByUpdates(), nil
}

// MessagesDeleteParticipantReactions
// messages.deleteParticipantReactions
func (c *ReactionsCore) MessagesDeleteParticipantReactions(in *mtproto.TLMessagesDeleteParticipantReactions) (*mtproto.Bool, error) {
	return mtproto.BoolTrue, nil
}

// MessagesDeleteParticipantReaction
// messages.deleteParticipantReaction
func (c *ReactionsCore) MessagesDeleteParticipantReaction(in *mtproto.TLMessagesDeleteParticipantReaction) (*mtproto.Updates, error) {
	return mtproto.MakeUpdatesByUpdates(), nil
}
