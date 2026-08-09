package mls

import (
	"context"
	"errors"
	"testing"
)

// The whole point: starting a conversation with somebody takes one package for
// every device they have, so every device of theirs can be a member.
func TestStartingAConversationTakesOnePackagePerDevice(t *testing.T) {
	d, store := newTestDirectory()
	ctx := context.Background()

	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone-a"), []byte("phone-b")}, nil, 1)
	_, _ = d.Publish(ctx, 7, 200, [][]byte{[]byte("laptop-a")}, nil, 1)

	claimed, err := d.Claim(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 2 {
		t.Fatalf("took %d packages for two devices", len(claimed))
	}

	devices := map[int64]bool{}
	for _, p := range claimed {
		devices[p.AuthKeyId] = true
	}
	if !devices[100] || !devices[200] {
		t.Fatalf("both devices should have contributed, got %v", devices)
	}
	if store.remaining(7, 100) != 1 {
		t.Errorf("the phone has %d left, expected 1", store.remaining(7, 100))
	}
}

// Used once. Handing the same package to two conversations costs the forward
// secrecy of every message that follows.
func TestAPackageIsNeverHandedOutTwice(t *testing.T) {
	d, _ := newTestDirectory()
	ctx := context.Background()
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("one"), []byte("two")}, nil, 1)

	first, _ := d.Claim(ctx, 7)
	second, _ := d.Claim(ctx, 7)

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected one each, got %d and %d", len(first), len(second))
	}
	if string(first[0].Bytes) == string(second[0].Bytes) {
		t.Fatalf("the same package was handed out twice: %q", first[0].Bytes)
	}
}

// Running dry must not stop a conversation from starting - that is what the
// last-resort package is for, and it is the only one that repeats.
func TestAnEmptySupplyFallsBackRatherThanRefusing(t *testing.T) {
	d, _ := newTestDirectory()
	ctx := context.Background()
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("only-one")}, []byte("last-resort"), 1)

	_, _ = d.Claim(ctx, 7) // takes the ordinary one

	after, err := d.Claim(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 {
		t.Fatalf("an empty supply gave %d packages, expected the last resort", len(after))
	}
	if string(after[0].Bytes) != "last-resort" || !after[0].LastResort {
		t.Fatalf("that was not the last-resort package: %q", after[0].Bytes)
	}

	again, _ := d.Claim(ctx, 7)
	if len(again) != 1 {
		t.Fatal("the last-resort package stopped answering; it is the one that repeats")
	}
}

// A device with nothing left and no last resort is skipped. One silent device
// must not stop a conversation with the rest.
func TestASilentDeviceDoesNotBlockTheOthers(t *testing.T) {
	d, _ := newTestDirectory()
	ctx := context.Background()
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone")}, nil, 1)
	_, _ = d.Publish(ctx, 7, 200, [][]byte{[]byte("laptop")}, nil, 1)

	_, _ = d.Claim(ctx, 7) // empties both

	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone-again")}, nil, 2)

	claimed, err := d.Claim(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].AuthKeyId != 100 {
		t.Fatalf("the device that had something should have answered alone, got %v", claimed)
	}
}

// A client that retries after a lost answer must not be punished for it, and
// the same bytes must not end up in two conversations.
func TestPublishingTheSameThingTwiceAddsItOnce(t *testing.T) {
	d, store := newTestDirectory()
	ctx := context.Background()

	added, err := d.Publish(ctx, 7, 100, [][]byte{[]byte("a"), []byte("b")}, nil, 1)
	if err != nil || added != 2 {
		t.Fatalf("first publish added %d (%v)", added, err)
	}

	added, err = d.Publish(ctx, 7, 100, [][]byte{[]byte("a"), []byte("b")}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 {
		t.Errorf("a repeat added %d packages", added)
	}
	if store.remaining(7, 100) != 2 {
		t.Errorf("the device holds %d packages after a repeat, expected 2", store.remaining(7, 100))
	}
}

// The device is told when to make more, because it is the only party that can.
func TestADeviceIsToldWhenToPublishMore(t *testing.T) {
	d, _ := newTestDirectory()
	ctx := context.Background()

	count, refill, err := d.Available(ctx, 7, 100)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 || !refill {
		t.Fatalf("a device with nothing published was told %d, refill=%v", count, refill)
	}

	packages := make([][]byte, LowWaterMark+1)
	for i := range packages {
		packages[i] = []byte{byte(i), 0xAA}
	}
	_, _ = d.Publish(ctx, 7, 100, packages, nil, 1)

	count, refill, _ = d.Available(ctx, 7, 100)
	if count != LowWaterMark+1 || refill {
		t.Fatalf("a full device was told %d, refill=%v", count, refill)
	}
}

// Without a bound, one client with a bug fills the table.
func TestADeviceCannotFillTheTable(t *testing.T) {
	d, _ := newTestDirectory()
	ctx := context.Background()

	packages := make([][]byte, MaxPerDevice+10)
	for i := range packages {
		packages[i] = []byte{byte(i / 256), byte(i % 256), 0xBB}
	}

	added, err := d.Publish(ctx, 7, 100, packages, nil, 1)
	if !errors.Is(err, ErrTooMany) {
		t.Fatalf("publishing past the bound was allowed: added %d, err %v", added, err)
	}
	if added > MaxPerDevice {
		t.Errorf("%d packages went in, the bound is %d", added, MaxPerDevice)
	}
}

// An empty package is a client fault and has to be said so, not stored.
func TestAnEmptyPackageIsRefused(t *testing.T) {
	d, _ := newTestDirectory()

	_, err := d.Publish(context.Background(), 7, 100, [][]byte{{}}, nil, 1)
	if !errors.Is(err, ErrEmptyPackage) {
		t.Fatalf("an empty package gave %v", err)
	}
}

// Somebody who published nothing yields nothing - and not an error, because a
// person with no device yet is ordinary, not broken.
func TestSomebodyWithNoDevicesYieldsNothing(t *testing.T) {
	d, _ := newTestDirectory()

	claimed, err := d.Claim(context.Background(), 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 0 {
		t.Fatalf("got %d packages from somebody who published none", len(claimed))
	}
}

func newTestDirectory() (*Directory, *mapStore) {
	store := &mapStore{}
	return New(store), store
}

// mapStore is the smallest thing that behaves like the table.
type mapStore struct {
	packages []KeyPackage
}

func (m *mapStore) remaining(userId, authKeyId int64) int {
	n, _ := m.CountAvailable(context.Background(), userId, authKeyId)
	return n
}

func (m *mapStore) Insert(_ context.Context, p KeyPackage) (bool, error) {
	for _, existing := range m.packages {
		if existing.UserId == p.UserId && existing.AuthKeyId == p.AuthKeyId &&
			existing.Fingerprint == p.Fingerprint {
			return false, nil
		}
	}
	m.packages = append(m.packages, p)
	return true, nil
}

func (m *mapStore) TakeOne(_ context.Context, userId, authKeyId int64) (*KeyPackage, error) {
	for i, p := range m.packages {
		if p.UserId == userId && p.AuthKeyId == authKeyId && !p.LastResort {
			m.packages = append(m.packages[:i], m.packages[i+1:]...)
			return &p, nil
		}
	}
	return nil, nil
}

func (m *mapStore) LastResort(_ context.Context, userId, authKeyId int64) (*KeyPackage, error) {
	for _, p := range m.packages {
		if p.UserId == userId && p.AuthKeyId == authKeyId && p.LastResort {
			return &p, nil
		}
	}
	return nil, nil
}

func (m *mapStore) CountAvailable(_ context.Context, userId, authKeyId int64) (int, error) {
	count := 0
	for _, p := range m.packages {
		if p.UserId == userId && p.AuthKeyId == authKeyId && !p.LastResort {
			count++
		}
	}
	return count, nil
}

func (m *mapStore) Devices(_ context.Context, userId int64) ([]int64, error) {
	seen := map[int64]bool{}
	var devices []int64
	for _, p := range m.packages {
		if p.UserId == userId && !seen[p.AuthKeyId] {
			seen[p.AuthKeyId] = true
			devices = append(devices, p.AuthKeyId)
		}
	}
	return devices, nil
}
