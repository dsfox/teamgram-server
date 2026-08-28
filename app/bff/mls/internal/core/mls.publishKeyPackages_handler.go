package core

import (
	"errors"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/mls"
)

// MlsPublishKeyPackages stores the supply a device made, so that somebody can
// start an encrypted conversation with it while it is asleep.
//
// mls.publishKeyPackages key_packages:Vector<bytes> last_resort:bytes = mls.PublishResult;
func (c *MlsCore) MlsPublishKeyPackages(in *mtproto.TLMlsPublishKeyPackages) (*mtproto.Mls_PublishResult, error) {
	// Per device, not per person: several devices of one person are several
	// members of the same conversation, which is the whole reason for MLS here.
	added, err := c.svcCtx.Directory.Publish(
		c.ctx,
		c.MD.UserId,
		c.MD.PermAuthKeyId,
		in.GetKeyPackages(),
		in.GetLastResort(),
		int32(time.Now().Unix()))
	if err != nil {
		// A client that sent rubbish or too much is told so; anything else is
		// ours and says nothing about itself to the phone.
		switch {
		case errors.Is(err, mls.ErrEmptyPackage):
			c.Logger.Errorf("mls.publishKeyPackages - empty package from %d", c.MD.UserId)
			return nil, mtproto.ErrInputRequestInvalid
		case errors.Is(err, mls.ErrTooMany):
			c.Logger.Errorf("mls.publishKeyPackages - %d is holding as much as it may", c.MD.UserId)
			// An hour is arbitrary and generous: a device at the bound has
			// either stopped consuming or has a bug, and neither is urgent.
			return nil, mtproto.NewErrFloodWaitX(3600)
		default:
			c.Logger.Errorf("mls.publishKeyPackages - %v", err)
			return nil, mtproto.ErrInternalServerError
		}
	}

	available, shouldRefill, err := c.svcCtx.Directory.Available(c.ctx, c.MD.UserId, c.MD.PermAuthKeyId)
	if err != nil {
		c.Logger.Errorf("mls.publishKeyPackages - cannot count: %v", err)
		return nil, mtproto.ErrInternalServerError
	}

	// And how many devices this account has published from, which is the only
	// way a phone can tell that another phone of the same person has signed in
	// since a conversation started: the comparison of members is about people,
	// so a second device of somebody already in it is invisible to it (#41).
	//
	// Nothing new is learnt by saying so - the server hands these devices out to
	// whoever starts a conversation with this person anyway.
	devices, err := c.svcCtx.Directory.DeviceCount(c.ctx, c.MD.UserId)
	if err != nil {
		c.Logger.Errorf("mls.publishKeyPackages - cannot count devices: %v", err)
		return nil, mtproto.ErrInternalServerError
	}

	return &mtproto.Mls_PublishResult{
		Added:        int32(added),
		Available:    int32(available),
		ShouldRefill: shouldRefill,
		Devices:      int32(devices),
	}, nil
}
