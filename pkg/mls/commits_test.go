package mls

import (
	"context"
	"errors"
	"testing"
)

// The whole reason this exists: two people change a group at the same moment,
// and only one of them can be right.
//
// Both commits are made from epoch 4 and both are valid MLS. If both were taken
// the group would fork - two halves each certain of a different membership,
// each unable to read the other - and nothing would say so until somebody's
// messages stopped opening. So the second is refused, and refused in a way that
// tells its author what to do: catch up, rebuild, send again.
func TestTwoCommitsFromOneEpochAndOnlyOneIsTaken(t *testing.T) {
	groups := &mapGroups{}
	commits := &mapCommits{devices: map[int64][]int64{9: {100}}}
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
	waiting, _ := WaitingCommits(ctx, commits, 9, 100)
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
	groups := &mapGroups{}
	commits := &mapCommits{devices: map[int64][]int64{9: {100}}}
	ctx := context.Background()
	group := []byte("conversation")

	_, _ = Accept(ctx, groups, commits, group, 4, 1, []int64{9}, []byte("winner"), 1)
	if _, err := Accept(ctx, groups, commits, group, 5, 2, []int64{9}, []byte("rebuilt"), 1); err != nil {
		t.Fatalf("the rebuilt commit was refused: %v", err)
	}

	waiting, _ := WaitingCommits(ctx, commits, 9, 100)
	if len(waiting) != 2 {
		t.Fatalf("%d commits waiting, expected both", len(waiting))
	}
}

// A conversation nobody has mentioned yet is the first commit after it was
// created. There is nothing to disagree with, so it is taken at whatever epoch
// it names.
func TestTheFirstCommitOfAGroupIsTaken(t *testing.T) {
	groups := &mapGroups{}
	commits := &mapCommits{devices: map[int64][]int64{9: {100}}}

	if _, err := Accept(context.Background(), groups, commits,
		[]byte("new"), 1, 1, []int64{9}, []byte("first"), 1); err != nil {
		t.Fatalf("the first commit of a group was refused: %v", err)
	}
}

// Every device of every member, because a person reads on more than one phone
// and the one left out sits an epoch behind for ever.
func TestACommitReachesEveryDeviceOfEveryMember(t *testing.T) {
	groups := &mapGroups{}
	commits := &mapCommits{devices: map[int64][]int64{
		9: {100, 200},
		8: {300},
	}}

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
	groups := &mapGroups{}
	commits := &mapCommits{devices: map[int64][]int64{9: {100}}}
	ctx := context.Background()

	_, _ = Accept(ctx, groups, commits, []byte("g"), 1, 1, []int64{9}, []byte("c"), 1)
	waiting, _ := WaitingCommits(ctx, commits, 9, 100)
	if len(waiting) != 1 {
		t.Fatalf("%d waiting", len(waiting))
	}

	// Fetched and not confirmed: it must still be there.
	again, _ := WaitingCommits(ctx, commits, 9, 100)
	if len(again) != 1 {
		t.Fatalf("a commit that was never confirmed was forgotten")
	}

	if _, err := ConfirmCommits(ctx, commits, 9, 100, []int64{waiting[0].Id}); err != nil {
		t.Fatal(err)
	}
	after, _ := WaitingCommits(ctx, commits, 9, 100)
	if len(after) != 0 {
		t.Fatalf("%d waiting after it was applied", len(after))
	}
}

// ----------------------------------------------------------------------
// Stores that live in a map, so the ordering can be tested without MySQL.
// ----------------------------------------------------------------------

type mapGroups struct {
	epochs map[string]int64
}

func (m *mapGroups) Epoch(_ context.Context, groupId []byte) (int64, bool, error) {
	if m.epochs == nil {
		return 0, false, nil
	}
	epoch, ok := m.epochs[string(groupId)]
	return epoch, ok, nil
}

func (m *mapGroups) Advance(_ context.Context, groupId []byte, from int64) (bool, error) {
	if m.epochs == nil {
		m.epochs = map[string]int64{}
	}
	known, ok := m.epochs[string(groupId)]
	if ok && known != from {
		return false, nil
	}
	m.epochs[string(groupId)] = from + 1
	return true, nil
}

type mapCommits struct {
	rows    []Commit
	devices map[int64][]int64
	nextId  int64
}

func (m *mapCommits) Put(_ context.Context, c Commit) error {
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

func (m *mapCommits) Devices(_ context.Context, userId int64) ([]int64, error) {
	return m.devices[userId], nil
}
