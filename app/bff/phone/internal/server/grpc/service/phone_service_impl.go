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

// PhoneAcceptCall answers a ringing call (#14).
func (s *Service) PhoneAcceptCall(ctx context.Context, request *mtproto.TLPhoneAcceptCall) (*mtproto.Phone_PhoneCall, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("phone.acceptCall - metadata: {%s}, request: {%s}", c.MD, request)
	return c.PhoneAcceptCall(request)
}

// PhoneConfirmCall closes the key exchange and hands over the connections (#14).
func (s *Service) PhoneConfirmCall(ctx context.Context, request *mtproto.TLPhoneConfirmCall) (*mtproto.Phone_PhoneCall, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("phone.confirmCall - metadata: {%s}, request: {%s}", c.MD, request)
	return c.PhoneConfirmCall(request)
}

// PhoneDiscardCall hangs up (#14).
func (s *Service) PhoneDiscardCall(ctx context.Context, request *mtproto.TLPhoneDiscardCall) (*mtproto.Updates, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("phone.discardCall - metadata: {%s}, request: {%s}", c.MD, request)
	return c.PhoneDiscardCall(request)
}

// PhoneSendSignalingData forwards a candidate blob to the other leg (#14).
//
// phone.sendSignalingData peer:InputPhoneCall data:bytes = Bool;
func (s *Service) PhoneSendSignalingData(ctx context.Context, request *mtproto.TLPhoneSendSignalingData) (*mtproto.Bool, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("phone.sendSignalingData - metadata: {%s}, call: %d, %d bytes", c.MD, request.GetPeer().GetId(), len(request.GetData()))
	return c.PhoneSendSignalingData(request)
}

// PhoneGetCallConfig is the engine tuning both phones poll (#14).
//
// phone.getCallConfig = DataJSON;
func (s *Service) PhoneGetCallConfig(ctx context.Context, request *mtproto.TLPhoneGetCallConfig) (*mtproto.DataJSON, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("phone.getCallConfig - metadata: {%s}", c.MD)
	return c.PhoneGetCallConfig(request)
}

// PhoneReceivedCall is the callee's phone saying it is ringing (#14).
//
// phone.receivedCall peer:InputPhoneCall = Bool;
func (s *Service) PhoneReceivedCall(ctx context.Context, request *mtproto.TLPhoneReceivedCall) (*mtproto.Bool, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("phone.receivedCall - metadata: {%s}, request: {%s}", c.MD, request)
	return c.PhoneReceivedCall(request)
}

// PhoneSetCallRating takes the stars a person gave a finished call (#14).
//
// phone.setCallRating flags:# user_initiative:flags.0?true peer:InputPhoneCall
//
//	rating:int comment:string = Updates;
func (s *Service) PhoneSetCallRating(ctx context.Context, request *mtproto.TLPhoneSetCallRating) (*mtproto.Updates, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("phone.setCallRating - metadata: {%s}, request: {%s}", c.MD, request)
	return c.PhoneSetCallRating(request)
}

// RecallRinging rings one device of a person for the call still ringing for
// them, if it was not rung yet (#14). Not an RPC: the updates service calls
// it when that device asks for its difference, which is the first thing a
// phone woken by a push does.
func (s *Service) RecallRinging(ctx context.Context, userId, permAuthKeyId int64) {
	c := core.New(ctx, s.svcCtx)
	c.MD.UserId = userId
	c.RecallRinging(permAuthKeyId)
}
