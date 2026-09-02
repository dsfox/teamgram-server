package service

import (
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/bff/invite/internal/svc"
)

type Service struct {
	// Embedded so that a method added to the service later fails here with a
	// name rather than at a phone with silence.
	mtproto.UnimplementedRPCInviteServer
	svcCtx *svc.ServiceContext
}

func (s *Service) GetServiceContext() *svc.ServiceContext {
	return s.svcCtx
}

func New(ctx *svc.ServiceContext) *Service {
	return &Service{
		svcCtx: ctx,
	}
}
