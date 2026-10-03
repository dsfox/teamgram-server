package service

import (
	"context"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/bff/reactions/internal/core"
)

func (s *Service) MessagesSendReaction(ctx context.Context, request *mtproto.TLMessagesSendReaction) (*mtproto.Updates, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.sendReaction - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesSendReaction(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.sendReaction - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesGetMessagesReactions(ctx context.Context, request *mtproto.TLMessagesGetMessagesReactions) (*mtproto.Updates, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.getMessagesReactions - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesGetMessagesReactions(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.getMessagesReactions - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesGetMessageReactionsList(ctx context.Context, request *mtproto.TLMessagesGetMessageReactionsList) (*mtproto.Messages_MessageReactionsList, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.getMessageReactionsList - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesGetMessageReactionsList(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.getMessageReactionsList - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesSetChatAvailableReactions(ctx context.Context, request *mtproto.TLMessagesSetChatAvailableReactions) (*mtproto.Updates, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.setChatAvailableReactions - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesSetChatAvailableReactions(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.setChatAvailableReactions - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesGetAvailableReactions(ctx context.Context, request *mtproto.TLMessagesGetAvailableReactions) (*mtproto.Messages_AvailableReactions, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.getAvailableReactions - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesGetAvailableReactions(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.getAvailableReactions - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesSetDefaultReaction(ctx context.Context, request *mtproto.TLMessagesSetDefaultReaction) (*mtproto.Bool, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.setDefaultReaction - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesSetDefaultReaction(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.setDefaultReaction - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesGetUnreadReactions(ctx context.Context, request *mtproto.TLMessagesGetUnreadReactions) (*mtproto.Messages_Messages, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.getUnreadReactions - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesGetUnreadReactions(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.getUnreadReactions - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesReadReactions(ctx context.Context, request *mtproto.TLMessagesReadReactions) (*mtproto.Messages_AffectedHistory, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.readReactions - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesReadReactions(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.readReactions - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesReportReaction(ctx context.Context, request *mtproto.TLMessagesReportReaction) (*mtproto.Bool, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.reportReaction - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesReportReaction(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.reportReaction - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesGetTopReactions(ctx context.Context, request *mtproto.TLMessagesGetTopReactions) (*mtproto.Messages_Reactions, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.getTopReactions - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesGetTopReactions(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.getTopReactions - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesGetRecentReactions(ctx context.Context, request *mtproto.TLMessagesGetRecentReactions) (*mtproto.Messages_Reactions, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.getRecentReactions - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesGetRecentReactions(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.getRecentReactions - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesClearRecentReactions(ctx context.Context, request *mtproto.TLMessagesClearRecentReactions) (*mtproto.Bool, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.clearRecentReactions - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesClearRecentReactions(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.clearRecentReactions - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesSendPaidReaction(ctx context.Context, request *mtproto.TLMessagesSendPaidReaction) (*mtproto.Updates, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.sendPaidReaction - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesSendPaidReaction(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.sendPaidReaction - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesTogglePaidReactionPrivacy(ctx context.Context, request *mtproto.TLMessagesTogglePaidReactionPrivacy) (*mtproto.Bool, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.togglePaidReactionPrivacy - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesTogglePaidReactionPrivacy(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.togglePaidReactionPrivacy - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesGetPaidReactionPrivacy(ctx context.Context, request *mtproto.TLMessagesGetPaidReactionPrivacy) (*mtproto.Updates, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.getPaidReactionPrivacy - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesGetPaidReactionPrivacy(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.getPaidReactionPrivacy - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesDeleteParticipantReactions(ctx context.Context, request *mtproto.TLMessagesDeleteParticipantReactions) (*mtproto.Bool, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.deleteParticipantReactions - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesDeleteParticipantReactions(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.deleteParticipantReactions - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesDeleteParticipantReaction(ctx context.Context, request *mtproto.TLMessagesDeleteParticipantReaction) (*mtproto.Updates, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.deleteParticipantReaction - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesDeleteParticipantReaction(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.deleteParticipantReaction - reply: {%s}", r)
	return r, err
}
