package mls

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/mod/sumdb/tlog"
)

// The key log (#44): which signature key belongs to which device, in a log that
// can only grow.
//
// A key package is signed by its device's own key, and a phone adding the
// device checks that signature - but which key is whose it could only take on
// the server's word, so a server could hand out a package built on a key of its
// own under a real device's name and join the conversation. Every key the
// directory hands out is written here first. The tree is RFC 6962's, through
// golang.org/x/mod/sumdb/tlog, so a phone can check that a key is in the log
// (an inclusion proof) and that the log it sees now holds the one it saw
// before (a consistency proof). The owner's own devices read their account's
// entries, and a key none of them made is seen.
//
// The head is signed by the log's own Ed25519 key, made once and kept with the
// log. The key does not make the server honest - it holds it - but it makes
// what the server said provable: a head shown to one person can be shown to
// another, and two heads of one size with different roots are a proof the
// server lied.

// KeyRecord is one entry: a device's leaf name and the key it signs with.
type KeyRecord struct {
	Index        int64
	UserId       int64
	Name         []byte
	SignatureKey []byte
	Date         int32
}

// RecordBytes is what the tree hashes for an entry. The clients build the same
// bytes from what they asked about, so the user id is bound into the proof and
// a key cannot be moved from one person to another.
func RecordBytes(userId int64, name, signatureKey []byte) []byte {
	var out bytes.Buffer
	out.WriteString("ice9-key-log-1")
	_ = binary.Write(&out, binary.BigEndian, userId)
	_ = binary.Write(&out, binary.BigEndian, uint16(len(name)))
	out.Write(name)
	_ = binary.Write(&out, binary.BigEndian, uint16(len(signatureKey)))
	out.Write(signatureKey)
	return out.Bytes()
}

// HeadBytes is what the log key signs.
func HeadBytes(size int64, root tlog.Hash, date int32) []byte {
	var out bytes.Buffer
	out.WriteString("ice9-key-log-head-1")
	_ = binary.Write(&out, binary.BigEndian, size)
	out.Write(root[:])
	_ = binary.Write(&out, binary.BigEndian, date)
	return out.Bytes()
}

// KeyLogStore keeps the log. Append is the one write, and the store makes it
// atomic: the record, the hashes it completes and the new size go together, or
// a reader could be given a size whose hashes are not all there yet.
type KeyLogStore interface {
	// Append adds the record unless the same name and key are already logged,
	// and says whether it did. hashesFor gives the hashes to store for record
	// n, reading the earlier ones it needs through r.
	Append(ctx context.Context, r KeyRecord, hashesFor func(n int64, r tlog.HashReader) ([]tlog.Hash, error)) (bool, error)
	// Size is the number of records whose hashes are all stored.
	Size(ctx context.Context) (int64, error)
	ReadHashes(ctx context.Context, indexes []int64) ([]tlog.Hash, error)
	// Records lists a person's entries, oldest first.
	Records(ctx context.Context, userId int64) ([]KeyRecord, error)
	// LogKey is the seed of the log's signing key, made on first use.
	LogKey(ctx context.Context) ([]byte, error)
}

// KeyLog writes to the log and answers for it.
type KeyLog struct {
	store KeyLogStore
	// What is known to be logged already, so a device's every publish does not
	// ask the database again. The log only grows, so an entry here stays true.
	known sync.Map
}

func NewKeyLog(store KeyLogStore) *KeyLog {
	return &KeyLog{store: store}
}

// Witness logs the key of a package published by userId, if it is a key
// package naming one of that person's devices and the key is not logged yet.
//
// A package that names somebody else is not logged, and so no phone that
// checks the log will add it: a device naming itself after another person
// would be claiming their leaf.
func (l *KeyLog) Witness(ctx context.Context, userId int64, raw []byte, now int32) error {
	identity, signatureKey, err := ReadKeyPackage(raw)
	if err != nil {
		return err
	}
	if who, ok := userOf(identity); !ok || who != userId {
		return fmt.Errorf("a package of %d names %q, which is not one of theirs", userId, identity)
	}
	seen := string(identity) + "\x00" + string(signatureKey)
	if _, ok := l.known.Load(seen); ok {
		return nil
	}
	appended, err := l.store.Append(ctx, KeyRecord{
		UserId: userId, Name: identity, SignatureKey: signatureKey, Date: now,
	}, func(n int64, r tlog.HashReader) ([]tlog.Hash, error) {
		return tlog.StoredHashes(n, RecordBytes(userId, identity, signatureKey), r)
	})
	if err != nil {
		return fmt.Errorf("cannot log the key of %q: %w", identity, err)
	}
	if appended {
		logx.WithContext(ctx).Infof("mls: the key log now holds a key of %q", identity)
	}
	l.known.Store(seen, true)
	return nil
}

// Head is the log as it stands, signed.
type Head struct {
	Size      int64
	Root      tlog.Hash
	Date      int32
	LogKey    ed25519.PublicKey
	Signature []byte
}

// Answer is what mls.getKeyLog says about one person.
type Answer struct {
	Head        Head
	Entries     []KeyRecord
	Proofs      []tlog.RecordProof
	Consistency tlog.TreeProof
}

// Read answers for one person: the signed head, their entries with a proof
// each, and - when the caller says which size it saw last - the proof that
// the log it sees now holds that one.
func (l *KeyLog) Read(ctx context.Context, userId, knownSize int64, now int32) (*Answer, error) {
	seed, err := l.store.LogKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot read the log key: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, errors.New("the log key is not an Ed25519 seed")
	}
	signer := ed25519.NewKeyFromSeed(seed)

	// The size first, and everything after it for that size: records only
	// ever join the end, so a tree of this size is the same whatever is being
	// appended while this answer is made.
	size, err := l.store.Size(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot read the size of the log: %w", err)
	}
	reader := tlog.HashReaderFunc(func(indexes []int64) ([]tlog.Hash, error) {
		return l.store.ReadHashes(ctx, indexes)
	})
	root, err := tlog.TreeHash(size, reader)
	if err != nil {
		return nil, fmt.Errorf("cannot hash the tree of %d: %w", size, err)
	}

	answer := &Answer{Head: Head{
		Size:      size,
		Root:      root,
		Date:      now,
		LogKey:    signer.Public().(ed25519.PublicKey),
		Signature: ed25519.Sign(signer, HeadBytes(size, root, now)),
	}}

	records, err := l.store.Records(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("cannot read the entries of %d: %w", userId, err)
	}
	for _, r := range records {
		if r.Index >= size {
			continue // appended after the size was read; in the next answer
		}
		proof, err := tlog.ProveRecord(size, r.Index, reader)
		if err != nil {
			return nil, fmt.Errorf("cannot prove entry %d: %w", r.Index, err)
		}
		answer.Entries = append(answer.Entries, r)
		answer.Proofs = append(answer.Proofs, proof)
	}

	if knownSize > 0 && knownSize < size {
		answer.Consistency, err = tlog.ProveTree(size, knownSize, reader)
		if err != nil {
			return nil, fmt.Errorf("cannot prove the log of %d holds the one of %d: %w", size, knownSize, err)
		}
	}
	return answer, nil
}
