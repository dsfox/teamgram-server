package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/calls"
)

// connectionsFor is what the two phones are told to try. STUN comes first
// because a direct path is the whole point; relays follow only as the fallback
// for a symmetric NAT on both sides.
//
// p2pAllowed will come from the two people's "hide my IP" setting (Part 3 of
// the plan); until that is wired the answer is yes, which is the default the
// product asks for - connect them directly.
func (c *PhoneCore) connectionsFor(p2pAllowed bool, now time.Time) ([]*mtproto.PhoneConnection, error) {
	cfg := c.svcCtx.Config.Calls
	servers := make([]calls.Server, 0, len(cfg.Servers))
	for _, s := range cfg.Servers {
		servers = append(servers, calls.Server{
			Id: s.Id, Host: s.Host, HostV6: s.HostV6, Port: s.Port,
			Stun: s.Stun, Turn: s.Turn, Tcp: s.Tcp,
		})
	}
	return calls.Connections(servers, p2pAllowed, cfg.RelaySecret, cfg.RelayTTL(), now)
}

// find looks a call up by what the client was told about it. The access hash is
// the capability: an id on its own opens nothing.
func (c *PhoneCore) find(peer *mtproto.InputPhoneCall, now time.Time) (*calls.Call, error) {
	return c.svcCtx.Registry.Get(peer.GetId(), peer.GetAccessHash(), now)
}
