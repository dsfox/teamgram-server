package core

import (
	"context"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/teamgram/teamgram-server/app/bff/reactions/internal/svc"
	sync_client "github.com/teamgram/teamgram-server/app/messenger/sync/client"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	chatpb "github.com/teamgram/teamgram-server/app/service/biz/chat/chat"
	chat_client "github.com/teamgram/teamgram-server/app/service/biz/chat/client"
	message_client "github.com/teamgram/teamgram-server/app/service/biz/message/client"
	"github.com/teamgram/teamgram-server/app/service/biz/message/message"
	"github.com/teamgram/teamgram-server/pkg/reactions"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	alice = int64(1001)
	bob   = int64(1002)
	carol = int64(1003)
	group = int64(900)
)

// The boxes of a chat between Alice and Bob and of a group of the three, each
// message numbered differently in every box, as the server numbers them.
type boxes struct {
	message_client.MessageClient
	byOwner map[[2]int64]*mtproto.MessageBox
}

func (b *boxes) add(owner int64, id int32, dialogMessageId int64, a, z int64) {
	b.byOwner[[2]int64{owner, int64(id)}] = mtproto.MakeTLMessageBox(&mtproto.MessageBox{
		UserId: owner, MessageId: id, DialogMessageId: dialogMessageId, DialogId1: a, DialogId2: z,
	}).To_MessageBox()
}

func (b *boxes) MessageGetUserMessage(_ context.Context, in *message.TLMessageGetUserMessage) (*mtproto.MessageBox, error) {
	if box, ok := b.byOwner[[2]int64{in.UserId, int64(in.Id)}]; ok {
		return box, nil
	}
	return nil, mtproto.ErrMessageIdInvalid
}

func (b *boxes) MessageGetUserMessageList(_ context.Context, in *message.TLMessageGetUserMessageList) (*message.Vector_MessageBox, error) {
	found := &message.Vector_MessageBox{}
	for _, id := range in.IdList {
		if box, ok := b.byOwner[[2]int64{in.UserId, int64(id)}]; ok {
			found.Datas = append(found.Datas, box)
		}
	}
	return found, nil
}

func (b *boxes) MessageGetUserMessageListByDataIdUserIdList(_ context.Context, in *message.TLMessageGetUserMessageListByDataIdUserIdList) (*message.Vector_MessageBox, error) {
	found := &message.Vector_MessageBox{}
	for _, box := range b.byOwner {
		if box.DialogMessageId == in.Id && contains(in.UserIdList, box.UserId) {
			found.Datas = append(found.Datas, box)
		}
	}
	return found, nil
}

type chats struct {
	chat_client.ChatClient
	members map[int64][]int64
}

func (c *chats) ChatGetChatParticipantIdList(_ context.Context, in *chatpb.TLChatGetChatParticipantIdList) (*chatpb.Vector_Long, error) {
	return &chatpb.Vector_Long{Datas: c.members[in.ChatId]}, nil
}

type told struct {
	sync_client.SyncClient
	pushed map[int64]*mtproto.Update
	notMe  map[int64]*mtproto.Update
}

func (t *told) SyncPushUpdates(_ context.Context, in *sync.TLSyncPushUpdates) (*mtproto.Void, error) {
	t.pushed[in.UserId] = in.Updates.Updates[0]
	return mtproto.EmptyVoid, nil
}

func (t *told) SyncUpdatesNotMe(_ context.Context, in *sync.TLSyncUpdatesNotMe) (*mtproto.Void, error) {
	t.notMe[in.UserId] = in.Updates.Updates[0]
	return mtproto.EmptyVoid, nil
}

type mapStore map[[2]int64]reactions.MessageReactionsDO

func (m mapStore) Set(_ context.Context, dialogMessageId, userId int64, reaction string, at int32) error {
	m[[2]int64{dialogMessageId, userId}] = reactions.MessageReactionsDO{
		DialogMessageId: dialogMessageId, UserId: userId, Reaction: reaction, ReactedAt: at,
	}
	return nil
}

func (m mapStore) Remove(_ context.Context, dialogMessageId, userId int64) error {
	delete(m, [2]int64{dialogMessageId, userId})
	return nil
}

func (m mapStore) Of(_ context.Context, dialogMessageId int64) ([]reactions.MessageReactionsDO, error) {
	var rows []reactions.MessageReactionsDO
	for key, row := range m {
		if key[0] == dialogMessageId {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

type world struct {
	t     *testing.T
	boxes *boxes
	told  *told
	store mapStore
	ctx   *svc.ServiceContext
}

func newWorld(t *testing.T) *world {
	catalog, err := reactions.Load("../../../../../teamgramd/reactions")
	if err != nil {
		t.Fatal(err)
	}
	w := &world{
		t:     t,
		boxes: &boxes{byOwner: map[[2]int64]*mtproto.MessageBox{}},
		told:  &told{},
		store: mapStore{},
	}
	// Alice wrote to Bob: message 500 is 10 in her box and 11 in his.
	w.boxes.add(alice, 10, 500, alice, bob)
	w.boxes.add(bob, 11, 500, alice, bob)
	// Alice wrote to the group: message 600 is 20, 21 and 22. Carol has left,
	// and still holds her copy.
	for owner, id := range map[int64]int32{alice: 20, bob: 21, carol: 22} {
		w.boxes.add(owner, id, 600, -int64(mtproto.PEER_CHAT), group)
	}
	w.ctx = &svc.ServiceContext{
		Catalog:       catalog,
		Store:         w.store,
		MessageClient: w.boxes,
		ChatClient:    &chats{members: map[int64][]int64{group: {alice, bob}}},
		SyncClient:    w.told,
	}
	return w
}

func (w *world) react(who int64, peer *mtproto.InputPeer, msgId int32, emoji ...string) (*mtproto.Updates, error) {
	w.told.pushed = map[int64]*mtproto.Update{}
	w.told.notMe = map[int64]*mtproto.Update{}
	list := []*mtproto.Reaction{}
	for _, e := range emoji {
		list = append(list, mtproto.FromReaction(e))
	}
	ctx := context.Background()
	c := &ReactionsCore{ctx: ctx, svcCtx: w.ctx, Logger: logx.WithContext(ctx),
		MD: &metadata.RpcMetadata{UserId: who, PermAuthKeyId: who * 10}}
	return c.MessagesSendReaction(&mtproto.TLMessagesSendReaction{
		Peer:                        peer,
		MsgId:                       msgId,
		Reaction_FLAGVECTORREACTION: list,
	})
}

func user(id int64) *mtproto.InputPeer {
	return mtproto.MakeTLInputPeerUser(&mtproto.InputPeer{UserId: id}).To_InputPeer()
}

func chat(id int64) *mtproto.InputPeer {
	return mtproto.MakeTLInputPeerChat(&mtproto.InputPeer{ChatId: id}).To_InputPeer()
}

// One update as its reader sees it: the message by their number, the
// conversation by their name for it, the counts, and which one is theirs.
type seen struct {
	peerUser, peerChat int64
	msgId              int32
	counts             map[string]int32
	mine               string
}

func read(update *mtproto.Update) seen {
	s := seen{
		peerUser: update.Peer_PEER.GetUserId(),
		peerChat: update.Peer_PEER.GetChatId(),
		msgId:    update.MsgId_INT32,
		counts:   map[string]int32{},
	}
	for _, count := range update.Reactions_MESSAGEREACTIONS.GetResults() {
		s.counts[count.Reaction_REACTION.GetEmoticon()] = count.Count
		if count.ChosenOrder != nil {
			s.mine = count.Reaction_REACTION.GetEmoticon()
		}
	}
	return s
}

func TestAReactionInAChatBetweenTwoReachesTheOtherAsTheySeeIt(t *testing.T) {
	w := newWorld(t)
	answer, err := w.react(bob, user(alice), 11, "👍")
	if err != nil {
		t.Fatal(err)
	}

	forBob := read(answer.Updates[0])
	if forBob.peerUser != alice || forBob.msgId != 11 || forBob.counts["👍"] != 1 || forBob.mine != "👍" {
		t.Fatalf("Bob is answered about his message 11 with Alice, his own 👍 marked: %+v", forBob)
	}
	forAlice, ok := w.told.pushed[alice]
	if !ok {
		t.Fatal("Alice was not told")
	}
	if got := read(forAlice); got.peerUser != bob || got.msgId != 10 || got.counts["👍"] != 1 || got.mine != "" {
		t.Fatalf("Alice hears of her message 10 with Bob, and the 👍 is not hers: %+v", got)
	}
	if _, ok = w.told.pushed[bob]; ok {
		t.Fatal("Bob is answered, not pushed to: the phone that asked would hear it twice")
	}
	if notMe, ok := w.told.notMe[bob]; !ok || read(notMe).msgId != 11 {
		t.Fatal("Bob's other phones are told")
	}
	if forAlice.Pts_INT32 != 0 {
		t.Fatal("a reaction takes no place in the update sequence: a pts here is a gap on every phone")
	}
}

func TestAnotherReactionReplacesTheFirstAndNoneTakesItBack(t *testing.T) {
	w := newWorld(t)
	if _, err := w.react(bob, user(alice), 11, "👍"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.react(bob, user(alice), 11, "❤️"); err != nil {
		t.Fatal(err)
	}
	if got := read(w.told.pushed[alice]); len(got.counts) != 1 || got.counts["❤️"] != 1 {
		t.Fatalf("one reaction per person: the heart replaced the thumb: %+v", got.counts)
	}
	if _, err := w.react(bob, user(alice), 11); err != nil {
		t.Fatal(err)
	}
	if got := read(w.told.pushed[alice]); len(got.counts) != 0 {
		t.Fatalf("taken back, Alice is told there is nothing: %+v", got.counts)
	}
	if len(w.store) != 0 {
		t.Fatalf("and nothing is kept: %v", w.store)
	}
}

func TestWhatTheSetDoesNotHoldChangesNothingAndIsStillAnswered(t *testing.T) {
	w := newWorld(t)
	if _, err := w.react(bob, user(alice), 11, "👍"); err != nil {
		t.Fatal(err)
	}
	answer, err := w.react(bob, user(alice), 11, "🤡")
	if err != nil {
		t.Fatalf("an answer, not a refusal - a refused client retries: %v", err)
	}
	if got := read(answer.Updates[0]); got.counts["👍"] != 1 || got.mine != "👍" {
		t.Fatalf("Bob is told how things stand: %+v", got)
	}
	if len(w.told.pushed) != 0 || len(w.told.notMe) != 0 {
		t.Fatalf("and nobody else is told of nothing: %v %v", w.told.pushed, w.told.notMe)
	}
}

func TestAMessageIsFoundOnlyInTheConversationTheClientNames(t *testing.T) {
	w := newWorld(t)
	if _, err := w.react(bob, user(carol), 11, "👍"); err == nil {
		t.Fatal("Bob's message 11 is in his chat with Alice, not with Carol")
	}
	if _, err := w.react(bob, user(alice), 99, "👍"); err == nil {
		t.Fatal("there is no message 99 in Bob's box")
	}
	if len(w.store) != 0 {
		t.Fatalf("neither is kept: %v", w.store)
	}
}

func TestAGroupHearsEachInTheirOwnBoxAndOneWhoLeftChangesNothing(t *testing.T) {
	w := newWorld(t)
	if _, err := w.react(alice, chat(group), 20, "🎉"); err != nil {
		t.Fatal(err)
	}
	forBob, ok := w.told.pushed[bob]
	if !ok {
		t.Fatal("Bob was not told")
	}
	if got := read(forBob); got.peerChat != group || got.msgId != 21 || got.counts["🎉"] != 1 {
		t.Fatalf("Bob hears of his message 21 in the group: %+v", got)
	}
	if _, ok = w.told.pushed[carol]; ok {
		t.Fatal("Carol has left the group and is not told what happens in it")
	}

	if _, err := w.react(carol, chat(group), 22, "👎"); err != nil {
		t.Fatalf("an answer, not a refusal: %v", err)
	}
	if len(w.store) != 1 {
		t.Fatalf("somebody who left adds nothing: %v", w.store)
	}
	if len(w.told.pushed) != 0 {
		t.Fatalf("and tells nobody: %v", w.told.pushed)
	}
}
