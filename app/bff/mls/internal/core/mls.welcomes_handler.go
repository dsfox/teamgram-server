package core

import (
	"errors"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/mls"
)

// MlsSendWelcome leaves a welcome for every device of a person, so they can join
// a conversation somebody started with them.
//
// It travels here rather than as a message: handshake traffic never touches the
// message pipeline, so no client has to hide anything from a chat list.
//
// mls.sendWelcome user_id:long welcome:bytes = mls.Ok;
func (c *MlsCore) MlsSendWelcome(in *mtproto.TLMlsSendWelcome) (*mtproto.Mls_Ok, error) {
	posted, err := c.svcCtx.Directory.Post(
		c.ctx,
		c.svcCtx.Welcomes,
		in.GetUserId(),
		c.MD.UserId,
		in.GetPeerId(),
		in.GetWelcome(),
		int32(time.Now().Unix()))
	if err != nil {
		switch {
		case errors.Is(err, mls.ErrEmptyPackage):
			c.Logger.Errorf("mls.sendWelcome - an empty welcome from %d", c.MD.UserId)
			return nil, mtproto.ErrInputRequestInvalid
		case errors.Is(err, mls.ErrTooMany):
			c.Logger.Errorf("mls.sendWelcome - %d has as many waiting as it may", in.GetUserId())
			return nil, mtproto.NewErrFloodWaitX(3600)
		default:
			c.Logger.Errorf("mls.sendWelcome - %v", err)
			return nil, mtproto.ErrInternalServerError
		}
	}

	// Nobody to leave it for is not a failure: that person simply has no device
	// here yet. The sender learns it from the count rather than from an error
	// they cannot act on.
	c.Logger.Infof("mls.sendWelcome - left for %d device(s) of %d", posted, in.GetUserId())
	return &mtproto.Mls_Ok{Ok: posted > 0}, nil
}

// MlsGetWelcomes is what this device has not opened yet.
//
// Handed out again until confirmed: a device that read one and stopped before
// saving the conversation must get it again, or the conversation exists on one
// side only - and that surfaces much later, as messages that will not open.
//
// mls.getWelcomes = mls.Welcomes;
func (c *MlsCore) MlsGetWelcomes(in *mtproto.TLMlsGetWelcomes) (*mtproto.Mls_Welcomes, error) {
	waiting, err := c.svcCtx.Directory.Waiting(c.ctx, c.svcCtx.Welcomes, c.MD.UserId, c.MD.PermAuthKeyId,
		int32(time.Now().Unix()))
	if err != nil {
		c.Logger.Errorf("mls.getWelcomes - %v", err)
		return nil, mtproto.ErrInternalServerError
	}

	welcomes := make([]*mtproto.Mls_Welcome, 0, len(waiting))
	for _, w := range waiting {
		welcomes = append(welcomes, &mtproto.Mls_Welcome{
			Id:      w.Id,
			FromId:  w.FromId,
			PeerId:  w.PeerId,
			Welcome: w.Bytes,
		})
	}

	return &mtproto.Mls_Welcomes{Welcomes: welcomes}, nil
}

// MlsConfirmWelcomes forgets what this device says it has opened and saved.
//
// Its own only, which the storage enforces rather than this: a device
// confirming somebody else's would drop a conversation that person never
// joined, and they would never learn why.
//
// mls.confirmWelcomes ids:Vector<long> = mls.Ok;
func (c *MlsCore) MlsConfirmWelcomes(in *mtproto.TLMlsConfirmWelcomes) (*mtproto.Mls_Ok, error) {
	confirmed, err := c.svcCtx.Directory.Confirm(
		c.ctx, c.svcCtx.Welcomes, c.MD.UserId, c.MD.PermAuthKeyId, in.GetIds())
	if err != nil {
		c.Logger.Errorf("mls.confirmWelcomes - %v", err)
		return nil, mtproto.ErrInternalServerError
	}

	c.Logger.Infof("mls.confirmWelcomes - %d forgotten", confirmed)
	return &mtproto.Mls_Ok{Ok: true}, nil
}
