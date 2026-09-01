package mls

import (
	"context"
	"errors"
	"testing"
)

// The moment these tests ask at, which is the one they write at: the rows here
// are made with Accept(..., 1), so asking at the same second asks about commits
// that cannot be stale. A wall clock instead would sweep every one of them - a
// row dated 1 is older than any fortnight - and four tests failed exactly that
// way before this was written down.
const now int32 = 1

// The whole reason this exists: two people change a group at the same moment,
// and only one of them can be right.
//
// Both commits are made from epoch 4 and both are valid MLS. If both were taken
// the group would fork - two halves each certain of a different membership,
// each unable to read the other - and nothing would say so until somebody's
// messages stopped opening. So the second is refused, and refused in a way that
// tells its author what to do: catch up, rebuild, send again.
func TestTwoCommitsFromOneEpochAndOnlyOneIsTaken(t *testing.T) {
	commits := &mapCommits{devices: map[int64][]int64{9: {100}}}
	groups := &mapGroups{commits: commits}
	ctx := context.Background()
	group := []byte("conversation")

	posted, err := Accept(ctx, groups, commits, group, 4, 1, []int64{9}, []byte("alice adds dave"), 1)
	if err != nil {
		t.Fatalf("the first commit should have been taken: %v", err)
	}
	if posted != 1 {
		t.Fatalf("left %d copies for one device", posted)
	}

	_, err = Accept(ctx, groups, commits, group, 4, 2, []int64{9}, []byte("bob adds erin"), 1)
	if !errors.Is(err, ErrBehind) {
		t.Fatalf("the second commit from epoch 4 was answered with %v", err)
	}

	// And the loser's commit was not handed out: a device that applied both
	// would be applying one to an epoch it was not made from.
	waiting, _ := WaitingCommits(ctx, commits, 9, 100, now)
	if len(waiting) != 1 {
		t.Fatalf("%d commits waiting; only the winner should be", len(waiting))
	}
	if string(waiting[0].Bytes) != "alice adds dave" {
		t.Errorf("the wrong commit is waiting: %s", waiting[0].Bytes)
	}
}

// Having caught up, the loser tries again from the epoch the group is now on,
// and that one is taken. Without this the answer above would be a dead end
// rather than a race.
func TestTheLoserSucceedsFromTheNewEpoch(t *testing.T) {
	commits := &mapCommits{devices: map[int64][]int64{9: {100}}}
	groups := &mapGroups{commits: commits}
	ctx := context.Background()
	group := []byte("conversation")

	_, _ = Accept(ctx, groups, commits, group, 4, 1, []int64{9}, []byte("winner"), 1)
	if _, err := Accept(ctx, groups, commits, group, 5, 2, []int64{9}, []byte("rebuilt"), 1); err != nil {
		t.Fatalf("the rebuilt commit was refused: %v", err)
	}

	waiting, _ := WaitingCommits(ctx, commits, 9, 100, now)
	if len(waiting) != 2 {
		t.Fatalf("%d commits waiting, expected both", len(waiting))
	}
}

// A conversation nobody has mentioned yet is the first commit after it was
// created. There is nothing to disagree with, so it is taken at whatever epoch
// it names.
func TestTheFirstCommitOfAGroupIsTaken(t *testing.T) {
	commits := &mapCommits{devices: map[int64][]int64{9: {100}}}
	groups := &mapGroups{commits: commits}

	if _, err := Accept(context.Background(), groups, commits,
		[]byte("new"), 1, 1, []int64{9}, []byte("first"), 1); err != nil {
		t.Fatalf("the first commit of a group was refused: %v", err)
	}
}

// Every device of every member, because a person reads on more than one phone
// and the one left out sits an epoch behind for ever.
func TestACommitReachesEveryDeviceOfEveryMember(t *testing.T) {
	commits := &mapCommits{devices: map[int64][]int64{
		9: {100, 200},
		8: {300},
	}}
	groups := &mapGroups{commits: commits}

	posted, err := Accept(context.Background(), groups, commits,
		[]byte("g"), 1, 1, []int64{9, 8}, []byte("c"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if posted != 3 {
		t.Fatalf("left %d copies for three devices", posted)
	}
}

// Applied, not delivered. A device that took a commit and stopped before saving
// the state it produced has to be given it again - otherwise it is an epoch
// behind and the conversation goes quiet for that person alone.
func TestACommitIsKeptUntilTheDeviceSaysItApplied(t *testing.T) {
	commits := &mapCommits{devices: map[int64][]int64{9: {100}}}
	groups := &mapGroups{commits: commits}
	ctx := context.Background()

	_, _ = Accept(ctx, groups, commits, []byte("g"), 1, 1, []int64{9}, []byte("c"), 1)
	waiting, _ := WaitingCommits(ctx, commits, 9, 100, now)
	if len(waiting) != 1 {
		t.Fatalf("%d waiting", len(waiting))
	}

	// Fetched and not confirmed: it must still be there.
	again, _ := WaitingCommits(ctx, commits, 9, 100, now)
	if len(again) != 1 {
		t.Fatalf("a commit that was never confirmed was forgotten")
	}

	if _, err := ConfirmCommits(ctx, commits, 9, 100, []int64{waiting[0].Id}); err != nil {
		t.Fatal(err)
	}
	after, _ := WaitingCommits(ctx, commits, 9, 100, now)
	if len(after) != 0 {
		t.Fatalf("%d waiting after it was applied", len(after))
	}
}

// The device that made the commit is handed it back too.
//
// It looks like waste and is the opposite. A client leaves its own commit
// unapplied until it hears that it won its epoch - that is what stops two
// simultaneous changes forking the group - and if that answer never arrives,
// because the connection dropped or the phone stopped, there is no other way to
// learn the outcome. The commit box is that way. MLS names the case of being
// handed your own commit (`StageCommitError::OwnCommit`) exactly because
// delivery services echo, and the remedy is to apply what is already staged.
//
// Skipping the author instead leaves them stuck at an epoch everybody else has
// left, reading nothing, in a group that goes on without them.
func TestTheCommitComesBackToTheDeviceThatMadeIt(t *testing.T) {
	commits := &mapCommits{devices: map[int64][]int64{
		7: {500, 600}, // the author, on two phones
		9: {100},
	}}
	groups := &mapGroups{commits: commits}
	ctx := context.Background()

	posted, err := Accept(ctx, groups, commits, []byte("g"), 1, 7,
		[]int64{7, 9}, []byte("c"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if posted != 3 {
		t.Fatalf("left %d copies for three devices", posted)
	}

	own, _ := WaitingCommits(ctx, commits, 7, 500, now)
	if len(own) != 1 {
		t.Errorf("the phone that made the commit cannot learn that it won")
	}
	// And the same person's other phone, which is a separate leaf and needs it
	// exactly as much as anybody else's does.
	other, _ := WaitingCommits(ctx, commits, 7, 600, now)
	if len(other) != 1 {
		t.Errorf("the author's other phone got %d commits, not one", len(other))
	}
}

// ----------------------------------------------------------------------
// Stores that live in a map, so the ordering can be tested without MySQL.
// ----------------------------------------------------------------------

type mapGroups struct {
	epochs  map[string]int64
	commits *mapCommits
	failAt  int
}

func (m *mapGroups) Epoch(_ context.Context, groupId []byte) (int64, bool, error) {
	if m.epochs == nil {
		return 0, false, nil
	}
	epoch, ok := m.epochs[string(groupId)]
	return epoch, ok, nil
}

// Take, all of it or none of it - the same promise the transaction in the MySQL
// store makes. `failEvery` lets a test make one delivery fail partway.
func (m *mapGroups) Take(_ context.Context, groupId []byte, from int64, deliveries []Commit) (bool, error) {
	if m.epochs == nil {
		m.epochs = map[string]int64{}
	}
	known, ok := m.epochs[string(groupId)]
	if ok && known != from {
		return false, nil
	}
	// Nothing is written until all of it can be. That is what the transaction
	// in the MySQL store does, and a fake that wrote as it went would be
	// holding a promise nobody makes.
	for i := range deliveries {
		if m.failAt > 0 && i+1 == m.failAt {
			return false, errors.New("the storage gave out halfway through")
		}
	}
	for _, c := range deliveries {
		if err := m.commits.put(c); err != nil {
			return false, err
		}
	}
	m.epochs[string(groupId)] = from + 1
	return true, nil
}

func TestACommitNobodyAppliedIsForgottenWhenTheDeviceAsks(t *testing.T) {
	commits := &mapCommits{devices: map[int64][]int64{9: {100}}}
	groups := &mapGroups{commits: commits}
	ctx := context.Background()

	// One made a fortnight and a day ago, one made now. The clock is the
	// argument, so this needs no waiting and no wall time.
	made := int32(10 * CommitLife)
	stale := made - CommitLife - 86400
	if _, err := Accept(ctx, groups, commits, []byte("old"), 1, 1, []int64{9},
		[]byte("nobody will apply this"), stale); err != nil {
		t.Fatalf("the old commit should have been taken: %v", err)
	}
	if _, err := Accept(ctx, groups, commits, []byte("new"), 1, 1, []int64{9},
		[]byte("this one is fresh"), made); err != nil {
		t.Fatalf("the fresh commit should have been taken: %v", err)
	}

	waiting, err := WaitingCommits(ctx, commits, 9, 100, made)
	if err != nil {
		t.Fatalf("asking failed: %v", err)
	}

	// Both halves. That the stale one goes is just as true of a box that hands
	// over nothing at all, so the fresh one has to arrive beside it.
	if len(waiting) != 1 {
		t.Fatalf("%d commits waiting, expected only the fresh one", len(waiting))
	}
	if string(waiting[0].Bytes) != "this one is fresh" {
		t.Errorf("the wrong commit survived: %s", waiting[0].Bytes)
	}

	// And the box was tidied, not just the answer: leaving the row means the
	// next phone to ask pays for it again.
	if len(commits.rows) != 1 {
		t.Errorf("%d rows are left rather than one", len(commits.rows))
	}
}

type mapCommits struct {
	rows    []Commit
	devices map[int64][]int64
	nextId  int64
}

func (m *mapCommits) put(c Commit) error {
	m.nextId++
	c.Id = m.nextId
	m.rows = append(m.rows, c)
	return nil
}

func (m *mapCommits) Waiting(_ context.Context, userId, authKeyId int64) ([]Commit, error) {
	var found []Commit
	for _, row := range m.rows {
		if row.UserId == userId && row.AuthKeyId == authKeyId {
			found = append(found, row)
		}
	}
	return found, nil
}

func (m *mapCommits) Confirm(_ context.Context, userId, authKeyId int64, ids []int64) (int, error) {
	wanted := map[int64]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	var kept []Commit
	removed := 0
	for _, row := range m.rows {
		if row.UserId == userId && row.AuthKeyId == authKeyId && wanted[row.Id] {
			removed++
			continue
		}
		kept = append(kept, row)
	}
	m.rows = kept
	return removed, nil
}

func (m *mapCommits) ForgetOlderThan(_ context.Context, userId, authKeyId int64, before int32) (int, error) {
	var kept []Commit
	forgotten := 0
	for _, row := range m.rows {
		if row.UserId == userId && row.AuthKeyId == authKeyId && row.Date < before {
			forgotten++
			continue
		}
		kept = append(kept, row)
	}
	m.rows = kept
	return forgotten, nil
}

func (m *mapCommits) Devices(_ context.Context, userId int64) ([]int64, error) {
	return m.devices[userId], nil
}

// A delivery that gives out halfway must leave the group where it was.
//
// This is the failure the two-step version could not survive. It moved the
// epoch first and handed out the copies afterwards, so an error on the third
// device of five left the group ahead with two members holding the commit and
// three without it - and those three can never catch up, because everything
// after it needs the one they never got. Nothing on either side can undo that:
// the device says it has fallen out, and a person has to take it out of the
// chat and let it back in.
//
// So the promise is all of it or none of it, and this is where that is held.
func TestADeliveryThatFailsHalfwayLeavesTheEpochAlone(t *testing.T) {
	commits := &mapCommits{devices: map[int64][]int64{
		9: {100}, 8: {200}, 7: {300}, 6: {400}, 5: {500},
	}}
	groups := &mapGroups{commits: commits, failAt: 3}
	ctx := context.Background()
	group := []byte("conversation")

	// Somewhere to fall back to, so the epoch has a value worth checking.
	groups.epochs = map[string]int64{string(group): 4}

	if _, err := Accept(ctx, groups, commits, group, 4, 1,
		[]int64{9, 8, 7, 6, 5}, []byte("adds frank"), 1); err == nil {
		t.Fatal("a delivery that failed was reported as a commit taken")
	}

	if epoch, _, _ := groups.Epoch(ctx, group); epoch != 4 {
		t.Errorf("the group moved to epoch %d although the commit was never "+
			"handed to everybody it names", epoch)
	}

	// And nobody is holding a commit for an epoch the group is not on.
	for _, device := range []struct{ user, key int64 }{{9, 100}, {8, 200}} {
		waiting, _ := WaitingCommits(ctx, commits, device.user, device.key, now)
		if len(waiting) != 0 {
			t.Errorf("device %d/%d is holding %d commit(s) from a change that "+
				"never happened", device.user, device.key, len(waiting))
		}
	}
}
