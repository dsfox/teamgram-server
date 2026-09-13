package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
)

// PhoneSendSignalingData carries a blob from one leg of a call to the other.
//
// phone.sendSignalingData peer:InputPhoneCall data:bytes = Bool;
//
// This is the ICE-candidate channel: the two phones trade candidates through
// it until one pair connects, so a direct path exists only if these bytes
// arrive, whole and quickly. The server does not look inside - the blob is
// encrypted under the call key, which it never had.
func (c *PhoneCore) PhoneSendSignalingData(in *mtproto.TLPhoneSendSignalingData) (*mtproto.Bool, error) {
	now := time.Now()
	call, err := c.find(in.GetPeer(), now)
	if err != nil {
		return nil, err
	}
	otherLeg, err := call.Other(c.MD.UserId)
	if err != nil {
		c.Logger.Errorf("phone.sendSignalingData - %d is not in call %d: %v", c.MD.UserId, call.Id, err)
		return nil, err
	}

	update := mtproto.MakeTLUpdatePhoneCallSignalingData(&mtproto.Update{
		PhoneCallId:    call.Id,
		Data_FLAGBYTES: in.GetData(),
	}).To_Update()

	// Unlike a ring, a lost candidate is a lost candidate: the phone does not
	// resend it, so the caller is told when the push failed.
	if _, err = c.svcCtx.SyncClient.SyncPushUpdates(c.ctx, &sync.TLSyncPushUpdates{
		UserId: otherLeg,
		Updates: mtproto.MakeTLUpdateShort(&mtproto.Updates{
			Update: update,
			Date:   int32(now.Unix()),
		}).To_Updates(),
	}); err != nil {
		c.Logger.Errorf("phone.sendSignalingData - could not reach %d for call %d: %v", otherLeg, call.Id, err)
		return nil, err
	}

	return mtproto.BoolTrue, nil
}
