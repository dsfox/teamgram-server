package langpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A client that already has the current version must be told there is nothing
// new. Answering it with the whole pack does not move its version, so it asks
// again - four times a second, for as long as the app is open, with everything
// else it wants to do queued behind eleven thousand strings. That is what a
// message taking a minute to appear looked like from the inside.
func TestDifferenceIsEmptyForACurrentClient(t *testing.T) {
	version := writeTestPack(t)

	current := Difference("ru", "ios", version)
	if n := len(current.GetStrings()); n != 0 {
		t.Errorf("a client on the current version was sent %d strings, expected none", n)
	}
	if current.GetVersion() != version {
		t.Errorf("version %d, expected %d", current.GetVersion(), version)
	}

	ahead := Difference("ru", "ios", version+1)
	if n := len(ahead.GetStrings()); n != 0 {
		t.Errorf("a client ahead of us was sent %d strings, expected none", n)
	}
}

// A client that is behind gets everything: we keep no history, and the whole
// pack is a correct if generous answer.
func TestDifferenceCarriesThePackForAnOldClient(t *testing.T) {
	version := writeTestPack(t)

	behind := Difference("ru", "ios", version-1)
	if len(behind.GetStrings()) == 0 {
		t.Fatal("a client behind the current version was sent nothing")
	}
	if behind.GetFromVersion() != version-1 {
		t.Errorf("from_version %d, expected %d - the client is told what it was "+
			"brought forward from", behind.GetFromVersion(), version-1)
	}

	fresh := Difference("ru", "ios", 0)
	if len(fresh.GetStrings()) == 0 {
		t.Fatal("a client with no pack at all was sent nothing")
	}
}

// writeTestPack puts a small pack where the loader will find it and returns its
// version. Kept tiny on purpose: the point is the version arithmetic, not size.
func writeTestPack(t *testing.T) int32 {
	t.Helper()

	dir := t.TempDir()
	const version = int32(42)
	body, err := json.Marshal(map[string]any{
		"version": version,
		"strings": map[string]string{"Common.Yes": "Да", "Common.No": "Нет"},
		"plurals": map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "ru.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}

	// The loader reads once per process, so the directory has to be in place
	// before anything else in this package touches it.
	Dir = dir
	resetForTest()

	return version
}
