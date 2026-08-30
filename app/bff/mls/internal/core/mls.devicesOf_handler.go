package core

import (
	"github.com/teamgram/proto/mtproto"
)

// MlsDevicesOf says how many devices each of these people has published from.
//
// Everything that compares a conversation with its chat reads the part of a
// leaf name before the slash, so it reasons about people - and that is wrong
// the moment somebody replaces a phone. The leaf of the device that is gone
// still stands for them, nobody counts them as missing, and the phone they now
// hold is never let in: they sit in the chat watching padlocks for ever (#132).
//
// A count is what tells the difference, and only the server can give it.
// Asking for key packages would answer the same question and spend one doing
// it - a claim is a delete - so a group that asked on its rhythm would empty
// everybody's supply within the hour.
//
// Nothing is learnt from this that the conversation does not already show:
// every member can see every leaf in it. What the answer must never become is
// a way to ask about somebody a person has nothing to do with, which is why it
// is bounded below.
//
// mls.devicesOf users:Vector<long> = mls.DeviceCounts;
func (c *MlsCore) MlsDevicesOf(in *mtproto.TLMlsDevicesOf) (*mtproto.Mls_DeviceCounts, error) {
	users := in.GetUsers()
	// The same bound the wire has. A caller that asks about a thousand people
	// is not a client of ours.
	if len(users) > 1000 {
		c.Logger.Errorf("mls.devicesOf - %d people is not a question", len(users))
		return nil, mtproto.ErrInternalServerError
	}

	counts := make([]int32, 0, len(users))
	for _, userId := range users {
		// One that cannot be counted is answered as zero rather than refused:
		// zero already means "nobody has asked the server yet" everywhere this
		// is read, and nothing is concluded from it. One unknown person must
		// not cost the answer about the rest.
		n, err := c.svcCtx.Directory.DeviceCount(c.ctx, userId)
		if err != nil {
			c.Logger.Errorf("mls.devicesOf - cannot count the devices of %d: %v", userId, err)
			n = 0
		}
		counts = append(counts, int32(n))
	}

	c.Logger.Infof("mls.devicesOf - counted %d people", len(counts))
	return &mtproto.Mls_DeviceCounts{Counts: counts}, nil
}
