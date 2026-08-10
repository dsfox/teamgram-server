package mls

import (
	"context"
	"fmt"
)

// A welcome is what lets a device into a conversation somebody started with it.
//
// It waits on the server rather than travelling as a message, which keeps
// handshake traffic out of the message pipeline entirely - no client has to hide
// anything from a chat list, and nothing about a conversation starting is
// visible as a chat until there is something to show.
//
// It is kept until the receiving device says the conversation is open and
// saved. Delivered is not the same as read: a welcome lost between the two is a
// conversation that exists on one side and not the other, and that surfaces
// much later as messages which will not open, far from where it broke.

// Welcome is one waiting for a device.
type Welcome struct {
	Id        int64
	UserId    int64
	AuthKeyId int64
	FromId    int64
	Bytes     []byte
	Date      int32
}

// WelcomeStore is the part of storage this needs.
type WelcomeStore interface {
	Put(ctx context.Context, w Welcome) error
	Waiting(ctx context.Context, userId, authKeyId int64) ([]Welcome, error)
	Confirm(ctx context.Context, userId, authKeyId int64, ids []int64) (int, error)
	Devices(ctx context.Context, userId int64) ([]int64, error)
}

// MaxWaitingPerDevice bounds what can pile up for one device. Without it,
// anybody could fill the table by starting conversations nobody answers.
const MaxWaitingPerDevice = 500

// Post leaves a welcome for every device of a person, because a conversation is
// with a person and each of their devices is a member of it.
//
// A person with no devices published yet gets nothing and no error: they have
// simply not been seen by this yet, which is ordinary rather than broken.
func (d *Directory) Post(ctx context.Context, welcomes WelcomeStore, toUserId, fromId int64, welcome []byte, now int32) (int, error) {
	if len(welcome) == 0 {
		return 0, ErrEmptyPackage
	}

	devices, err := welcomes.Devices(ctx, toUserId)
	if err != nil {
		return 0, fmt.Errorf("cannot list the devices: %w", err)
	}

	posted := 0
	for _, authKeyId := range devices {
		waiting, err := welcomes.Waiting(ctx, toUserId, authKeyId)
		if err != nil {
			return posted, fmt.Errorf("cannot count what is waiting: %w", err)
		}
		if len(waiting) >= MaxWaitingPerDevice {
			// Silently skipping would look like the conversation started.
			return posted, ErrTooMany
		}

		if err = welcomes.Put(ctx, Welcome{
			UserId:    toUserId,
			AuthKeyId: authKeyId,
			FromId:    fromId,
			Bytes:     welcome,
			Date:      now,
		}); err != nil {
			return posted, fmt.Errorf("cannot leave the welcome: %w", err)
		}
		posted++
	}

	return posted, nil
}

// Waiting is what a device has not opened yet. Handed out repeatedly until
// confirmed: a device that read one and crashed before saving asks again.
func (d *Directory) Waiting(ctx context.Context, welcomes WelcomeStore, userId, authKeyId int64) ([]Welcome, error) {
	return welcomes.Waiting(ctx, userId, authKeyId)
}

// Confirm forgets welcomes a device says it has opened and saved.
//
// Only its own: a device confirming somebody else's would drop a conversation
// that person never joined, and they would never learn why.
func (d *Directory) Confirm(ctx context.Context, welcomes WelcomeStore, userId, authKeyId int64, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	return welcomes.Confirm(ctx, userId, authKeyId, ids)
}
