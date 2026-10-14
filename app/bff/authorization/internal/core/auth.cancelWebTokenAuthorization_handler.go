package core

import (
	"github.com/teamgram/proto/mtproto"
)

// AuthCancelWebTokenAuthorization
// auth.cancelWebTokenAuthorization#490dada1 web_auth_token:string = Bool;
//
// Layer 231 (#276). ice9 signs in by its own code alone, so there is no web token to cancel
func (c *AuthorizationCore) AuthCancelWebTokenAuthorization(in *mtproto.TLAuthCancelWebTokenAuthorization) (*mtproto.Bool, error) {
	c.Logger.Errorf("auth.cancelWebTokenAuthorization: not served, ice9 signs in by its own code alone, so there is no web token to cancel")

	return nil, mtproto.ErrMethodNotImpl
}
