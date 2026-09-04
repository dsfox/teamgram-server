package core

import (
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/code/invite"
)

// InviteMint mints a code bound to this number for the person asking, who
// vouches for whoever holds it (#47).
//
// Bound, and the inviter's own carrier is the check: the phone's SMS app sends
// the code to this number, so whoever registers with it is whoever the
// carrier delivered to. Any member may ask, as often as they like - one
// invitation is one person vouched for.
//
// The number is kept as digits, which is how the user directory keys it and
// how the verifier compares it at sign-in.
//
// invite.mint phone:string = invite.Minted;
func (c *InviteCore) InviteMint(in *mtproto.TLInviteMint) (*mtproto.Invite_Minted, error) {
	phone := invite.Digits(in.GetPhone())
	if phone == "" {
		return nil, mtproto.ErrPhoneNumberInvalid
	}

	return c.mint(phone, 0)
}
