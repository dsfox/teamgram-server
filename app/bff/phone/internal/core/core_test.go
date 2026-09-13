package core

import (
	"context"
	"testing"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/teamgram/teamgram-server/app/bff/phone/internal/config"
	"github.com/teamgram/teamgram-server/app/bff/phone/internal/svc"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
	"github.com/teamgram/teamgram-server/app/service/status/status"
	"github.com/teamgram/teamgram-server/pkg/calls"
	"github.com/zeromicro/go-zero/core/logx"
)

// recordingSync stands in for the sync service and keeps every push, so a test
// can say who was told what - and that nobody else was.
type recordingSync struct {
	pushes []*sync.TLSyncPushUpdates
	// The rings addressed to one device (sync.updatesMe), kept beside the
	// pushes so a test can say which device was rung.
	me []*sync.TLSyncUpdatesMe
}

func (r *recordingSync) SyncPushUpdates(_ context.Context, in *sync.TLSyncPushUpdates) (*mtproto.Void, error) {
	r.pushes = append(r.pushes, in)
	return mtproto.EmptyVoid, nil
}

func (r *recordingSync) SyncUpdatesMe(_ context.Context, in *sync.TLSyncUpdatesMe) (*mtproto.Void, error) {
	r.me = append(r.me, in)
	r.pushes = append(r.pushes, &sync.TLSyncPushUpdates{UserId: in.GetUserId(), Updates: in.GetUpdates()})
	return mtproto.EmptyVoid, nil
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

// recordingRinger stands in for the push side: it keeps what it was asked to
// ring, so a test can decode the blob the iPhone would get.
type recordingRinger struct {
	rings []ring
}

type ring struct {
	callee, caller, callId int64
	updates                []byte
}

func (r *recordingRinger) IncomingCall(_ context.Context, calleeId, callerId, callId int64, updates []byte) {
	r.rings = append(r.rings, ring{calleeId, callerId, callId, updates})
}

// fakeSessions answers "who is connected" with what the test put there.
type fakeSessions struct {
	permKeys map[int64][]int64
}

func (f *fakeSessions) StatusGetUserOnlineSessions(_ context.Context, in *status.TLStatusGetUserOnlineSessions) (*status.UserSessionEntryList, error) {
	out := &status.UserSessionEntryList{}
	for _, key := range f.permKeys[in.GetUserId()] {
		out.UserSessions = append(out.UserSessions, &status.SessionEntry{UserId: in.GetUserId(), PermAuthKeyId: key})
	}
	return out, nil
}

// fakeUsers knows the three people of the stand by first name.
type fakeUsers struct{}

func (fakeUsers) UserGetMutableUsers(_ context.Context, in *userpb.TLUserGetMutableUsers) (*userpb.Vector_ImmutableUser, error) {
	names := map[int64]string{alice: "Alice", bob: "Bob", carol: "Carol"}
	out := &userpb.Vector_ImmutableUser{}
	for _, id := range in.GetId() {
		out.Datas = append(out.Datas, mtproto.MakeTLImmutableUser(&mtproto.ImmutableUser{
			User: mtproto.MakeTLUserData(&mtproto.UserData{Id: id, FirstName: names[id], AccessHash: id * 10}).To_UserData(),
		}).To_ImmutableUser())
	}
	return out, nil
}

// stand is a phone service with a real registry and a recording sync, plus
// one call already in the air between alice and bob.
type stand struct {
	svcCtx   *svc.ServiceContext
	sync     *recordingSync
	ringer   *recordingRinger
	sessions *fakeSessions
	call     *calls.Call
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
	ringer := &recordingRinger{}
	sessions := &fakeSessions{permKeys: map[int64][]int64{}}
	return &stand{
		svcCtx: &svc.ServiceContext{
			Config:     standConfig(),
			SyncClient: recorder,
			Registry:   registry,
			Sessions:   sessions,
			Users:      fakeUsers{},
			Ringer:     ringer,
		},
		sync:     recorder,
		ringer:   ringer,
		sessions: sessions,
		call:     call,
	}
}

// standConfig is one STUN and one relay, the shape of the production config,
// so confirmCall has something to hand out and the order can be checked.
func standConfig() config.Config {
	return config.Config{Calls: config.Calls{
		Servers: []config.Server{
			{Id: 2, Host: "198.51.100.2", HostV6: "2001:db8::2", Port: 3478, Turn: true},
			{Id: 1, Host: "198.51.100.1", HostV6: "2001:db8::1", Port: 3478, Stun: true},
		},
		RelaySecret: "stand-secret",
	}}
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

func nowish() time.Time { return time.Now() }

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
