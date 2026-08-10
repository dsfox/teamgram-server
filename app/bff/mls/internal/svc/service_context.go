package svc

import (
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/app/bff/mls/internal/config"
	"github.com/teamgram/teamgram-server/pkg/mls"
)

type ServiceContext struct {
	Config    config.Config
	Directory *mls.Directory
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:    c,
		Directory: mls.New(mls.NewMysqlStore(sqlx.NewMySQL(&c.Mysql))),
	}
}
