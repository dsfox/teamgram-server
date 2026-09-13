package config

import (
	"net"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

// A call is confirmed with the endpoints this config lists, and with none the
// server refuses the confirm. So the config the machine runs has to load the
// way the process loads it and name at least one STUN endpoint, on both
// address families - two phones on carrier IPv6 connect with no NAT at all.
func TestTheBffConfigHandsOutAStunEndpoint(t *testing.T) {
	t.Setenv("MYSQL_PASSWORD", "unused-by-this-test")

	var c Config
	if err := conf.Load("../../../../../teamgramd/etc2/bff.yaml", &c, conf.UseEnv()); err != nil {
		t.Fatalf("cannot load the bff config: %v", err)
	}

	stun := 0
	for _, s := range c.Calls.Servers {
		if !s.Stun {
			continue
		}
		stun++
		if ip := net.ParseIP(s.Host); ip == nil || ip.To4() == nil {
			t.Errorf("server %d: Host %q is not an IPv4 literal, and the phones are handed it as one", s.Id, s.Host)
		}
		if ip := net.ParseIP(s.HostV6); ip == nil || ip.To4() != nil {
			t.Errorf("server %d: HostV6 %q is not an IPv6 literal", s.Id, s.HostV6)
		}
		if s.Port == 0 {
			t.Errorf("server %d has no port", s.Id)
		}
	}
	if stun == 0 {
		t.Fatal("no STUN endpoint in the config, so every confirmCall would be refused")
	}
}
