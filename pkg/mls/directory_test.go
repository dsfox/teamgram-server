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

	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone-a"), []byte("phone-b")}, nil, []byte("me"), 1)
	_, _ = d.Publish(ctx, 7, 200, [][]byte{[]byte("laptop-a")}, nil, []byte("me"), 1)

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
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("one"), []byte("two")}, nil, []byte("me"), 1)

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
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("only-one")}, []byte("last-resort"), []byte("me"), 1)

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
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone")}, nil, []byte("me"), 1)
	_, _ = d.Publish(ctx, 7, 200, [][]byte{[]byte("laptop")}, nil, []byte("me"), 1)

	_, _ = d.Claim(ctx, 7) // empties both

	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("phone-again")}, nil, []byte("me"), 2)

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

	added, err := d.Publish(ctx, 7, 100, [][]byte{[]byte("a"), []byte("b")}, nil, []byte("me"), 1)
	if err != nil || added != 2 {
		t.Fatalf("first publish added %d (%v)", added, err)
	}

	added, err = d.Publish(ctx, 7, 100, [][]byte{[]byte("a"), []byte("b")}, nil, []byte("me"), 1)
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
	_, _ = d.Publish(ctx, 7, 100, packages, nil, []byte("me"), 1)

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

	added, err := d.Publish(ctx, 7, 100, packages, nil, []byte("me"), 1)
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

	_, err := d.Publish(context.Background(), 7, 100, [][]byte{{}}, nil, []byte("me"), 1)
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

func (m *mapStore) ForgetOtherNames(_ context.Context, userId, authKeyId int64, name []byte) (int, error) {
	kept := m.packages[:0]
	gone := 0
	for _, existing := range m.packages {
		if existing.UserId == userId && existing.AuthKeyId == authKeyId &&
			string(existing.Name) != string(name) {
			gone++
			continue
		}
		kept = append(kept, existing)
	}
	m.packages = kept
	return gone, nil
}

func (m *mapStore) Insert(_ context.Context, p KeyPackage) (bool, error) {
	// The column is NOT NULL and a nil slice arrives there as NULL, which MySQL
	// refuses. A fake that accepts what the real table rejects is how a client
	// that sends no name got a 500 on every publish and could never refill.
	if p.Name == nil {
		return false, errors.New("name cannot be null")
	}

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

func (m *mapStore) CountDevices(ctx context.Context, userId int64) (int, error) {
	devices, err := m.Devices(ctx, userId)
	return len(devices), err
}

// Publishing nothing is how a device asks how many it has left, and a device
// that is full has to be able to ask.
//
// The client used to publish thirty every few minutes whatever the answer, so a
// phone nobody was starting conversations with reached the bound within the
// hour and was refused from then on - an error line on the server every few
// minutes, and the health check firing about it daily. It now asks first and
// makes packages only when the answer says to, which only works if asking is
// free: an empty publish must store nothing, refuse nothing, and still count.
func TestAskingHowManyAreLeftIsNotPublishing(t *testing.T) {
	d, _ := newTestDirectory()
	ctx := context.Background()

	packages := make([][]byte, MaxPerDevice)
	for i := range packages {
		packages[i] = []byte{byte(i / 256), byte(i % 256), 0xCC}
	}
	if _, err := d.Publish(ctx, 7, 100, packages, nil, []byte("me"), 1); err != nil {
		t.Fatal(err)
	}

	added, err := d.Publish(ctx, 7, 100, nil, nil, []byte("me"), 2)
	if err != nil {
		t.Fatalf("a full device cannot ask how many it has: %v", err)
	}
	if added != 0 {
		t.Errorf("asking stored %d packages", added)
	}

	count, refill, err := d.Available(ctx, 7, 100)
	if err != nil {
		t.Fatal(err)
	}
	if count != MaxPerDevice || refill {
		t.Errorf("a full device was told %d, refill=%v", count, refill)
	}
}

// A person's devices are counted, and counted the same way the list is made.
//
// The count is what tells a phone that another phone of the same person has
// signed in (#41), and it went out wrong the first time: the list was taken and
// measured, which answered zero for a person the database had a row for. So the
// count has a test of its own rather than being assumed to agree with the list.
func TestCountingDevicesAgreesWithListingThem(t *testing.T) {
	directory, store := newTestDirectory()
	ctx := context.Background()

	if count, _ := directory.DeviceCount(ctx, 7); count != 0 {
		t.Fatalf("a person with nothing published has %d devices", count)
	}

	for _, device := range []int64{100, 100, 200} {
		if _, err := directory.Publish(ctx, 7, device,
			[][]byte{[]byte("package-" + string(rune('a'+len(store.packages))))},
			nil, []byte("me"), 1); err != nil {
			t.Fatalf("cannot publish: %v", err)
		}
	}

	listed, err := store.Devices(ctx, 7)
	if err != nil {
		t.Fatalf("cannot list: %v", err)
	}
	count, err := directory.DeviceCount(ctx, 7)
	if err != nil {
		t.Fatalf("cannot count: %v", err)
	}
	if count != len(listed) || count != 2 {
		t.Fatalf("counted %d devices, listed %d, expected 2", count, len(listed))
	}
}

// A device that has started its state over offers nothing of the identity it
// lost.
//
// It makes a new identity - after a reinstall, or because what it held was
// named for somebody else - and what it published under the old one stays here.
// The supply is counted by the device rather than by the identity, so the
// server sees a full shelf, never asks for more, and the new identity publishes
// nothing at all. Everybody who then starts a conversation with that person
// claims a package of the identity that is gone and builds an invitation the
// person can never open. Nothing fails anywhere: two ticks on one screen, a
// padlock that never opens on the other (#136).
func TestAnIdentityThatIsGoneStopsBeingOffered(t *testing.T) {
	d, store := newTestDirectory()
	ctx := context.Background()

	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("old-a"), []byte("old-b")}, nil, []byte("7/before"), 1)
	if store.remaining(7, 100) != 2 {
		t.Fatalf("the old identity published %d packages, wanted 2", store.remaining(7, 100))
	}

	// The same device, a new identity, saying so.
	added, err := d.Publish(ctx, 7, 100, [][]byte{[]byte("new-a")}, nil, []byte("7/after"), 2)
	if err != nil {
		t.Fatalf("publishing under a new identity failed: %v", err)
	}
	if added != 1 {
		t.Fatalf("added %d packages, wanted 1", added)
	}
	if left := store.remaining(7, 100); left != 1 {
		t.Fatalf("%d packages are on offer, wanted only the one of the identity "+
			"this device has - the rest can still be claimed, and what is built "+
			"from them can never be opened", left)
	}

	claimed, err := d.Claim(ctx, 7)
	if err != nil {
		t.Fatalf("claiming failed: %v", err)
	}
	if len(claimed) != 1 || string(claimed[0].Bytes) != "new-a" {
		t.Fatalf("claimed %v, wanted only the package of the identity that is here", claimed)
	}
}

// A device that cannot say which identity it has keeps what it published.
//
// An empty name is a client that has not been taught to say it, and throwing
// its supply away on that would be worse than the fault this is for: nobody
// could start a conversation with it at all.
func TestADeviceThatCannotSayKeepsWhatItHas(t *testing.T) {
	d, store := newTestDirectory()
	ctx := context.Background()

	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("a"), []byte("b")}, nil, []byte("7/one"), 1)
	_, _ = d.Publish(ctx, 7, 100, [][]byte{[]byte("c")}, nil, nil, 2)

	if left := store.remaining(7, 100); left != 3 {
		t.Fatalf("%d packages left, wanted all 3 kept", left)
	}
}
