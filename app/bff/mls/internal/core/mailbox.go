package core

import (
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
)

// tellToCheckTheBox says, to every device of each named person, that there is
// something in its box (#156).
//
// It carries no payload. The device fetches the box through mls.getWelcomes and
// mls.getCommits and empties it through their confirmations, so there is one
// path to apply a commit or a welcome and one to say it was applied - the push
// only says "now, not on your next poll".
//
// Best effort on purpose. The box is the truth; this is the fast path to it.
// A device that misses the push still fetches on its next start and on the
// catch-up a message that will not open triggers, so a dropped push costs a
// delay, never a lost commit. That is why the error is logged and stepped over
// rather than failing the send that has already been recorded.
//
// One push per person, to all their sessions, rather than one per device: the
// server would otherwise have to match a mailbox row's auth key to a live
// session's, and a device with nothing waiting simply fetches an empty box -
// the same cheap request the old poll made, now made when there is a reason.
func (c *MlsCore) tellToCheckTheBox(userIds ...int64) {
	if c.svcCtx.SyncClient == nil {
		return
	}
	seen := make(map[int64]bool, len(userIds))
	for _, userId := range userIds {
		if userId == 0 || seen[userId] {
			continue
		}
		seen[userId] = true
		updates := mtproto.MakeTLUpdateShort(&mtproto.Updates{
			Update: mtproto.MakeTLUpdateMlsMailbox(nil).To_Update(),
			Date:   int32(0),
		}).To_Updates()
		if _, err := c.svcCtx.SyncClient.SyncPushUpdates(c.ctx, &sync.TLSyncPushUpdates{
			UserId:  userId,
			Updates: updates,
		}); err != nil {
			c.Logger.Errorf("mls: could not tell %d to check its box: %v", userId, err)
		}
	}
}
