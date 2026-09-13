package calls

import (
	"strings"
	"testing"
	"time"
)

// The rules these tests hold are the product decision, not a detail:
// a call goes direct unless the caller asked to hide their IP, and "hide my IP"
// means the phone is never even told a direct candidate.

var (
	when = time.Unix(1_700_000_000, 0)

	stunOnly = Server{Id: 1, Host: "203.0.113.10", HostV6: "2001:db8::10", Port: 3478, Stun: true}
	relay    = Server{Id: 2, Host: "203.0.113.20", HostV6: "2001:db8::20", Port: 3478, Turn: true}
	v4Relay  = Server{Id: 3, Host: "203.0.113.30", Port: 3478, Turn: true}
)

func TestDirectIsOfferedWhenPeerToPeerIsAllowed(t *testing.T) {
	got, err := Connections([]Server{stunOnly, relay}, true, "secret", time.Hour, when)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var stun, turn int
	for _, c := range got {
		if c.Stun {
			stun++
		}
		if c.Turn {
			turn++
		}
	}
	if stun == 0 {
		t.Error("no STUN was offered, so the phones cannot find a direct path")
	}
	if turn == 0 {
		t.Error("no TURN fallback was offered, so a symmetric NAT has nowhere to go")
	}
	if !got[0].Stun {
		t.Error("STUN must come first: direct is what we want tried before the relay")
	}
}

func TestHidingTheIpOffersNoDirectCandidateAtAll(t *testing.T) {
	got, err := Connections([]Server{stunOnly, relay}, false, "secret", time.Hour, when)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("a relay-only call still needs the relay")
	}
	for _, c := range got {
		if c.Stun {
			t.Fatal("a STUN candidate leaks the address the setting exists to hide")
		}
		if !c.Turn {
			t.Fatal("with peer-to-peer refused every connection must be a relay")
		}
	}
}

func TestHidingTheIpWithoutARelayIsRefusedRatherThanLeaked(t *testing.T) {
	if _, err := Connections([]Server{stunOnly}, false, "secret", time.Hour, when); err == nil {
		t.Error("with no relay and no peer-to-peer there is nothing honest to answer; " +
			"falling back to STUN would leak the address")
	}
}

func TestIpv6TravelsWithTheCandidate(t *testing.T) {
	got, _ := Connections([]Server{stunOnly}, true, "secret", time.Hour, when)
	if got[0].Ipv6 != stunOnly.HostV6 {
		t.Errorf("IPv6 dropped: %q - it is the surest way to a direct path, with no NAT in between", got[0].Ipv6)
	}
	if got[0].Ip != stunOnly.Host {
		t.Errorf("IPv4 dropped: %q", got[0].Ip)
	}
}

func TestRelayCredentialsExpire(t *testing.T) {
	got, _ := Connections([]Server{relay}, true, "secret", time.Hour, when)
	c := got[0]
	if c.Username == "" || c.Password == "" {
		t.Fatal("a relay with no credentials is an open relay")
	}
	if !strings.HasPrefix(c.Username, "1700003600") {
		t.Errorf("the username carries the expiry so the relay can refuse a stale one, got %q", c.Username)
	}
	later, _ := Connections([]Server{relay}, true, "secret", time.Hour, when.Add(time.Minute))
	if later[0].Password == c.Password {
		t.Error("the password must change with the expiry, or it never expires")
	}
	other, _ := Connections([]Server{relay}, true, "another", time.Hour, when)
	if other[0].Password == c.Password {
		t.Error("the password must depend on the secret")
	}
}

func TestStunCandidatesCarryNoCredentials(t *testing.T) {
	got, _ := Connections([]Server{stunOnly}, true, "secret", time.Hour, when)
	if got[0].Username != "" || got[0].Password != "" {
		t.Error("STUN asks nothing of the phone; handing it credentials only invites reuse")
	}
}

func TestAServerWithoutIpv6IsStillUsable(t *testing.T) {
	got, err := Connections([]Server{v4Relay}, false, "secret", time.Hour, when)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Ipv6 != "" {
		t.Error("an IPv4-only relay must not invent an IPv6 address")
	}
}
