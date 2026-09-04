package core

import (
	"strconv"
	"time"

	"github.com/teamgram/proto/mtproto"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
	"github.com/teamgram/teamgram-server/pkg/code/invite"

	"google.golang.org/grpc/status"
)

// mint is what both methods do once they know whom the code is for: refuse a
// number with an account, replace the inviter's live code for it, mint, and
// record who vouched - and, when chatId is not zero, where the code leads.
func (c *InviteCore) mint(phone string, chatId int64) (*mtproto.Invite_Minted, error) {
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
		Chat:  chatId,
	})
	if code == "" {
		c.Logger.Errorf("invite.mint - %v", err)
		return nil, mtproto.ErrInternalServerError
	}
	if err != nil {
		// Minted and usable, only not on the CLI's --list: said, not charged.
		c.Logger.Errorf("invite.mint - %v", err)
	}
	if err := c.svcCtx.Invitations.Minted(c.ctx, code, phone, c.MD.UserId, now, chatId); err != nil {
		// The code works without the record; the record is what the owner
		// reads later, and a gap there is logged, not charged to the person.
		c.Logger.Errorf("invite.mint - %v", err)
	}

	c.Logger.Infof("invite.mint - %d vouches for %s into chat %d, code lives %d days", c.MD.UserId, phone, chatId, c.svcCtx.Config.Days())
	return &mtproto.Invite_Minted{Code: code, Expires: now + int32(seconds)}, nil
}
