package bff_proxy_client

import (
	"context"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/zeromicro/go-zero/core/logx"
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
			answer, err := proxy.TryReturnFakeRpcResult(context.Background(), nil, request)
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

// Everything the client asks for at startup and gets a stub answer to.
func stubRequests() []mtproto.TLObject {
	return []mtproto.TLObject{
		&mtproto.TLMessagesReportReadMetrics{},
		&mtproto.TLMessagesSetChatWallPaper{},
		&mtproto.TLMessagesGetDefaultTagReactions{},
		&mtproto.TLAccountGetDefaultEmojiStatuses{},
		&mtproto.TLAccountGetRecentEmojiStatuses{},
		&mtproto.TLAccountGetChannelDefaultEmojiStatuses{},
		&mtproto.TLAccountGetCollectibleEmojiStatuses{},
		&mtproto.TLAccountGetDefaultProfilePhotoEmojis{},
		&mtproto.TLAccountGetDefaultGroupPhotoEmojis{},
		&mtproto.TLAccountGetChannelRestrictedStatusEmojis{},
		&mtproto.TLAccountGetReactionsNotifySettings{},
		&mtproto.TLAccountGetSavedRingtones{},
		&mtproto.TLAccountGetConnectedBots{},
		&mtproto.TLAccountGetWebBrowserSettings{},
		&mtproto.TLCommunitiesGetJoinedCommunities{},
		&mtproto.TLAicomposeGetTones{},
		&mtproto.TLAccountGetPasskeys{},
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
		&mtproto.TLMessagesSearchEmojiStickerSets{},
		&mtproto.TLMessagesSearchStickers{},
		&mtproto.TLStoriesGetStoriesArchive{},
		&mtproto.TLStoriesCanSendStory30EB63F0{},
		&mtproto.TLStoriesCanSendStoryC7DFDFDD{},
		&mtproto.TLStoriesCanSendStoryB100D45D{},
		&mtproto.TLPhoneGetGroupCallJoinAs{},
		&mtproto.TLAccountInstallWallPaper{},
		&mtproto.TLMessagesGetWebPagePreview570D6F6F{},
		&mtproto.TLMessagesGetCustomEmojiDocuments{},
		&mtproto.TLAccountSetReactionsNotifySettings{},
		&mtproto.TLStoriesGetAlbums{},
		&mtproto.TLStoriesGetAllReadPeerStories{},
		&mtproto.TLMessagesGetAttachMenuBots{},
		&mtproto.TLMessagesGetAvailableEffects{},
		&mtproto.TLMessagesGetEmojiGroups{},
		&mtproto.TLMessagesGetEmojiProfilePhotoGroups{},
		&mtproto.TLMessagesGetEmojiStatusGroups{},
		&mtproto.TLMessagesGetEmojiStickerGroups{},
		&mtproto.TLMessagesGetSuggestedDialogFilters{},
		&mtproto.TLMessagesGetScheduledHistory{},
		&mtproto.TLMessagesGetQuickReplies{},
		&mtproto.TLMessagesGetSavedReactionTags{},
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
		&mtproto.TLAccountGetMultiWallPapers{},
		&mtproto.TLBotsGetBotRecommendations{},
		&mtproto.TLAccountGetPassword{},
		&mtproto.TLHelpAcceptTermsOfService{},
		&mtproto.TLHelpGetTermsOfServiceUpdate{},
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
			tuple, ok := registers[name]
			if !ok {
				t.Skipf("%s is not in the proto registry, nothing to compare against", name)
			}

			answer, err := proxy.TryReturnFakeRpcResult(context.Background(), nil, request)
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
	covered := make(map[string]bool)
	for _, r := range stubRequests() {
		covered[reflect.TypeOf(r).Elem().Name()] = true
	}
	registered := mtproto.GetRPCContextRegisters()

	// Ours, and upstream's table beside it (#198).
	for _, file := range []string{"ice9_fake_results.go", "fake_rpc_result.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("cannot read the stubs: %v", err)
		}
		// Every quoted method name in the switch, not only the one after `case`. A
		// case can list several - `case "A",\n"B":` - and matching the first hid the
		// other thirteen, one of which was answering with the wrong type entirely.
		for _, m := range regexp.MustCompile(`"(TL[A-Za-z0-9]+)"`).FindAllStringSubmatch(string(source), -1) {
			name := m[1]
			if covered[name] || answeredByAService[name] || refusedOnPurpose[name] != nil {
				continue
			}
			if _, known := registered[name]; !known {
				continue // upstream's case for a method this schema names otherwise; never matched
			}
			t.Errorf("%s answers with a stub that no test ever encodes", name)
		}
	}
}

// Upstream's table still holds stubs for these, and they are never reached: a
// service of ours is registered for each, and the table is consulted only for
// methods with none (app/bff/appearance, reactions, the passport service).
var answeredByAService = map[string]bool{
	"TLAccountGetThemes":              true,
	"TLAccountGetChatThemes":          true,
	"TLMessagesGetAvailableReactions": true,
	"TLAccountGetAuthorizations":      true,
}

// Stubs whose answer is an error of the method's own kind, not a refusal for
// want of a licence: the thing asked for does not exist here.
var refusedOnPurpose = map[string]error{
	"TLAccountGetWallPaper": mtproto.ErrWallpaperInvalid,
}

func TestARefusalOnPurposeIsTheMethodsOwnError(t *testing.T) {
	var proxy *BFFProxyClient
	requests := map[string]mtproto.TLObject{
		"TLAccountGetWallPaper": &mtproto.TLAccountGetWallPaper{},
	}
	for name, want := range refusedOnPurpose {
		request, ok := requests[name]
		if !ok {
			t.Fatalf("%s is refused on purpose and nothing here asks for it", name)
		}
		_, err := proxy.TryReturnFakeRpcResult(context.Background(), nil, request)
		if err != want {
			t.Errorf("%s answered %v, want %v", name, err, want)
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

// A report has to reach the operator, not merely return success to the client.
// The message content is end-to-end encrypted, so the report can only name who
// reported whom - which is enough to lock or ban the target by hand. Before
// this the stub answered BoolTrue and dropped the report on the floor. The
// operator watches the server log, so the report is a tagged line there; this
// proves the line is written and names both sides, and that the client is still
// answered.
func TestReportReachesTheOperator(t *testing.T) {
	var proxy *BFFProxyClient
	var buf strings.Builder
	logx.Reset()
	logx.SetWriter(logx.NewWriter(&buf))
	logx.SetLevel(logx.InfoLevel)
	defer logx.Reset()

	const reporter, target = int64(1001), int64(2002)
	peer := mtproto.MakeTLInputPeerUser(&mtproto.InputPeer{UserId: target}).To_InputPeer()
	md := &metadata.RpcMetadata{UserId: reporter}
	request := &mtproto.TLAccountReportPeer{Peer: peer, Message: "abuse"}

	answer, err := proxy.TryReturnFakeRpcResult(context.Background(), md, request)
	if err != nil || answer == nil {
		t.Fatalf("the report must still answer the client: answer=%v err=%v", answer, err)
	}
	line := buf.String()
	if !strings.Contains(line, "MODERATION_REPORT") ||
		!strings.Contains(line, "reporter=1001") ||
		!strings.Contains(line, "target=2002") {
		t.Fatalf("the report did not reach the operator log: %q", line)
	}
}
