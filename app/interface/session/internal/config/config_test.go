package config

import (
	"strings"
	"testing"

	"github.com/teamgram/proto/mtproto"
	"github.com/zeromicro/go-zero/core/conf"
)

// The session sends a request to the bff whose etcd key the IDMap names for
// the longest-matching prefix of the method, and a method with no prefix falls
// through to the stub layer, which refuses it with a licence message that
// explains nothing. The rule below is the one NewBFFProxyClients applies; this
// reads the config the way the service does, so a commented-out line is a
// missing line here too.
func TestEveryCallMethodHasAClientInTheSessionConfig(t *testing.T) {
	var c Config
	if err := conf.Load("../../../../../teamgramd/etc2/session.yaml", &c); err != nil {
		t.Fatalf("cannot load the session config: %v", err)
	}

	clients := map[string]bool{}
	for _, client := range c.BFFProxyClients.Clients {
		clients[client.Etcd.Key] = true
	}

	for _, request := range []string{
		"TLPhoneGetCallConfig",
		"TLPhoneRequestCall",
		"TLPhoneAcceptCall",
		"TLPhoneConfirmCall",
		"TLPhoneDiscardCall",
		"TLPhoneSendSignalingData",
	} {
		tuple, ok := mtproto.GetRPCContextRegisters()[request]
		if !ok {
			t.Errorf("%s has no gRPC route at all", request)
			continue
		}
		routed := false
		for prefix, key := range c.BFFProxyClients.IDMap {
			if strings.HasPrefix(tuple.Method, prefix) {
				routed = true
				if !clients[key] {
					t.Errorf("%s goes to %q, which no client in the session config serves", tuple.Method, key)
				}
			}
		}
		if !routed {
			t.Errorf("%s (%s) has no prefix in the IDMap, so it would be refused by the stub layer", request, tuple.Method)
		}
	}
}
