package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

type countingStore struct {
	counts  map[string]int64
	expires map[string]int
	fail    bool
}

func (s *countingStore) IncrCtx(_ context.Context, key string) (int64, error) {
	if s.fail {
		return 0, errors.New("redis is away")
	}
	s.counts[key]++
	return s.counts[key], nil
}

func (s *countingStore) ExpireCtx(_ context.Context, key string, seconds int) error {
	s.expires[key] = seconds
	return nil
}

func newCountingStore() *countingStore {
	return &countingStore{counts: map[string]int64{}, expires: map[string]int{}}
}

func TestSixtyLookupsAnHourAndThenAWait(t *testing.T) {
	store := newCountingStore()
	at := time.Date(2026, 9, 5, 12, 20, 0, 0, time.UTC)
	for i := 1; i <= lookupsPerHour; i++ {
		ok, _, err := lookupAllowed(context.Background(), store, 7, at)
		if err != nil || !ok {
			t.Fatalf("lookup %d refused: ok=%v err=%v", i, ok, err)
		}
	}
	ok, wait, err := lookupAllowed(context.Background(), store, 7, at)
	if err != nil || ok {
		t.Fatalf("the 61st lookup was allowed")
	}
	// The wait is what is left of the hour: 40 minutes at 12:20.
	if wait != 40*60 {
		t.Fatalf("the wait is %d s, not the rest of the hour", wait)
	}
	if len(store.expires) != 1 {
		t.Fatalf("the counter was not given a lifetime once: %v", store.expires)
	}
}

func TestAnotherAccountAndAnotherHourCountSeparately(t *testing.T) {
	store := newCountingStore()
	at := time.Date(2026, 9, 5, 12, 59, 0, 0, time.UTC)
	for i := 0; i < lookupsPerHour; i++ {
		if ok, _, _ := lookupAllowed(context.Background(), store, 7, at); !ok {
			t.Fatal("refused early")
		}
	}
	if ok, _, _ := lookupAllowed(context.Background(), store, 8, at); !ok {
		t.Fatal("another account was refused for this one's lookups")
	}
	if ok, _, _ := lookupAllowed(context.Background(), store, 7, at.Add(2*time.Minute)); !ok {
		t.Fatal("the next hour did not open")
	}
}

func TestAStoreThatIsAwayDoesNotRefuse(t *testing.T) {
	store := newCountingStore()
	store.fail = true
	ok, _, err := lookupAllowed(context.Background(), store, 7, time.Now())
	if !ok || err == nil {
		t.Fatalf("with the store away: ok=%v err=%v - a lookup must go through, and the failure be told", ok, err)
	}
}

func TestNoStoreAtAllLetsTheLookupThrough(t *testing.T) {
	var none lookupCounter
	ok, _, err := lookupAllowed(context.Background(), none, 7, time.Now())
	if !ok || err == nil {
		t.Fatalf("with no store: ok=%v err=%v", ok, err)
	}
}
