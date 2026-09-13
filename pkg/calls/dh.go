package calls

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/teamgram/proto/mtproto"
)

// The Diffie-Hellman group the two phones derive the call key in. The server
// names it and never learns the key: g_a_hash, g_b and g_a pass through
// unread (see Call).
//
// The same 2048-bit safe prime and generator the MTProto handshake uses
// (app/interface/gnetway/.../handshake.go), which is the one both clients
// hold as known-good and check without a full primality test. Any other prime
// would be tested on the phone with every call, and one that failed a check
// would end the call with no error the server ever sees.
//
// messages.getDhConfig version:int random_length:int = messages.DhConfig;
const dhPrimeHex = "" +
	"C71CAEB9C6B1C9048E6C522F70F13F73980D40238E3E21C14934D037563D930F" +
	"48198A0AA7C14058229493D22530F4DBFA336F6E0AC925139543AED44CCE7C37" +
	"20FD51F69458705AC68CD4FE6B6B13ABDC9746512969328454F18FAF8C595F64" +
	"2477FE96BB2A941D5BCD1D4AC8CC49880708FA9B378E3C4F3A9060BEE67CF9A4" +
	"A4A695811051907E162753B56B0F6B410DBA74D8A84B2A14B3144E0EF1284754" +
	"FD17ED950D5965B4B9DD46582DB1178D169C6BC465B0D6FF9CA3928FEF5B9AE4" +
	"E418FC15E83EBEA0F87FA9FF5EED70050DED2849F47BF959D956850CE929851F" +
	"0D8115F635B105EE2E4E15D04B2454BF6F4FADF034B10403119CD8E3B92FCC5B"

// DhVersion changes only if the group does. A phone that already holds this
// version is told so and spared the 256 bytes.
const DhVersion = 1

var dhPrime = mustHex(dhPrimeHex)

// DhConfig is the group a phone asks for before it places or answers a call,
// with fresh randomness for the phone to mix into its exponent.
func DhConfig(version, randomLength int32) *mtproto.Messages_DhConfig {
	random := make([]byte, randomLength)
	if _, err := rand.Read(random); err != nil {
		panic("calls: no randomness for a call: " + err.Error())
	}
	if version == DhVersion {
		return mtproto.MakeTLMessagesDhConfigNotModified(&mtproto.Messages_DhConfig{
			Random: random,
		}).To_Messages_DhConfig()
	}
	return mtproto.MakeTLMessagesDhConfig(&mtproto.Messages_DhConfig{
		G:       3,
		P:       dhPrime,
		Version: DhVersion,
		Random:  random,
	}).To_Messages_DhConfig()
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic("calls: the DH prime is not hex: " + err.Error())
	}
	return b
}
