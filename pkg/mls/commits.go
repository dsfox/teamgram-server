package mls

import (
	"context"
	"errors"
	"fmt"
)

// A commit is what moves a conversation from one epoch to the next: somebody
// added, somebody removed. MLS validates it against the epoch it was made from,
// so of two commits made from the same epoch exactly one can be taken. The other
// has to be refused, and its author has to apply the winner and try again.
//
// RFC 9420 gives that ordering to the delivery service. Here that is this file,
// and it is the one place the server stops being dumb: it learns that a group
// exists and which epoch it is on. Not who is in it, not a word of what is said.

// ErrBehind is a commit made from an epoch the conversation has already left.
// The caller lost a race; the answer is to catch up and send again, not to
// retry the same bytes.
var ErrBehind = errors.New("mls: that commit was made from an older epoch")

// GroupStore remembers which epoch each conversation is on.
type GroupStore interface {
	// Epoch is what the next commit must declare, and whether anything is
	// known about this group at all.
	Epoch(ctx context.Context, groupId []byte) (int64, bool, error)
	// Advance moves the group from `from` to `from+1`, and says whether it
	// was the one to do it. Two callers arriving with the same `from` must
	// not both get true - that is the whole job, and it belongs in the
	// storage because only the storage can be atomic about it.
	Advance(ctx context.Context, groupId []byte, from int64) (bool, error)
}

// CommitStore keeps commits waiting for the devices that have to apply them.
type CommitStore interface {
	Put(ctx context.Context, c Commit) error
	Waiting(ctx context.Context, userId, authKeyId int64) ([]Commit, error)
	Confirm(ctx context.Context, userId, authKeyId int64, ids []int64) (int, error)
	Devices(ctx context.Context, userId int64) ([]int64, error)
}

// Commit is one waiting for a device.
type Commit struct {
	Id        int64
	UserId    int64
	AuthKeyId int64
	FromId    int64
	GroupId   []byte
	Epoch     int64
	Bytes     []byte
	Date      int32
}

// Accept takes a commit if it was made from the epoch the group is actually on,
// and leaves it for every device of everybody named.
//
// The order is deliberate: the epoch moves first, and only then is the commit
// handed out. The other way round, two callers could both hand out a commit and
// one of those groups would be unreadable for everybody afterwards. Handing out
// a commit for an epoch that then fails to advance is recoverable - the devices
// refuse a commit they cannot apply - and losing the group is not.
//
// A group nobody has told us about yet is accepted at whatever epoch the caller
// names. That is the first commit after the conversation was created, and there
// is nothing to disagree with it.
//
// Every device, including the one that made the commit. That looks wasteful and
// is the opposite: a client leaves its own commit unapplied until it hears that
// it won, and if that answer is lost - a dropped connection, a phone that
// stopped - it has no other way to find out. Handed its own commit back, it
// knows. MLS names that case (`StageCommitError::OwnCommit`) precisely because
// delivery services echo, and the remedy is to apply what is already staged.
//
// So the answer to this call is a shortcut, not the truth. The truth is the
// commit box, and a device that only ever read from there would still be right.
func Accept(
	ctx context.Context,
	groups GroupStore,
	commits CommitStore,
	groupId []byte,
	epoch int64,
	fromId int64,
	members []int64,
	commit []byte,
	now int32,
) (int, error) {
	if len(groupId) == 0 || len(commit) == 0 {
		return 0, ErrEmptyPackage
	}

	known, exists, err := groups.Epoch(ctx, groupId)
	if err != nil {
		return 0, fmt.Errorf("cannot read the epoch: %w", err)
	}
	if exists && known != epoch {
		return 0, ErrBehind
	}

	moved, err := groups.Advance(ctx, groupId, epoch)
	if err != nil {
		return 0, fmt.Errorf("cannot move the epoch: %w", err)
	}
	if !moved {
		// Somebody else got there between the read and the write. Same
		// answer as an epoch that was already old: catch up and try again.
		return 0, ErrBehind
	}

	posted := 0
	for _, userId := range members {
		devices, err := commits.Devices(ctx, userId)
		if err != nil {
			return posted, fmt.Errorf("cannot list the devices: %w", err)
		}
		for _, authKeyId := range devices {
			waiting, err := commits.Waiting(ctx, userId, authKeyId)
			if err != nil {
				return posted, fmt.Errorf("cannot count what is waiting: %w", err)
			}
			if len(waiting) >= MaxWaitingPerDevice {
				return posted, ErrTooMany
			}
			if err = commits.Put(ctx, Commit{
				UserId:    userId,
				AuthKeyId: authKeyId,
				FromId:    fromId,
				GroupId:   groupId,
				Epoch:     epoch,
				Bytes:     commit,
				Date:      now,
			}); err != nil {
				return posted, fmt.Errorf("cannot leave the commit: %w", err)
			}
			posted++
		}
	}

	return posted, nil
}

// WaitingCommits is what a device has not applied yet, oldest first.
//
// Order is not a nicety here: a commit can only be applied to the epoch it was
// made from, so out of order they all fail but the first.
func WaitingCommits(ctx context.Context, commits CommitStore, userId, authKeyId int64) ([]Commit, error) {
	return commits.Waiting(ctx, userId, authKeyId)
}

// ConfirmCommits forgets what a device says it has applied.
//
// Applied, not received: a device that took one and stopped before saving the
// new state must get it again, or it sits an epoch behind and the conversation
// goes quiet for that person alone.
func ConfirmCommits(ctx context.Context, commits CommitStore, userId, authKeyId int64, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	return commits.Confirm(ctx, userId, authKeyId, ids)
}
