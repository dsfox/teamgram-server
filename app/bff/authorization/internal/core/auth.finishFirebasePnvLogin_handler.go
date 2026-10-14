package core

import (
	"github.com/teamgram/proto/mtproto"
)

// AuthFinishFirebasePnvLogin
// auth.finishFirebasePnvLogin#2c85094c google_token:string = auth.Authorization;
//
// Layer 231 (#276). ice9 verifies a phone by its own code, never by Firebase phone number verification
func (c *AuthorizationCore) AuthFinishFirebasePnvLogin(in *mtproto.TLAuthFinishFirebasePnvLogin) (*mtproto.Auth_Authorization, error) {
	c.Logger.Errorf("auth.finishFirebasePnvLogin: not served, ice9 verifies a phone by its own code, never by Firebase phone number verification")

	return nil, mtproto.ErrMethodNotImpl
}
