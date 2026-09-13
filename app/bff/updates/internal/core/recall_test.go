package core

import (
	"context"
	"testing"

	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/teamgram/teamgram-server/app/bff/updates/internal/config"
	"github.com/teamgram/teamgram-server/app/bff/updates/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type recordingRecaller struct {
	asked [][2]int64
}

func (r *recordingRecaller) RecallRinging(_ context.Context, userId, permAuthKeyId int64) {
	r.asked = append(r.asked, [2]int64{userId, permAuthKeyId})
}

// A phone woken by a call push asks for its difference first; that is the
// moment it is rung, by the device that asked, for the person it belongs to.
// A process with no calls service has nothing to ask and must not crash.
func TestGettingTheDifferenceAsksTheCallsToRingThisDevice(t *testing.T) {
	recaller := &recordingRecaller{}
	c := &UpdatesCore{
		ctx:    context.Background(),
		svcCtx: &svc.ServiceContext{Config: config.Config{Calls: recaller}},
		Logger: logx.WithContext(context.Background()),
		MD:     &metadata.RpcMetadata{UserId: 1002},
	}
	c.recallRinging(502)
	if len(recaller.asked) != 1 || recaller.asked[0] != [2]int64{1002, 502} {
		t.Fatalf("asked %v", recaller.asked)
	}

	without := &UpdatesCore{ctx: context.Background(), svcCtx: &svc.ServiceContext{}, Logger: c.Logger, MD: c.MD}
	without.recallRinging(502)
}
