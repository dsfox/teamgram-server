package service

import (
	"context"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/bff/mls/internal/core"
)

// MlsPublishKeyPackages
// mls.publishKeyPackages key_packages:Vector<bytes> last_resort:bytes = mls.PublishResult;
func (s *Service) MlsPublishKeyPackages(ctx context.Context, request *mtproto.TLMlsPublishKeyPackages) (*mtproto.Mls_PublishResult, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("mls.publishKeyPackages - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MlsPublishKeyPackages(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("mls.publishKeyPackages - reply: {%s}", r)
	return r, err
}

// MlsClaimKeyPackages
// mls.claimKeyPackages user_id:long = mls.KeyPackages;
func (s *Service) MlsClaimKeyPackages(ctx context.Context, request *mtproto.TLMlsClaimKeyPackages) (*mtproto.Mls_KeyPackages, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("mls.claimKeyPackages - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MlsClaimKeyPackages(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("mls.claimKeyPackages - reply: {%s}", r)
	return r, err
}

// MlsClaimConversation
// mls.claimConversation peer_id:long group_id:bytes holds:Vector<bytes> = mls.Conversation;
func (s *Service) MlsClaimConversation(ctx context.Context, request *mtproto.TLMlsClaimConversation) (*mtproto.Mls_Conversation, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("mls.claimConversation - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MlsClaimConversation(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("mls.claimConversation - reply: {%s}", r)
	return r, err
}

// MlsDevicesOf
// mls.devicesOf users:Vector<long> = mls.DeviceCounts;
func (s *Service) MlsDevicesOf(ctx context.Context, request *mtproto.TLMlsDevicesOf) (*mtproto.Mls_DeviceCounts, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("mls.devicesOf - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MlsDevicesOf(request)
	if err != nil {
		return nil, err
	}

	c.Logger.Debugf("mls.devicesOf - reply: {%s}", r)
	return r, err
}

// MlsSendWelcome
// mls.sendWelcome user_id:long welcome:bytes = mls.Ok;
func (s *Service) MlsSendWelcome(ctx context.Context, request *mtproto.TLMlsSendWelcome) (*mtproto.Mls_Ok, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("mls.sendWelcome - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MlsSendWelcome(request)
	if err != nil {
		return nil, err
	}
	return r, err
}

// MlsGetWelcomes
// mls.getWelcomes = mls.Welcomes;
func (s *Service) MlsGetWelcomes(ctx context.Context, request *mtproto.TLMlsGetWelcomes) (*mtproto.Mls_Welcomes, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("mls.getWelcomes - metadata: {%s}", c.MD)

	r, err := c.MlsGetWelcomes(request)
	if err != nil {
		return nil, err
	}
	return r, err
}

// MlsConfirmWelcomes
// mls.confirmWelcomes ids:Vector<long> = mls.Ok;
func (s *Service) MlsConfirmWelcomes(ctx context.Context, request *mtproto.TLMlsConfirmWelcomes) (*mtproto.Mls_Ok, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("mls.confirmWelcomes - metadata: {%s}, request: {%s}", c.MD, request)

	r, err := c.MlsConfirmWelcomes(request)
	if err != nil {
		return nil, err
	}
	return r, err
}

// MlsSetRecoverySecret registers the way back into an account.
//
// mls.setRecoverySecret secret:string = mls.Ok;
func (s *Service) MlsSetRecoverySecret(ctx context.Context, request *mtproto.TLMlsSetRecoverySecret) (*mtproto.Mls_Ok, error) {
	c := core.New(ctx, s.svcCtx)
	// The secret itself is not logged. It is not the phrase, but it is what a
	// sign-in is checked against, and a log is a place things get copied out of.
	c.Logger.Debugf("mls.setRecoverySecret - metadata: {%s}", c.MD)

	r, err := c.MlsSetRecoverySecret(request)
	if err != nil {
		return nil, err
	}
	return r, err
}

// MlsSendCommit
// mls.sendCommit group_id:bytes epoch:long members:Vector<long> commit:bytes = mls.CommitResult;
func (s *Service) MlsSendCommit(ctx context.Context, request *mtproto.TLMlsSendCommit) (*mtproto.Mls_CommitResult, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("mls.sendCommit - metadata: {%s}, epoch: %d", c.MD, request.GetEpoch())

	r, err := c.MlsSendCommit(request)
	if err != nil {
		return nil, err
	}
	return r, err
}

// MlsGetCommits
// mls.getCommits = mls.Commits;
func (s *Service) MlsGetCommits(ctx context.Context, request *mtproto.TLMlsGetCommits) (*mtproto.Mls_Commits, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("mls.getCommits - metadata: {%s}", c.MD)

	r, err := c.MlsGetCommits(request)
	if err != nil {
		return nil, err
	}
	return r, err
}

// MlsConfirmCommits
// mls.confirmCommits ids:Vector<long> = mls.Ok;
func (s *Service) MlsConfirmCommits(ctx context.Context, request *mtproto.TLMlsConfirmCommits) (*mtproto.Mls_Ok, error) {
	c := core.New(ctx, s.svcCtx)
	c.Logger.Debugf("mls.confirmCommits - metadata: {%s}", c.MD)

	r, err := c.MlsConfirmCommits(request)
	if err != nil {
		return nil, err
	}
	return r, err
}
