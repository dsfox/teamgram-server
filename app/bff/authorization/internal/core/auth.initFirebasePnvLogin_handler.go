package core

import (
	"github.com/teamgram/proto/mtproto"
)

// AuthInitFirebasePnvLogin
// auth.initFirebasePnvLogin#644f15c0 flags:# except_ids:flags.0?Vector<long> api_id:int api_hash:string = auth.FirebasePnvIntent;
//
// Layer 231 (#276). ice9 verifies a phone by its own code, never by Firebase phone number verification
func (c *AuthorizationCore) AuthInitFirebasePnvLogin(in *mtproto.TLAuthInitFirebasePnvLogin) (*mtproto.Auth_FirebasePnvIntent, error) {
	c.Logger.Errorf("auth.initFirebasePnvLogin: not served, ice9 verifies a phone by its own code, never by Firebase phone number verification")

	return nil, mtproto.ErrMethodNotImpl
}
