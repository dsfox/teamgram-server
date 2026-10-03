package svc

import (
	"context"

	"github.com/teamgram/marmota/pkg/net/rpcx"
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/bff/appearance/internal/config"
	msg_client "github.com/teamgram/teamgram-server/app/messenger/msg/msg/client"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	dialog_client "github.com/teamgram/teamgram-server/app/service/biz/dialog/client"
	"github.com/teamgram/teamgram-server/app/service/biz/dialog/dialog"
	user_client "github.com/teamgram/teamgram-server/app/service/biz/user/client"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
	"github.com/teamgram/teamgram-server/pkg/appearance"
	"github.com/teamgram/teamgram-server/pkg/queue"

	"github.com/zeromicro/go-zero/core/logx"
)

// What this service asks of the others, and nothing more: the tests stand in
// for each with a few lines.
type Dialogs interface {
	DialogGetDialogById(ctx context.Context, in *dialog.TLDialogGetDialogById) (*dialog.DialogExt, error)
	DialogSetChatTheme(ctx context.Context, in *dialog.TLDialogSetChatTheme) (*mtproto.Bool, error)
}

type Users interface {
	UserSetColor(ctx context.Context, in *userpb.TLUserSetColor) (*mtproto.Bool, error)
	UserGetMutableUsers(ctx context.Context, in *userpb.TLUserGetMutableUsers) (*userpb.Vector_ImmutableUser, error)
}

type Msg interface {
	MsgSendMessageV2(ctx context.Context, in *msgpb.TLMsgSendMessageV2) (*mtproto.Updates, error)
}

type Sync interface {
	SyncUpdatesNotMe(ctx context.Context, in *sync.TLSyncUpdatesNotMe) (*mtproto.Void, error)
}

type ServiceContext struct {
	Config config.Config
	// Nil when the lists could not be read: the server then offers no themes
	// and the clients' own seven name colours, as before, and says why.
	Catalog *appearance.Catalog
	Dialogs Dialogs
	Users   Users
	Msg     Msg
	Sync    Sync
}

func NewServiceContext(c config.Config) *ServiceContext {
	catalog, err := appearance.Load(appearance.DefaultDir)
	if err != nil {
		logx.Errorf("appearance: no themes or colours to offer: %v", err)
		catalog = nil
	} else {
		logx.Infof("appearance: %d chat themes, %d app themes, %d name and %d profile colours offered",
			len(catalog.ChatThemes()), len(catalog.AppThemes()), len(catalog.NameColours()), len(catalog.ProfileColours()))
	}
	return &ServiceContext{
		Config:  c,
		Catalog: catalog,
		Dialogs: dialog_client.NewDialogClient(rpcx.GetCachedRpcClient(c.DialogClient)),
		Users:   user_client.NewUserClient(rpcx.GetCachedRpcClient(c.UserClient)),
		Msg:     msg_client.NewMsgClient(rpcx.GetCachedRpcClient(c.MsgClient)),
		Sync:    queue.NewSyncClient(c.SyncClient),
	}
}
