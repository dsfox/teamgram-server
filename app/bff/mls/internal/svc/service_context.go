package svc

import (
	"github.com/teamgram/marmota/pkg/net/rpcx"
	"github.com/teamgram/marmota/pkg/stores/kv"
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/app/bff/mls/internal/config"
	user_client "github.com/teamgram/teamgram-server/app/service/biz/user/client"
	"github.com/teamgram/teamgram-server/pkg/mls"
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
	Store         kv.Store
	UserClient    user_client.UserClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMySQL(&c.Mysql)
	return &ServiceContext{
		Config:        c,
		Directory:     mls.New(mls.NewMysqlStore(db)),
		Welcomes:      mls.NewMysqlWelcomes(db),
		Groups:        mls.NewMysqlGroups(db),
		Commits:       mls.NewMysqlCommits(db),
		Conversations: mls.NewMysqlConversations(db),
		Members:       mls.NewMysqlMembers(db),
		Store:         kv.NewStore(c.KV),
		UserClient:    user_client.NewUserClient(rpcx.GetCachedRpcClient(c.UserClient)),
	}
}
