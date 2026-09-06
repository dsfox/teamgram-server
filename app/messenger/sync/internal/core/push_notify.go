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

	peer, msgId := incomingMessagePeer(userId, ups)
	if peer != nil {
		c.svcCtx.Dao.Notifier.NewMessage(c.ctx, userId, peer.PeerType, peer.PeerId, msgId, onlineAuthKeyIds)
		return
	}

	// A read mark of this user's own, when it comes this way: the devices
	// that are asleep get the new badge, once the burst settles (#173).
	if readSomething(userId, ups) {
		c.svcCtx.Dao.Notifier.ReadElsewhere(c.ctx, userId, onlineAuthKeyIds)
	}
}

// readSomething reports whether these updates carry a read mark of the
// user's own - the inbox of a chat or a channel marked read.
func readSomething(userId int64, ups *mtproto.Updates) bool {
	read := false
	mtproto.VisitUpdates(userId, ups, map[string]mtproto.UpdateVisitedFunc{
		mtproto.Predicate_updateReadHistoryInbox: func(int64, *mtproto.Update, []*mtproto.User, []*mtproto.Chat, int32) {
			read = true
		},
		mtproto.Predicate_updateReadChannelInbox: func(int64, *mtproto.Update, []*mtproto.User, []*mtproto.Chat, int32) {
			read = true
		},
	})
	return read
}

// incomingMessagePeer returns the chat that received someone else's message and
// the message's id in this person's box, or nil when the batch of updates holds
// nothing worth notifying about. The id travels in the push envelope: the
// notification extension polls the difference and then reads the words of this
// one message by it (#42).
//
// Only incoming messages count: waking a person for read marks, edits and other
// housekeeping synchronisation is pointless.
func incomingMessagePeer(userId int64, ups *mtproto.Updates) (*mtproto.PeerUtil, int32) {
	var peer *mtproto.PeerUtil
	var msgId int32

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
				msgId = msg.GetId()
			}
		},
	})

	return peer, msgId
}
