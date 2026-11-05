package mls

import (
	"errors"
	"fmt"
)

// What the key log needs out of a key package (#44): the leaf's credential
// identity - the name every member uses for the device, "<user_id>/<n>" - and
// the signature key everything that device signs is checked with.
//
// The package is RFC 9420's MLSMessage carrying a KeyPackage, in the TLS
// presentation language: a version and a wire format, then the KeyPackage's
// version and cipher suite, the init key, and the leaf node, which begins with
// the encryption key, the signature key and the credential. Only that prefix
// is read. Nothing here checks the package's own signature: a phone adding the
// device checks that with the very key read here, and what this reading is for
// is to write down whose key it is.

const (
	mlsVersion10        = 1
	wireFormatKeyPacket = 5
	credentialBasic     = 1
)

// ErrNotAKeyPackage is bytes that do not begin like a key package. Published
// packages that are not real ones (a test's placeholder) are stored as before
// and never logged, so no phone that checks the log can be handed them.
var ErrNotAKeyPackage = errors.New("not an MLS key package")

// ReadKeyPackage returns the credential identity and the signature key of a
// published key package.
func ReadKeyPackage(raw []byte) (identity, signatureKey []byte, err error) {
	r := tlsReader{b: raw}
	if r.uint16() != mlsVersion10 || r.uint16() != wireFormatKeyPacket {
		return nil, nil, ErrNotAKeyPackage
	}
	if r.uint16() != mlsVersion10 {
		return nil, nil, ErrNotAKeyPackage
	}
	_ = r.uint16() // cipher suite
	_ = r.vector() // init key
	_ = r.vector() // the leaf's encryption key
	signatureKey = r.vector()
	if r.uint16() != credentialBasic {
		return nil, nil, fmt.Errorf("%w: the credential is not a basic one", ErrNotAKeyPackage)
	}
	identity = r.vector()
	if r.err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrNotAKeyPackage, r.err)
	}
	if len(signatureKey) == 0 || len(identity) == 0 {
		return nil, nil, fmt.Errorf("%w: an empty key or identity", ErrNotAKeyPackage)
	}
	return identity, signatureKey, nil
}

// tlsReader reads RFC 9420's encoding: big-endian integers and vectors whose
// length is a variable-length integer (RFC 9000, section 16).
type tlsReader struct {
	b   []byte
	err error
}

func (r *tlsReader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.b) {
		r.err = errors.New("the package ends early")
		return nil
	}
	out := r.b[:n]
	r.b = r.b[n:]
	return out
}

func (r *tlsReader) uint16() int {
	b := r.take(2)
	if b == nil {
		return -1
	}
	return int(b[0])<<8 | int(b[1])
}

func (r *tlsReader) vector() []byte {
	first := r.take(1)
	if first == nil {
		return nil
	}
	length := int(first[0] & 0x3f)
	switch first[0] >> 6 {
	case 0:
	case 1:
		rest := r.take(1)
		if rest == nil {
			return nil
		}
		length = length<<8 | int(rest[0])
	case 2:
		rest := r.take(3)
		if rest == nil {
			return nil
		}
		length = length<<24 | int(rest[0])<<16 | int(rest[1])<<8 | int(rest[2])
	default:
		// RFC 9420 allows no eight-byte lengths.
		r.err = errors.New("a length too long for a key package")
		return nil
	}
	return r.take(length)
}
