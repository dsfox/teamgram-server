package core

import (
	"strconv"
	"time"

	"github.com/teamgram/proto/mtproto"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
	"github.com/teamgram/teamgram-server/pkg/code/invite"

	"google.golang.org/grpc/status"
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

	// A number with an account needs no invitation - and the client says so
	// in words rather than sending a code that can never open anything.
	if _, err := c.svcCtx.UserClient.UserGetImmutableUserByPhone(c.ctx, &userpb.TLUserGetImmutableUserByPhone{
		Phone: phone,
	}); err == nil {
		c.Logger.Infof("invite.mint - %d asked for %s, who is already here", c.MD.UserId, phone)
		return nil, status.Error(mtproto.ErrBadRequest, "PHONE_ALREADY_HERE")
	} else if _, isRpc := status.FromError(err); !isRpc {
		c.Logger.Errorf("invite.mint - cannot ask whether %s has an account: %v", phone, err)
		return nil, mtproto.ErrInternalServerError
	}

	// One live code per inviter and number: minting again replaces it.
	if old, err := c.svcCtx.Invitations.LiveCode(c.ctx, phone, c.MD.UserId); err != nil {
		c.Logger.Errorf("invite.mint - %v", err)
	} else if old != "" {
		if err := invite.Revoke(c.ctx, c.svcCtx.Store, old); err != nil {
			c.Logger.Errorf("invite.mint - %v", err)
		}
	}

	seconds := c.svcCtx.Config.Days() * 24 * 3600
	now := int32(time.Now().Unix())
	code, err := invite.Mint(c.ctx, c.svcCtx.Store, seconds, invite.Invitation{
		Phone: phone,
		Note:  strconv.FormatInt(c.MD.UserId, 10),
	})
	if code == "" {
		c.Logger.Errorf("invite.mint - %v", err)
		return nil, mtproto.ErrInternalServerError
	}
	if err != nil {
		// Minted and usable, only not on the CLI's --list: said, not charged.
		c.Logger.Errorf("invite.mint - %v", err)
	}
	if err := c.svcCtx.Invitations.Minted(c.ctx, code, phone, c.MD.UserId, now, 0); err != nil {
		// The code works without the record; the record is what the owner
		// reads later, and a gap there is logged, not charged to the person.
		c.Logger.Errorf("invite.mint - %v", err)
	}

	c.Logger.Infof("invite.mint - %d vouches for %s, code lives %d days", c.MD.UserId, phone, c.svcCtx.Config.Days())
	return &mtproto.Invite_Minted{Code: code, Expires: now + int32(seconds)}, nil
}
