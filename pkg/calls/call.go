package calls

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/teamgram/proto/mtproto"
)

// RingingFor is how long a placed call keeps ringing before it gives up. A call
// nobody answered must not stay answerable for ever: the key material in it is
// half-exchanged, and a stale half is worth nothing to anyone but an attacker.
const RingingFor = 90 * time.Second

// State is where a call has got to. It only ever moves forward.
type State int

const (
	// Waiting - placed, ringing at the one called.
	Waiting State = iota
	// Accepted - the one called sent g_b back.
	Accepted
	// Active - the caller confirmed; the two phones have a key and may talk.
	Active
	// Discarded - over, by either side or by giving up.
	Discarded
)

func (s State) String() string {
	switch s {
	case Waiting:
		return "waiting"
	case Accepted:
		return "accepted"
	case Active:
		return "active"
	case Discarded:
		return "discarded"
	}
	return fmt.Sprintf("state(%d)", int(s))
}

var (
	// ErrWrongParty - whoever asked is not in this call, or not the side whose
	// turn it is. A third party must not be able to answer, confirm, hang up or
	// signal into a call they are not part of.
	ErrWrongParty = errors.New("calls: not this person's call to move")
	// ErrWrongState - the call is not at the point that step belongs to.
	ErrWrongState = errors.New("calls: the call is not at that point")
	// ErrSelfCall - a call needs two people.
	ErrSelfCall = errors.New("calls: a call needs two people")
)

// Call is one conversation between two people, and the courier's copy of their
// key exchange. The blobs below are carried, never opened: the phones derive
// the media key from them, the server only moves them across and remembers who
// is allowed to move what.
type Call struct {
	Id          int64
	AccessHash  int64
	Admin       int64 // who placed it
	Participant int64 // who was called

	State State

	GAHash         []byte
	GB             []byte
	GA             []byte
	KeyFingerprint int64

	Created time.Time
	Changed time.Time
	// Received is when the callee's phone first said it was ringing; zero
	// until then. The caller's screen turns from waiting to ringing on it.
	Received time.Time

	// What the caller asked for, echoed back in every "waiting" the caller is
	// shown. Opaque here: the phones negotiate it, the server carries it. Set
	// by whoever places the call, before anyone else can see it.
	Protocol *mtproto.PhoneCallProtocol
	Video    bool

	// The two devices in the call, as permanent auth keys: the caller's from
	// the moment it asks, the callee's from the moment one of their phones
	// answers. What the two trade after that is addressed to these, not to
	// the person: a push by person looks the session up in the status list,
	// which a phone woken a moment ago is not yet in. Zero means unknown.
	AdminKey       int64
	ParticipantKey int64

	// The devices (permanent auth keys) already told about the call. Told
	// twice, an Android answers the second phoneCallRequested with "busy" and
	// the call is over - so a device that comes back later is rung only if
	// it is not in here.
	rung map[int64]struct{}
}

// MarkRung records that these devices were told about the call.
func (c *Call) MarkRung(permAuthKeyIds ...int64) {
	if c.rung == nil {
		c.rung = make(map[int64]struct{}, len(permAuthKeyIds))
	}
	for _, id := range permAuthKeyIds {
		c.rung[id] = struct{}{}
	}
}

// WasRung says whether this device was already told.
func (c *Call) WasRung(permAuthKeyId int64) bool {
	_, ok := c.rung[permAuthKeyId]
	return ok
}

// Request places a call. The caller has committed to a secret by sending only
// the hash of g_a; g_a itself follows at Confirm, once g_b is in.
func Request(admin, participant int64, gAHash []byte, now time.Time) (*Call, error) {
	if admin == participant {
		return nil, ErrSelfCall
	}
	return &Call{
		Id:          randomInt64(),
		AccessHash:  randomInt64(),
		Admin:       admin,
		Participant: participant,
		State:       Waiting,
		GAHash:      gAHash,
		Created:     now,
		Changed:     now,
	}, nil
}

// Receive is the callee's phone saying it is ringing, before anyone picks up.
// Only the callee may say it, only while the call still rings; a second device
// of the same person saying it again keeps the first time.
func (c *Call) Receive(by int64, now time.Time) error {
	if by != c.Participant {
		return ErrWrongParty
	}
	if c.State != Waiting || c.Expired(now) {
		return ErrWrongState
	}
	if c.Received.IsZero() {
		c.Received = now
	}
	return nil
}

// Accept answers the call with g_b. Only the person who was called may do it,
// only while it is still ringing.
func (c *Call) Accept(by int64, gB []byte, now time.Time) error {
	if by != c.Participant {
		return ErrWrongParty
	}
	if c.State != Waiting || c.Expired(now) {
		return ErrWrongState
	}
	c.GB = gB
	c.State = Accepted
	c.Changed = now
	return nil
}

// Confirm hands over g_a and the fingerprint the two sides will compare. Only
// the caller may, and only once g_b has come back.
func (c *Call) Confirm(by int64, gA []byte, keyFingerprint int64, now time.Time) error {
	if by != c.Admin {
		return ErrWrongParty
	}
	if c.State != Accepted {
		return ErrWrongState
	}
	c.GA = gA
	c.KeyFingerprint = keyFingerprint
	c.State = Active
	c.Changed = now
	return nil
}

// Discard ends the call. Either side may hang up; nobody else may.
func (c *Call) Discard(by int64, now time.Time) error {
	if by != c.Admin && by != c.Participant {
		return ErrWrongParty
	}
	if c.State == Discarded {
		return ErrWrongState
	}
	c.State = Discarded
	c.Changed = now
	return nil
}

// Other is the leg a message from this person goes to - and the guard on
// signalling: a candidate blob travels between these two and nowhere else.
func (c *Call) Other(user int64) (int64, error) {
	switch user {
	case c.Admin:
		return c.Participant, nil
	case c.Participant:
		return c.Admin, nil
	}
	return 0, ErrWrongParty
}

// KeyOf is the device of one of the two in the call, or zero while unknown.
func (c *Call) KeyOf(user int64) int64 {
	switch user {
	case c.Admin:
		return c.AdminKey
	case c.Participant:
		return c.ParticipantKey
	}
	return 0
}

// Expired says the ringing gave up. Only an unanswered call expires; one being
// spoken has no such clock.
func (c *Call) Expired(now time.Time) bool {
	if c.State != Waiting {
		return false
	}
	return now.After(c.Created.Add(RingingFor))
}

func randomInt64() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on the platforms we ship; if it ever does,
		// a predictable call id is not something to paper over.
		panic("calls: no randomness for a call id: " + err.Error())
	}
	return int64(binary.BigEndian.Uint64(b[:]) >> 1)
}
