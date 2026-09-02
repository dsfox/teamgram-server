package svc

import (
	"github.com/teamgram/marmota/pkg/net/rpcx"
	"github.com/teamgram/marmota/pkg/stores/kv"
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/app/bff/invite/internal/config"
	user_client "github.com/teamgram/teamgram-server/app/service/biz/user/client"
	"github.com/teamgram/teamgram-server/pkg/code/invite"
)

type ServiceContext struct {
	Config config.Config
	// Where the codes live, with their lifetime.
	Store kv.Store
	// The record of who brought whom (#47).
	Invitations *invite.MysqlInvitations
	// Says whether a number already has an account.
	UserClient user_client.UserClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:      c,
		Store:       kv.NewStore(c.KV),
		Invitations: invite.NewMysqlInvitations(sqlx.NewMySQL(&c.Mysql)),
		UserClient:  user_client.NewUserClient(rpcx.GetCachedRpcClient(c.UserClient)),
	}
}
