package dao

import (
	"errors"
	"testing"
	"time"

	sessionclient "github.com/teamgram/teamgram-server/app/interface/session/client"

	"github.com/zeromicro/go-zero/core/hash"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A node that stops answering must come back to the ring by itself.
//
// It did not, and nothing anywhere would have: with direct addresses the thing
// that adds nodes runs once at startup, so a node taken out after three
// failures was out for ever. On this server there is one node, so the ring was
// then empty and every request answered "not found session" until the container
// was restarted - nobody could sign in and nobody could write (#150).
//
// The pause is a var so this can watch it pass instead of waiting five seconds.
func TestASidelinedNodeComesBackToTheRing(t *testing.T) {
	was := nodeReturnsAfter
	nodeReturnsAfter = 20 * time.Millisecond
	defer func() { nodeReturnsAfter = was }()

	sess := &ShardingSessionClient{
		dispatcher:   hash.NewConsistentHash(),
		sessions:     map[string]sessionclient.SessionClient{"one": nil},
		failCounters: map[string]int{},
		sidelined:    map[string]time.Time{},
	}
	sess.dispatcher.Add("one")

	dead := status.Error(codes.Unavailable, "connection refused")
	for i := 0; i < maxNodeFailures; i++ {
		if err := sess.InvokeByKey("whoever", func(sessionclient.SessionClient) error {
			return dead
		}); err == nil {
			t.Fatalf("failure %d was reported as a success", i+1)
		}
	}

	// The control: it really is out, or what follows proves nothing.
	if err := sess.InvokeByKey("whoever", func(sessionclient.SessionClient) error {
		t.Fatal("the node was still dispatched to after failing three times")
		return nil
	}); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("with the ring empty the answer was %v, not %v", err, ErrSessionNotFound)
	}

	time.Sleep(nodeReturnsAfter * 3)

	reached := false
	if err := sess.InvokeByKey("whoever", func(sessionclient.SessionClient) error {
		reached = true
		return nil
	}); err != nil {
		t.Fatalf("after the pause the call answered %v", err)
	}
	if !reached {
		t.Fatal("the node is in the ring again but nothing was sent to it")
	}
}
