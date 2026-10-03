package config

import (
	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/pkg/queue"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// Where the reactions are kept.
	Mysql sqlx.Config
	// Messages are found in the boxes of the people they were sent to, and a
	// group's members in the chat service; both live in the biz process.
	MessageClient zrpc.RpcClientConf
	ChatClient    zrpc.RpcClientConf
	UserClient    zrpc.RpcClientConf
	IdgenClient   zrpc.RpcClientConf
	// Tells the others in the conversation, at once.
	SyncClient *queue.Conf
}
