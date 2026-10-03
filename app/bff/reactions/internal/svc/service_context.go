package svc

import (
	"github.com/teamgram/marmota/pkg/net/rpcx"
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/app/bff/reactions/internal/config"
	sync_client "github.com/teamgram/teamgram-server/app/messenger/sync/client"
	chat_client "github.com/teamgram/teamgram-server/app/service/biz/chat/client"
	message_client "github.com/teamgram/teamgram-server/app/service/biz/message/client"
	user_client "github.com/teamgram/teamgram-server/app/service/biz/user/client"
	idgen_client "github.com/teamgram/teamgram-server/app/service/idgen/client"
	"github.com/teamgram/teamgram-server/pkg/queue"
	"github.com/teamgram/teamgram-server/pkg/reactions"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config config.Config
	// Nil when the pictures could not be read: the server then offers no
	// reactions, as it did before there were any, and says why at start.
	Catalog       *reactions.Catalog
	Store         reactions.Store
	MessageClient message_client.MessageClient
	ChatClient    chat_client.ChatClient
	UserClient    user_client.UserClient
	IdgenClient   idgen_client.IDGenClient2
	SyncClient    sync_client.SyncClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:        c,
		Catalog:       loadCatalog(reactions.DefaultDir, reactions.DefaultFiles),
		Store:         reactions.NewMysqlStore(sqlx.NewMySQL(&c.Mysql)),
		MessageClient: message_client.NewMessageClient(rpcx.GetCachedRpcClient(c.MessageClient)),
		ChatClient:    chat_client.NewChatClient(rpcx.GetCachedRpcClient(c.ChatClient)),
		UserClient:    user_client.NewUserClient(rpcx.GetCachedRpcClient(c.UserClient)),
		IdgenClient:   idgen_client.NewIDGenClient2(rpcx.GetCachedRpcClient(c.IdgenClient)),
		SyncClient:    queue.NewSyncClient(c.SyncClient),
	}
}

// The pictures are put where the file service hands documents out before the
// list naming them is offered, so a phone is never told of one it cannot fetch.
func loadCatalog(dir, files string) *reactions.Catalog {
	catalog, err := reactions.Load(dir)
	if err != nil {
		logx.Errorf("reactions: no set to offer, phones will see none: %v", err)
		return nil
	}
	written, err := catalog.Place(files)
	if err != nil {
		logx.Errorf("reactions: the pictures are not where the file service reads them, phones will see none: %v", err)
		return nil
	}
	logx.Infof("reactions: %d offered, %d pictures written to %s", len(catalog.Reactions()), written, files)
	return catalog
}
