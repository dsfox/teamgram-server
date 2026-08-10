package core

import (
	"github.com/teamgram/proto/mtproto"
)

// MlsClaimKeyPackages takes one package for every device of a person, which is
// what starting an encrypted conversation with them needs.
//
// Taking removes: a package handed to two conversations costs the forward
// secrecy of every message that follows. A device whose supply has run dry
// contributes its last-resort package if it left one, and is missing from the
// answer otherwise - one silent device must not stop a conversation with the
// rest.
//
// mls.claimKeyPackages user_id:long = mls.KeyPackages;
func (c *MlsCore) MlsClaimKeyPackages(in *mtproto.TLMlsClaimKeyPackages) (*mtproto.Mls_KeyPackages, error) {
	claimed, err := c.svcCtx.Directory.Claim(c.ctx, in.GetUserId())
	if err != nil {
		c.Logger.Errorf("mls.claimKeyPackages - %v", err)
		return nil, mtproto.ErrInternalServerError
	}

	packages := make([][]byte, 0, len(claimed))
	for _, p := range claimed {
		packages = append(packages, p.Bytes)
	}

	c.Logger.Infof("mls.claimKeyPackages - %d device(s) of %d answered", len(packages), in.GetUserId())
	return &mtproto.Mls_KeyPackages{Packages: packages}, nil
}
