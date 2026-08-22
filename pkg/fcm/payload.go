// The push payload the Android client can open.
//
// Firebase draws the banner itself for any message carrying a "notification"
// block, and the app never wakes - which cost us every notification setting in
// the app and the ability to say who wrote (#94). A message with data only is
// handed to the app instead, and the app expects that data in one field, "p":
// Telegram's own encrypted envelope, keyed by the secret the device sent us
// when it registered.
//
// The envelope, as PushListenerController reads it:
//
//	base64url( authKeyId[8] | msgKey[16] | AES-IGE( int32 length | json | pad ) )
//
// with MTProto 2.0 key derivation and x = 8, the value the client uses for
// anything arriving at it. Nothing in the json needs to describe the message:
// an unknown loc_key makes the client wake its connection and fetch, and the
// notification it then draws is its own, with the real name of whoever wrote
// and - for an encrypted conversation - text only that device can read. That is
// the only way this product can ever show more than "New message", since the
// server cannot read the message either.
package fcm

import (
	"crypto/aes"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// x is the offset into the auth key for a message travelling to the client.
const x = 8

// Envelope encrypts what the client is to be woken with. The secret is the
// device's push key as it was registered, hex as the database holds it.
func Envelope(secretHex string, payload map[string]any) (string, error) {
	authKey, err := hex.DecodeString(secretHex)
	if err != nil {
		return "", fmt.Errorf("fcm: the push secret is not hex: %w", err)
	}
	if len(authKey) < 88+x+32 {
		return "", fmt.Errorf("fcm: the push secret is %d bytes, too short to key anything", len(authKey))
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	// int32 length, the json, then padding to a whole number of AES blocks.
	// The client reads the length and ignores the rest; the padding is random
	// so two identical notifications do not encrypt alike.
	plain := make([]byte, 4+len(body))
	binary.LittleEndian.PutUint32(plain, uint32(len(body)))
	copy(plain[4:], body)
	if pad := (16 - len(plain)%16) % 16; pad > 0 {
		tail := make([]byte, pad)
		if _, err := rand.Read(tail); err != nil {
			return "", err
		}
		plain = append(plain, tail...)
	}

	// The message key is a hash of the plaintext under part of the auth key, so
	// a payload that was tampered with cannot survive it.
	sum := sha256.Sum256(append(append([]byte{}, authKey[88+x:88+x+32]...), plain...))
	msgKey := sum[8:24]

	aesKey, aesIv := keyPair(authKey, msgKey)
	cipherText, err := igeEncrypt(plain, aesKey, aesIv)
	if err != nil {
		return "", err
	}

	keyId := sha1.Sum(authKey)
	envelope := make([]byte, 0, 8+16+len(cipherText))
	envelope = append(envelope, keyId[len(keyId)-8:]...)
	envelope = append(envelope, msgKey...)
	envelope = append(envelope, cipherText...)

	return base64.URLEncoding.EncodeToString(envelope), nil
}

// keyPair is MTProto 2.0's derivation, written the way the client reads it so
// the two can be compared line by line.
func keyPair(authKey, msgKey []byte) (aesKey, aesIv []byte) {
	a := sha256.Sum256(concat(msgKey, authKey[x:x+36]))
	b := sha256.Sum256(concat(authKey[40+x:40+x+36], msgKey))

	aesKey = concat(a[0:8], b[8:24], a[24:32])
	aesIv = concat(b[0:8], a[8:24], b[24:32])
	return aesKey, aesIv
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// igeEncrypt is AES in infinite garble extension mode, which MTProto uses and
// the standard library does not carry.
func igeEncrypt(plain, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(plain)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("fcm: %d bytes is not a whole number of blocks", len(plain))
	}
	if len(iv) != 2*aes.BlockSize {
		return nil, fmt.Errorf("fcm: the iv is %d bytes, not %d", len(iv), 2*aes.BlockSize)
	}

	prevCipher := iv[:aes.BlockSize]
	prevPlain := iv[aes.BlockSize:]
	out := make([]byte, len(plain))

	for i := 0; i < len(plain); i += aes.BlockSize {
		in := plain[i : i+aes.BlockSize]
		buf := make([]byte, aes.BlockSize)
		for j := range buf {
			buf[j] = in[j] ^ prevCipher[j]
		}
		block.Encrypt(buf, buf)
		for j := range buf {
			buf[j] ^= prevPlain[j]
		}
		copy(out[i:], buf)
		prevCipher = out[i : i+aes.BlockSize]
		prevPlain = in
	}
	return out, nil
}
