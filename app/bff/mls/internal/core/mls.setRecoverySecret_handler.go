package core

import (
	"github.com/teamgram/proto/mtproto"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
	"github.com/teamgram/teamgram-server/pkg/code/invite"
)

// MlsSetRecoverySecret registers the way back into an account.
//
// What arrives is not the recovery phrase. It is a one-way derivation of the
// words, made on the device: enough to recognise somebody typing them and
// nothing else. The words themselves are made there too and never travel.
//
// They used to be made here and delivered as a message, which left every one of
// them sitting in the message table in plain text. A phrase signs in without a
// code - it is the whole account - so that was a copy of every key in one
// place. It also emptied the encrypted history backup of meaning, because its
// key comes from the same words: a server that has the phrase can read the
// backup, and this one had thirty-three of them.
//
// The phone number is looked up rather than taken from the request. Believing
// the caller would let anybody register a way into somebody else's account.
//
// mls.setRecoverySecret secret:string = mls.Ok;
func (c *MlsCore) MlsSetRecoverySecret(in *mtproto.TLMlsSetRecoverySecret) (*mtproto.Mls_Ok, error) {
	secret := in.GetSecret()
	if secret == "" {
		c.Logger.Errorf("mls.setRecoverySecret - an empty secret from %d", c.MD.UserId)
		return nil, mtproto.ErrInputRequestInvalid
	}

	user, err := c.svcCtx.UserClient.UserGetImmutableUser(c.ctx, &userpb.TLUserGetImmutableUser{
		Id: c.MD.UserId,
	})
	if err != nil || user == nil || user.Phone() == "" {
		c.Logger.Errorf("mls.setRecoverySecret - cannot find the number of %d: %v", c.MD.UserId, err)
		return nil, mtproto.ErrInternalServerError
	}

	if c.svcCtx.Store == nil {
		c.Logger.Errorf("mls.setRecoverySecret - no store, so %d has no way back", c.MD.UserId)
		return nil, mtproto.ErrInternalServerError
	}

	// Delivered, because the device that sent this is the device showing the
	// words to the person who owns them.
	if err = invite.RecordRecoverySecret(c.ctx, c.svcCtx.Store, user.Phone(), secret, true); err != nil {
		c.Logger.Errorf("mls.setRecoverySecret - cannot record for %d: %v", c.MD.UserId, err)
		return nil, mtproto.ErrInternalServerError
	}

	c.Logger.Infof("mls.setRecoverySecret - %d registered a way back", c.MD.UserId)
	return &mtproto.Mls_Ok{Ok: true}, nil
}
