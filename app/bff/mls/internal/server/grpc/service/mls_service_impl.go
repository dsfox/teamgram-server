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
