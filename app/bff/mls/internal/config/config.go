package config

import (
	"github.com/teamgram/marmota/pkg/stores/kv"
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/pkg/queue"

	"github.com/zeromicro/go-zero/zrpc"
)

// Config is what this service needs: a database, and a way to tell a device
// that its box has something in it.
//
// The directory stores opaque bytes and hands them back. Everything it knows,
// it knows from its tables; the one thing it says unasked is that there is
// something to fetch.
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

	// For the one thing this service says to a device on its own: there is
	// something in your box (#156).
	SyncClient *queue.Conf
}
