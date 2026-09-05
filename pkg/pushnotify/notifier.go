// Package pushnotify decides which of a person's devices should be told about a
// new message, and tells them.
//
// A notification goes only to devices where the app is not open right now: if
// the person is reading the conversation, the message arrives over the
// connection anyway.
package pushnotify

import (
	"context"
	"errors"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/devices"
	"github.com/teamgram/teamgram-server/pkg/pushrelay"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
)

// Notifier sends notifications about new messages.
//
// It is always created, and each platform switches on when its own credentials
// is: a relay that holds the Apple and Google keys (#167), reached with the
// key it gave this server. Without one the server stays fully functional and
// nothing else notices.
type Notifier struct {
	registry deviceRegistry
	db       *sqlx.DB
	title    string
	body     string
	relayURL string
	// The relay's client once this server has a key: set at start from the
	// database, or by the registration that runs until the relay answers.
	relay atomic.Pointer[pushrelay.Client]
}

// deviceRegistry is what the notifier needs of pkg/devices, so that a test
// can hand it a fake.
type deviceRegistry interface {
	ListByUser(ctx context.Context, userId int64) ([]devices.DeviceDO, error)
	Forget(ctx context.Context, tokenType int32, token string) error
}

// New builds the sender from the environment settings: the relay's URL is
// the whole of it. The words are still this server's, sealed into the
// envelope; the relay carries its own for the banner.
func New(db *sqlx.DB) *Notifier {
	n := &Notifier{
		registry: devices.NewRegistry(db),
		db:       db,
		title:    envOr("APNS_TITLE", "ice9"),
		// The message text never reaches the notification and cannot: showing it
		// in the banner would mean handing the text to Apple, or to Google. This
		// is only a hint that something arrived.
		body: envOr("APNS_BODY", "New message"),
	}

	return n.withRelay()
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}

	return fallback
}

// Enabled reports whether anything can be sent at all: a relay, and a key
// from it.
func (n *Notifier) Enabled() bool {
	return n != nil && n.relay.Load() != nil
}

// NewMessage notifies the recipient about a new message.
//
// onlineAuthKeyIds are the session keys the message already went to over the
// connection; their devices are skipped. peerType/peerId identify the chat the
// message arrived in and are used to check whether it is muted.
//
// Sending happens aside from message delivery: Apple answers in hundreds of
// milliseconds, and up to a minute when the network is down. Waiting for that
// before showing the message on the other devices is not acceptable.
func (n *Notifier) NewMessage(ctx context.Context, userId int64, peerType int32, peerId int64, msgId int32, onlineAuthKeyIds []int64) {
	if !n.Enabled() {
		return
	}

	threading.GoSafe(func() {
		// A context of our own: the original one is cancelled as soon as the
		// message is delivered.
		sendCtx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()
		n.notify(sendCtx, userId, peerType, peerId, msgId, onlineAuthKeyIds)
	})
}

// sendTimeout is the total budget for notifying one person, database queries and
// all of their devices included.
const sendTimeout = 30 * time.Second

func (n *Notifier) notify(ctx context.Context, userId int64, peerType int32, peerId int64, msgId int32, onlineAuthKeyIds []int64) {
	if n.muted(ctx, userId, peerType, peerId) {
		return
	}

	list, err := n.registry.ListByUser(ctx, userId)
	if err != nil {
		return
	}

	targets := offlineTargets(list, onlineAuthKeyIds)
	if len(targets) == 0 {
		return
	}

	badge := n.unreadCount(ctx, userId)
	// Who it is from, in the shape the client reads: a decimal string under
	// `from_id`. Only for a conversation between two people - a group has its
	// own key on the other side, and getting that wrong opens the wrong chat.
	fromId := ""
	if peerType == int32(mtproto.PEER_USER) {
		fromId = strconv.FormatInt(peerId, 10)
	}
	for _, d := range targets {
		n.send(ctx, d, badge, fromId, peerType, peerId, msgId)
	}
}

// offlineTargets returns the devices that need a notification: those where the
// app is closed. A device with a live session already got the message over the
// connection, and a notification about what the person sees on screen is merely
// annoying.
func offlineTargets(list []devices.DeviceDO, onlineAuthKeyIds []int64) []devices.DeviceDO {
	online := make(map[int64]bool, len(onlineAuthKeyIds))
	for _, id := range onlineAuthKeyIds {
		online[id] = true
	}

	targets := make([]devices.DeviceDO, 0, len(list))
	for _, d := range list {
		if d.Reachable() && !online[d.AuthKeyId] {
			targets = append(targets, d)
		}
	}

	return targets
}

func (n *Notifier) send(ctx context.Context, d devices.DeviceDO, badge int, fromId string, peerType int32, peerId int64, msgId int32) {
	var err error

	// Said before the attempt, whatever Apple or Google then answer: whether
	// the phone was handed something its extension can open, or only the
	// words "New message" (#42). The scenarios read this line.
	carrying := "alert only"
	if d.Secret != "" {
		carrying = "envelope"
	}
	logx.WithContext(ctx).Infof("notification for user %d, device %d: %s", d.UserId, d.AuthKeyId, carrying)

	relay := n.relay.Load()
	if relay == nil {
		return
	}

	// Sealed here, with the secret the device registered, so that what goes
	// on from here - to the relay, and through it to Apple or Google - carries
	// no key and no words of ours. An envelope that cannot be sealed does not
	// stop the push: the alert alone still says a message came.
	push := pushrelay.Push{Token: d.Token, Sandbox: d.AppSandbox, Badge: badge, FromId: fromId}
	switch {
	case d.IsAPNs():
		push.Platform = pushrelay.PlatformApple
		if d.Secret != "" {
			push.P, _ = pushrelay.SealForApple(d.Secret, n.title, n.body, badge, peerType, peerId, msgId)
		}
	case d.IsFCM():
		push.Platform = pushrelay.PlatformGoogle
		if d.Secret != "" {
			push.P, _ = pushrelay.SealForGoogle(d.Secret, badge, fromId)
		}
	default:
		// A device of a kind we cannot reach. Not an error, and not worth a
		// line per message.
		return
	}
	err = relay.Send(ctx, push)

	switch {
	case err == nil:
		logx.WithContext(ctx).Infof("notification sent: user %d, device %d", d.UserId, d.AuthKeyId)
	case errors.Is(err, pushrelay.ErrTokenGone):
		logx.WithContext(ctx).Infof("token is gone, forgetting it: user %d, device %d", d.UserId, d.AuthKeyId)
		_ = n.registry.Forget(ctx, d.TokenType, d.Token)
	default:
		logx.WithContext(ctx).Errorf("notification not sent: user %d - %v", d.UserId, err)
	}
}

// muted reports whether the recipient muted this chat. Mute settings belong to
// the biz layer, but a network call per message is expensive and the table is
// simple.
func (n *Notifier) muted(ctx context.Context, userId int64, peerType int32, peerId int64) bool {
	var muteUntil int32
	query := "select mute_until from user_notify_settings where user_id = ? and peer_type = ? and peer_id = ? and deleted = 0"

	if err := n.db.QueryRow(ctx, &muteUntil, query, userId, peerType, peerId); err != nil {
		// No row means the chat is not muted, which is the common case.
		return false
	}

	// -1 means "not configured", 0 means "sound on", a large value means muted
	// for a long time.
	return int64(muteUntil) > time.Now().Unix()
}

// unreadCount is the total number of unread messages. iOS puts this number on
// the icon; nobody else can compute it since we ship no app extension.
func (n *Notifier) unreadCount(ctx context.Context, userId int64) int {
	var total int64
	query := "select coalesce(sum(unread_count), 0) from dialogs where user_id = ? and deleted = 0"

	if err := n.db.QueryRow(ctx, &total, query, userId); err != nil {
		logx.WithContext(ctx).Errorf("cannot count unread messages for %d: %v", userId, err)
		// A negative value asks iOS to leave the badge alone: better keep it as
		// is than show a wrong number.
		return -1
	}

	return int(total)
}
