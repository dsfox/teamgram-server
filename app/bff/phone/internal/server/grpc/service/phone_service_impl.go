package service

import (
	"context"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/bff/phone/internal/core"
)

// PhoneRequestCall places a call (#14).
//
// phone.requestCall video:flags.0?true user_id:InputUser random_id:int
//
//	g_a_hash:bytes protocol:PhoneCallProtocol = phone.PhoneCall;
func (s *Service) PhoneRequestCall(ctx context.Context, request *mtproto.TLPhoneRequestCall) (*mtproto.Phone_PhoneCall, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("phone.requestCall - metadata: {%s}, request: {%s}", c.MD, request)
	r, err := c.PhoneRequestCall(request)
	if err != nil {
		return nil, err
	}
	c.Logger.Debugf("phone.requestCall - reply: {%s}", r)
	return r, err
}
