package config

import (
	"github.com/teamgram/teamgram-server/pkg/queue"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// A chat's theme is kept on both sides' dialogs, a colour on the account.
	DialogClient zrpc.RpcClientConf
	UserClient   zrpc.RpcClientConf
	// Who sees a person's name: the members of the groups they are in.
	ChatClient zrpc.RpcClientConf
	// The service message that tells both sides a chat's theme changed.
	MsgClient zrpc.RpcClientConf
	// A person's other phones hear of their new colour, and so does
	// everybody who sees their name.
	SyncClient *queue.Conf
}
