package calls

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrNoCall - no call by that id, or the access hash did not match. The two
	// are the same answer on purpose: a wrong hash must not confirm that an id
	// exists.
	ErrNoCall = errors.New("calls: no such call")
	// ErrBusy - one of the two is already in a call.
	ErrBusy = errors.New("calls: already in a call")
)

// Registry holds the calls that are in the air.
//
// A call is found by id *and* access hash: the id alone is a number anyone can
// guess, the hash is the capability that proves this caller was told about the
// call. Anything finished or done ringing is dropped, which is also what frees
// the two of them to call again.
//
// This is the seam a durable store replaces: the live leg belongs in Redis once
// there is more than one node. Note that the *Call handed back is mutated by
// the caller (Accept, Confirm, Discard); the layer above must not move one call
// from two goroutines at once.
type Registry struct {
	mu    sync.Mutex
	calls map[int64]*Call
}

// NewRegistry makes an empty one.
func NewRegistry() *Registry {
	return &Registry{calls: make(map[int64]*Call)}
}

// Place starts a call, unless either of the two is already in one.
func (r *Registry) Place(admin, participant int64, gAHash []byte, now time.Time) (*Call, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweep(now)

	for _, c := range r.calls {
		if c.involves(admin) || c.involves(participant) {
			return nil, ErrBusy
		}
	}

	c, err := Request(admin, participant, gAHash, now)
	if err != nil {
		return nil, err
	}
	r.calls[c.Id] = c
	return c, nil
}

// Get finds a live call. A wrong hash is the same answer as a wrong id.
func (r *Registry) Get(id, accessHash int64, now time.Time) (*Call, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweep(now)

	c, ok := r.calls[id]
	if !ok || c.AccessHash != accessHash {
		return nil, ErrNoCall
	}
	return c, nil
}

// RingingFor is the call still ringing for this person, if any: placed for
// them, not yet answered, not yet given up. What a device that was away asks
// when it comes back.
func (r *Registry) RingingFor(userId int64, now time.Time) *Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweep(now)

	for _, c := range r.calls {
		if c.Participant == userId && c.State == Waiting {
			return c
		}
	}
	return nil
}

// Sweep drops what is over and reports how many went. Call it on a timer as
// well: a call nobody ever answered would otherwise keep its two people busy.
func (r *Registry) Sweep(now time.Time) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sweep(now)
}

func (r *Registry) sweep(now time.Time) int {
	gone := 0
	for id, c := range r.calls {
		if c.State == Discarded || c.Expired(now) {
			delete(r.calls, id)
			gone++
		}
	}
	return gone
}

func (c *Call) involves(user int64) bool {
	return user == c.Admin || user == c.Participant
}
