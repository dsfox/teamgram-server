package service

import (
	"context"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/bff/appearance/internal/core"
)

func (s *Service) AccountUploadTheme(ctx context.Context, request *mtproto.TLAccountUploadTheme) (*mtproto.Document, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.uploadTheme - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountUploadTheme(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.uploadTheme - reply: {%s}", r)
	return r, err
}

func (s *Service) AccountCreateTheme(ctx context.Context, request *mtproto.TLAccountCreateTheme) (*mtproto.Theme, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.createTheme - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountCreateTheme(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.createTheme - reply: {%s}", r)
	return r, err
}

func (s *Service) AccountUpdateTheme(ctx context.Context, request *mtproto.TLAccountUpdateTheme) (*mtproto.Theme, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.updateTheme - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountUpdateTheme(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.updateTheme - reply: {%s}", r)
	return r, err
}

func (s *Service) AccountSaveTheme(ctx context.Context, request *mtproto.TLAccountSaveTheme) (*mtproto.Bool, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.saveTheme - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountSaveTheme(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.saveTheme - reply: {%s}", r)
	return r, err
}

func (s *Service) AccountInstallTheme(ctx context.Context, request *mtproto.TLAccountInstallTheme) (*mtproto.Bool, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.installTheme - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountInstallTheme(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.installTheme - reply: {%s}", r)
	return r, err
}

func (s *Service) AccountGetTheme(ctx context.Context, request *mtproto.TLAccountGetTheme) (*mtproto.Theme, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.getTheme - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountGetTheme(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.getTheme - reply: {%s}", r)
	return r, err
}

func (s *Service) AccountGetThemes(ctx context.Context, request *mtproto.TLAccountGetThemes) (*mtproto.Account_Themes, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.getThemes - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountGetThemes(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.getThemes - reply: {%s}", r)
	return r, err
}

func (s *Service) AccountGetChatThemes(ctx context.Context, request *mtproto.TLAccountGetChatThemes) (*mtproto.Account_Themes, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.getChatThemes - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountGetChatThemes(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.getChatThemes - reply: {%s}", r)
	return r, err
}

func (s *Service) AccountGetUniqueGiftChatThemes(ctx context.Context, request *mtproto.TLAccountGetUniqueGiftChatThemes) (*mtproto.Account_ChatThemes, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.getUniqueGiftChatThemes - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountGetUniqueGiftChatThemes(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.getUniqueGiftChatThemes - reply: {%s}", r)
	return r, err
}

func (s *Service) MessagesSetChatTheme(ctx context.Context, request *mtproto.TLMessagesSetChatTheme) (*mtproto.Updates, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("messages.setChatTheme - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MessagesSetChatTheme(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("messages.setChatTheme - reply: {%s}", r)
	return r, err
}

func (s *Service) AccountUpdateColor(ctx context.Context, request *mtproto.TLAccountUpdateColor) (*mtproto.Bool, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.updateColor - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountUpdateColor(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.updateColor - reply: {%s}", r)
	return r, err
}

func (s *Service) AccountGetDefaultBackgroundEmojis(ctx context.Context, request *mtproto.TLAccountGetDefaultBackgroundEmojis) (*mtproto.EmojiList, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("account.getDefaultBackgroundEmojis - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.AccountGetDefaultBackgroundEmojis(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("account.getDefaultBackgroundEmojis - reply: {%s}", r)
	return r, err
}

func (s *Service) HelpGetPeerColors(ctx context.Context, request *mtproto.TLHelpGetPeerColors) (*mtproto.Help_PeerColors, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("help.getPeerColors - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.HelpGetPeerColors(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("help.getPeerColors - reply: {%s}", r)
	return r, err
}

func (s *Service) HelpGetPeerProfileColors(ctx context.Context, request *mtproto.TLHelpGetPeerProfileColors) (*mtproto.Help_PeerColors, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("help.getPeerProfileColors - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.HelpGetPeerProfileColors(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("help.getPeerProfileColors - reply: {%s}", r)
	return r, err
}

func (s *Service) ChannelsUpdateColor(ctx context.Context, request *mtproto.TLChannelsUpdateColor) (*mtproto.Updates, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("channels.updateColor - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.ChannelsUpdateColor(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("channels.updateColor - reply: {%s}", r)
	return r, err
}
