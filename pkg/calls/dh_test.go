package calls

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/teamgram/proto/mtproto"
)

// The two phones derive the call key in this group and never send it; the
// server only names the group. Both clients check the prime before they use
// it - 2048 bits, prime, (p-1)/2 prime, and for g=3 that p mod 3 is 2 - and a
// prime that fails a check ends the call before it starts, with no error the
// server would ever see. So the checks are here too.
func TestTheCallGroupPassesTheChecksBothPhonesMake(t *testing.T) {
	cfg := DhConfig(0, 256)
	if cfg.GetPredicateName() != mtproto.Predicate_messages_dhConfig {
		t.Fatalf("an unknown version was answered with %s", cfg.GetPredicateName())
	}
	if cfg.GetG() != 3 {
		t.Errorf("g is %d", cfg.GetG())
	}
	p := new(big.Int).SetBytes(cfg.GetP())
	if p.BitLen() != 2048 {
		t.Errorf("p has %d bits", p.BitLen())
	}
	if !p.ProbablyPrime(30) {
		t.Error("p is not prime")
	}
	half := new(big.Int).Rsh(new(big.Int).Sub(p, big.NewInt(1)), 1)
	if !half.ProbablyPrime(30) {
		t.Error("(p-1)/2 is not prime, so p is not a safe prime")
	}
	if new(big.Int).Mod(p, big.NewInt(3)).Int64() != 2 {
		t.Error("p mod 3 is not 2, which is what a client demands of g=3")
	}
	if len(cfg.GetRandom()) != 256 || bytes.Equal(cfg.GetRandom(), make([]byte, 256)) {
		t.Errorf("random is %d bytes and/or zero", len(cfg.GetRandom()))
	}
	if cfg.GetVersion() != DhVersion || DhVersion <= 0 {
		t.Errorf("version is %d", cfg.GetVersion())
	}
}

// A phone that already holds this version is told so, and still gets fresh
// randomness for its exponent.
func TestAPhoneWithTheCurrentGroupIsToldNotModified(t *testing.T) {
	cfg := DhConfig(DhVersion, 256)
	if cfg.GetPredicateName() != mtproto.Predicate_messages_dhConfigNotModified {
		t.Fatalf("answered with %s", cfg.GetPredicateName())
	}
	if len(cfg.GetRandom()) != 256 {
		t.Errorf("random is %d bytes", len(cfg.GetRandom()))
	}
	again := DhConfig(DhVersion, 256)
	if bytes.Equal(cfg.GetRandom(), again.GetRandom()) {
		t.Error("two phones were handed the same randomness")
	}
}
