package dao

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/teamgram/proto/mtproto"
	sessionclient "github.com/teamgram/teamgram-server/app/interface/session/client"
	"github.com/teamgram/teamgram-server/app/interface/session/session"

	"google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
)

// A session client that fails the way a session server that has gone away
// fails, and then answers again the way one that has come back answers.
//
// The shape matters more than the mechanism: gRPC reports a server that is not
// there as codes.Unavailable, which is the code `isSessionConnError` looks for
// and the code that set the mark that could never be taken off.
type fakeSession struct {
	// Embedded so that anything this test does not expect to be called
	// panics on a nil interface rather than quietly answering.
	sessionclient.SessionClient

	mu      sync.Mutex
	answers []error
	calls   int
	seen    chan int
}

func (f *fakeSession) SessionPushUpdatesData(_ context.Context, _ *session.TLSessionPushUpdatesData) (*mtproto.Bool, error) {
	f.mu.Lock()
	n := f.calls
	f.calls++
	var err error
	if n < len(f.answers) {
		err = f.answers[n]
	}
	f.mu.Unlock()

	f.seen <- n + 1
	return mtproto.BoolTrue, err
}

func newTestSession(client sessionclient.SessionClient) *Session {
	c := &Session{
		serverId:    "127.0.0.1:0",
		client:      client,
		options:     SessionOptions{RoutineSize: 1, RoutineChan: 8},
		sessionChan: make([]chan sessionDataCtx, 1),
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.sessionChan[0] = make(chan sessionDataCtx, 8)
	go c.process(c.sessionChan[0])
	return c
}

func waitForCall(t *testing.T, seen chan int, want int) {
	t.Helper()
	select {
	case got := <-seen:
		if got != want {
			t.Fatalf("the session client was called %d times, expected call %d", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("the session client was never called a %d%s time", want, "th")
	}
}

// A session that failed is asked again, and delivery comes back without
// anybody restarting anything.
//
// This is #146. The mark that says "this session is not answering" was set on
// the first connection error and had no way off: `unavailable` was only ever
// stored true. Every push after it was dropped for the life of the process,
// and on 31 August that was 192 updates that never reached anybody while the
// session server had been back for hours. Restarting the container was the
// only cure anybody found.
//
// The second call below is the whole test. The old code could not make it: a
// marked session refused everything for ever, so the client was never asked
// again and nothing could ever discover that it was alive.
func TestASessionThatFailedIsAskedAgain(t *testing.T) {
	fake := &fakeSession{
		answers: []error{grpcStatus.Error(codes.Unavailable, "session server is gone")},
		seen:    make(chan int, 4),
	}
	c := newTestSession(fake)
	defer c.cancel()

	if err := c.PushUpdates(context.Background(), &session.TLSessionPushUpdatesData{}); err != nil {
		t.Fatalf("the first push was refused before anything had failed: %v", err)
	}
	waitForCall(t, fake.seen, 1)

	// The mark went on, and while it is fresh it does its job: pushes are
	// refused rather than queued behind a server that is not there. Waited for
	// rather than assumed, because the mark is set on the worker's thread.
	deadline := time.Now().Add(2 * time.Second)
	for !c.refusing() {
		if time.Now().After(deadline) {
			t.Fatal("a session that answered Unavailable is still being pushed to")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := c.PushUpdates(context.Background(), &session.TLSessionPushUpdatesData{}); err == nil {
		t.Fatal("a push was accepted while the session was known not to answer")
	}

	// The pause passes. Moved rather than slept through: the wait is five
	// seconds and a gate that takes five seconds gets run less.
	c.markedAt.Store(time.Now().Add(-2 * sessionRetryAfter).UnixNano())

	if err := c.PushUpdates(context.Background(), &session.TLSessionPushUpdatesData{}); err != nil {
		t.Fatalf("the push was still refused after the pause, so there is no way back: %v", err)
	}
	waitForCall(t, fake.seen, 2)

	// It got through, so the mark is off and the next push does not wait for
	// another pause.
	deadline = time.Now().Add(2 * time.Second)
	for c.unavailable.Load() {
		if time.Now().After(deadline) {
			t.Fatal("the push got through and the session is still marked as not answering")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if c.refusing() {
		t.Fatal("a session that is answering is still refusing pushes")
	}
}

// And the mark still protects: a session that goes on failing is not asked on
// every push, which is what the mark was added for and what must survive it
// being made temporary.
func TestASessionThatGoesOnFailingIsNotAskedEveryTime(t *testing.T) {
	fake := &fakeSession{
		answers: []error{
			grpcStatus.Error(codes.Unavailable, "session server is gone"),
			grpcStatus.Error(codes.Unavailable, "still gone"),
		},
		seen: make(chan int, 4),
	}
	c := newTestSession(fake)
	defer c.cancel()

	if err := c.PushUpdates(context.Background(), &session.TLSessionPushUpdatesData{}); err != nil {
		t.Fatalf("the first push was refused: %v", err)
	}
	waitForCall(t, fake.seen, 1)

	deadline := time.Now().Add(2 * time.Second)
	for !c.refusing() {
		if time.Now().After(deadline) {
			t.Fatal("the failure left no mark at all")
		}
		time.Sleep(5 * time.Millisecond)
	}

	for i := 0; i < 20; i++ {
		if err := c.PushUpdates(context.Background(), &session.TLSessionPushUpdatesData{}); err == nil {
			t.Fatal("pushes are reaching a session that is known not to answer")
		}
	}

	select {
	case n := <-fake.seen:
		t.Fatalf("the session client was called %d times while it was marked", n)
	case <-time.After(200 * time.Millisecond):
	}
}

// A session that accepts the call and then says nothing is given up on.
//
// This is the half of #146 that the deadline was supposed to close and did not.
// `deliver` in the core wraps the push in three seconds, but `PushUpdates` only
// puts the work on a channel and returns; the call is made later by `process`,
// with a context that came through `detach` - `context.WithoutCancel` - which
// by contract carries no deadline. So the call was unbounded, and a session
// that stopped answering held that goroutine for ever. On 31 August that was
// 192 updates nobody ever got.
//
// A refused connection is the easy half and answers at once. This is the other
// one: the fake accepts and waits, exactly as a hung server does, and gRPC's
// own answer to a passed deadline is what it returns.
func TestAPushToASessionThatSaysNothingIsGivenUpOn(t *testing.T) {
	was := pushCallDeadline
	pushCallDeadline = 100 * time.Millisecond
	defer func() { pushCallDeadline = was }()

	entered := make(chan struct{}, 1)
	fake := &hangingSession{entered: entered}
	c := newTestSession(fake)
	defer c.cancel()

	if err := c.PushUpdates(context.Background(), &session.TLSessionPushUpdatesData{}); err != nil {
		t.Fatalf("the push was refused before it was even tried: %v", err)
	}

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the session client was never called at all")
	}

	// The point: the call comes back rather than holding the goroutine, and the
	// session is marked so the next push is not thrown into the same hole.
	deadline := time.Now().Add(2 * time.Second)
	for !c.unavailable.Load() {
		if time.Now().After(deadline) {
			t.Fatal("the call never came back: a session that says nothing still holds this goroutine for ever, which is #146")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A session that accepts a call and never answers, which is what a hung server
// looks like from here. It waits for the context, and answers what gRPC answers
// when a deadline passes.
type hangingSession struct {
	sessionclient.SessionClient
	entered chan struct{}
}

func (h *hangingSession) SessionPushUpdatesData(ctx context.Context, _ *session.TLSessionPushUpdatesData) (*mtproto.Bool, error) {
	select {
	case h.entered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil, grpcStatus.FromContextError(ctx.Err()).Err()
}

// The streaming path is refused rather than left as a switch to find.
//
// #149: its latch is only ever set, and both goroutines end when the stream
// breaks, so nothing reopens them and clearing the latch would turn an honest
// refusal into silent loss. Nothing measures that path - no scenario, no test,
// no deployed config - and upstream's file stays as it is, because it is theirs
// and every carry brings it back. What is ours is refusing to start on it.
func TestSyncRefusesToRunOnTheStreamingPath(t *testing.T) {
	if err := refuseTheStreamingPath(false); err != nil {
		t.Fatalf("sync refused to start with the switch off: %v", err)
	}

	err := refuseTheStreamingPath(true)
	if err == nil {
		t.Fatal("sync would have started on a path that accepts pushes and loses them")
	}
	// The message has to name the issue, because whoever meets it is somebody
	// who turned the switch on and needs to know what to read.
	if !strings.Contains(err.Error(), "#149") {
		t.Errorf("the refusal does not say where to look: %v", err)
	}
}
