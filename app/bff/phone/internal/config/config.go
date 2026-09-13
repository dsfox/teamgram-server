package config

import (
	"time"

	"github.com/teamgram/teamgram-server/pkg/queue"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config is what matchmaking a call needs: a way to tell the other phone that
// it is being called, and the endpoints the two of them are told to try.
//
// No media passes through here. The servers below are STUN, which only tells a
// phone its own public address, and relays, which are the fallback for the one
// case a direct path cannot win.
type Config struct {
	zrpc.RpcServerConf
	SyncClient *queue.Conf
	Calls      Calls
}

// Calls is what a phone is handed to reach the other phone.
type Calls struct {
	Servers []Server
	// RelaySecret signs the short-lived credentials a relay accepts. Empty
	// means we hand out no relay credentials at all.
	RelaySecret string
	// RelayTTLSeconds is how long such a pair lives. Unset means the default.
	RelayTTLSeconds int
}

// Server is one STUN or TURN endpoint.
type Server struct {
	Id     int64
	Host   string
	HostV6 string
	Port   int32
	Stun   bool
	Turn   bool
	Tcp    bool
}

// DefaultRelayTTL is short on purpose: a leaked credential should die on its own.
const DefaultRelayTTL = time.Hour

// RelayTTL is the lifetime to use, defaulted in code rather than in a struct
// tag - a tag the loader does not read is a rule that never applied (#151).
func (c Calls) RelayTTL() time.Duration {
	if c.RelayTTLSeconds <= 0 {
		return DefaultRelayTTL
	}
	return time.Duration(c.RelayTTLSeconds) * time.Second
}
