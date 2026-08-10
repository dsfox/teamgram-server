package svc

import (
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/app/bff/mls/internal/config"
	"github.com/teamgram/teamgram-server/pkg/mls"
)

type ServiceContext struct {
	Config    config.Config
	Directory *mls.Directory
	Welcomes  *mls.MysqlWelcomes
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMySQL(&c.Mysql)
	return &ServiceContext{
		Config:    c,
		Directory: mls.New(mls.NewMysqlStore(db)),
		Welcomes:  mls.NewMysqlWelcomes(db),
	}
}
