// Copyright 2022 Teamgram Authors
//  All rights reserved.
//
// Author: Benqi (wubenqi@gmail.com)
//

package bff_proxy_client

import (
	"context"
	"reflect"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/crypto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/teamgram/teamgram-server/pkg/langpack"

	"github.com/zeromicro/go-zero/core/logx"
)

var (
	gNewAlgoSalt1 = []byte{0xEC, 0xF8, 0x73, 0x76, 0x65, 0xBC, 0x77, 0x5A}
	gNewAlgoSalt2 = []byte{0xBE, 0xDE, 0x48, 0x88, 0x8C, 0x0F, 0x42, 0xAC, 0x34, 0xFF, 0xD1, 0xD4, 0x93, 0x5D, 0x8B, 0x21}

	gNewAlgoP = []byte{
		0xc7, 0x1c, 0xae, 0xb9, 0xc6, 0xb1, 0xc9, 0x04, 0x8e, 0x6c, 0x52, 0x2f,
		0x70, 0xf1, 0x3f, 0x73, 0x98, 0x0d, 0x40, 0x23, 0x8e, 0x3e, 0x21, 0xc1,
		0x49, 0x34, 0xd0, 0x37, 0x56, 0x3d, 0x93, 0x0f, 0x48, 0x19, 0x8a, 0x0a,
		0xa7, 0xc1, 0x40, 0x58, 0x22, 0x94, 0x93, 0xd2, 0x25, 0x30, 0xf4, 0xdb,
		0xfa, 0x33, 0x6f, 0x6e, 0x0a, 0xc9, 0x25, 0x13, 0x95, 0x43, 0xae, 0xd4,
		0x4c, 0xce, 0x7c, 0x37, 0x20, 0xfd, 0x51, 0xf6, 0x94, 0x58, 0x70, 0x5a,
		0xc6, 0x8c, 0xd4, 0xfe, 0x6b, 0x6b, 0x13, 0xab, 0xdc, 0x97, 0x46, 0x51,
		0x29, 0x69, 0x32, 0x84, 0x54, 0xf1, 0x8f, 0xaf, 0x8c, 0x59, 0x5f, 0x64,
		0x24, 0x77, 0xfe, 0x96, 0xbb, 0x2a, 0x94, 0x1d, 0x5b, 0xcd, 0x1d, 0x4a,
		0xc8, 0xcc, 0x49, 0x88, 0x07, 0x08, 0xfa, 0x9b, 0x37, 0x8e, 0x3c, 0x4f,
		0x3a, 0x90, 0x60, 0xbe, 0xe6, 0x7c, 0xf9, 0xa4, 0xa4, 0xa6, 0x95, 0x81,
		0x10, 0x51, 0x90, 0x7e, 0x16, 0x27, 0x53, 0xb5, 0x6b, 0x0f, 0x6b, 0x41,
		0x0d, 0xba, 0x74, 0xd8, 0xa8, 0x4b, 0x2a, 0x14, 0xb3, 0x14, 0x4e, 0x0e,
		0xf1, 0x28, 0x47, 0x54, 0xfd, 0x17, 0xed, 0x95, 0x0d, 0x59, 0x65, 0xb4,
		0xb9, 0xdd, 0x46, 0x58, 0x2d, 0xb1, 0x17, 0x8d, 0x16, 0x9c, 0x6b, 0xc4,
		0x65, 0xb0, 0xd6, 0xff, 0x9c, 0xa3, 0x92, 0x8f, 0xef, 0x5b, 0x9a, 0xe4,
		0xe4, 0x18, 0xfc, 0x15, 0xe8, 0x3e, 0xbe, 0xa0, 0xf8, 0x7f, 0xa9, 0xff,
		0x5e, 0xed, 0x70, 0x05, 0x0d, 0xed, 0x28, 0x49, 0xf4, 0x7b, 0xf9, 0x59,
		0xd9, 0x56, 0x85, 0x0c, 0xe9, 0x29, 0x85, 0x1f, 0x0d, 0x81, 0x15, 0xf6,
		0x35, 0xb1, 0x05, 0xee, 0x2e, 0x4e, 0x15, 0xd0, 0x4b, 0x24, 0x54, 0xbf,
		0x6f, 0x4f, 0xad, 0xf0, 0x34, 0xb1, 0x04, 0x03, 0x11, 0x9c, 0xd8, 0xe3,
		0xb9, 0x2f, 0xcc, 0x5b,
	}

	gNewAlgoG = int32(3)

	// salt: 7D 04 B3 4B 94 82 8C 3D [8 BYTES],
	gNewSecureAlgoSalt = []byte{0x7D, 0x04, 0xB3, 0x4B, 0x94, 0x82, 0x8C, 0x3D}
)

var (
	gNewAlgo       *mtproto.PasswordKdfAlgo
	gNewSecureAlgo *mtproto.SecurePasswordKdfAlgo
)

func init() {
	gNewAlgo = mtproto.MakeTLPasswordKdfAlgoModPow(&mtproto.PasswordKdfAlgo{
		Salt1: gNewAlgoSalt1,
		Salt2: gNewAlgoSalt2,
		G:     gNewAlgoG,
		P:     gNewAlgoP,
	}).To_PasswordKdfAlgo()

	gNewSecureAlgo = mtproto.MakeTLSecurePasswordKdfAlgoPBKDF2(&mtproto.SecurePasswordKdfAlgo{
		Salt: gNewSecureAlgoSalt,
	}).To_SecurePasswordKdfAlgo()
}

// platformOf answers which language pack the caller needs. The request field is
// authoritative when the client fills it in; when it does not - and the Android
// client never does - the connection already told us who it is.
//
// The metadata is passed in rather than read from the context: these answers are
// produced before any gRPC call is made, so the context carries no incoming
// metadata to read.
func platformOf(md *metadata.RpcMetadata, langPack string) string {
	if langPack != "" {
		return langPack
	}
	if md != nil {
		return md.GetClient()
	}
	return ""
}

func (c *BFFProxyClient) TryReturnFakeRpcResult(ctx context.Context, md *metadata.RpcMetadata, object mtproto.TLObject) (mtproto.TLObject, error) {
	rt := reflect.TypeOf(object)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	switch rt.Name() {
	// langpack
	// The platform decides which file answers: the same language is a different
	// set of keys on iOS and on Android. lang_pack is where the client is meant
	// to say which it is, and the Android client leaves it empty - so the answer
	// came from the iOS file, eleven thousand strings of keys it had never heard
	// of. What it does say, in every request, is who it is: the metadata carries
	// client "android".
	case "TLLangpackGetDifference":
		in := object.(*mtproto.TLLangpackGetDifference)
		return langpack.Difference(in.GetLangCode(), platformOf(md, in.GetLangPack()), in.GetFromVersion()), nil
	case "TLLangpackGetLangPack":
		in := object.(*mtproto.TLLangpackGetLangPack)
		// getLangPack is the whole thing by definition: no version to start from.
		return langpack.Difference(in.GetLangCode(), platformOf(md, in.GetLangPack()), 0), nil
	case "TLMessagesGetEmojiGameInfo":
		// There is no dice game here; saying so plainly beats an error the
		// client retries against.
		return mtproto.MakeTLMessagesEmojiGameUnavailable(&mtproto.Messages_EmojiGameInfo{}).To_Messages_EmojiGameInfo(), nil

	case "TLPaymentsGetStarGiftCollections":
		return mtproto.MakeTLPaymentsStarGiftCollectionsNotModified(&mtproto.Payments_StarGiftCollections{
			//
		}).To_Payments_StarGiftCollections(), nil

	case "TLLangpackGetLanguage":
		in := object.(*mtproto.TLLangpackGetLanguage)
		if language := langpack.LanguageByCode(in.GetLangCode()); language != nil {
			return language, nil
		}
		return nil, mtproto.ErrLangPackInvalid
	case "TLLangpackGetLanguages":
		// An empty list left the language picker spinning forever.
		return &mtproto.Vector_LangPackLanguage{
			Datas: langpack.Languages(),
		}, nil
	case "TLLangpackGetStrings":
		in := object.(*mtproto.TLLangpackGetStrings)
		return &mtproto.Vector_LangPackString{
			Datas: langpack.Strings(in.GetLangCode(), platformOf(md, in.GetLangPack()), in.GetKeys()),
		}, nil

	// webpage
	// The two versions of this method do not answer with the same type, and the
	// registry in the proto package is the one place that says so. The newer one
	// wants the wrapper, which carries the page along with the chats and users it
	// mentions; sending it a bare webPageEmpty made the Android client say
	// "can't parse magic 211a1788" while iOS dropped the answer without a word.
	case "TLMessagesGetWebPage32CA8F91":
		return mtproto.MakeTLWebPageEmpty(&mtproto.WebPage{Id: 0}).To_WebPage(), nil

	case "TLMessagesGetWebPage8D9692A3":
		return mtproto.MakeTLMessagesWebPage(&mtproto.Messages_WebPage{
			Webpage: mtproto.MakeTLWebPageEmpty(&mtproto.WebPage{Id: 0}).To_WebPage(),
			Chats:   []*mtproto.Chat{},
			Users:   []*mtproto.User{},
		}).To_Messages_WebPage(), nil
	// wallpaper
	case "TLAccountGetWallPapers":
		return mtproto.MakeTLAccountWallPapers(&mtproto.Account_WallPapers{
			Hash:       0,
			Wallpapers: []*mtproto.WallPaper{},
		}).To_Account_WallPapers(), nil

	// twofa
	case "TLAccountGetPassword":
		return mtproto.MakeTLAccountPassword(&mtproto.Account_Password{
			HasRecovery:             false,
			HasSecureValues:         false,
			HasPassword:             false,
			CurrentAlgo:             nil,
			Srp_B:                   nil,
			SrpId:                   nil,
			Hint:                    nil,
			EmailUnconfirmedPattern: nil,
			NewAlgo:                 gNewAlgo,
			NewSecureAlgo:           gNewSecureAlgo,
			SecureRandom:            crypto.RandomBytes(256),
		}).To_Account_Password(), nil

	// tos
	case "TLHelpAcceptTermsOfService":
		return mtproto.BoolTrue, nil
	case "TLHelpGetTermsOfServiceUpdate":
		return mtproto.MakeTLHelpTermsOfServiceUpdateEmpty(&mtproto.Help_TermsOfServiceUpdate{
			Expires: int32(time.Now().Unix() + 3600),
		}).To_Help_TermsOfServiceUpdate(), nil

	// themes
	case "TLAccountGetThemes":
		return mtproto.MakeTLAccountThemes(&mtproto.Account_Themes{
			Hash:   0,
			Themes: []*mtproto.Theme{},
		}).To_Account_Themes(), nil
	case "TLAccountGetChatThemes":
		return mtproto.MakeTLAccountThemes(&mtproto.Account_Themes{
			Hash:   0,
			Themes: []*mtproto.Theme{},
		}).To_Account_Themes(), nil

	// stickers
	case "TLMessagesGetAllStickers":
		return mtproto.MakeTLMessagesAllStickers(&mtproto.Messages_AllStickers{
			Hash: 0,
			Sets: []*mtproto.StickerSet{},
		}).To_Messages_AllStickers(), nil
	case "TLMessagesGetArchivedStickers":
		return mtproto.MakeTLMessagesArchivedStickers(&mtproto.Messages_ArchivedStickers{
			Count: 0,
			Sets:  []*mtproto.StickerSetCovered{},
		}).To_Messages_ArchivedStickers(), nil
	case "TLMessagesGetFavedStickers":
		return mtproto.MakeTLMessagesFavedStickers(&mtproto.Messages_FavedStickers{
			Hash:     0,
			Packs:    []*mtproto.StickerPack{},
			Stickers: []*mtproto.Document{},
		}).To_Messages_FavedStickers(), nil
	case "TLMessagesGetMaskStickers":
		fallthrough
	case "TLMessagesGetEmojiStickers":
		return mtproto.MakeTLMessagesAllStickers(&mtproto.Messages_AllStickers{
			Hash: 0,
			Sets: []*mtproto.StickerSet{},
		}).To_Messages_AllStickers(), nil
	case "TLMessagesGetOldFeaturedStickers":
		return mtproto.MakeTLMessagesFeaturedStickers(&mtproto.Messages_FeaturedStickers{
			Count:  0,
			Hash:   0,
			Sets:   []*mtproto.StickerSetCovered{},
			Unread: []int64{},
		}).To_Messages_FeaturedStickers(), nil
	case "TLMessagesGetRecentStickers":
		return mtproto.MakeTLMessagesRecentStickers(&mtproto.Messages_RecentStickers{
			Hash:     0,
			Packs:    []*mtproto.StickerPack{},
			Stickers: []*mtproto.Document{},
			Dates:    []int32{},
		}).To_Messages_RecentStickers(), nil
	case "TLMessagesGetStickers":
		return mtproto.MakeTLMessagesStickers(&mtproto.Messages_Stickers{
			Hash:     0,
			Stickers: []*mtproto.Document{},
		}).To_Messages_Stickers(), nil
	case "TLMessagesGetFeaturedStickers":
		fallthrough
	case "TLMessagesGetFeaturedEmojiStickers":
		return mtproto.MakeTLMessagesFeaturedStickers(&mtproto.Messages_FeaturedStickers{
			Count:  0,
			Hash:   0,
			Sets:   []*mtproto.StickerSetCovered{},
			Unread: []int64{},
		}).To_Messages_FeaturedStickers(), nil
	//case "TLMessagesGetStickerSet":
	//	// logx.WithContext(ct)
	//	return nil, mtproto.ErrMethodNotImpl
	//	// return mtproto.MakeTLMessagesStickerSetNotModified(&mtproto.Messages_StickerSet{}).To_Messages_StickerSet(), nil

	// 	scheduledmessages
	case "TLMessagesGetScheduledMessages":
		return mtproto.MakeTLMessagesMessages(&mtproto.Messages_Messages{
			Messages: []*mtproto.Message{},
			Chats:    []*mtproto.Chat{},
			Users:    []*mtproto.User{},
		}).To_Messages_Messages(), nil

	// reactions
	case "TLMessagesGetAvailableReactions":
		return mtproto.MakeTLMessagesAvailableReactions(&mtproto.Messages_AvailableReactions{
			Hash:      0,
			Reactions: []*mtproto.AvailableReaction{},
		}).To_Messages_AvailableReactions(), nil

	// folders
	case "TLMessagesGetDialogFiltersEFD48C89":
		// The newer form answers with a wrapper, not a bare vector. Handing back
		// a vector made the client fail to decode it - "Type constructor
		// 1cb5c415 not found" - and it asks for this at startup and keeps
		// retrying, so the whole service queue stalls behind it. One user sat on
		// connecting/updating for hours with 45 of these in four.
		return mtproto.MakeTLMessagesDialogFilters(&mtproto.Messages_DialogFilters{
			TagsEnabled: false,
			Filters:     []*mtproto.DialogFilter{},
		}).To_Messages_DialogFilters(), nil

	case "TLMessagesGetDialogFiltersF19ED96D":
		return &mtproto.Vector_DialogFilter{
			Datas: []*mtproto.DialogFilter{},
		}, nil

	// gifs
	case "TLMessagesGetSavedGifs":
		return mtproto.MakeTLMessagesSavedGifs(&mtproto.Messages_SavedGifs{
			Hash: 0,
			Gifs: []*mtproto.Document{},
		}).To_Messages_SavedGifs(), nil
	case "TLMessagesSaveGif":
		return mtproto.BoolTrue, nil

	// promodata
	case "TLHelpGetPromoData":
		return mtproto.MakeTLHelpPromoDataEmpty(&mtproto.Help_PromoData{
			Expires: int32(time.Now().Unix() + 60*60),
		}).To_Help_PromoData(), nil
	case "TLHelpHidePromoData":
		return mtproto.BoolTrue, nil

	// emoji
	case "TLMessagesGetEmojiKeywords":
		in := object.(*mtproto.TLMessagesGetEmojiKeywords)
		return mtproto.MakeTLEmojiKeywordsDifference(&mtproto.EmojiKeywordsDifference{
			LangCode:    in.LangCode,
			FromVersion: 0,
			Version:     0,
			Keywords:    []*mtproto.EmojiKeyword{},
		}).To_EmojiKeywordsDifference(), nil
	case "TLMessagesGetEmojiKeywordsDifference":
		in := object.(*mtproto.TLMessagesGetEmojiKeywordsDifference)
		return mtproto.MakeTLEmojiKeywordsDifference(&mtproto.EmojiKeywordsDifference{
			LangCode:    in.LangCode,
			FromVersion: in.FromVersion,
			Version:     in.FromVersion,
			Keywords:    []*mtproto.EmojiKeyword{},
		}).To_EmojiKeywordsDifference(), nil
	case "TLMessagesGetEmojiKeywordsLanguages":
		return &mtproto.Vector_EmojiLanguage{
			Datas: []*mtproto.EmojiLanguage{},
		}, nil

	// reports
	case "TLAccountReportPeer":
		return mtproto.BoolTrue, nil
	case "TLAccountReportProfilePhoto":
		return mtproto.BoolTrue, nil
	case "TLChannelsReportSpam":
		return mtproto.BoolTrue, nil
	// Same story as getWebPage: the older version answers Bool, the newer one a
	// ReportResult. Found by the type gate, not by a person.
	case "TLMessagesReport8953AB4E":
		return mtproto.BoolTrue, nil

	case "TLMessagesReportFC78AF9B":
		return mtproto.MakeTLReportResultReported(nil).To_ReportResult(), nil
	case "TLMessagesReportSpam":
		return mtproto.BoolTrue, nil

	// phone
	case "TLPhoneGetCallConfig":
		return mtproto.MakeTLDataJSON(&mtproto.DataJSON{
			Data: "{}",
		}).To_DataJSON(), nil

	case "TLAccountGetWebAuthorizations":
		return mtproto.MakeTLAccountWebAuthorizations(&mtproto.Account_WebAuthorizations{
			Authorizations: []*mtproto.WebAuthorization{},
			Users:          []*mtproto.User{},
		}).To_Account_WebAuthorizations(), nil

	// Decorations the client asks for at every start: reaction sets, emoji lists,
	// the star balance. We have none of it, but an error is not an acceptable
	// answer — the client retries the failed call in a loop and never finishes
	// its initial load, showing an endless "Updating". An empty answer it accepts
	// calmly.
	// An empty list, not "not modified": the client asks with hash 0 because it
	// holds nothing, and answering "unchanged" contradicts that. It then asks
	// again, and again - 431 repeats of a single method were measured on a live
	// phone, which is what made sending a photo take a minute.
	case "TLMessagesGetTopReactions",
		"TLMessagesGetRecentReactions",
		"TLMessagesGetDefaultTagReactions":
		return mtproto.MakeTLMessagesReactions(&mtproto.Messages_Reactions{
			Hash:      0,
			Reactions: []*mtproto.Reaction{},
		}).To_Messages_Reactions(), nil

	// Not messages.Reactions, however much the name suggests it: this one
	// answers with its own type, and being folded in with its neighbours is how
	// it came to send a shape the client could not read - "can't parse magic
	// eafdf716", said out loud by the Android client on the profile screen.
	case "TLMessagesGetSavedReactionTags":
		return mtproto.MakeTLMessagesSavedReactionTags(&mtproto.Messages_SavedReactionTags{
			Tags: []*mtproto.SavedReactionTag{},
			Hash: 0,
		}).To_Messages_SavedReactionTags(), nil

	case "TLAccountGetDefaultEmojiStatuses",
		"TLAccountGetRecentEmojiStatuses",
		"TLAccountGetChannelDefaultEmojiStatuses",
		"TLAccountGetCollectibleEmojiStatuses":
		return mtproto.MakeTLAccountEmojiStatuses(&mtproto.Account_EmojiStatuses{
			Hash:     0,
			Statuses: []*mtproto.EmojiStatus{},
		}).To_Account_EmojiStatuses(), nil

	case "TLAccountGetDefaultProfilePhotoEmojis",
		"TLAccountGetDefaultGroupPhotoEmojis",
		"TLAccountGetDefaultBackgroundEmojis",
		"TLAccountGetChannelRestrictedStatusEmojis":
		return mtproto.MakeTLEmojiList(&mtproto.EmojiList{
			Hash:       0,
			DocumentId: []int64{},
		}).To_EmojiList(), nil

	case "TLAccountGetReactionsNotifySettings":
		return mtproto.MakeTLReactionsNotifySettings(&mtproto.ReactionsNotifySettings{
			Sound:        mtproto.MakeTLNotificationSoundDefault(nil).To_NotificationSound(),
			ShowPreviews: mtproto.BoolTrue,
		}).To_ReactionsNotifySettings(), nil

	// Stars are an internal currency we do not have. A zero balance keeps the
	// client from asking again.
	case "TLPaymentsGetStarsStatus":
		return mtproto.MakeTLPaymentsStarsStatus(&mtproto.Payments_StarsStatus{
			Balance_STARSAMOUNT: mtproto.MakeTLStarsAmount(&mtproto.StarsAmount{
				Amount: 0,
				Nanos:  0,
			}).To_StarsAmount(),
			History: []*mtproto.StarsTransaction{},
			Chats:   []*mtproto.Chat{},
			Users:   []*mtproto.User{},
		}).To_Payments_StarsStatus(), nil

	case "TLPaymentsGetStarGiftActiveAuctions":
		return mtproto.MakeTLPaymentsStarGiftActiveAuctionsNotModified(nil).To_Payments_StarGiftActiveAuctions(), nil

	case "TLStoriesGetAllStories":
		// stealth_mode is not optional in the wire format: encoding a nil one
		// panics, the answer never leaves the server and the client waits for it
		// forever. That is exactly how an empty stub turned into an endless
		// "Updating" on a cold start.
		return mtproto.MakeTLStoriesAllStoriesNotModified(&mtproto.Stories_AllStories{
			State:       "",
			StealthMode: mtproto.MakeTLStoriesStealthMode(nil).To_StoriesStealthMode(),
		}).To_Stories_AllStories(), nil

	// Stories, ringtones, attachment bots, sticker sets, gift options: features we
	// do not have. Same reasoning as above — an error makes the client back off
	// and retry, blocking its service queue, and a file download can then wait
	// minutes behind it.
	case "TLStoriesGetPeerStories":
		return mtproto.MakeTLStoriesPeerStories(&mtproto.Stories_PeerStories{
			Stories: mtproto.MakeTLPeerStories(&mtproto.PeerStories{
				Peer:    mtproto.MakePeerUser(0),
				Stories: []*mtproto.StoryItem{},
			}).To_PeerStories(),
			Chats: []*mtproto.Chat{},
			Users: []*mtproto.User{},
		}).To_Stories_PeerStories(), nil

	case "TLStoriesGetPinnedStories":
		return mtproto.MakeTLStoriesStories(&mtproto.Stories_Stories{
			Count:   0,
			Stories: []*mtproto.StoryItem{},
			Chats:   []*mtproto.Chat{},
			Users:   []*mtproto.User{},
		}).To_Stories_Stories(), nil

	case "TLAccountGetSavedRingtones":
		return mtproto.MakeTLAccountSavedRingtonesNotModified(nil).To_Account_SavedRingtones(), nil

	case "TLAccountGetConnectedBots":
		return mtproto.MakeTLAccountConnectedBots(&mtproto.Account_ConnectedBots{
			ConnectedBots: []*mtproto.ConnectedBot{},
			Users:         []*mtproto.User{},
		}).To_Account_ConnectedBots(), nil

	case "TLHelpGetPeerColors", "TLHelpGetPeerProfileColors":
		return mtproto.MakeTLHelpPeerColorsNotModified(nil).To_Help_PeerColors(), nil

	case "TLMessagesGetAttachMenuBots":
		return mtproto.MakeTLAttachMenuBotsNotModified(nil).To_AttachMenuBots(), nil

	case "TLMessagesGetAvailableEffects":
		return mtproto.MakeTLMessagesAvailableEffectsNotModified(nil).To_Messages_AvailableEffects(), nil

	case "TLMessagesGetEmojiGroups",
		"TLMessagesGetEmojiProfilePhotoGroups",
		"TLMessagesGetEmojiStatusGroups",
		"TLMessagesGetEmojiStickerGroups":
		return mtproto.MakeTLMessagesEmojiGroupsNotModified(nil).To_Messages_EmojiGroups(), nil

	case "TLMessagesGetSuggestedDialogFilters":
		return &mtproto.Vector_DialogFilterSuggested{
			Datas: []*mtproto.DialogFilterSuggested{},
		}, nil

	case "TLPaymentsGetPremiumGiftCodeOptions":
		return &mtproto.Vector_PremiumGiftCodeOption{
			Datas: []*mtproto.PremiumGiftCodeOption{},
		}, nil

	case "TLPaymentsGetStarGifts":
		return mtproto.MakeTLPaymentsStarGiftsNotModified(nil).To_Payments_StarGifts(), nil

	case "TLPaymentsGetSavedStarGifts":
		return mtproto.MakeTLPaymentsSavedStarGifts(&mtproto.Payments_SavedStarGifts{
			Count: 0,
			Gifts: []*mtproto.SavedStarGift{},
			Chats: []*mtproto.Chat{},
			Users: []*mtproto.User{},
		}).To_Payments_SavedStarGifts(), nil

	// help.test is what the client sends to mark the end of an update poll. The
	// answer only has to arrive; an error here does not block the poll, but it
	// has no business being an error either.
	case "TLHelpTest":
		return mtproto.BoolTrue, nil

	case "TLHelpGetPremiumPromo":
		return mtproto.MakeTLHelpPremiumPromo(&mtproto.Help_PremiumPromo{
			StatusText:     "",
			StatusEntities: []*mtproto.MessageEntity{},
			VideoSections:  []string{},
			Videos:         []*mtproto.Document{},
			PeriodOptions:  []*mtproto.PremiumSubscriptionOption{},
			Users:          []*mtproto.User{},
		}).To_Help_PremiumPromo(), nil

	case "TLChannelsGetChannelRecommendations":
		// The client asks for these right after opening a chat and retries hard
		// on an error: 35 attempts in three hours from a single phone.
		return mtproto.MakeTLMessagesChats(&mtproto.Messages_Chats{
			Chats: []*mtproto.Chat{},
		}).To_Messages_Chats(), nil

	case "TLStoriesGetPeerMaxIDs78499170":
		return &mtproto.Vector_RecentStory{
			Datas: []*mtproto.RecentStory{},
		}, nil

	case "TLChannelsGetAdminedPublicChannels":
		return mtproto.MakeTLMessagesChats(&mtproto.Messages_Chats{
			Chats: []*mtproto.Chat{},
		}).To_Messages_Chats(), nil

	case "TLMessagesGetScheduledHistory":
		return mtproto.MakeTLMessagesMessagesNotModified(nil).To_Messages_Messages(), nil

	// Reactions are not implemented (#18), but the client asks for them as it
	// scrolls, and an error there stalls the queue the messages come down.
	case "TLMessagesGetMessagesReactions":
		return mtproto.MakeTLUpdates(&mtproto.Updates{
			Updates: []*mtproto.Update{},
			Users:   []*mtproto.User{},
			Chats:   []*mtproto.Chat{},
			Date:    int32(time.Now().Unix()),
		}).To_Updates(), nil

	// Quick replies are not implemented, and the client asks as it opens a chat.
	// An empty list is an answer; an error is a retry - the lesson getStickerSet
	// taught twice.
	case "TLMessagesGetQuickReplies":
		return mtproto.MakeTLMessagesQuickReplies(&mtproto.Messages_QuickReplies{
			QuickReplies: []*mtproto.QuickReply{},
			Messages:     []*mtproto.Message{},
			Chats:        []*mtproto.Chat{},
			Users:        []*mtproto.User{},
		}).To_Messages_QuickReplies(), nil

	case "TLMessagesGetStickerSet":
		// Third answer to this question, and the first that holds. We have no
		// sticker sets (#20), and the client asks for one at startup.
		//
		// stickerSetNotModified crashed the Android client: there
		// TL_messages_stickerSetNotModified extends TL_messages_stickerSet, so the
		// instanceof check passes and the dereference follows.
		//
		// STICKERSET_INVALID, which is what a real server says, looked safe - both
		// clients catch it where I read them - and put Android into a retry once a
		// second, forever, with its request queue stalled behind it and the app
		// stuck on "Connecting". There was a third call site I had not read.
		//
		// So: an empty set that is a real set. The client gets the shape it asks
		// for, finds nothing in it, and stops asking.
		return mtproto.MakeTLMessagesStickerSet(&mtproto.Messages_StickerSet{
			Set: mtproto.MakeTLStickerSet(&mtproto.StickerSet{
				Id:         0,
				AccessHash: 0,
				Title:      "",
				ShortName:  "",
				Count:      0,
				Hash:       0,
				Thumbs:     []*mtproto.PhotoSize{},
			}).To_StickerSet(),
			Packs:     []*mtproto.StickerPack{},
			Keywords:  []*mtproto.StickerKeyword{},
			Documents: []*mtproto.Document{},
		}).To_Messages_StickerSet(), nil

	case "TLStoriesGetAlbums":
		return mtproto.MakeTLStoriesAlbumsNotModified(nil).To_Stories_Albums(), nil

	case "TLStoriesGetAllReadPeerStories":
		return mtproto.MakeTLUpdates(&mtproto.Updates{
			Updates: []*mtproto.Update{},
			Users:   []*mtproto.User{},
			Chats:   []*mtproto.Chat{},
			Date:    int32(time.Now().Unix()),
		}).To_Updates(), nil
	}

	logx.WithContext(ctx).Errorf("%s blocked, License key from https://teamgram.net required to unlock enterprise features.", rt.Name())
	return nil, mtproto.ErrEnterpriseIsBlocked
}
