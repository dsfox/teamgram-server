package gnet

import (
	"testing"

	"github.com/teamgram/teamgram-server/app/interface/gnetway/internal/config"
)

// teamgram's stock key, whose fingerprint every copy of the clients carried
// until #242: 0xa9e071c1771060cd.
const stockKey = "../../../../../../teamgramd/bin/server_pkcs1.key"

func TestAKeyIsNamedByItsFingerprint(t *testing.T) {
	got, err := fingerprintOfKeyFile(stockKey)
	if err != nil {
		t.Fatal(err)
	}
	if got != 12240908862933197005 {
		t.Fatalf("the stock key came out as %d (0x%x), not 0xa9e071c1771060cd", got, got)
	}
}

func TestAConfigThatMisnamesItsKeyIsRefused(t *testing.T) {
	h := mustNewHandshake([]config.RSAKey{{KeyFile: stockKey, KeyFingerprint: "12240908862933197005"}})
	if len(h.keyFingerprints) != 1 || uint64(h.keyFingerprints[0]) != 12240908862933197005 {
		t.Fatalf("the rightly named key was not offered: %v", h.keyFingerprints)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("a key named by somebody else's fingerprint was taken")
		}
	}()
	mustNewHandshake([]config.RSAKey{{KeyFile: stockKey, KeyFingerprint: "9317290418914058974"}})
}
