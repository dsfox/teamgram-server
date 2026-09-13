package pushnotify

import (
	"context"
	"errors"

	"github.com/teamgram/teamgram-server/pkg/devices"
	"github.com/teamgram/teamgram-server/pkg/pushrelay"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
)

// IncomingCall rings the callee's phones that a push can reach (#14): the
// iPhone through its VoIP token, with the call's own update sealed inside so
// the app can report it to CallKit and pick it up without asking; the Android
// through its one token, with a wake-up - it reconnects and rings on the
// update it then receives. Every device, awake or not: the phones drop a call
// they already know by its id, and a ring that never comes is the failure.
//
// updates is the TL Updates container of the call, encoded at the layer the
// iPhone speaks, built by the caller who has the call and the caller's User.
//
// Aside from the request, like a message: Apple answers in hundreds of
// milliseconds, and the caller's phone is waiting on the reply.
func (n *Notifier) IncomingCall(ctx context.Context, calleeId, callerId, callId int64, updates []byte) {
	if !n.Enabled() {
		return
	}
	threading.GoSafe(func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()
		n.ringDevices(sendCtx, calleeId, callerId, callId, updates)
	})
}

func (n *Notifier) ringDevices(ctx context.Context, calleeId, callerId, callId int64, updates []byte) {
	list, err := n.registry.ListByUser(ctx, calleeId)
	if err != nil {
		logx.WithContext(ctx).Errorf("call push: cannot list devices of %d: %v", calleeId, err)
		return
	}
	for _, d := range list {
		n.ringDevice(ctx, d, calleeId, callerId, callId, updates)
	}
}

func (n *Notifier) ringDevice(ctx context.Context, d devices.DeviceDO, calleeId, callerId, callId int64, updates []byte) {
	push := pushrelay.Push{Token: d.Token, Sandbox: d.AppSandbox, Call: true}
	var err error
	switch {
	case d.IsVoIP():
		push.Platform = pushrelay.PlatformApple
		push.P, err = pushrelay.SealForAppleCall(d.Secret, updates)
	case d.IsFCM():
		push.Platform = pushrelay.PlatformGoogle
		push.P, err = pushrelay.SealForGoogleCall(d.Secret, calleeId, callerId, callId)
	default:
		// The iPhone's message token among them: a banner is not a ring, and
		// the VoIP token is the one that rings.
		return
	}
	if err != nil {
		// Without a secret there is nothing the phone could open, and a VoIP
		// push with nothing inside gets the app killed. Said, not sent.
		logx.WithContext(ctx).Errorf("call push for user %d, device %d: cannot seal, not sent: %v", d.UserId, d.AuthKeyId, err)
		return
	}
	logx.WithContext(ctx).Infof("call push for user %d, device %d: %s, call %d", d.UserId, d.AuthKeyId, push.Platform, callId)

	relay := n.relay.Load()
	if relay == nil {
		return
	}
	switch err = relay.Send(ctx, push); {
	case err == nil:
		logx.WithContext(ctx).Infof("call push sent: user %d, device %d", d.UserId, d.AuthKeyId)
	case errors.Is(err, pushrelay.ErrTokenGone):
		logx.WithContext(ctx).Infof("token is gone, forgetting it: user %d, device %d", d.UserId, d.AuthKeyId)
		_ = n.registry.Forget(ctx, d.TokenType, d.Token)
	default:
		logx.WithContext(ctx).Errorf("call push not sent: user %d - %v", d.UserId, err)
	}
}
