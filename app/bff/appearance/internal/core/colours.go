package core

import (
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
)

// The colour a person has not chosen (sql-patches/15): 0 is red.
const noColour = -1

// HelpGetPeerColors
// help.getPeerColors
//
// The seven name colours both clients carry, by number.
func (c *AppearanceCore) HelpGetPeerColors(in *mtproto.TLHelpGetPeerColors) (*mtproto.Help_PeerColors, error) {
	catalog := c.svcCtx.Catalog
	if catalog == nil {
		return mtproto.MakeTLHelpPeerColors(&mtproto.Help_PeerColors{Hash: 0, Colors: []*mtproto.Help_PeerColorOption{}}).To_Help_PeerColors(), nil
	}
	return c.peerColors(in.Hash, catalog.ColoursHash(), catalog.NameColours()), nil
}

// HelpGetPeerProfileColors
// help.getPeerProfileColors
//
// No client carries these, so they come with their colours. An empty list is
// what left the profile tab of the colour screen empty.
func (c *AppearanceCore) HelpGetPeerProfileColors(in *mtproto.TLHelpGetPeerProfileColors) (*mtproto.Help_PeerColors, error) {
	catalog := c.svcCtx.Catalog
	if catalog == nil {
		return mtproto.MakeTLHelpPeerColors(&mtproto.Help_PeerColors{Hash: 0, Colors: []*mtproto.Help_PeerColorOption{}}).To_Help_PeerColors(), nil
	}
	return c.peerColors(in.Hash, catalog.ColoursHash(), catalog.ProfileColours()), nil
}

// "Not modified" only for the list's own hash. The stub this replaces said it
// to a client asking with zero, and Android then never stopped waiting.
func (c *AppearanceCore) peerColors(asked, hash int32, colours []*mtproto.Help_PeerColorOption) *mtproto.Help_PeerColors {
	if asked != 0 && asked == hash {
		return mtproto.MakeTLHelpPeerColorsNotModified(nil).To_Help_PeerColors()
	}
	return mtproto.MakeTLHelpPeerColors(&mtproto.Help_PeerColors{Hash: hash, Colors: colours}).To_Help_PeerColors()
}

// AccountUpdateColor
// account.updateColor
//
// A person's name colour, or with for_profile their profile colour; for_profile
// with no colour takes the profile colour off. Three constructors, one meaning.
// The person's other phones are told at once; everybody else sees the colour
// on the user that comes with the next thing this person sends, because the
// user's cached copy is renewed as the colour is written.
func (c *AppearanceCore) AccountUpdateColor(in *mtproto.TLAccountUpdateColor) (*mtproto.Bool, error) {
	me := c.MD.UserId
	colour, emoji := colourAsked(in)
	// Seven name colours, 0..6, and eight profile colours, 0..7.
	if colour < 0 {
		colour = noColour
	} else if in.ForProfile && colour > 7 || !in.ForProfile && colour > 6 {
		c.Logger.Infof("account.updateColor - %d asked for colour %d, which is not offered", me, colour)
		return mtproto.BoolTrue, nil
	}
	if _, err := c.svcCtx.Users.UserSetColor(c.ctx, &userpb.TLUserSetColor{
		UserId:            me,
		ForProfile:        in.ForProfile,
		Color:             colour,
		BackgroundEmojiId: emoji,
	}); err != nil {
		c.Logger.Errorf("account.updateColor - keeping colour %d for %d: %v", colour, me, err)
		return nil, mtproto.ErrInternalServerError
	}

	users, err := c.svcCtx.Users.UserGetMutableUsers(c.ctx, &userpb.TLUserGetMutableUsers{Id: []int64{me}, To: []int64{me}})
	if err != nil {
		c.Logger.Errorf("account.updateColor - reading %d back: %v", me, err)
		return mtproto.BoolTrue, nil
	}
	if _, err = c.svcCtx.Sync.SyncUpdatesNotMe(c.ctx, &sync.TLSyncUpdatesNotMe{
		UserId:        me,
		PermAuthKeyId: c.MD.PermAuthKeyId,
		Updates: mtproto.MakeUpdatesByUpdatesUsers(users.GetUserListByIdList(me, me),
			mtproto.MakeTLUpdateUser(&mtproto.Update{UserId: me}).To_Update()),
	}); err != nil {
		c.Logger.Errorf("account.updateColor - telling the other phones of %d: %v", me, err)
	}
	c.Logger.Infof("account.updateColor - %d chose colour %d (profile: %v)", me, colour, in.ForProfile)
	return mtproto.BoolTrue, nil
}

func colourAsked(in *mtproto.TLAccountUpdateColor) (int32, int64) {
	switch in.Constructor {
	case mtproto.CRC32_account_updateColor_7cefa15d:
		if in.Color_FLAGINT32 == nil {
			return noColour, in.BackgroundEmojiId.GetValue()
		}
		return in.Color_FLAGINT32.Value, in.BackgroundEmojiId.GetValue()
	case mtproto.CRC32_account_updateColor_a001cc43:
		return in.Color_INT32, in.BackgroundEmojiId.GetValue()
	}
	peerColor := in.Color_FLAGPEERCOLOR
	if peerColor == nil {
		return noColour, 0
	}
	colour := int32(noColour)
	if peerColor.Color != nil {
		colour = peerColor.Color.Value
	}
	return colour, peerColor.BackgroundEmojiId_FLAGINT64.GetValue()
}

// AccountGetDefaultBackgroundEmojis
// account.getDefaultBackgroundEmojis
//
// The emoji a colour can carry come from emoji packs, which are not offered
// (#20). A colour needs none.
func (c *AppearanceCore) AccountGetDefaultBackgroundEmojis(in *mtproto.TLAccountGetDefaultBackgroundEmojis) (*mtproto.EmojiList, error) {
	return mtproto.MakeTLEmojiList(&mtproto.EmojiList{Hash: 0, DocumentId: []int64{}}).To_EmojiList(), nil
}

// ChannelsUpdateColor
// channels.updateColor
//
// There are no channels (#16).
func (c *AppearanceCore) ChannelsUpdateColor(in *mtproto.TLChannelsUpdateColor) (*mtproto.Updates, error) {
	return nil, mtproto.ErrChannelInvalid
}
