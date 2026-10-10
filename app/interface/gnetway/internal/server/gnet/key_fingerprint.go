package gnet

import (
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
)

// stockFingerprint names teamgram's stock key, whose private half is public:
// it may still be offered to old apps, but never handed out as a server's own
// (#242, #244).
const stockFingerprint uint64 = 12240908862933197005

// readServerKey parses a PKCS#1 private key file, the only form the
// handshake's RSA code reads.
func readServerKey(keyFile string, data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("%s: no PEM block", keyFile)
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", keyFile, err)
	}
	return key, nil
}

// fingerprintOf is the number a handshake names an RSA key by: the lower 64
// bits of SHA-1 over the modulus and the exponent, each written as TL bytes.
// The config may state it beside the key file; a wrong one used to be taken
// as it was - every client then failed to find a key it knew, with nothing
// said here (#242). tools/rsa_fingerprint.py computes the same.
func fingerprintOf(key *rsa.PublicKey) uint64 {
	digest := sha1.Sum(append(tlBytes(key.N), tlBytes(big.NewInt(int64(key.E)))...))
	return binary.LittleEndian.Uint64(digest[12:])
}

func fingerprintOfKeyFile(keyFile string) (uint64, error) {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return 0, err
	}
	key, err := readServerKey(keyFile, data)
	if err != nil {
		return 0, err
	}
	return fingerprintOf(&key.PublicKey), nil
}

func tlBytes(value *big.Int) []byte {
	raw := value.Bytes()
	var out []byte
	if len(raw) < 254 {
		out = append([]byte{byte(len(raw))}, raw...)
	} else {
		out = append([]byte{254, byte(len(raw)), byte(len(raw) >> 8), byte(len(raw) >> 16)}, raw...)
	}
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out
}
