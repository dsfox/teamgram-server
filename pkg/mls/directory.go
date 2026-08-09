// Package mls keeps the directory of key packages (#38).
//
// A key package is what somebody needs in order to add a device to an encrypted
// conversation. Every device publishes a supply; whoever starts a conversation
// takes one per device of the other side. The server understands none of it - it
// stores opaque bytes, hands them out and counts what is left.
//
// Two rules decide almost everything here, and both are about what happens when
// they are broken:
//
//   - a package is used once. Handing the same one to two conversations costs
//     the forward secrecy of every message that follows, so taking removes.
//   - running out must not stop a conversation from starting. A device may
//     publish one package marked last resort, which is handed out when the
//     supply is dry. It is reused, so it is the weaker path, and a device that
//     keeps serving it has stopped publishing - which is worth noticing rather
//     than hiding.
//
// What this package cannot do is tell a real key package from one the server
// made up. That is not an oversight: MLS puts that trust in the Authentication
// Service, which here is us. Key transparency (#44) is what turns it from a
// promise into a property, and until it exists this directory is the weak point
// of the whole scheme.
package mls

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// KeyPackage is one published package as the directory holds it.
type KeyPackage struct {
	UserId      int64
	AuthKeyId   int64
	Bytes       []byte
	Fingerprint string
	LastResort  bool
	Date        int32
}

// Store is the part of storage this needs, named here so the directory can be
// tested against a map instead of a database.
type Store interface {
	// Insert adds a package. It reports false without an error when the same
	// bytes are already published for that device.
	Insert(ctx context.Context, p KeyPackage) (bool, error)

	// TakeOne removes and returns one ordinary package for the device, oldest
	// first. It returns nil when the supply is dry.
	TakeOne(ctx context.Context, userId, authKeyId int64) (*KeyPackage, error)

	// LastResort returns the device's last-resort package without removing it,
	// or nil when it published none.
	LastResort(ctx context.Context, userId, authKeyId int64) (*KeyPackage, error)

	// CountAvailable is how many ordinary packages the device has left.
	CountAvailable(ctx context.Context, userId, authKeyId int64) (int, error)

	// Devices lists the devices of a person that have published anything.
	Devices(ctx context.Context, userId int64) ([]int64, error)
}

// LowWaterMark is the count below which a device should publish more. It is
// answered to the device that asks, so the decision to refill stays with the
// only party that can make packages.
const LowWaterMark = 20

// MaxPerDevice bounds what one device may leave here. Without a bound, a
// client with a bug - or somebody with an account - fills the table.
const MaxPerDevice = 100

var (
	// ErrEmptyPackage is a package with nothing in it, which is a client fault
	// rather than an empty supply.
	ErrEmptyPackage = errors.New("the key package is empty")

	// ErrTooMany is the device holding as much as it may.
	ErrTooMany = errors.New("this device has published as many key packages as it may")
)

// Directory is the server's half of the key exchange.
type Directory struct {
	store Store
}

func New(store Store) *Directory {
	return &Directory{store: store}
}

// Fingerprint names a package by its bytes, so the same one cannot be published
// twice and handed to two conversations.
func Fingerprint(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

// Publish stores packages a device made. Duplicates are ignored rather than
// refused: a client that retries after a lost answer must not be punished for
// it. The count of what was actually added is returned so the client knows.
func (d *Directory) Publish(ctx context.Context, userId, authKeyId int64, packages [][]byte, lastResort []byte, now int32) (int, error) {
	available, err := d.store.CountAvailable(ctx, userId, authKeyId)
	if err != nil {
		return 0, fmt.Errorf("cannot count what is published: %w", err)
	}

	added := 0
	for _, bytes := range packages {
		if len(bytes) == 0 {
			return added, ErrEmptyPackage
		}
		if available+added >= MaxPerDevice {
			return added, ErrTooMany
		}

		inserted, err := d.store.Insert(ctx, KeyPackage{
			UserId:      userId,
			AuthKeyId:   authKeyId,
			Bytes:       bytes,
			Fingerprint: Fingerprint(bytes),
			Date:        now,
		})
		if err != nil {
			return added, fmt.Errorf("cannot store a key package: %w", err)
		}
		if inserted {
			added++
		}
	}

	if len(lastResort) > 0 {
		if _, err = d.store.Insert(ctx, KeyPackage{
			UserId:      userId,
			AuthKeyId:   authKeyId,
			Bytes:       lastResort,
			Fingerprint: Fingerprint(lastResort),
			LastResort:  true,
			Date:        now,
		}); err != nil {
			return added, fmt.Errorf("cannot store the last-resort package: %w", err)
		}
	}

	return added, nil
}

// Available is how many ordinary packages a device has left, and whether it
// should make more.
func (d *Directory) Available(ctx context.Context, userId, authKeyId int64) (count int, shouldRefill bool, err error) {
	count, err = d.store.CountAvailable(ctx, userId, authKeyId)
	if err != nil {
		return 0, false, fmt.Errorf("cannot count what is published: %w", err)
	}
	return count, count < LowWaterMark, nil
}

// Claim takes one package for every device of a person - which is what starting
// a conversation with them needs, since each device is a member of its own.
//
// A device with nothing left contributes its last-resort package if it has one,
// and is skipped otherwise: one silent device must not stop a conversation from
// starting with the rest.
func (d *Directory) Claim(ctx context.Context, userId int64) ([]KeyPackage, error) {
	devices, err := d.store.Devices(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("cannot list the devices: %w", err)
	}

	claimed := make([]KeyPackage, 0, len(devices))
	for _, authKeyId := range devices {
		p, err := d.store.TakeOne(ctx, userId, authKeyId)
		if err != nil {
			return nil, fmt.Errorf("cannot take a key package: %w", err)
		}
		if p == nil {
			if p, err = d.store.LastResort(ctx, userId, authKeyId); err != nil {
				return nil, fmt.Errorf("cannot read the last-resort package: %w", err)
			}
		}
		if p != nil {
			claimed = append(claimed, *p)
		}
	}

	return claimed, nil
}
