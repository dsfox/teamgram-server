package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// A rating comes after the call is over and forgotten. iOS sends it through
// retryRequest - an error is retried every five seconds for as long as the app
// lives - so it is answered, with an empty container, and nobody is told.
func TestARatingIsTakenAndNobodyIsTold(t *testing.T) {
	s := newStand(t)
	if _, err := s.as(bob).PhoneDiscardCall(&mtproto.TLPhoneDiscardCall{Peer: s.peer()}); err != nil {
		t.Fatalf("cannot hang up: %v", err)
	}
	s.sync.pushes = nil

	reply, err := s.as(alice).PhoneSetCallRating(&mtproto.TLPhoneSetCallRating{
		Peer: s.peer(), Rating: 4, Comment: "a bit of echo",
	})
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if reply.GetPredicateName() != mtproto.Predicate_updates {
		t.Fatalf("answered with %s", reply.GetPredicateName())
	}
	if len(reply.GetUpdates()) != 0 {
		t.Fatalf("the answer carries %d updates, expected none", len(reply.GetUpdates()))
	}
	if len(s.sync.pushes) != 0 {
		t.Fatalf("%d pushes went out on a rating", len(s.sync.pushes))
	}
}
