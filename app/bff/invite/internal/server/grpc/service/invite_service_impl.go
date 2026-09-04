package service

import (
	"context"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/bff/invite/internal/core"
)

// InviteMint mints an invitation for one person (#47).
//
// invite.mint phone:string = invite.Minted;
func (s *Service) InviteMint(ctx context.Context, request *mtproto.TLInviteMint) (*mtproto.Invite_Minted, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("invite.mint - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.InviteMint(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("invite.mint - reply: {%s}", r)
	return r, err
}

// InviteMintForChat mints an invitation that leads into a group (#164).
//
// invite.mintForChat chat_id:long phone:string = invite.Minted;
func (s *Service) InviteMintForChat(ctx context.Context, request *mtproto.TLInviteMintForChat) (*mtproto.Invite_Minted, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("invite.mintForChat - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.InviteMintForChat(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("invite.mintForChat - reply: {%s}", r)
	return r, err
}
