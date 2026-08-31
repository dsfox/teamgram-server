package mls

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"
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
	// Which chat the invitation is for, as a dialog id. Without it a welcome
	// says only who sent it, and the device joining files the conversation
	// under that person - so a group is recorded as the conversation with
	// whoever invited them (#115). Zero for one written before this existed.
	PeerId int64
	Bytes  []byte
	Date   int32
}

// WelcomeStore is the part of storage this needs.
type WelcomeStore interface {
	Put(ctx context.Context, w Welcome) error
	Waiting(ctx context.Context, userId, authKeyId int64) ([]Welcome, error)
	Confirm(ctx context.Context, userId, authKeyId int64, ids []int64) (int, error)
	Devices(ctx context.Context, userId int64) ([]int64, error)

	// ForgetOlderThan drops what has been waiting for this person since before
	// that moment, and says how many went. Not addressed to a device: the ones
	// this is for belong to devices that are gone, and a device that is gone
	// asks for nothing.
	ForgetOlderThan(ctx context.Context, userId int64, before int32) (int, error)
}

// MaxWaitingPerDevice bounds what can pile up for one device. Without it,
// anybody could fill the table by starting conversations nobody answers.
const MaxWaitingPerDevice = 500

// WelcomeLife is how long an invitation waits before it is taken to be waiting
// for nobody.
//
// A welcome is left for every device of a person and only one of them can open
// it, so most copies are dead weight from birth. A device that is running drops
// its share within seconds and says so; a device that is gone says nothing, and
// what it was left keeps its place for ever.
//
// The cost is not the rows, it is what a phone finds when it comes back after a
// long absence: a pile of invitations, each describing the group at an epoch it
// has long left. They are opened in order, and every one naming a later epoch
// than the copy the phone holds makes it let that copy go and join afresh - a
// new leaf and a ratchet at zero, while everybody else is still reading the
// leaf it used to speak from (#139).
//
// A fortnight, which is the line #138 already draws: a device that has not
// asked for one is not a device, so an invitation that has waited that long is
// waiting for nobody. Losing it is recoverable and keeping it is not - a phone
// that does come back publishes, is counted again, and is let into whatever it
// missed by the comparison that already runs.
const WelcomeLife int32 = 14 * 24 * 3600

// Post leaves a welcome for every device of a person, because a conversation is
// with a person and each of their devices is a member of it.
//
// A person with no devices published yet gets nothing and no error: they have
// simply not been seen by this yet, which is ordinary rather than broken.
func (d *Directory) Post(ctx context.Context, welcomes WelcomeStore, toUserId, fromId, peerId int64, welcome []byte, now int32) (int, error) {
	if len(welcome) == 0 {
		return 0, ErrEmptyPackage
	}

	// The stale ones first, so that what is counted against the bound below is
	// what somebody might still come for. Housekeeping, so a failure here is
	// logged and stepped over: it would otherwise cost this person the
	// invitation they are being sent.
	if forgotten, err := welcomes.ForgetOlderThan(ctx, toUserId, now-WelcomeLife); err != nil {
		logx.WithContext(ctx).Errorf(
			"mls: cannot forget the invitations nobody came for: %v", err)
	} else if forgotten > 0 {
		logx.WithContext(ctx).Infof(
			"mls: %d invitation(s) of %d had been waiting longer than anybody will come",
			forgotten, toUserId)
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
			PeerId:    peerId,
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
