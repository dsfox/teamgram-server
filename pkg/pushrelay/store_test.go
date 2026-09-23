package pushrelay

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTheFileStoreKeepsHashesAndBlocks(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "relay.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	id, key, err := store.Register("1.2.3.4", "host:10443", now)
	if err != nil || len(id) != 16 || len(key) != 64 {
		t.Fatalf("registration: %q %q %v", id, key, err)
	}
	server, known, err := store.Lookup(KeyHash(key))
	if err != nil || !known || server.Id != id || server.Blocked {
		t.Fatalf("lookup: %+v %v %v", server, known, err)
	}
	if _, known, _ := store.Lookup(KeyHash("wrong")); known {
		t.Fatal("a wrong key was known")
	}
	if err := store.Block(id); err != nil {
		t.Fatal(err)
	}
	if server, _, _ := store.Lookup(KeyHash(key)); !server.Blocked {
		t.Fatal("blocking did not take")
	}
	if err := store.Block("nobody"); err == nil {
		t.Fatal("blocking a server that is not there succeeded")
	}
	for i := 0; i < RegistrationsPerDay-1; i++ {
		if _, _, err := store.Register("1.2.3.4", "x", now); err != nil {
			t.Fatalf("registration %d: %v", i+2, err)
		}
	}
	if _, _, err := store.Register("1.2.3.4", "x", now); err != ErrTooMany {
		t.Fatalf("the eleventh registration in a day: %v", err)
	}
	if _, _, err := store.Register("1.2.3.4", "x", now.Add(24*time.Hour)); err != nil {
		t.Fatalf("the next day: %v", err)
	}
}

// A day's count that cannot be read is not a count of zero. The limit is what
// keeps one address from minting servers without end, and it used to read a
// failed query as "nobody registered today" and let the registration through.
func TestAnUnreadableCountRefusesTheRegistration(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "relay.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	if _, err := store.db.Exec(`insert into registrations (ip, day, count) values (?, ?, ?)`,
		"1.2.3.4", day(now), "not a number"); err != nil {
		t.Fatal(err)
	}
	if id, _, err := store.Register("1.2.3.4", "x", now); err == nil {
		t.Fatalf("registered %q on a count nobody could read", id)
	}
}
