package mls

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"
)

// A commit is what moves a conversation from one epoch to the next: somebody
// added, somebody removed. MLS validates it against the epoch it was made from,
// so of two commits made from the same epoch exactly one can be taken. The other
// has to be refused, and its author has to apply the winner and try again.
//
// RFC 9420 gives that ordering to the delivery service. Here that is this file:
// it learns that a group exists and which epoch it is on. Who is in the group
// it is told outright and keeps in mls_members (#147). What it cannot do is
// read a word of what is said, and that is the line that matters.

// ErrBehind is a commit made from an epoch the conversation has already left.
// The caller lost a race; the answer is to catch up and send again, not to
// retry the same bytes.
var ErrBehind = errors.New("mls: that commit was made from an older epoch")

// GroupStore remembers which epoch each conversation is on.
type GroupStore interface {
	// Epoch is what the next commit must declare, and whether anything is
	// known about this group at all.
	Epoch(ctx context.Context, groupId []byte) (int64, bool, error)
	// Take moves the group from `from` to `from+1` and leaves the commit for
	// every device named, or does neither of those things.
	//
	// Two callers arriving with the same `from` must not both get true - that
	// is the ordering job, and it belongs in the storage because only the
	// storage can be atomic about it. Handing the commit over belongs here for
	// the same reason and was not, once: the epoch moved first and the copies
	// followed, so between them the group had gone somewhere the commit that
	// leads there could not yet be fetched from. A delivery that failed halfway
	// left the group ahead and some of its members behind for good - the exact
	// shape of #116, and nothing on either side could undo it (#120).
	Take(ctx context.Context, groupId []byte, from int64, deliveries []Commit) (bool, error)
}

// CommitStore keeps commits waiting for the devices that have to apply them.
type CommitStore interface {
	Waiting(ctx context.Context, userId, authKeyId int64) ([]Commit, error)
	Confirm(ctx context.Context, userId, authKeyId int64, ids []int64) (int, error)
	ForgetOlderThan(ctx context.Context, userId, authKeyId int64, before int32) (int, error)
	Devices(ctx context.Context, userId int64) ([]int64, error)
}

// CommitLife is how long a commit waits for the device it was left for.
//
// Until this there was no such thing: the only way a commit left the table was
// a device saying it had applied one, so a commit it cannot apply and never
// will - for a group it has left, or waiting behind an earlier one that is not
// coming - stayed for ever and was handed over every thirty seconds. Seven of
// them sat on the stand against one device, a day old (#145).
//
// The same fortnight an invitation has, and it is worth saying why rather than
// copying it. Past that long a device is not a device (#138): its invitations
// have already been swept, so it cannot be caught up by one either, and when it
// does come back it is let in afresh by the comparison that runs anyway (#132,
// #139) rather than by working through a chain. A commit older than that has
// nothing left it could be applied to, so the two are the same number rather
// than two that happen to agree.
//
// It is generous for the case it must not break - a phone off for a week comes
// back and finishes its chain - because what is bounded here is a device that
// will never apply them, not one that is late.
const CommitLife int32 = WelcomeLife

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

	// Everybody who has to apply this, worked out before anything is written.
	// Reading first and writing once is what makes the write a single step: a
	// device that signs in between the two rounds misses this commit and is
	// caught up by its welcome, which is the ordinary path and not a hole.
	deliveries := make([]Commit, 0, len(members))
	for _, userId := range members {
		devices, err := commits.Devices(ctx, userId)
		if err != nil {
			return 0, fmt.Errorf("cannot list the devices: %w", err)
		}
		for _, authKeyId := range devices {
			waiting, err := commits.Waiting(ctx, userId, authKeyId)
			if err != nil {
				return 0, fmt.Errorf("cannot count what is waiting: %w", err)
			}
			if len(waiting) >= MaxWaitingPerDevice {
				return 0, ErrTooMany
			}
			deliveries = append(deliveries, Commit{
				UserId:    userId,
				AuthKeyId: authKeyId,
				FromId:    fromId,
				GroupId:   groupId,
				Epoch:     epoch,
				Bytes:     commit,
				Date:      now,
			})
		}
	}

	// The epoch and the copies together. Either the group has moved and every
	// device named can fetch what moved it, or nothing happened at all and the
	// caller is told to catch up and try again.
	moved, err := groups.Take(ctx, groupId, epoch, deliveries)
	if err != nil {
		return 0, fmt.Errorf("cannot move the epoch: %w", err)
	}
	if !moved {
		// Somebody else got there between the read and the write. Same
		// answer as an epoch that was already old: catch up and try again.
		return 0, ErrBehind
	}

	return len(deliveries), nil
}

// WaitingCommits is what a device has not applied yet, oldest first.
//
// Order is not a nicety here: a commit can only be applied to the epoch it was
// made from, so out of order they all fail but the first.
func WaitingCommits(ctx context.Context, commits CommitStore, userId, authKeyId int64, now int32) ([]Commit, error) {
	forgetTheStaleCommits(ctx, commits, userId, authKeyId, now)
	return commits.Waiting(ctx, userId, authKeyId)
}

// forgetTheStaleCommits drops what this device has left waiting past CommitLife.
//
// Hung on the call the device asks with, which is the shape #140 arrived at
// after getting it wrong for invitations: a sweep hanging on somebody else's
// send runs for the device somebody is committing to and never for the one
// drowning in them. Asking is also where the harm is - a phone handed a pile it
// cannot apply works through it instead of reading.
//
// Housekeeping, so a failure is logged and stepped over: handing over what is
// waiting must not fail because the tidying did.
func forgetTheStaleCommits(ctx context.Context, commits CommitStore, userId, authKeyId int64, now int32) {
	forgotten, err := commits.ForgetOlderThan(ctx, userId, authKeyId, now-CommitLife)
	if err != nil {
		logx.WithContext(ctx).Errorf(
			"mls: cannot forget the commits nobody applied: %v", err)
		return
	}
	if forgotten > 0 {
		logx.WithContext(ctx).Infof(
			"mls: %d commit(s) of %d/%d had been waiting longer than any device "+
				"comes back from", forgotten, userId, authKeyId)
	}
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
