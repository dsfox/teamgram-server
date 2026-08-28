package mls

import (
	"context"
	"errors"
	"testing"
)

// A conversation is with a person, and every device of theirs is a member - so
// a welcome goes to all of them. Leaving one out is a phone that will not open
// the messages, with nothing to say why.
func TestAWelcomeReachesEveryDeviceOfThePerson(t *testing.T) {
	d, packages := newTestDirectory()
	welcomes := &mapWelcomes{}
	ctx := context.Background()

	// Two devices, known because they published key packages.
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone")}, nil, 1)
	_, _ = d.Publish(ctx, 7, 200, [][]byte{[]byte("laptop")}, nil, 1)
	welcomes.devices = packages

	posted, err := d.Post(ctx, welcomes, 7, 42, -120057, []byte("come in"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if posted != 2 {
		t.Fatalf("left %d welcomes for two devices", posted)
	}

	phone, _ := d.Waiting(ctx, welcomes, 7, 100)
	laptop, _ := d.Waiting(ctx, welcomes, 7, 200)
	if len(phone) != 1 || len(laptop) != 1 {
		t.Fatalf("phone has %d, laptop has %d", len(phone), len(laptop))
	}
	if phone[0].FromId != 42 {
		t.Errorf("the welcome says it is from %d", phone[0].FromId)
	}
}

// Delivered is not read. A device that fetched a welcome and died before saving
// the conversation must get it again, or the conversation exists on one side
// only and nobody finds out until a message will not open.
func TestAWelcomeIsKeptUntilTheDeviceSaysItOpenedIt(t *testing.T) {
	d, packages := newTestDirectory()
	welcomes := &mapWelcomes{}
	ctx := context.Background()
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone")}, nil, 1)
	welcomes.devices = packages
	_, _ = d.Post(ctx, welcomes, 7, 42, -120057, []byte("come in"), 1)

	first, _ := d.Waiting(ctx, welcomes, 7, 100)
	again, _ := d.Waiting(ctx, welcomes, 7, 100)
	if len(first) != 1 || len(again) != 1 {
		t.Fatal("a welcome disappeared merely by being read")
	}

	confirmed, err := d.Confirm(ctx, welcomes, 7, 100, []int64{first[0].Id})
	if err != nil || confirmed != 1 {
		t.Fatalf("confirming gave %d (%v)", confirmed, err)
	}

	after, _ := d.Waiting(ctx, welcomes, 7, 100)
	if len(after) != 0 {
		t.Fatalf("%d welcomes survived being confirmed", len(after))
	}
}

// A device confirming somebody else's welcome would drop a conversation that
// person never joined, and they would never learn why.
func TestADeviceCannotConfirmSomebodyElsesWelcome(t *testing.T) {
	d, packages := newTestDirectory()
	welcomes := &mapWelcomes{}
	ctx := context.Background()
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone")}, nil, 1)
	_, _ = d.Publish(ctx, 9, 300, [][]byte{[]byte("other")}, nil, 1)
	welcomes.devices = packages
	_, _ = d.Post(ctx, welcomes, 7, 42, -120057, []byte("come in"), 1)

	waiting, _ := d.Waiting(ctx, welcomes, 7, 100)

	confirmed, _ := d.Confirm(ctx, welcomes, 9, 300, []int64{waiting[0].Id})
	if confirmed != 0 {
		t.Fatal("another person's device confirmed this welcome")
	}

	still, _ := d.Waiting(ctx, welcomes, 7, 100)
	if len(still) != 1 {
		t.Fatal("the welcome was dropped by somebody it was not for")
	}
}

// Somebody who has never published is not broken, they are simply not here yet.
func TestAPersonWithNoDevicesTakesNoWelcome(t *testing.T) {
	d, _ := newTestDirectory()
	welcomes := &mapWelcomes{}

	posted, err := d.Post(context.Background(), welcomes, 999, 42, -120057, []byte("come in"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if posted != 0 {
		t.Fatalf("left %d welcomes for somebody with no devices", posted)
	}
}

// Without a bound, anybody could fill the table by starting conversations
// nobody answers.
func TestWelcomesCannotPileUpForever(t *testing.T) {
	d, packages := newTestDirectory()
	welcomes := &mapWelcomes{}
	ctx := context.Background()
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone")}, nil, 1)
	welcomes.devices = packages

	var err error
	for i := 0; i <= MaxWaitingPerDevice; i++ {
		if _, err = d.Post(ctx, welcomes, 7, 42, -120057, []byte("come in"), 1); err != nil {
			break
		}
	}
	if !errors.Is(err, ErrTooMany) {
		t.Fatalf("welcomes piled up past the bound: %v", err)
	}
}

// An empty welcome is a client fault and has to be said so, not stored: a
// device would fetch nothing and be unable to explain why it cannot join.
func TestAnEmptyWelcomeIsRefused(t *testing.T) {
	d, _ := newTestDirectory()

	_, err := d.Post(context.Background(), &mapWelcomes{}, 7, 42, -120057, nil, 1)
	if !errors.Is(err, ErrEmptyPackage) {
		t.Fatalf("an empty welcome gave %v", err)
	}
}

// mapWelcomes is the smallest thing that behaves like the table.
type mapWelcomes struct {
	rows    []Welcome
	nextId  int64
	devices *mapStore
}

func (m *mapWelcomes) Put(_ context.Context, w Welcome) error {
	m.nextId++
	w.Id = m.nextId
	m.rows = append(m.rows, w)
	return nil
}

func (m *mapWelcomes) Waiting(_ context.Context, userId, authKeyId int64) ([]Welcome, error) {
	var waiting []Welcome
	for _, w := range m.rows {
		if w.UserId == userId && w.AuthKeyId == authKeyId {
			waiting = append(waiting, w)
		}
	}
	return waiting, nil
}

func (m *mapWelcomes) Confirm(_ context.Context, userId, authKeyId int64, ids []int64) (int, error) {
	wanted := map[int64]bool{}
	for _, id := range ids {
		wanted[id] = true
	}

	kept := m.rows[:0]
	removed := 0
	for _, w := range m.rows {
		if wanted[w.Id] && w.UserId == userId && w.AuthKeyId == authKeyId {
			removed++
			continue
		}
		kept = append(kept, w)
	}
	m.rows = kept
	return removed, nil
}

func (m *mapWelcomes) Devices(ctx context.Context, userId int64) ([]int64, error) {
	if m.devices == nil {
		return nil, nil
	}
	return m.devices.Devices(ctx, userId)
}

// An invitation says which chat it is for, and says it back.
//
// Without that a welcome carries only who sent it, and the device joining files
// the conversation under that person - so a group is recorded as the
// conversation with whoever invited them, and a private message to that person
// is written with the group's keys. On the stand it split a group in two: the
// commit that let a second device in went to one member, because the code
// believed a one-to-one conversation has one recipient (#115).
func TestAnInvitationRemembersItsChat(t *testing.T) {
	packages := &mapStore{}
	welcomes := &mapWelcomes{devices: packages}
	d := New(packages)
	ctx := context.Background()

	// A device is only known because it published, so there has to be one.
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone")}, nil, 1)

	_, _ = d.Post(ctx, welcomes, 7, 42, -120057, []byte("come in"), 1)

	waiting, err := welcomes.Waiting(ctx, 7, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(waiting) != 1 {
		t.Fatalf("expected one invitation waiting, got %d", len(waiting))
	}
	if waiting[0].PeerId != -120057 {
		t.Fatalf("the invitation forgot its chat: %d", waiting[0].PeerId)
	}
}
