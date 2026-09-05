package pushrelay

import (
	"sync"
	"time"
)

// Quotas bounds how often a key may do something: a fixed window a minute
// long and one a day long, counted in memory. A relay that restarts starts
// counting again, which is the cheap side to err on.
type Quotas struct {
	mu     sync.Mutex
	minute map[string]window
	day    map[string]window
}

type window struct {
	opened time.Time
	count  int
}

func NewQuotas() *Quotas {
	return &Quotas{minute: map[string]window{}, day: map[string]window{}}
}

// Allow answers whether one more in this minute is within the limit.
func (q *Quotas) Allow(key string, perMinute int, now time.Time) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return allow(q.minute, key, perMinute, now, time.Minute)
}

// AllowDay answers whether one more today is within the limit.
func (q *Quotas) AllowDay(key string, perDay int, now time.Time) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return allow(q.day, key, perDay, now, 24*time.Hour)
}

func allow(windows map[string]window, key string, limit int, now time.Time, span time.Duration) bool {
	w := windows[key]
	if now.Sub(w.opened) >= span {
		w = window{opened: now}
	}
	if w.count >= limit {
		windows[key] = w
		return false
	}
	w.count++
	windows[key] = w
	// Bounded: keys nobody has used for a whole span are dropped once the map
	// has grown, so a relay that runs for months does not grow with it.
	if len(windows) > 50000 {
		for k, old := range windows {
			if now.Sub(old.opened) >= span {
				delete(windows, k)
			}
		}
	}
	return true
}
