package core

import (
	"context"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"

	"github.com/zeromicro/go-zero/core/logx"
)

func (w *world) as(who int64) *ReactionsCore {
	ctx := context.Background()
	return &ReactionsCore{ctx: ctx, svcCtx: w.ctx, Logger: logx.WithContext(ctx), MD: &metadata.RpcMetadata{UserId: who}}
}

// Every client cached the empty list the server gave under zero, so zero must
// always bring the whole list, and only the list's own hash "not modified".
func TestTheListIsNotModifiedOnlyForItsOwnHash(t *testing.T) {
	w := newWorld(t)
	c := w.as(alice)
	hash := w.ctx.Catalog.Hash()

	for _, asked := range []int32{0, hash + 1} {
		answer, _ := c.MessagesGetAvailableReactions(&mtproto.TLMessagesGetAvailableReactions{Hash: asked})
		if answer.GetPredicateName() != mtproto.Predicate_messages_availableReactions || len(answer.Reactions) != 9 || answer.Hash != hash {
			t.Fatalf("asked with %d: %s with %d reactions", asked, answer.GetPredicateName(), len(answer.Reactions))
		}
	}
	answer, _ := c.MessagesGetAvailableReactions(&mtproto.TLMessagesGetAvailableReactions{Hash: hash})
	if answer.GetPredicateName() != mtproto.Predicate_messages_availableReactionsNotModified {
		t.Fatalf("asked with the list's hash: %s", answer.GetPredicateName())
	}

	top, _ := c.MessagesGetTopReactions(&mtproto.TLMessagesGetTopReactions{Limit: 32})
	if len(top.Reactions) != 9 || top.Reactions[0].GetEmoticon() != "👍" {
		t.Fatalf("the menu of a chat between two is the whole set, the thumb first: %v", top.Reactions)
	}
	w.ctx.Catalog = nil
	empty, _ := c.MessagesGetAvailableReactions(&mtproto.TLMessagesGetAvailableReactions{Hash: hash})
	if empty.GetPredicateName() != mtproto.Predicate_messages_availableReactions || len(empty.Reactions) != 0 {
		t.Fatalf("with no pictures to offer, an empty list rather than an error: %v", empty)
	}
}

// What a phone asks for the messages on its screen: every one is answered, so
// a reaction taken back while it was away goes from it too.
func TestThePhoneShowingAChatHearsOfEveryMessageItShows(t *testing.T) {
	w := newWorld(t)
	if _, err := w.react(bob, user(alice), 11, "😮"); err != nil {
		t.Fatal(err)
	}
	w.boxes.add(alice, 12, 501, alice, bob)

	answer, err := w.as(alice).MessagesGetMessagesReactions(&mtproto.TLMessagesGetMessagesReactions{
		Peer: user(bob),
		Id:   []int32{10, 12, 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[int32]seen{}
	for _, update := range answer.Updates {
		got[update.MsgId_INT32] = read(update)
	}
	if len(got) != 2 {
		t.Fatalf("10 and 12 are in the chat with Bob, 20 is in the group: %v", got)
	}
	if got[10].counts["😮"] != 1 || got[10].peerUser != bob {
		t.Fatalf("message 10 carries Bob's reaction: %+v", got[10])
	}
	if len(got[12].counts) != 0 {
		t.Fatalf("message 12 is answered, with nothing on it: %+v", got[12])
	}
}
