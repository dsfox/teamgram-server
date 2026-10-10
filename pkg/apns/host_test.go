package apns

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/sideshow/apns2"
)

func keyFile(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Where Apple is can be said, and unsaid is Apple's own hosts (#188): the
// stand puts its front there, and nothing else ever does.
func TestWhereAppleIs(t *testing.T) {
	base := Config{KeyPath: keyFile(t), KeyId: "KEY1234567", TeamId: "TEAM123456", Topic: "app.twobytes.ios"}

	sender, err := New(base)
	if err != nil {
		t.Fatal(err)
	}
	if sender.sandbox.Host != apns2.HostDevelopment || sender.production.Host != apns2.HostProduction {
		t.Fatalf("with nothing said, Apple is not where Apple is: %q, %q", sender.sandbox.Host, sender.production.Host)
	}

	told := base
	told.SandboxHost, told.ProductionHost = "http://standapple:8401", "http://standapple:8402"
	sender, err = New(told)
	if err != nil {
		t.Fatal(err)
	}
	if sender.sandbox.Host != told.SandboxHost || sender.production.Host != told.ProductionHost {
		t.Fatalf("told where Apple is, the sender went elsewhere: %q, %q", sender.sandbox.Host, sender.production.Host)
	}
}
