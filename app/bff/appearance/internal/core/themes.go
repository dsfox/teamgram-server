package core

import (
	"math/rand"
	"time"

	"github.com/teamgram/proto/mtproto"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"
	"github.com/teamgram/teamgram-server/app/service/biz/dialog/dialog"
)

// AccountGetChatThemes
// account.getChatThemes
//
// "Not modified" only for this list's own hash: a client asking with zero
// holds nothing.
func (c *AppearanceCore) AccountGetChatThemes(in *mtproto.TLAccountGetChatThemes) (*mtproto.Account_Themes, error) {
	catalog := c.svcCtx.Catalog
	if catalog == nil {
		return mtproto.MakeTLAccountThemes(&mtproto.Account_Themes{Hash: 0, Themes: []*mtproto.Theme{}}).To_Account_Themes(), nil
	}
	if in.Hash != 0 && in.Hash == catalog.ThemesHash() {
		return mtproto.MakeTLAccountThemesNotModified(nil).To_Account_Themes(), nil
	}
	return mtproto.MakeTLAccountThemes(&mtproto.Account_Themes{
		Hash:   catalog.ThemesHash(),
		Themes: catalog.ChatThemes(),
	}).To_Account_Themes(), nil
}

// AccountGetUniqueGiftChatThemes
// account.getUniqueGiftChatThemes
//
// There are no gifts. Android's theme sheet asks for these first and stays on
// its placeholders after an error, so the answer is that there is nothing new.
func (c *AppearanceCore) AccountGetUniqueGiftChatThemes(in *mtproto.TLAccountGetUniqueGiftChatThemes) (*mtproto.Account_ChatThemes, error) {
	return mtproto.MakeTLAccountChatThemesNotModified(nil).To_Account_ChatThemes(), nil
}

// AccountGetThemes
// account.getThemes
//
// Both clients build their colour theme row from this list: Android the
// "Color theme" row of Chat Settings, from the themes marked default, which
// waited on placeholders for ever while there were none (#223); iOS the
// "COLOR THEME" row of Appearance, from the themes that carry an emoji, which
// showed only its own default one. Each picks the settings for day or night
// by their base. iOS was told there were none until 4 October, on the wrong
// belief that it read the list only as cloud themes.
func (c *AppearanceCore) AccountGetThemes(in *mtproto.TLAccountGetThemes) (*mtproto.Account_Themes, error) {
	catalog := c.svcCtx.Catalog
	if catalog == nil {
		return mtproto.MakeTLAccountThemes(&mtproto.Account_Themes{Hash: 0, Themes: []*mtproto.Theme{}}).To_Account_Themes(), nil
	}
	if in.Hash != 0 && in.Hash == catalog.AppThemesHash() {
		return mtproto.MakeTLAccountThemesNotModified(nil).To_Account_Themes(), nil
	}
	return mtproto.MakeTLAccountThemes(&mtproto.Account_Themes{
		Hash:   catalog.AppThemesHash(),
		Themes: catalog.AppThemes(),
	}).To_Account_Themes(), nil
}

// MessagesSetChatTheme
// messages.setChatTheme
//
// The theme of a chat between two. Both keep it, and both are told with the
// service message Telegram uses, which the other phone applies as it arrives.
// Nothing at all, inputChatThemeEmpty or an empty emoji takes it off - iOS
// sends the last of these. A group, a theme not in the list or the theme the
// chat already has changes nothing and is answered with nothing new.
func (c *AppearanceCore) MessagesSetChatTheme(in *mtproto.TLMessagesSetChatTheme) (*mtproto.Updates, error) {
	me := c.MD.UserId
	nothing := mtproto.MakeUpdatesByUpdates()
	peer := mtproto.FromInputPeer2(me, in.Peer)
	if peer.PeerType != mtproto.PEER_USER || peer.PeerId == me {
		c.Logger.Infof("messages.setChatTheme - %d: themes are for a chat between two, not %v", me, in.Peer)
		return nothing, nil
	}

	emoticon, ok := themeAsked(in)
	if !ok || emoticon != "" && (c.svcCtx.Catalog == nil || !c.svcCtx.Catalog.Offers(emoticon)) {
		c.Logger.Infof("messages.setChatTheme - %d: %q is not offered, nothing changes", me, emoticon)
		return nothing, nil
	}

	current, err := c.svcCtx.Dialogs.DialogGetDialogById(c.ctx, &dialog.TLDialogGetDialogById{
		UserId:   me,
		PeerType: mtproto.PEER_USER,
		PeerId:   peer.PeerId,
	})
	if err != nil {
		c.Logger.Errorf("messages.setChatTheme - the chat of %d with %d: %v", me, peer.PeerId, err)
		return nil, mtproto.ErrPeerIdInvalid
	}
	if current.GetThemeEmoticon() == emoticon {
		return nothing, nil
	}

	if _, err = c.svcCtx.Dialogs.DialogSetChatTheme(c.ctx, &dialog.TLDialogSetChatTheme{
		UserId:        me,
		PeerType:      mtproto.PEER_USER,
		PeerId:        peer.PeerId,
		ThemeEmoticon: emoticon,
	}); err != nil {
		c.Logger.Errorf("messages.setChatTheme - keeping %q for %d and %d: %v", emoticon, me, peer.PeerId, err)
		return nil, mtproto.ErrInternalServerError
	}

	updates, err := c.svcCtx.Msg.MsgSendMessageV2(c.ctx, &msgpb.TLMsgSendMessageV2{
		UserId:    me,
		AuthKeyId: c.MD.PermAuthKeyId,
		PeerType:  mtproto.PEER_USER,
		PeerId:    peer.PeerId,
		Message: []*msgpb.OutboxMessage{
			msgpb.MakeTLOutboxMessage(&msgpb.OutboxMessage{
				NoWebpage: true,
				RandomId:  rand.Int63(),
				Message: mtproto.MakeTLMessageService(&mtproto.Message{
					Out:    true,
					FromId: mtproto.MakePeerUser(me),
					PeerId: mtproto.MakePeerUser(peer.PeerId),
					Date:   int32(time.Now().Unix()),
					Action: mtproto.MakeTLMessageActionSetChatTheme(&mtproto.MessageAction{
						Theme:    mtproto.MakeTLChatTheme(&mtproto.ChatTheme{Emoticon: emoticon}).To_ChatTheme(),
						Emoticon: emoticon,
					}).To_MessageAction(),
				}).To_Message(),
			}).To_OutboxMessage(),
		},
	})
	if err != nil {
		// The theme is kept; the other phone finds it in the profile it
		// opens next. Said, because the live change is what it lost.
		c.Logger.Errorf("messages.setChatTheme - telling %d of %q: %v", peer.PeerId, emoticon, err)
		return nothing, nil
	}
	c.Logger.Infof("messages.setChatTheme - %d gave the chat with %d the theme %q", me, peer.PeerId, emoticon)
	return updates, nil
}

// The theme a client asks for, and whether it is one of ours to give: a
// unique gift theme never is.
func themeAsked(in *mtproto.TLMessagesSetChatTheme) (string, bool) {
	if in.Theme == nil {
		return in.Emoticon, true
	}
	switch in.Theme.GetPredicateName() {
	case mtproto.Predicate_inputChatTheme:
		return in.Theme.Emoticon, true
	case mtproto.Predicate_inputChatThemeEmpty:
		return "", true
	}
	return "", false
}

// AccountInstallTheme
// account.installTheme
func (c *AppearanceCore) AccountInstallTheme(in *mtproto.TLAccountInstallTheme) (*mtproto.Bool, error) {
	return mtproto.BoolTrue, nil
}

// AccountSaveTheme
// account.saveTheme
func (c *AppearanceCore) AccountSaveTheme(in *mtproto.TLAccountSaveTheme) (*mtproto.Bool, error) {
	return mtproto.BoolTrue, nil
}

// AccountGetTheme
// account.getTheme
//
// One of the themes offered, by its id and access hash: iOS asks again for the
// theme it is using, every so often, to see whether it changed. Anything else
// - a slug from a link, a theme somebody made - is a cloud theme, and those
// are not offered.
func (c *AppearanceCore) AccountGetTheme(in *mtproto.TLAccountGetTheme) (*mtproto.Theme, error) {
	if c.svcCtx.Catalog != nil && in.Theme.GetPredicateName() == mtproto.Predicate_inputTheme {
		if theme := c.svcCtx.Catalog.Theme(in.Theme.Id, in.Theme.AccessHash); theme != nil {
			return theme, nil
		}
	}
	return nil, mtproto.ErrThemeInvalid
}

// The three that make a cloud theme are things a person does, behind a
// switch that is off; no client asks for them on its own.

// AccountCreateTheme
// account.createTheme
func (c *AppearanceCore) AccountCreateTheme(in *mtproto.TLAccountCreateTheme) (*mtproto.Theme, error) {
	return nil, mtproto.ErrThemeInvalid
}

// AccountUpdateTheme
// account.updateTheme
func (c *AppearanceCore) AccountUpdateTheme(in *mtproto.TLAccountUpdateTheme) (*mtproto.Theme, error) {
	return nil, mtproto.ErrThemeInvalid
}

// AccountUploadTheme
// account.uploadTheme
func (c *AppearanceCore) AccountUploadTheme(in *mtproto.TLAccountUploadTheme) (*mtproto.Document, error) {
	return nil, mtproto.ErrThemeInvalid
}
