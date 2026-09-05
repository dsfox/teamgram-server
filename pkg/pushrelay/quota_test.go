package pushrelay

import (
	"testing"
	"time"
)

func TestAMinuteQuotaOpensAgainNextMinute(t *testing.T) {
	q := NewQuotas()
	at := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if !q.Allow("k", 3, at) {
			t.Fatalf("call %d refused", i)
		}
	}
	if q.Allow("k", 3, at.Add(30*time.Second)) {
		t.Fatal("a fourth call in the minute was allowed")
	}
	if !q.Allow("k", 3, at.Add(61*time.Second)) {
		t.Fatal("the next minute did not open")
	}
}

func TestADayQuotaCountsTheWholeDay(t *testing.T) {
	q := NewQuotas()
	at := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if !q.AllowDay("k", 5, at.Add(time.Duration(i)*time.Hour)) {
			t.Fatalf("call %d refused", i)
		}
	}
	if q.AllowDay("k", 5, at.Add(6*time.Hour)) {
		t.Fatal("a sixth call in the day was allowed")
	}
	// A day is twenty-four hours from the first call, not the calendar's.
	if !q.AllowDay("k", 5, at.Add(25*time.Hour)) {
		t.Fatal("the next day did not open")
	}
}
