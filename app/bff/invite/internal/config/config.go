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
	ChatClient zrpc.RpcClientConf
	// How long a code a person sent lives. Seven days, not the CLI's one: a
	// person does not install an app the day the SMS arrives. Unset means
	// seven - see Days. Not read from a file: the combined bff builds this
	// config in code, so a tag here would be a rule that never applied (#151).
	InvitationDays int
}

// DefaultInvitationDays is what a code lives when the config says nothing.
const DefaultInvitationDays = 7

// Days is the lifetime to use, the default applied in code rather than in a
// struct tag: a tag the loader does not read is a rule that never applied
// (#151), and this one has to.
func (c Config) Days() int {
	if c.InvitationDays <= 0 {
		return DefaultInvitationDays
	}
	return c.InvitationDays
}
