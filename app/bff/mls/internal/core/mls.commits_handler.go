package core

import (
	"errors"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/mls"
)

// MlsSendCommit takes a membership change, if it was made from the epoch the
// conversation is actually on, and leaves it for everybody who has to apply it.
//
// This is the one place the server has an opinion about a conversation. MLS
// validates a commit against the epoch it was made from, so of two made from
// the same epoch exactly one can be taken - RFC 9420 gives that ordering to the
// delivery service, and here that is us. What it learns is that a group exists
// and which epoch it is on; not who is in it, and not a word of what is said.
//
// mls.sendCommit group_id:bytes epoch:long members:Vector<long> commit:bytes = mls.CommitResult;
func (c *MlsCore) MlsSendCommit(in *mtproto.TLMlsSendCommit) (*mtproto.Mls_CommitResult, error) {
	posted, err := mls.Accept(
		c.ctx,
		c.svcCtx.Groups,
		c.svcCtx.Commits,
		in.GetGroupId(),
		in.GetEpoch(),
		c.MD.UserId,
		in.GetMembers(),
		in.GetCommit(),
		int32(time.Now().Unix()))

	if err != nil {
		switch {
		case errors.Is(err, mls.ErrBehind):
			// Not a failure, and it must not be answered as one: an error
			// would make the client back off and retry the same bytes,
			// which can never be accepted. What it needs is the epoch the
			// group is really on, so it can apply what it missed, rebuild
			// its change on top and send that.
			epoch, _, readErr := c.svcCtx.Groups.Epoch(c.ctx, in.GetGroupId())
			if readErr != nil {
				c.Logger.Errorf("mls.sendCommit - cannot read the epoch: %v", readErr)
				return nil, mtproto.ErrInternalServerError
			}
			c.Logger.Infof("mls.sendCommit - %d was at epoch %d, the group is at %d",
				c.MD.UserId, in.GetEpoch(), epoch)
			return &mtproto.Mls_CommitResult{Accepted: false, Epoch: epoch}, nil
		case errors.Is(err, mls.ErrEmptyPackage):
			c.Logger.Errorf("mls.sendCommit - an empty commit from %d", c.MD.UserId)
			return nil, mtproto.ErrInputRequestInvalid
		case errors.Is(err, mls.ErrTooMany):
			c.Logger.Errorf("mls.sendCommit - a device has as many waiting as it may")
			return nil, mtproto.NewErrFloodWaitX(3600)
		default:
			c.Logger.Errorf("mls.sendCommit - %v", err)
			return nil, mtproto.ErrInternalServerError
		}
	}

	c.Logger.Infof("mls.sendCommit - epoch %d taken, left for %d device(s)",
		in.GetEpoch(), posted)
	return &mtproto.Mls_CommitResult{Accepted: true, Epoch: in.GetEpoch() + 1}, nil
}

// MlsGetCommits is what this device has not applied yet, oldest first.
//
// The order matters: a commit applies only to the epoch it was made from, so
// out of order every one but the first fails.
//
// mls.getCommits = mls.Commits;
func (c *MlsCore) MlsGetCommits(in *mtproto.TLMlsGetCommits) (*mtproto.Mls_Commits, error) {
	waiting, err := mls.WaitingCommits(c.ctx, c.svcCtx.Commits, c.MD.UserId, c.MD.PermAuthKeyId)
	if err != nil {
		c.Logger.Errorf("mls.getCommits - %v", err)
		return nil, mtproto.ErrInternalServerError
	}

	commits := make([]*mtproto.Mls_Commit, 0, len(waiting))
	for _, w := range waiting {
		commits = append(commits, &mtproto.Mls_Commit{
			Id:      w.Id,
			FromId:  w.FromId,
			GroupId: w.GroupId,
			Epoch:   w.Epoch,
			Commit:  w.Bytes,
		})
	}

	return &mtproto.Mls_Commits{Commits: commits}, nil
}

// MlsConfirmCommits forgets what this device says it has applied.
//
// Applied, not received. A device that took a commit and stopped before saving
// the state it produced has to be given it again, or it sits an epoch behind
// and the conversation goes quiet for that person alone - which surfaces much
// later and looks like anything but a lost commit.
//
// mls.confirmCommits ids:Vector<long> = mls.Ok;
func (c *MlsCore) MlsConfirmCommits(in *mtproto.TLMlsConfirmCommits) (*mtproto.Mls_Ok, error) {
	confirmed, err := mls.ConfirmCommits(
		c.ctx, c.svcCtx.Commits, c.MD.UserId, c.MD.PermAuthKeyId, in.GetIds())
	if err != nil {
		c.Logger.Errorf("mls.confirmCommits - %v", err)
		return nil, mtproto.ErrInternalServerError
	}

	c.Logger.Infof("mls.confirmCommits - %d applied and forgotten", confirmed)
	return &mtproto.Mls_Ok{Ok: true}, nil
}
