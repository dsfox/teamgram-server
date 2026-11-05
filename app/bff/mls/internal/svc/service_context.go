package svc

import (
	"context"
	"time"

	"github.com/teamgram/marmota/pkg/net/rpcx"
	"github.com/teamgram/marmota/pkg/stores/kv"
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/app/bff/mls/internal/config"
	sync_client "github.com/teamgram/teamgram-server/app/messenger/sync/client"
	user_client "github.com/teamgram/teamgram-server/app/service/biz/user/client"
	"github.com/teamgram/teamgram-server/pkg/mls"
	"github.com/teamgram/teamgram-server/pkg/queue"
)

type ServiceContext struct {
	Config    config.Config
	Directory *mls.Directory
	Welcomes  *mls.MysqlWelcomes
	// Where a group is in its history, and the commits waiting for the
	// devices that have to apply them (#40).
	Groups  *mls.MysqlGroups
	Commits *mls.MysqlCommits
	// Which conversation belongs to which chat, decided once (#135).
	Conversations *mls.MysqlConversations
	// Who holds a leaf in each conversation, as the committer says (#147).
	Members *mls.MysqlMembers
	// Which signature key is whose, in a log that only grows (#44).
	KeyLog     *mls.KeyLog
	Store      kv.Store
	UserClient user_client.UserClient
	// Says "there is something in your box" to the devices of a person (#156).
	SyncClient sync_client.SyncClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMySQL(&c.Mysql)
	keyLogStore := mls.NewMysqlKeyLog(db)
	keyLog := mls.NewKeyLog(keyLogStore)
	// The log starts from the devices there are, not from the next publish.
	go keyLogStore.Backfill(context.Background(), keyLog, int32(time.Now().Unix()))
	return &ServiceContext{
		KeyLog:        keyLog,
		Config:        c,
		Directory:     mls.New(mls.NewMysqlStore(db)),
		Welcomes:      mls.NewMysqlWelcomes(db),
		Groups:        mls.NewMysqlGroups(db),
		Commits:       mls.NewMysqlCommits(db),
		Conversations: mls.NewMysqlConversations(db),
		Members:       mls.NewMysqlMembers(db),
		Store:         kv.NewStore(c.KV),
		UserClient:    user_client.NewUserClient(rpcx.GetCachedRpcClient(c.UserClient)),
		SyncClient:    queue.NewSyncClient(c.SyncClient),
	}
}
