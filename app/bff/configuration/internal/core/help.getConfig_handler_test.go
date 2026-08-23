package core

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// Which language a phone is offered before anybody chooses one. The rule lives
// on the server so that both clients make the same decision, and it is worth a
// test because the answer is a product decision rather than a technical one:
// Russian where it is what people read, English everywhere else.
//
// The config file this handler starts from suggested "classic-zh-cn" to
// everyone, inherited from upstream, until this was written.
func TestSuggestedLanguage(t *testing.T) {
	cases := []struct {
		client string
		want   string
	}{
		{"ru", "ru"},
		{"ru-RU", "ru"},
		{"ru_RU", "ru"},
		{"RU", "ru"},
		{"uk", "ru"},
		{"uk-UA", "ru"},
		{"kk", "ru"},
		{"kk-KZ", "ru"},
		{"be", "ru"},
		{"be-BY", "ru"},

		{"en", "en"},
		{"en-US", "en"},
		{"de", "en"},
		{"fr-FR", "en"},
		{"zh-CN", "en"},
		{"", "en"},
	}

	for _, c := range cases {
		if got := suggestedLanguage(c.client); got != c.want {
			t.Errorf("a client reporting %q is offered %q, expected %q", c.client, got, c.want)
		}
	}
}

// dc_options is the list a phone keeps and dials from then on, so a server put
// up by somebody else has to name itself here. If it handed out the address
// baked into the file, its people would be moved onto our machine without
// anybody choosing that. See ice9 #65.
func TestTheServerNamesItself(t *testing.T) {
	cases := []struct {
		address string
		host    string
		port    int32
	}{
		{"common.ice9.app:10443", "common.ice9.app", 10443},
		// A name with no port gets the one both clients dial.
		{"talk.example.org", "talk.example.org", 10443},
		{"203.0.113.9", "203.0.113.9", 10443},
		{"203.0.113.9:443", "203.0.113.9", 443},
		// Whitespace from a shell variable is not an address.
		{"  common.ice9.app:10443  ", "common.ice9.app", 10443},
	}

	for _, c := range cases {
		config = mtproto.TLConfig{Data2: &mtproto.Config{ThisDc: 1}}
		t.Setenv("ICE9_ADDRESS", c.address)

		adoptAddressFromEnvironment()

		options := config.GetDcOptions()
		if len(options) != 1 {
			t.Fatalf("%q produced %d addresses, want exactly one", c.address, len(options))
		}
		got := options[0]
		if got.GetIpAddress() != c.host || got.GetPort() != c.port {
			t.Errorf("%q became %s:%d, want %s:%d",
				c.address, got.GetIpAddress(), got.GetPort(), c.host, c.port)
		}
		if got.GetId() != 1 {
			t.Errorf("%q was announced as datacenter %d, want this one", c.address, got.GetId())
		}
	}
}

// Nothing in the environment means the file decides, which is what the local
// stand relies on. And a port that is not a port must leave the file alone
// rather than announce something no client can dial.
func TestTheFileDecidesWhenNobodySaysOtherwise(t *testing.T) {
	fromTheFile := []*mtproto.DcOption{
		mtproto.MakeTLDcOption(&mtproto.DcOption{
			Id: 1, IpAddress: "5.23.53.210", Port: 10443, Static: true,
		}).To_DcOption(),
	}

	for _, address := range []string{"", "   ", "example.org:no-such-port", "example.org:99999"} {
		config = mtproto.TLConfig{Data2: &mtproto.Config{ThisDc: 1, DcOptions: fromTheFile}}
		t.Setenv("ICE9_ADDRESS", address)

		adoptAddressFromEnvironment()

		options := config.GetDcOptions()
		if len(options) != 1 || options[0].GetIpAddress() != "5.23.53.210" {
			t.Errorf("ICE9_ADDRESS=%q changed the address the file gave", address)
		}
	}
}
