package config

import (
	"github.com/teamgram/marmota/pkg/stores/sqlx"

	"github.com/zeromicro/go-zero/zrpc"
)

// Config is what this service needs, which is a database and nothing else.
//
// The directory stores opaque bytes and hands them back. It talks to no other
// service, has no cache and no queue: everything it knows, it knows from one
// table.
type Config struct {
	zrpc.RpcServerConf
	Mysql sqlx.Config
}
