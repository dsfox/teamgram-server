package bff_proxy_client

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/teamgram/teamgram-server/pkg/langpack"
	"github.com/teamgram/teamgram-server/pkg/tones"

	"github.com/zeromicro/go-zero/core/logx"
)

// ice9FakeRpcResult is consulted first by upstream's TryReturnFakeRpcResult,
// which keeps upstream's table as upstream writes it (#198). It answers the
// methods ice9 answers differently or that upstream's table does not know, and
// says so: decided is false for everything else, which upstream's table then
// answers or refuses as before.
//
// Like upstream's table, this is reached only for methods no service is
// registered for (BFFProxyClient.InvokeContext).
func ice9FakeRpcResult(ctx context.Context, md *metadata.RpcMetadata, object mtproto.TLObject) (result mtproto.TLObject, decided bool, err error) {
	result, err = ice9FakeAnswer(ctx, md, object)
	if errors.Is(err, errNotIce9s) {
		return nil, false, nil
	}
	return result, true, err
}

var errNotIce9s = errors.New("not one of ice9's answers")

func ice9FakeAnswer(ctx context.Context, md *metadata.RpcMetadata, object mtproto.TLObject) (mtproto.TLObject, error) {
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

	// Themes and name colours are answered by app/bff/appearance (#23, #24).
	// A chat wallpaper of one's own is not offered, and Android clears it each
	// time a theme is applied: an answer, not the licence refusal that the
	// alerts count.
	case "TLMessagesSetChatWallPaper":
		return mtproto.MakeTLUpdates(&mtproto.Updates{
			Updates: []*mtproto.Update{},
			Users:   []*mtproto.User{},
			Chats:   []*mtproto.Chat{},
			Date:    int32(time.Now().Unix()),
		}).To_Updates(), nil

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

	// reports. The answer the client wants is a plain success; the substance is
	// the record left for the operator (recordReport), since the content is
	// end-to-end encrypted and the server can only say who reported whom.
	case "TLAccountReportPeer":
		in := object.(*mtproto.TLAccountReportPeer)
		recordReport(md, in.GetPeer().GetUserId(), "account.reportPeer", nil, in.GetMessage())
		return mtproto.BoolTrue, nil

	// Same story as getWebPage: the older version answers Bool, the newer one a
	// ReportResult. Found by the type gate, not by a person.
	case "TLMessagesReport8953AB4E":
		in := object.(*mtproto.TLMessagesReport8953AB4E)
		recordReport(md, in.GetPeer().GetUserId(), "messages.report", in.GetId(), in.GetMessage())
		return mtproto.BoolTrue, nil

	case "TLMessagesReportFC78AF9B":
		in := object.(*mtproto.TLMessagesReportFC78AF9B)
		recordReport(md, in.GetPeer().GetUserId(), "messages.report", in.GetId(), in.GetMessage())
		return mtproto.MakeTLReportResultReported(nil).To_ReportResult(), nil

	case "TLMessagesReportSpam":
		in := object.(*mtproto.TLMessagesReportSpam)
		recordReport(md, in.GetPeer().GetUserId(), "messages.reportSpam", nil, "")
		return mtproto.BoolTrue, nil

	// Which domains open outside the app. Nothing is stored, so the answer is
	// none in either direction - which is true. Clients from 12.9 on ask at
	// startup, and the refusal they used to get is what the health check counts
	// as a rejected method: the client backs off and the rest of its service
	// queue waits behind it. See ice9 #57.
	case "TLAccountGetWebBrowserSettings":
		return mtproto.MakeTLAccountWebBrowserSettings(&mtproto.Account_WebBrowserSettings{
			OpenExternalBrowser: false,
			DisplayCloseButton:  true,
			ExternalExceptions:  []*mtproto.WebDomainException{},
			InappExceptions:     []*mtproto.WebDomainException{},
			Hash:                0,
		}).To_Account_WebBrowserSettings(), nil

	// Decorations the client asks for at every start: reaction sets, emoji lists,
	// the star balance. We have none of it, but an error is not an acceptable
	// answer — the client retries the failed call in a loop and never finishes
	// its initial load, showing an endless "Updating". An empty answer it accepts
	// calmly.
	// An empty list, not "not modified": the client asks with hash 0 because it
	// holds nothing, and answering "unchanged" contradicts that. It then asks
	// again, and again - 431 repeats of a single method were measured on a live
	// phone, which is what made sending a photo take a minute.
	// The reaction sets themselves are answered by app/bff/reactions (#18).
	case "TLMessagesGetDefaultTagReactions":
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

	// Stories, attachment bots, sticker sets, gift options: features we
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

	// Doors that are shut in the apps and asked for anyway: the sticker
	// search behind the emoji panel, a story archive, whether a story may be
	// sent, who a group call joins as. Each was a licence refusal the health
	// check counted as an alarm (30 October: 40 in fifteen minutes from one
	// phone typing into the emoji search). Nothing is there, and an empty
	// answer says so (#294).
	case "TLMessagesSearchEmojiStickerSets":
		return mtproto.MakeTLMessagesFoundStickerSets(&mtproto.Messages_FoundStickerSets{
			Hash: 0,
			Sets: []*mtproto.StickerSetCovered{},
		}).To_Messages_FoundStickerSets(), nil

	case "TLMessagesSearchStickers":
		return mtproto.MakeTLMessagesFoundStickers(&mtproto.Messages_FoundStickers{
			Hash:     0,
			Stickers: []*mtproto.Document{},
		}).To_Messages_FoundStickers(), nil

	case "TLStoriesGetStoriesArchive":
		return mtproto.MakeTLStoriesStories(&mtproto.Stories_Stories{
			Count:   0,
			Stories: []*mtproto.StoryItem{},
			Chats:   []*mtproto.Chat{},
			Users:   []*mtproto.User{},
		}).To_Stories_Stories(), nil

	case "TLStoriesCanSendStory30EB63F0":
		return mtproto.MakeTLStoriesCanSendStoryCount(&mtproto.Stories_CanSendStoryCount{
			CountRemains: 0,
		}).To_Stories_CanSendStoryCount(), nil

	case "TLStoriesCanSendStoryC7DFDFDD", "TLStoriesCanSendStoryB100D45D":
		return mtproto.BoolFalse, nil

	case "TLPhoneGetGroupCallJoinAs":
		return mtproto.MakeTLPhoneJoinAsPeers(&mtproto.Phone_JoinAsPeers{
			Peers: []*mtproto.Peer{},
			Chats: []*mtproto.Chat{},
			Users: []*mtproto.User{},
		}).To_Phone_JoinAsPeers(), nil

	// A wallpaper by reference is one we never handed out; the client asked
	// for a thing that does not exist, which is the error its own type has.
	case "TLAccountGetWallPaper":
		return nil, mtproto.ErrWallpaperInvalid

	// Choosing a wallpaper is kept on the phone; the server has nothing to
	// remember it in.
	case "TLAccountInstallWallPaper":
		return mtproto.BoolTrue, nil

	// The link preview while typing. A preview would mean the server fetching
	// what the person is writing, so there is none, said the way Telegram
	// says it for text without a link.
	case "TLMessagesGetWebPagePreview570D6F6F":
		return mtproto.MakeTLMessagesWebPagePreview(&mtproto.Messages_WebPagePreview{
			Media: mtproto.MakeTLMessageMediaEmpty(nil).To_MessageMedia(),
			Chats: []*mtproto.Chat{},
			Users: []*mtproto.User{},
		}).To_Messages_WebPagePreview(), nil

	case "TLMessagesGetCustomEmojiDocuments":
		return &mtproto.Vector_Document{Datas: []*mtproto.Document{}}, nil

	// Reaction notification settings are not stored (the getter above answers
	// the defaults); what the phone sets is what it keeps.
	case "TLAccountSetReactionsNotifySettings":
		if settings := object.(*mtproto.TLAccountSetReactionsNotifySettings).GetSettings(); settings != nil {
			return settings, nil
		}
		return mtproto.MakeTLReactionsNotifySettings(&mtproto.ReactionsNotifySettings{
			Sound:        mtproto.MakeTLNotificationSoundDefault(nil).To_NotificationSound(),
			ShowPreviews: mtproto.BoolTrue,
		}).To_ReactionsNotifySettings(), nil

	case "TLStoriesGetPinnedStories":
		return mtproto.MakeTLStoriesStories(&mtproto.Stories_Stories{
			Count:   0,
			Stories: []*mtproto.StoryItem{},
			Chats:   []*mtproto.Chat{},
			Users:   []*mtproto.User{},
		}).To_Stories_Stories(), nil

	// The notification sounds are the three tones that ship in the binary
	// (pkg/tones, #282). A phone asking with hash 0 holds nothing, and iOS
	// draws Default and None only once it holds a list, so "not modified" is
	// only for a phone that already has this list.
	case "TLAccountGetSavedRingtones":
		if object.(*mtproto.TLAccountGetSavedRingtones).GetHash() == tones.Hash() {
			return mtproto.MakeTLAccountSavedRingtonesNotModified(nil).To_Account_SavedRingtones(), nil
		}
		return mtproto.MakeTLAccountSavedRingtones(&mtproto.Account_SavedRingtones{
			Hash:      tones.Hash(),
			Ringtones: tones.Documents(),
		}).To_Account_SavedRingtones(), nil

	case "TLAccountGetConnectedBots":
		return mtproto.MakeTLAccountConnectedBots(&mtproto.Account_ConnectedBots{
			ConnectedBots: []*mtproto.ConnectedBot{},
			Users:         []*mtproto.User{},
		}).To_Account_ConnectedBots(), nil

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

	// The 12.10 clients report how long a message was on screen whenever a chat
	// is read. We keep no such metrics; taking the report and dropping it is
	// the honest answer, and the refusal it replaced fired the health check
	// every fifteen minutes for a week (#159).
	case "TLMessagesReportReadMetrics":
		return mtproto.BoolTrue, nil

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

	// Three the carried clients ask for at startup, each of which was answered
	// with an error the client backs off from and retries behind. None of them
	// exists here: there are no communities, no composing tones and no
	// passkeys, and "none" is the true answer rather than a refusal. See #57.
	case "TLCommunitiesGetJoinedCommunities":
		return mtproto.MakeTLMessagesChats(&mtproto.Messages_Chats{
			Chats: []*mtproto.Chat{},
		}).To_Messages_Chats(), nil

	case "TLAicomposeGetTones":
		return mtproto.MakeTLAicomposeTones(&mtproto.Aicompose_Tones{
			Hash:  0,
			Tones: []*mtproto.AiComposeTone{},
			Users: []*mtproto.User{},
		}).To_Aicompose_Tones(), nil

	case "TLAccountGetPasskeys":
		return mtproto.MakeTLAccountPasskeys(&mtproto.Account_Passkeys{
			Passkeys: []*mtproto.Passkey{},
		}).To_Account_Passkeys(), nil

	case "TLChannelsGetChannelRecommendations":
		// The client asks for these right after opening a chat and retries hard
		// on an error: 35 attempts in three hours from a single phone.
		return mtproto.MakeTLMessagesChats(&mtproto.Messages_Chats{
			Chats: []*mtproto.Chat{},
		}).To_Messages_Chats(), nil

	case "TLBotsGetBotRecommendations":
		// Android asks whenever any profile opens, iOS about a bot's profile -
		// and iOS asks again without end when refused (#229). No bots here.
		return mtproto.MakeTLUsersUsers(&mtproto.Users_Users{
			Users: []*mtproto.User{},
		}).To_Users_Users(), nil

	case "TLAccountGetMultiWallPapers":
		// Android fetches the patterns of its theme accents after every full
		// answer to account.getThemes, so since it has our colour themes
		// (#223) this was refused on every launch (#229). The server holds no
		// wallpapers; an empty vector is "none of those".
		return &mtproto.Vector_WallPaper{
			Datas: []*mtproto.WallPaper{},
		}, nil

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
	return nil, errNotIce9s
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

// recordReport writes a moderation report where the operator will find it. The
// message content is end-to-end encrypted and never reaches the server, so the
// report can only name who reported whom and, for a message report, which
// message ids - enough to lock or ban the target by hand. It is a tagged log
// line on purpose: the operator watches the server log, and a durable table can
// follow if the volume ever asks for one (#181). The client is answered either
// way, which is all it waits for.
func recordReport(md *metadata.RpcMetadata, target int64, via string, messageIds []int32, note string) {
	var reporter int64
	if md != nil {
		reporter = md.UserId
	}
	logx.Infof("MODERATION_REPORT reporter=%d target=%d via=%s messages=%v note=%q",
		reporter, target, via, messageIds, note)
}
