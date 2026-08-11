package config

import (
	"github.com/teamgram/marmota/pkg/stores/kv"
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

	// Both are here for one method: registering the way back into an account.
	// The store is where a recovery secret lives, keyed by phone number, and
	// the phone number of the device asking is something only the user
	// directory can say - taking the client's word for it would let anybody
	// register a way into somebody else's account.
	KV         kv.KvConf
	UserClient zrpc.RpcClientConf
}
