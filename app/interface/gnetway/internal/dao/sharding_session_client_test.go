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
// failures was out for ever. On a one-node install the ring was then empty and
// every request answered "not found session" until the container was restarted
// - nobody could sign in and nobody could write (#150). With two nodes the
// failing one goes round the other, and comes back.
//
// The pause is a var so this can watch it pass instead of waiting five seconds.
func TestASidelinedNodeComesBackToTheRing(t *testing.T) {
	was := nodeReturnsAfter
	nodeReturnsAfter = 20 * time.Millisecond
	defer func() { nodeReturnsAfter = was }()

	sess := &ShardingSessionClient{
		dispatcher:   hash.NewConsistentHash(),
		sessions:     map[string]sessionclient.SessionClient{"one": nil, "two": nil},
		failCounters: map[string]int{},
		sidelined:    map[string]time.Time{},
	}
	sess.dispatcher.Add("one")
	sess.dispatcher.Add("two")
	owner, _ := sess.dispatcher.Get("whoever")

	dead := status.Error(codes.Unavailable, "connection refused")
	for i := 0; i < maxNodeFailures; i++ {
		_ = sess.InvokeByKey("whoever", func(sessionclient.SessionClient) error {
			return dead
		})
	}

	// The control: it really is out, or what follows proves nothing.
	if _, out := sess.sidelined[owner.(string)]; !out {
		t.Fatalf("node %v failed %d times and is still in the ring", owner, maxNodeFailures)
	}
	if now, _ := sess.dispatcher.Get("whoever"); now == owner {
		t.Fatalf("the key still goes to %v, which is out of the ring", owner)
	}

	time.Sleep(nodeReturnsAfter * 3)

	if err := sess.InvokeByKey("whoever", func(sessionclient.SessionClient) error { return nil }); err != nil {
		t.Fatalf("after the pause the call answered %v", err)
	}
	if now, _ := sess.dispatcher.Get("whoever"); now != owner {
		t.Fatalf("after the pause the key goes to %v, not back to %v", now, owner)
	}
}

// The only node is never taken out of the ring (#228).
//
// Out of the ring it is "not found session" for five seconds, whether it
// answers again or not: on 5 October the session service was back at 41.273
// and four packets of a listener were dropped at 41.795 for that reason alone,
// its getDifference among them. With nothing to route round it to, the next
// packet tries it.
func TestTheOnlyNodeStaysInTheRing(t *testing.T) {
	sess := &ShardingSessionClient{
		dispatcher:   hash.NewConsistentHash(),
		sessions:     map[string]sessionclient.SessionClient{"one": nil},
		failCounters: map[string]int{},
		sidelined:    map[string]time.Time{},
	}
	sess.dispatcher.Add("one")

	dead := status.Error(codes.Unavailable, "connection refused")
	for i := 0; i < maxNodeFailures*2; i++ {
		if err := sess.InvokeByKey("whoever", func(sessionclient.SessionClient) error {
			return dead
		}); err == nil {
			t.Fatalf("failure %d was reported as a success", i+1)
		}
	}

	reached := false
	if err := sess.InvokeByKey("whoever", func(sessionclient.SessionClient) error {
		reached = true
		return nil
	}); errors.Is(err, ErrSessionNotFound) {
		t.Fatal("the only node was taken out of the ring: every packet is now 'not found session'")
	} else if err != nil {
		t.Fatalf("the node answered and the call said %v", err)
	}
	if !reached {
		t.Fatal("nothing was sent to the only node")
	}
}
