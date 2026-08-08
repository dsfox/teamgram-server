package bff_proxy_client

import (
	"context"
	"os"
	"reflect"
	"regexp"
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// Every stub answer must survive encoding, not merely compile.
//
// This is not hypothetical: the stub for stories.getAllStories built an object
// whose stealth_mode field is mandatory on the wire. It compiled, it looked
// right, and it panicked the moment the server tried to encode it. The request
// was handled, the answer never left, and the client waited for it forever —
// from outside an endless "Updating" on a cold start, with no error anywhere
// near the cause.
//
// The panic is caught by the framework, so the server keeps running and the only
// trace is a stack deep in the log. Hence this test: it encodes what every stub
// returns, exactly as the session code does.
func TestStubAnswersEncode(t *testing.T) {
	// The switch never touches the receiver for these methods.
	var proxy *BFFProxyClient

	for _, request := range stubRequests() {
		name := typeName(request)
		t.Run(name, func(t *testing.T) {
			answer, err := proxy.TryReturnFakeRpcResult(context.Background(), request)
			if refusesOnPurpose[reflect.TypeOf(request).Elem().Name()] {
				if err == nil {
					t.Fatalf("expected a refusal, got an answer: this method is listed "+
						"as one where an error is the truthful reply")
				}
				return
			}
			if err != nil {
				t.Fatalf("no stub answer, the client would get an error and retry in a loop: %v", err)
			}
			if answer == nil {
				t.Fatal("stub answers with nothing")
			}

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("encoding panics, so the answer never reaches the client: %v", r)
				}
			}()

			x := mtproto.GetEncodeBuf()
			defer mtproto.PutEncodeBuf(x)
			answer.Encode(x, 227)

			if x.GetOffset() == 0 {
				t.Fatal("encoded to nothing")
			}
		})
	}
}

// Where an error is the answer, not a failure to give one.
//
// The rule above holds for anything the client asks unconditionally at startup:
// an error there stalls its service queue. It does not hold when the client asks
// about a particular thing and we simply do not have it. Both clients catch this
// one and move on - iOS through `catch` in LoadedStickerPack and
// StickerManagement, Android by its instanceof check failing - while the
// friendly-looking "not modified" crashed Android outright, because there
// TL_messages_stickerSetNotModified extends TL_messages_stickerSet.
//
// Keyed by the request's Go type: an empty request prints as nothing, so the
// subtest name cannot identify it.
var refusesOnPurpose = map[string]bool{
	"TLMessagesGetStickerSet": true,
}

// Everything the client asks for at startup and gets a stub answer to.
func stubRequests() []mtproto.TLObject {
	return []mtproto.TLObject{
		&mtproto.TLMessagesGetTopReactions{},
		&mtproto.TLMessagesGetRecentReactions{},
		&mtproto.TLMessagesGetDefaultTagReactions{},
		&mtproto.TLAccountGetDefaultEmojiStatuses{},
		&mtproto.TLAccountGetRecentEmojiStatuses{},
		&mtproto.TLAccountGetChannelDefaultEmojiStatuses{},
		&mtproto.TLAccountGetCollectibleEmojiStatuses{},
		&mtproto.TLAccountGetDefaultProfilePhotoEmojis{},
		&mtproto.TLAccountGetDefaultGroupPhotoEmojis{},
		&mtproto.TLAccountGetDefaultBackgroundEmojis{},
		&mtproto.TLAccountGetChannelRestrictedStatusEmojis{},
		&mtproto.TLAccountGetReactionsNotifySettings{},
		&mtproto.TLAccountGetSavedRingtones{},
		&mtproto.TLAccountGetConnectedBots{},
		// English, because it needs no translation file: the others are read
		// from disk at runtime and a unit test has no such directory, so asking
		// for one here would test the environment rather than the stub.
		&mtproto.TLMessagesGetEmojiGameInfo{},
		&mtproto.TLPaymentsGetStarGiftCollections{},
		&mtproto.TLLangpackGetLanguage{LangCode: "en"},
		&mtproto.TLChannelsGetChannelRecommendations{},
		&mtproto.TLStoriesGetPeerMaxIDs78499170{},
		&mtproto.TLPaymentsGetStarsStatus{},
		&mtproto.TLPaymentsGetStarGiftActiveAuctions{},
		&mtproto.TLPaymentsGetStarGifts{},
		&mtproto.TLPaymentsGetSavedStarGifts{},
		&mtproto.TLPaymentsGetPremiumGiftCodeOptions{},
		&mtproto.TLStoriesGetAllStories{},
		&mtproto.TLStoriesGetPeerStories{},
		&mtproto.TLStoriesGetPinnedStories{},
		&mtproto.TLStoriesGetAlbums{},
		&mtproto.TLStoriesGetAllReadPeerStories{},
		&mtproto.TLHelpGetPeerColors{},
		&mtproto.TLHelpGetPeerProfileColors{},
		&mtproto.TLMessagesGetAttachMenuBots{},
		&mtproto.TLMessagesGetAvailableEffects{},
		&mtproto.TLMessagesGetEmojiGroups{},
		&mtproto.TLMessagesGetEmojiProfilePhotoGroups{},
		&mtproto.TLMessagesGetEmojiStatusGroups{},
		&mtproto.TLMessagesGetEmojiStickerGroups{},
		&mtproto.TLMessagesGetSuggestedDialogFilters{},
		&mtproto.TLMessagesGetScheduledHistory{},
		&mtproto.TLMessagesGetStickerSet{},
		&mtproto.TLChannelsGetAdminedPublicChannels{},
		&mtproto.TLHelpGetPremiumPromo{},
		&mtproto.TLLangpackGetDifference{},
		&mtproto.TLLangpackGetLangPack{},
		&mtproto.TLLangpackGetLanguages{},
		&mtproto.TLLangpackGetStrings{},
		&mtproto.TLMessagesGetWebPage32CA8F91{},
		&mtproto.TLMessagesGetWebPage8D9692A3{},
		&mtproto.TLAccountGetWallPapers{},
		&mtproto.TLAccountGetPassword{},
		&mtproto.TLHelpAcceptTermsOfService{},
		&mtproto.TLHelpGetTermsOfServiceUpdate{},
		&mtproto.TLAccountGetThemes{},
		&mtproto.TLAccountGetChatThemes{},
		&mtproto.TLMessagesGetAllStickers{},
		&mtproto.TLMessagesGetArchivedStickers{},
		&mtproto.TLMessagesGetFavedStickers{},
		&mtproto.TLMessagesGetMaskStickers{},
		&mtproto.TLMessagesGetEmojiStickers{},
		&mtproto.TLMessagesGetOldFeaturedStickers{},
		&mtproto.TLMessagesGetRecentStickers{},
		&mtproto.TLMessagesGetStickers{},
		&mtproto.TLMessagesGetFeaturedStickers{},
		&mtproto.TLMessagesGetFeaturedEmojiStickers{},
		&mtproto.TLMessagesGetScheduledMessages{},
		&mtproto.TLMessagesGetAvailableReactions{},
		&mtproto.TLMessagesGetDialogFiltersEFD48C89{},
		&mtproto.TLMessagesGetDialogFiltersF19ED96D{},
		&mtproto.TLMessagesGetSavedGifs{},
		&mtproto.TLMessagesSaveGif{},
		&mtproto.TLHelpGetPromoData{},
		&mtproto.TLHelpHidePromoData{},
		&mtproto.TLMessagesGetEmojiKeywords{},
		&mtproto.TLMessagesGetEmojiKeywordsDifference{},
		&mtproto.TLMessagesGetEmojiKeywordsLanguages{},
		&mtproto.TLAccountReportPeer{},
		&mtproto.TLAccountReportProfilePhoto{},
		&mtproto.TLChannelsReportSpam{},
		&mtproto.TLMessagesReport8953AB4E{},
		&mtproto.TLMessagesReportFC78AF9B{},
		&mtproto.TLMessagesReportSpam{},
		&mtproto.TLPhoneGetCallConfig{},
		&mtproto.TLAccountGetWebAuthorizations{},
		&mtproto.TLHelpTest{},
	}
}

// Encoding proves the answer is well formed; it says nothing about whether it is
// the right answer. messages.getDialogFilters once returned a bare vector where
// the wrapper belonged, and messages.getWebPage a bare webPageEmpty where the
// newer version of the method wants messages.WebPage. Both encoded perfectly.
// Both left the client unable to decode what it got, retrying behind a stalled
// queue - "Updating" with no error anywhere near the cause.
//
// The proto package knows what each method answers with: rpcContextRegisters
// maps the request type to a constructor for the reply. Comparing against it
// turns a whole class of bug into a build failure.
func TestStubAnswersHaveTheRightType(t *testing.T) {
	var proxy *BFFProxyClient

	registers := mtproto.GetRPCContextRegisters()
	for _, request := range stubRequests() {
		name := reflect.TypeOf(request).Elem().Name()
		t.Run(name, func(t *testing.T) {
			if refusesOnPurpose[name] {
				return
			}
			tuple, ok := registers[name]
			if !ok {
				t.Skipf("%s is not in the proto registry, nothing to compare against", name)
			}

			answer, err := proxy.TryReturnFakeRpcResult(context.Background(), request)
			if err != nil {
				t.Fatalf("no stub answer: %v", err)
			}

			want := reflect.TypeOf(tuple.NewReplyFunc())
			if got := reflect.TypeOf(answer); got != want {
				t.Fatalf("answers with %s where the method returns %s - the client "+
					"cannot decode this and will retry forever", got, want)
			}
		})
	}
}

// A stub added without a line in the list above would never be encoded here,
// and the panic it can hide would reach the phone instead of the build. So the
// list is checked against the source of truth rather than kept by hand.
func TestEveryStubIsCovered(t *testing.T) {
	source, err := os.ReadFile("fake_rpc_result.go")
	if err != nil {
		t.Fatalf("cannot read the stubs: %v", err)
	}

	covered := make(map[string]bool)
	for _, r := range stubRequests() {
		covered[reflect.TypeOf(r).Elem().Name()] = true
	}

	for _, m := range regexp.MustCompile(`case "(TL\w+)"`).FindAllStringSubmatch(string(source), -1) {
		if !covered[m[1]] {
			t.Errorf("%s answers with a stub that no test ever encodes", m[1])
		}
	}
}

func typeName(o mtproto.TLObject) string {
	switch o.(type) {
	case *mtproto.TLStoriesGetAllStories:
		return "TLStoriesGetAllStories"
	default:
		return o.String()[:min(40, len(o.String()))]
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
