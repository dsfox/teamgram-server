package bff_proxy_client

import (
	"context"
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
	// Everything the client asks for at startup and gets a stub answer to.
	requests := []mtproto.TLObject{
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
	}

	// The switch never touches the receiver for these methods.
	var proxy *BFFProxyClient

	for _, request := range requests {
		name := typeName(request)
		t.Run(name, func(t *testing.T) {
			answer, err := proxy.TryReturnFakeRpcResult(context.Background(), request)
			if err != nil {
				t.Fatalf("no stub answer, the client would get an error and retry in a loop: %v", err)
			}
			if answer == nil {
				t.Fatal("stub answer is nil: the client would get nothing")
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
