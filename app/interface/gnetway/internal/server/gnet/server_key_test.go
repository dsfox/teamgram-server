package gnet

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panjf2000/gnet/v2"
	"github.com/teamgram/teamgram-server/app/interface/gnetway/internal/config"
)

func TestTheKeyRequestIsToldApart(t *testing.T) {
	for _, c := range []struct {
		in   string
		want keyAsk
	}{
		{"GET /key HTTP/1.0\r\nHost: x\r\n\r\n", keyWhole},
		{"GET /key HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n", keyWhole},
		{"GET /k", keyPartial},
		{"GET /key HTTP/1.1\r\nHost: x\r\n", keyPartial},
		{"GET /key HTTP/1.1\r\nX: " + strings.Repeat("a", keyRequestLimit) + "\r\n\r\n", keyTooLong},
		{"GET /keys HTTP/1.1\r\n\r\n", notKeyRequest},
		{"GET /key?x HTTP/1.1\r\n\r\n", notKeyRequest},
		{"POST /key HTTP/1.1\r\n\r\n", notKeyRequest},
		{"GET / HTTP/1.1\r\n\r\n", notKeyRequest},
		{"\xef\x00\x00\x00", notKeyRequest},
		{"", notKeyRequest},
	} {
		if got := askedForKey([]byte(c.in)); got != c.want {
			t.Errorf("%q: %d, want %d", c.in, got, c.want)
		}
	}
}

// fakeConn is the little of a connection onTcpData touches before a codec.
type fakeConn struct {
	gnet.Conn
	in      []byte
	written bytes.Buffer
}

func (f *fakeConn) Peek(n int) ([]byte, error) {
	if n < 0 || n > len(f.in) {
		return f.in, nil
	}
	return f.in[:n], nil
}
func (f *fakeConn) Discard(n int) (int, error) { f.in = f.in[n:]; return n, nil }
func (f *fakeConn) Write(b []byte) (int, error) { return f.written.Write(b) }
func (f *fakeConn) String() string             { return "fake" }

func keyFile(t *testing.T, bits int) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "server_rsa.key")
	data := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, key
}

func servedKey(t *testing.T, answer []byte) *rsa.PublicKey {
	t.Helper()
	parts := bytes.SplitN(answer, []byte("\r\n\r\n"), 2)
	if len(parts) != 2 || !bytes.HasPrefix(parts[0], []byte("HTTP/1.1 200 OK")) {
		t.Fatalf("not a 200 with a body: %q", answer)
	}
	block, rest := pem.Decode(parts[1])
	if block == nil || block.Type != "RSA PUBLIC KEY" || len(bytes.TrimSpace(rest)) != 0 {
		t.Fatalf("not one RSA PUBLIC KEY block: %q", parts[1])
	}
	if len(block.Bytes) != 270 {
		t.Fatalf("the DER is %d bytes, not the 270 of a canonical RSA-2048 key", len(block.Bytes))
	}
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// A server of one's own hands out its own key, first in the list (#244).
func TestTheServersOwnKeyIsHandedOut(t *testing.T) {
	own, private := keyFile(t, 2048)
	h := mustNewHandshake([]config.RSAKey{{KeyFile: own}, {KeyFile: stockKey, KeyFingerprint: "12240908862933197005"}})
	if len(h.keyFingerprints) != 2 || uint64(h.keyFingerprints[0]) != fingerprintOf(&private.PublicKey) {
		t.Fatalf("its own key is not offered first: %v", h.keyFingerprints)
	}
	if got := servedKey(t, h.keyAnswer); got.N.Cmp(private.N) != 0 || got.E != 65537 {
		t.Fatal("GET /key hands out some other key")
	}

	s := &Server{handshake: h}
	conn := &fakeConn{in: []byte("GET /key HTTP/1.0\r\nHost: x\r\n\r\n")}
	if action := s.onTcpData(&connContext{}, conn); action != gnet.Close {
		t.Fatalf("a whole request: action %v, not Close", action)
	}
	if !bytes.Equal(conn.written.Bytes(), h.keyAnswer) {
		t.Fatalf("a whole request was answered with %q", conn.written.Bytes())
	}

	conn = &fakeConn{in: []byte("GET /key HTTP/1.1\r\nHost")}
	if action := s.onTcpData(&connContext{}, conn); action != gnet.None || conn.written.Len() != 0 {
		t.Fatalf("a request not yet whole: action %v, wrote %q", action, conn.written.Bytes())
	}

	conn = &fakeConn{in: []byte("GET / HTTP/1.1\r\n\r\n")}
	if action := s.onTcpData(&connContext{}, conn); action != gnet.Close || conn.written.Len() != 0 {
		t.Fatalf("another HTTP request: action %v, wrote %q", action, conn.written.Bytes())
	}
}

// The stock key's private half is public: offered to old apps, never handed out.
func TestTheStockKeyIsNeverHandedOut(t *testing.T) {
	h := mustNewHandshake([]config.RSAKey{{KeyFile: stockKey}})
	if len(h.keyFingerprints) != 1 || uint64(h.keyFingerprints[0]) != stockFingerprint {
		t.Fatalf("the stock key's fingerprint was not worked out: %v", h.keyFingerprints)
	}
	if !bytes.HasPrefix(h.keyAnswer, []byte("HTTP/1.1 404 Not Found")) {
		t.Fatalf("GET /key handed out the stock key: %q", h.keyAnswer)
	}
}

// An install from before #244 has no key of its own: it keeps working with
// the stock key, and says so.
func TestAMissingKeyOfItsOwnIsPassedOver(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "server_rsa.key")
	h := mustNewHandshake([]config.RSAKey{{KeyFile: missing}, {KeyFile: stockKey, KeyFingerprint: "12240908862933197005"}})
	if len(h.keyFingerprints) != 1 || uint64(h.keyFingerprints[0]) != stockFingerprint {
		t.Fatalf("offered %v, not the stock key alone", h.keyFingerprints)
	}
	if !bytes.HasPrefix(h.keyAnswer, []byte("HTTP/1.1 404 Not Found")) {
		t.Fatalf("GET /key answered %q", h.keyAnswer)
	}
}

func mustRefuse(t *testing.T, what string, keys []config.RSAKey) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s was taken", what)
		}
	}()
	mustNewHandshake(keys)
}

func TestKeysThatCannotServeAreRefused(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "ice9_server_rsa.key")
	mustRefuse(t, "a missing key the apps have built in", []config.RSAKey{{KeyFile: missing, KeyFingerprint: "9317290418914058974"}})
	short, _ := keyFile(t, 1024)
	mustRefuse(t, "a 1024-bit key", []config.RSAKey{{KeyFile: short}})
	mustRefuse(t, "a list with no key at all", []config.RSAKey{{KeyFile: missing}})
}
