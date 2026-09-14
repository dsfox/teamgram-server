package svc

import (
	"context"

	"github.com/teamgram/proto/mtproto"

	"github.com/teamgram/marmota/pkg/net/rpcx"
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/app/bff/phone/internal/config"
	sync_client "github.com/teamgram/teamgram-server/app/messenger/sync/client"
	user_client "github.com/teamgram/teamgram-server/app/service/biz/user/client"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
	status_client "github.com/teamgram/teamgram-server/app/service/status/client"
	"github.com/teamgram/teamgram-server/app/service/status/status"
	"github.com/teamgram/teamgram-server/pkg/calls"
	"github.com/teamgram/teamgram-server/pkg/pushnotify"
	"github.com/teamgram/teamgram-server/pkg/queue"
)

// Sessions says which devices of a person are connected: the ones a ring
// reaches over the session, so the push goes to the rest and a device that
// comes back later is rung once and not twice.
type Sessions interface {
	StatusGetUserOnlineSessions(ctx context.Context, in *status.TLStatusGetUserOnlineSessions) (*status.UserSessionEntryList, error)
}

// Users is where the caller comes from, as the callee sees them - the iPhone
// shows the name from the push before it has asked the server anything - and
// where "who may connect to me directly" is answered from.
type Users interface {
	UserGetMutableUsers(ctx context.Context, in *userpb.TLUserGetMutableUsers) (*userpb.Vector_ImmutableUser, error)
	UserCheckPrivacy(ctx context.Context, in *userpb.TLUserCheckPrivacy) (*mtproto.Bool, error)
}

// Ringer wakes the phones that are not connected (#14, Part 5).
type Ringer interface {
	IncomingCall(ctx context.Context, calleeId, callerId, callId int64, updates []byte)
}

// ServiceContext is deliberately thin: the calls in the air, and the ways of
// telling the other phone. There is no media path here to hold.
type ServiceContext struct {
	Config config.Config
	// Says "you are being called" to the connected devices of a person.
	SyncClient sync_client.SyncClient
	// The calls currently in the air. Shared with the updates service, which
	// rings a device that comes back while a call still rings.
	Registry *calls.Registry
	Sessions Sessions
	Users    Users
	Ringer   Ringer
}

func NewServiceContext(c config.Config, registry *calls.Registry) *ServiceContext {
	return &ServiceContext{
		Config:     c,
		SyncClient: queue.NewSyncClient(c.SyncClient),
		Registry:   registry,
		Sessions:   status_client.NewStatusClient(rpcx.GetCachedRpcClient(c.StatusClient)),
		Users:      user_client.NewUserClient(rpcx.GetCachedRpcClient(c.UserClient)),
		Ringer:     pushnotify.New(sqlx.NewMySQL(&c.Mysql)),
	}
}
