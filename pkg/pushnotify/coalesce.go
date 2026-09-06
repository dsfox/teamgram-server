package pushnotify

import (
	"sync"
	"time"
)

// later runs one thing per user after a quiet moment: a burst of calls in
// the window runs the last of them, once. Reading a chat is many read marks
// in a second, and the phone that is asleep wants one badge, not thirty
// pushes - which the relay would refuse past thirty a minute anyway.
type later struct {
	mu     sync.Mutex
	window time.Duration
	timers map[int64]*time.Timer
}

func newLater(window time.Duration) *later {
	return &later{window: window, timers: map[int64]*time.Timer{}}
}

func (l *later) after(userId int64, run func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t, held := l.timers[userId]; held {
		t.Stop()
	}
	l.timers[userId] = time.AfterFunc(l.window, func() {
		l.mu.Lock()
		delete(l.timers, userId)
		l.mu.Unlock()
		run()
	})
}
