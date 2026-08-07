package core

import (
	"github.com/teamgram/proto/mtproto"
)

// notifyOfflineDevices sends a notification to the devices where the app is
// currently closed.
//
// onlineAuthKeyIds are the sessions the update already went to over the
// connection: there the person sees everything anyway.
func (c *SyncCore) notifyOfflineDevices(userId int64, onlineAuthKeyIds []int64, ups *mtproto.Updates) {
	if !c.svcCtx.Dao.Notifier.Enabled() {
		return
	}

	peer := incomingMessagePeer(userId, ups)
	if peer == nil {
		return
	}

	c.svcCtx.Dao.Notifier.NewMessage(c.ctx, userId, peer.PeerType, peer.PeerId, onlineAuthKeyIds)
}

// incomingMessagePeer returns the chat that received someone else's message, or
// nil when the batch of updates holds nothing worth notifying about.
//
// Only incoming messages count: waking a person for read marks, edits and other
// housekeeping synchronisation is pointless.
func incomingMessagePeer(userId int64, ups *mtproto.Updates) *mtproto.PeerUtil {
	var peer *mtproto.PeerUtil

	mtproto.VisitUpdates(userId, ups, map[string]mtproto.UpdateVisitedFunc{
		mtproto.Predicate_updateNewMessage: func(
			userId int64,
			update *mtproto.Update,
			users []*mtproto.User,
			chats []*mtproto.Chat,
			date int32,
		) {
			msg := update.GetMessage_MESSAGE()
			if msg == nil || msg.GetOut() {
				return
			}
			// Service messages ("you were added to a group") carry no text, yet
			// they are worth a notification — the person should learn about it.
			if p := mtproto.FromPeer(msg.GetPeerId()); p != nil {
				peer = p
			}
		},
	})

	return peer
}
