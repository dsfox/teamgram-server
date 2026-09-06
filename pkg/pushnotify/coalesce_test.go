package pushnotify

import (
	"sync"
	"testing"
	"time"
)

func TestLaterRunsOnceForABurstAndKeepsTheLastCall(t *testing.T) {
	var mu sync.Mutex
	var ran []int
	l := newLater(30 * time.Millisecond)
	for i := 1; i <= 5; i++ {
		n := i
		l.after(7, func() {
			mu.Lock()
			defer mu.Unlock()
			ran = append(ran, n)
		})
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(ran) != 1 || ran[0] != 5 {
		t.Fatalf("a burst of five ran %v; wanted the last one, once", ran)
	}
}

func TestLaterKeepsUsersApart(t *testing.T) {
	var mu sync.Mutex
	ran := map[int64]int{}
	l := newLater(10 * time.Millisecond)
	for _, user := range []int64{1, 2, 1} {
		u := user
		l.after(u, func() {
			mu.Lock()
			defer mu.Unlock()
			ran[u]++
		})
	}
	time.Sleep(60 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if ran[1] != 1 || ran[2] != 1 {
		t.Fatalf("ran %v; wanted once each", ran)
	}
}
