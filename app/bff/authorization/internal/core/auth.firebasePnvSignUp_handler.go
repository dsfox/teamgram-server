package core

import (
	"github.com/teamgram/proto/mtproto"
)

// AuthFirebasePnvSignUp
// auth.firebasePnvSignUp#783f6b56 flags:# no_joined_notifications:flags.0?true first_name:string last_name:string = auth.Authorization;
//
// Layer 231 (#276). ice9 verifies a phone by its own code, never by Firebase phone number verification
func (c *AuthorizationCore) AuthFirebasePnvSignUp(in *mtproto.TLAuthFirebasePnvSignUp) (*mtproto.Auth_Authorization, error) {
	c.Logger.Errorf("auth.firebasePnvSignUp: not served, ice9 verifies a phone by its own code, never by Firebase phone number verification")

	return nil, mtproto.ErrMethodNotImpl
}
