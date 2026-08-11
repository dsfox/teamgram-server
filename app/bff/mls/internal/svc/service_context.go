package svc

import (
	"github.com/teamgram/marmota/pkg/stores/kv"
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	user_client "github.com/teamgram/teamgram-server/app/service/biz/user/client"
	"github.com/teamgram/teamgram-server/app/bff/mls/internal/config"
	"github.com/teamgram/teamgram-server/pkg/mls"
	"github.com/teamgram/marmota/pkg/net/rpcx"
)

type ServiceContext struct {
	Config    config.Config
	Directory *mls.Directory
	Welcomes  *mls.MysqlWelcomes
	Store     kv.Store
	UserClient user_client.UserClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMySQL(&c.Mysql)
	return &ServiceContext{
		Config:     c,
		Directory:  mls.New(mls.NewMysqlStore(db)),
		Welcomes:   mls.NewMysqlWelcomes(db),
		Store:      kv.NewStore(c.KV),
		UserClient: user_client.NewUserClient(rpcx.GetCachedRpcClient(c.UserClient)),
	}
}
