package mls

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"golang.org/x/mod/sumdb/tlog"
)

// A package a phone of the stand published (user 136922312), so the reader is
// held to what the clients really send, not to what this file thinks they send.
func realKeyPackage(t *testing.T) []byte {
	t.Helper()
	text, err := os.ReadFile("testdata/key_package.hex")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := hex.DecodeString(strings.TrimSpace(string(text)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAKeyPackageSaysWhoseItIsAndWhichKeySignsForIt(t *testing.T) {
	identity, key, err := ReadKeyPackage(realKeyPackage(t))
	if err != nil {
		t.Fatal(err)
	}
	if string(identity) != "136922312/3471655483" {
		t.Errorf("the identity read as %q", identity)
	}
	if len(key) != ed25519.PublicKeySize {
		t.Errorf("the signature key is %d bytes", len(key))
	}
	for _, junk := range [][]byte{[]byte("bob-one"), {}, realKeyPackage(t)[:40]} {
		if _, _, err := ReadKeyPackage(junk); err == nil {
			t.Errorf("%x read as a key package", junk)
		}
	}
}

// A package made the way a client makes one, with a key of the test's own -
// exactly what a server handing out a device of its own would build.
func makeKeyPackage(identity string, key []byte) []byte {
	vec := func(b []byte) []byte { return append([]byte{byte(len(b))}, b...) }
	out := []byte{0, 1, 0, 5, 0, 1, 0, 1}
	out = append(out, vec(bytes.Repeat([]byte{1}, 32))...)
	out = append(out, vec(bytes.Repeat([]byte{2}, 32))...)
	out = append(out, vec(key)...)
	out = append(out, 0, 1)
	return append(out, vec([]byte(identity))...)
}

func TestThePublishedKeysGoInTheLogOnceAndProveThemselves(t *testing.T) {
	ctx := context.Background()
	store := &memoryKeyLog{}
	l := NewKeyLog(store)

	if err := l.Witness(ctx, 136922312, realKeyPackage(t), 1); err != nil {
		t.Fatal(err)
	}
	// The same device publishing again is the same key, logged once.
	if err := NewKeyLog(store).Witness(ctx, 136922312, realKeyPackage(t), 2); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		key := bytes.Repeat([]byte{byte(10 + i)}, 32)
		if err := l.Witness(ctx, 7, makeKeyPackage("7/"+string(rune('a'+i)), key), 3); err != nil {
			t.Fatal(err)
		}
	}
	if size, _ := store.Size(ctx); size != 7 {
		t.Fatalf("the log holds %d keys, want 7", size)
	}

	first, err := l.Read(ctx, 136922312, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Entries) != 1 {
		t.Fatalf("%d entries for one device", len(first.Entries))
	}
	head := first.Head
	if !ed25519.Verify(head.LogKey, HeadBytes(head.Size, head.Root, head.Date), head.Signature) {
		t.Fatal("the head's signature does not hold")
	}
	e := first.Entries[0]
	leaf := tlog.RecordHash(RecordBytes(136922312, e.Name, e.SignatureKey))
	if err := tlog.CheckRecord(first.Proofs[0], head.Size, head.Root, e.Index, leaf); err != nil {
		t.Fatalf("the entry does not prove itself: %v", err)
	}
	// Bound to the person: the same key claimed for somebody else fails.
	other := tlog.RecordHash(RecordBytes(136922313, e.Name, e.SignatureKey))
	if tlog.CheckRecord(first.Proofs[0], head.Size, head.Root, e.Index, other) == nil {
		t.Fatal("a key proved itself for a person it does not belong to")
	}

	// The log grows, and the larger one proves it holds the smaller.
	if err := l.Witness(ctx, 7, makeKeyPackage("7/z", bytes.Repeat([]byte{99}, 32)), 4); err != nil {
		t.Fatal(err)
	}
	later, err := l.Read(ctx, 7, head.Size, 11)
	if err != nil {
		t.Fatal(err)
	}
	if later.Head.Size != 8 || len(later.Entries) != 7 {
		t.Fatalf("the log is %d with %d of 7's entries", later.Head.Size, len(later.Entries))
	}
	if err := tlog.CheckTree(later.Consistency, later.Head.Size, later.Head.Root, head.Size, head.Root); err != nil {
		t.Fatalf("the larger log does not prove it holds the smaller: %v", err)
	}
	if !bytes.Equal(later.Head.LogKey, head.LogKey) {
		t.Fatal("the log changed its key between two answers")
	}
}

// A device naming itself after somebody else would be claiming their leaf; a
// placeholder that is not a key package (a scenario's "bob-one") is stored by
// the directory as before and logged by nobody.
func TestOnlyAPersonsOwnRealKeysAreLogged(t *testing.T) {
	ctx := context.Background()
	store := &memoryKeyLog{}
	l := NewKeyLog(store)
	if err := l.Witness(ctx, 8, makeKeyPackage("7/a", bytes.Repeat([]byte{1}, 32)), 1); err == nil {
		t.Error("a package of 8 naming 7's device was logged")
	}
	if err := l.Witness(ctx, 8, []byte("bob-one"), 1); err == nil {
		t.Error("a placeholder was logged")
	}
	if size, _ := store.Size(ctx); size != 0 {
		t.Fatalf("the log holds %d", size)
	}
}

// memoryKeyLog behaves like the tables: appends in order, one per (name, key).
type memoryKeyLog struct {
	mu      sync.Mutex
	records []KeyRecord
	hashes  map[int64]tlog.Hash
	seed    []byte
}

func (m *memoryKeyLog) Append(_ context.Context, r KeyRecord, hashesFor func(int64, tlog.HashReader) ([]tlog.Hash, error)) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.records {
		if bytes.Equal(e.Name, r.Name) && bytes.Equal(e.SignatureKey, r.SignatureKey) {
			return false, nil
		}
	}
	n := int64(len(m.records))
	hashes, err := hashesFor(n, tlog.HashReaderFunc(m.read))
	if err != nil {
		return false, err
	}
	if m.hashes == nil {
		m.hashes = map[int64]tlog.Hash{}
	}
	for i, h := range hashes {
		m.hashes[tlog.StoredHashIndex(0, n)+int64(i)] = h
	}
	r.Index = n
	m.records = append(m.records, r)
	return true, nil
}

func (m *memoryKeyLog) read(indexes []int64) ([]tlog.Hash, error) {
	out := make([]tlog.Hash, len(indexes))
	for i, idx := range indexes {
		h, ok := m.hashes[idx]
		if !ok {
			return nil, os.ErrNotExist
		}
		out[i] = h
	}
	return out, nil
}

func (m *memoryKeyLog) Size(context.Context) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.records)), nil
}

func (m *memoryKeyLog) ReadHashes(_ context.Context, indexes []int64) ([]tlog.Hash, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.read(indexes)
}

func (m *memoryKeyLog) Records(_ context.Context, userId int64) ([]KeyRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []KeyRecord
	for _, r := range m.records {
		if r.UserId == userId {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out, nil
}

func (m *memoryKeyLog) LogKey(context.Context) ([]byte, error) {
	if m.seed == nil {
		m.seed = bytes.Repeat([]byte{42}, ed25519.SeedSize)
	}
	return m.seed, nil
}
