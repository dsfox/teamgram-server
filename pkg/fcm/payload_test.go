package fcm

import (
	"crypto/aes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// A key of the size a device registers, filled with something that is not all
// zeroes: a wrong offset in the derivation would be invisible against zeroes.
func testKey() string {
	key := make([]byte, 256)
	for i := range key {
		key[i] = byte(i*7 + 3)
	}
	return hex.EncodeToString(key)
}

// openLikeTheClient repeats what PushListenerController does, step for step:
// check the key id, derive, decrypt, check the message key against the
// plaintext, read the length, read the json. A payload this refuses is one the
// phone would refuse, and the phone says nothing but "DECRYPT ERROR".
func openLikeTheClient(t *testing.T, secretHex, envelope string) map[string]any {
	t.Helper()

	authKey, err := hex.DecodeString(secretHex)
	if err != nil {
		t.Fatalf("the key is not hex: %v", err)
	}
	raw, err := base64.URLEncoding.DecodeString(envelope)
	if err != nil {
		t.Fatalf("the envelope is not base64url: %v", err)
	}
	if len(raw) < 24 {
		t.Fatalf("the envelope is %d bytes, shorter than its own header", len(raw))
	}

	keyId := sha1.Sum(authKey)
	if string(raw[:8]) != string(keyId[len(keyId)-8:]) {
		t.Fatal("the key id does not name this key, so the client drops it")
	}

	msgKey := raw[8:24]
	aesKey, aesIv := keyPair(authKey, msgKey)
	plain := igeDecrypt(t, raw[24:], aesKey, aesIv)

	sum := sha256.Sum256(append(append([]byte{}, authKey[88+x:88+x+32]...), plain...))
	if string(sum[8:24]) != string(msgKey) {
		t.Fatal("the message key does not match the plaintext, so the client drops it")
	}

	length := binary.LittleEndian.Uint32(plain[:4])
	if int(length) > len(plain)-4 {
		t.Fatalf("the length says %d bytes, only %d follow", length, len(plain)-4)
	}

	var payload map[string]any
	if err := json.Unmarshal(plain[4:4+length], &payload); err != nil {
		t.Fatalf("what came out is not json: %v", err)
	}
	return payload
}

func igeDecrypt(t *testing.T, data, key, iv []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	prevCipher := iv[:aes.BlockSize]
	prevPlain := iv[aes.BlockSize:]
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += aes.BlockSize {
		in := data[i : i+aes.BlockSize]
		buf := make([]byte, aes.BlockSize)
		for j := range buf {
			buf[j] = in[j] ^ prevPlain[j]
		}
		block.Decrypt(buf, buf)
		for j := range buf {
			buf[j] ^= prevCipher[j]
		}
		copy(out[i:], buf)
		prevCipher = in
		prevPlain = out[i : i+aes.BlockSize]
	}
	return out
}

func TestTheClientCanOpenTheEnvelope(t *testing.T) {
	secret := testKey()
	envelope, err := Envelope(secret, map[string]any{
		"badge":   3,
		"custom":  map[string]any{"from_id": "42"},
		"loc_key": "",
	})
	if err != nil {
		t.Fatalf("cannot build the envelope: %v", err)
	}

	payload := openLikeTheClient(t, secret, envelope)
	if payload["badge"].(float64) != 3 {
		t.Errorf("the badge came out as %v", payload["badge"])
	}
	if from := payload["custom"].(map[string]any)["from_id"]; from != "42" {
		t.Errorf("from_id came out as %v", from)
	}
}

func TestEachEnvelopeDiffers(t *testing.T) {
	secret := testKey()
	same := map[string]any{"badge": 1, "loc_key": ""}

	first, err := Envelope(secret, same)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Envelope(secret, same)
	if err != nil {
		t.Fatal(err)
	}
	// The padding is random, so two identical notifications must not be
	// identical on the wire - otherwise anyone watching learns when the same
	// thing happened twice.
	if first == second {
		t.Error("the same notification encrypted to the same bytes twice")
	}
}

func TestAShortKeyIsRefusedRatherThanSentGarbled(t *testing.T) {
	if _, err := Envelope(hex.EncodeToString(make([]byte, 32)), map[string]any{}); err == nil {
		t.Error("a key too short to derive from was accepted")
	}
	if _, err := Envelope("not hex at all", map[string]any{}); err == nil {
		t.Error("a key that is not hex was accepted")
	}
}

func TestWithoutAKeyTheBannerIsSentInstead(t *testing.T) {
	s := &Sender{projectId: "p"}

	withKey, err := s.compose("token", Notify{Badge: 1, Secret: testKey()})
	if err != nil {
		t.Fatal(err)
	}
	message := withKey["message"].(map[string]any)
	if _, drawn := message["notification"]; drawn {
		t.Error("a message with a key still asks Firebase to draw it, so the app will not run")
	}
	if _, carried := message["data"].(map[string]any)["p"]; !carried {
		t.Error("the envelope is missing, so the app has nothing to open")
	}

	without, err := s.compose("token", Notify{Title: "ice9", Body: "New message"})
	if err != nil {
		t.Fatal(err)
	}
	fallback := without["message"].(map[string]any)
	if _, drawn := fallback["notification"]; !drawn {
		t.Error("a device with no key was sent nothing it can show")
	}
}

func TestTheTextIsNowhereInTheMessage(t *testing.T) {
	s := &Sender{projectId: "p"}
	message, err := s.compose("token", Notify{
		Badge: 1, FromId: "42", Secret: testKey(),
		Title: "ice9", Body: "New message",
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	// Whatever else changes here, nothing a person wrote may travel to Google -
	// and neither may the name of whoever wrote it.
	for _, forbidden := range []string{"New message", "ice9"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("%q is in the message sent to Google", forbidden)
		}
	}
}
