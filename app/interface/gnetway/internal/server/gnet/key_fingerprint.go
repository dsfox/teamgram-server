package gnet

import (
	"crypto/sha1"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
)

// fingerprintOfKeyFile is the number a handshake names an RSA key by: the lower 64
// bits of SHA-1 over the modulus and the exponent, each written as TL bytes.
// The config states it beside the key file by hand, and a wrong one used to
// be taken as it was - every client then failed to find a key it knew, with
// nothing said here (#242). tools/rsa_fingerprint.py computes the same.
func fingerprintOfKeyFile(keyFile string) (uint64, error) {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return 0, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return 0, fmt.Errorf("%s: no PEM block", keyFile)
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", keyFile, err)
	}
	digest := sha1.Sum(append(tlBytes(key.N), tlBytes(big.NewInt(int64(key.E)))...))
	return binary.LittleEndian.Uint64(digest[12:]), nil
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
