package core

import (
	"context"
	"testing"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/teamgram/teamgram-server/app/bff/phone/internal/svc"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	"github.com/teamgram/teamgram-server/pkg/calls"
	"github.com/zeromicro/go-zero/core/logx"
)

// recordingSync stands in for the sync service and keeps every push, so a test
// can say who was told what - and that nobody else was.
type recordingSync struct {
	pushes []*sync.TLSyncPushUpdates
}

func (r *recordingSync) SyncPushUpdates(_ context.Context, in *sync.TLSyncPushUpdates) (*mtproto.Void, error) {
	r.pushes = append(r.pushes, in)
	return mtproto.EmptyVoid, nil
}

func (r *recordingSync) SyncUpdatesMe(context.Context, *sync.TLSyncUpdatesMe) (*mtproto.Void, error) {
	panic("not used by the phone service")
}
func (r *recordingSync) SyncUpdatesNotMe(context.Context, *sync.TLSyncUpdatesNotMe) (*mtproto.Void, error) {
	panic("not used by the phone service")
}
func (r *recordingSync) SyncPushUpdatesIfNot(context.Context, *sync.TLSyncPushUpdatesIfNot) (*mtproto.Void, error) {
	panic("not used by the phone service")
}
func (r *recordingSync) SyncPushBotUpdates(context.Context, *sync.TLSyncPushBotUpdates) (*mtproto.Void, error) {
	panic("not used by the phone service")
}
func (r *recordingSync) SyncPushRpcResult(context.Context, *sync.TLSyncPushRpcResult) (*mtproto.Void, error) {
	panic("not used by the phone service")
}
func (r *recordingSync) SyncBroadcastUpdates(context.Context, *sync.TLSyncBroadcastUpdates) (*mtproto.Void, error) {
	panic("not used by the phone service")
}

// stand is a phone service with a real registry and a recording sync, plus
// one call already in the air between alice and bob.
type stand struct {
	svcCtx *svc.ServiceContext
	sync   *recordingSync
	call   *calls.Call
}

const (
	alice int64 = 1001
	bob   int64 = 1002
	carol int64 = 1003
)

func newStand(t *testing.T) *stand {
	t.Helper()
	recorder := &recordingSync{}
	registry := calls.NewRegistry()
	call, err := registry.Place(alice, bob, []byte("g_a_hash"), time.Now())
	if err != nil {
		t.Fatalf("cannot place the call: %v", err)
	}
	// As requestCall leaves it: the caller's protocol travels with the call.
	call.Protocol = mtproto.MakeTLPhoneCallProtocol(&mtproto.PhoneCallProtocol{
		UdpP2P: true, UdpReflector: true, MinLayer: 65, MaxLayer: 92,
		LibraryVersions: []string{"4.0.0"},
	}).To_PhoneCallProtocol()
	return &stand{
		svcCtx: &svc.ServiceContext{SyncClient: recorder, Registry: registry},
		sync:   recorder,
		call:   call,
	}
}

// as returns the core the way the gRPC layer builds it, for one person.
func (s *stand) as(user int64) *PhoneCore {
	ctx := context.Background()
	return &PhoneCore{
		ctx:    ctx,
		svcCtx: s.svcCtx,
		Logger: logx.WithContext(ctx),
		MD:     &metadata.RpcMetadata{UserId: user},
	}
}

func (s *stand) peer() *mtproto.InputPhoneCall {
	return mtproto.MakeTLInputPhoneCall(&mtproto.InputPhoneCall{
		Id:         s.call.Id,
		AccessHash: s.call.AccessHash,
	}).To_InputPhoneCall()
}

// onlyUpdate unwraps the one update a push carried, failing on any other shape.
func onlyUpdate(t *testing.T, push *sync.TLSyncPushUpdates) *mtproto.Update {
	t.Helper()
	updates := push.GetUpdates()
	if updates.GetPredicateName() == mtproto.Predicate_updateShort {
		return updates.GetUpdate()
	}
	if list := updates.GetUpdates(); len(list) == 1 {
		return list[0]
	}
	t.Fatalf("the push carried %s, expected one update", updates.GetPredicateName())
	return nil
}
