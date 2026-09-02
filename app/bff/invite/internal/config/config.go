package config

import (
	"github.com/teamgram/marmota/pkg/stores/kv"
	"github.com/teamgram/marmota/pkg/stores/sqlx"

	"github.com/zeromicro/go-zero/zrpc"
)

// Config is what minting an invitation needs: the store the codes live in,
// the database the record lives in, and the user directory that says whether
// a number already has an account.
type Config struct {
	zrpc.RpcServerConf
	KV         kv.KvConf
	Mysql      sqlx.Config
	UserClient zrpc.RpcClientConf
	// How long a code a person sent lives. Seven days, not the CLI's one: a
	// person does not install an app the day the SMS arrives.
	InvitationDays int `json:",default=7"`
}
