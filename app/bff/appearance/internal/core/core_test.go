package core

import (
	"context"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/teamgram/teamgram-server/app/bff/appearance/internal/svc"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	"github.com/teamgram/teamgram-server/app/service/biz/dialog/dialog"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
	"github.com/teamgram/teamgram-server/pkg/appearance"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	alice = int64(2001)
	bob   = int64(2002)
)

// The dialogs of both sides, as dialog.setChatTheme keeps them.
type dialogs map[[2]int64]string

func (d dialogs) DialogGetDialogById(_ context.Context, in *dialog.TLDialogGetDialogById) (*dialog.DialogExt, error) {
	return &dialog.DialogExt{ThemeEmoticon: d[[2]int64{in.UserId, in.PeerId}]}, nil
}

func (d dialogs) DialogSetChatTheme(_ context.Context, in *dialog.TLDialogSetChatTheme) (*mtproto.Bool, error) {
	d[[2]int64{in.UserId, in.PeerId}] = in.ThemeEmoticon
	d[[2]int64{in.PeerId, in.UserId}] = in.ThemeEmoticon
	return mtproto.BoolTrue, nil
}

type sent struct{ messages []*msgpb.TLMsgSendMessageV2 }

func (s *sent) MsgSendMessageV2(_ context.Context, in *msgpb.TLMsgSendMessageV2) (*mtproto.Updates, error) {
	s.messages = append(s.messages, in)
	return mtproto.MakeUpdatesByUpdates(mtproto.MakeTLUpdateNewMessage(&mtproto.Update{Message_MESSAGE: in.Message[0].Message}).To_Update()), nil
}

type users struct {
	colours map[bool]int32
	emoji   map[bool]int64
}

func (u *users) UserSetColor(_ context.Context, in *userpb.TLUserSetColor) (*mtproto.Bool, error) {
	u.colours[in.ForProfile] = in.Color
	u.emoji[in.ForProfile] = in.BackgroundEmojiId
	return mtproto.BoolTrue, nil
}

func (u *users) UserGetMutableUsers(context.Context, *userpb.TLUserGetMutableUsers) (*userpb.Vector_ImmutableUser, error) {
	return &userpb.Vector_ImmutableUser{}, nil
}

type told struct{ notMe []*sync.TLSyncUpdatesNotMe }

func (t *told) SyncUpdatesNotMe(_ context.Context, in *sync.TLSyncUpdatesNotMe) (*mtproto.Void, error) {
	t.notMe = append(t.notMe, in)
	return mtproto.EmptyVoid, nil
}

type world struct {
	dialogs dialogs
	sent    *sent
	users   *users
	told    *told
	ctx     *svc.ServiceContext
}

func newWorld(t *testing.T) *world {
	catalog, err := appearance.Load("../../../../../teamgramd/appearance")
	if err != nil {
		t.Fatal(err)
	}
	w := &world{dialogs: dialogs{}, sent: &sent{}, users: &users{colours: map[bool]int32{}, emoji: map[bool]int64{}}, told: &told{}}
	w.ctx = &svc.ServiceContext{Catalog: catalog, Dialogs: w.dialogs, Users: w.users, Msg: w.sent, Sync: w.told}
	return w
}

func (w *world) as(who int64) *AppearanceCore {
	ctx := context.Background()
	return &AppearanceCore{ctx: ctx, svcCtx: w.ctx, Logger: logx.WithContext(ctx),
		MD: &metadata.RpcMetadata{UserId: who, PermAuthKeyId: who * 10}}
}

func user(id int64) *mtproto.InputPeer {
	return mtproto.MakeTLInputPeerUser(&mtproto.InputPeer{UserId: id}).To_InputPeer()
}

func themed(emoticon string) *mtproto.InputChatTheme {
	return mtproto.MakeTLInputChatTheme(&mtproto.InputChatTheme{Emoticon: emoticon}).To_InputChatTheme()
}

func (w *world) setTheme(who int64, peer *mtproto.InputPeer, theme *mtproto.InputChatTheme) *mtproto.Updates {
	answer, err := w.as(who).MessagesSetChatTheme(&mtproto.TLMessagesSetChatTheme{Peer: peer, Theme: theme})
	if err != nil {
		panic(err)
	}
	return answer
}

func TestAThemeIsKeptForBothAndTheOtherIsTold(t *testing.T) {
	w := newWorld(t)
	answer := w.setTheme(alice, user(bob), themed("🌿"))

	if w.dialogs[[2]int64{alice, bob}] != "🌿" || w.dialogs[[2]int64{bob, alice}] != "🌿" {
		t.Fatalf("both sides keep the theme: %v", w.dialogs)
	}
	if len(w.sent.messages) != 1 {
		t.Fatalf("one service message tells both phones, sent %d", len(w.sent.messages))
	}
	message := w.sent.messages[0]
	action := message.Message[0].Message.Action
	if message.UserId != alice || message.PeerId != bob || message.AuthKeyId != alice*10 {
		t.Fatalf("sent by Alice's phone into the chat with Bob: %v", message)
	}
	if action.GetPredicateName() != mtproto.Predicate_messageActionSetChatTheme ||
		action.Theme.GetEmoticon() != "🌿" || action.Emoticon != "🌿" {
		t.Fatalf("the action names the theme for new and old clients alike: %v", action)
	}
	if len(answer.Updates) != 1 {
		t.Fatalf("Alice is answered with the message as it was sent: %v", answer)
	}
}

func TestTheSameThemeAgainSaysNothing(t *testing.T) {
	w := newWorld(t)
	w.setTheme(alice, user(bob), themed("🌸"))
	w.setTheme(bob, user(alice), themed("🌸"))
	if len(w.sent.messages) != 1 {
		t.Fatalf("a chat that already has the theme is not told it again: %d messages", len(w.sent.messages))
	}
}

// iOS takes a theme off with an empty emoji, Android with inputChatThemeEmpty,
// an older client by leaving the theme out.
func TestEveryWayOfTakingAThemeOffTakesItOff(t *testing.T) {
	ways := map[string]*mtproto.TLMessagesSetChatTheme{
		"an empty emoji":      {Peer: user(bob), Theme: themed("")},
		"inputChatThemeEmpty": {Peer: user(bob), Theme: mtproto.MakeTLInputChatThemeEmpty(nil).To_InputChatTheme()},
		"no theme at all":     {Peer: user(bob), Emoticon: ""},
	}
	for name, request := range ways {
		w := newWorld(t)
		w.setTheme(alice, user(bob), themed("🐳"))
		if _, err := w.as(alice).MessagesSetChatTheme(request); err != nil {
			t.Fatal(err)
		}
		if w.dialogs[[2]int64{bob, alice}] != "" || len(w.sent.messages) != 2 {
			t.Fatalf("%s: the theme is off for both and both are told: %v, %d", name, w.dialogs, len(w.sent.messages))
		}
		if w.sent.messages[1].Message[0].Message.Action.Theme.GetEmoticon() != "" {
			t.Fatalf("%s: the message says there is no theme now", name)
		}
	}
}

func TestWhatIsNotOursChangesNothingAndIsStillAnswered(t *testing.T) {
	w := newWorld(t)
	group := mtproto.MakeTLInputPeerChat(&mtproto.InputPeer{ChatId: 77}).To_InputPeer()
	for name, request := range map[string]*mtproto.TLMessagesSetChatTheme{
		"a theme not in the list": {Peer: user(bob), Theme: themed("🤡")},
		"a gift theme":            {Peer: user(bob), Theme: mtproto.MakeTLInputChatThemeUniqueGift(&mtproto.InputChatTheme{Slug: "x"}).To_InputChatTheme()},
		"a group":                 {Peer: group, Theme: themed("🌿")},
		"the chat with oneself":   {Peer: mtproto.MakeTLInputPeerSelf(nil).To_InputPeer(), Theme: themed("🌿")},
	} {
		answer, err := w.as(alice).MessagesSetChatTheme(request)
		if err != nil || answer == nil {
			t.Fatalf("%s: an answer, not a refusal: %v", name, err)
		}
	}
	if len(w.dialogs) != 0 || len(w.sent.messages) != 0 {
		t.Fatalf("nothing kept, nobody told: %v, %d", w.dialogs, len(w.sent.messages))
	}
}

func TestTheListsAreWholeForAClientHoldingNothing(t *testing.T) {
	w := newWorld(t)
	c := w.as(alice)
	themes, _ := c.AccountGetChatThemes(&mtproto.TLAccountGetChatThemes{Hash: 0})
	if themes.GetPredicateName() != mtproto.Predicate_account_themes || len(themes.Themes) != 7 {
		t.Fatalf("asked with zero: the whole list, got %s with %d", themes.GetPredicateName(), len(themes.Themes))
	}
	again, _ := c.AccountGetChatThemes(&mtproto.TLAccountGetChatThemes{Hash: themes.Hash})
	if again.GetPredicateName() != mtproto.Predicate_account_themesNotModified {
		t.Fatalf("asked with the list's hash: %s", again.GetPredicateName())
	}
	gifts, _ := c.AccountGetUniqueGiftChatThemes(&mtproto.TLAccountGetUniqueGiftChatThemes{})
	if gifts.GetPredicateName() != mtproto.Predicate_account_chatThemesNotModified {
		t.Fatalf("no gifts, said as nothing new so Android's sheet finishes loading: %s", gifts.GetPredicateName())
	}
	for name, ask := range map[string]func(int32) (*mtproto.Help_PeerColors, error){
		"name":    func(h int32) (*mtproto.Help_PeerColors, error) { return c.HelpGetPeerColors(&mtproto.TLHelpGetPeerColors{Hash: h}) },
		"profile": func(h int32) (*mtproto.Help_PeerColors, error) { return c.HelpGetPeerProfileColors(&mtproto.TLHelpGetPeerProfileColors{Hash: h}) },
	} {
		colours, _ := ask(0)
		if colours.GetPredicateName() != mtproto.Predicate_help_peerColors || len(colours.Colors) == 0 {
			t.Fatalf("%s colours asked with zero: the list, got %s", name, colours.GetPredicateName())
		}
		if again, _ := ask(colours.Hash); again.GetPredicateName() != mtproto.Predicate_help_peerColorsNotModified {
			t.Fatalf("%s colours asked with their hash: %s", name, again.GetPredicateName())
		}
	}
}

func colourRequest(forProfile bool, colour *int32) *mtproto.TLAccountUpdateColor {
	in := &mtproto.TLAccountUpdateColor{Constructor: mtproto.CRC32_account_updateColor_684d214e, ForProfile: forProfile}
	if colour != nil {
		in.Color_FLAGPEERCOLOR = mtproto.MakeTLPeerColor(&mtproto.PeerColor{Color: &wrapperspb.Int32Value{Value: *colour}}).To_PeerColor()
	}
	return in
}

func TestRedIsKeptAndAProfileColourCanBeTakenOff(t *testing.T) {
	w := newWorld(t)
	red, grey := int32(0), int32(7)
	if _, err := w.as(alice).AccountUpdateColor(colourRequest(false, &red)); err != nil {
		t.Fatal(err)
	}
	if got, ok := w.users.colours[false]; !ok || got != 0 {
		t.Fatalf("red is colour 0 and is kept as 0: %v", w.users.colours)
	}
	if len(w.told.notMe) != 1 || w.told.notMe[0].PermAuthKeyId != alice*10 {
		t.Fatalf("Alice's other phones are told: %v", w.told.notMe)
	}

	w.as(alice).AccountUpdateColor(colourRequest(true, &grey))
	if w.users.colours[true] != 7 {
		t.Fatalf("the eighth profile colour is offered: %v", w.users.colours)
	}
	w.as(alice).AccountUpdateColor(colourRequest(true, nil))
	if w.users.colours[true] != noColour {
		t.Fatalf("for_profile with no colour takes it off: %v", w.users.colours)
	}

	w.users.colours = map[bool]int32{}
	w.as(alice).AccountUpdateColor(colourRequest(false, &grey))
	if _, ok := w.users.colours[false]; ok {
		t.Fatal("there are seven name colours; an eighth is not kept")
	}
}
