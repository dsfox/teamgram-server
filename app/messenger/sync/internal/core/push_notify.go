package core

import (
	"github.com/teamgram/proto/mtproto"
)

// notifyOfflineDevices отправляет уведомление на устройства, где приложение
// сейчас закрыто.
//
// onlineAuthKeyIds — сессии, которым обновление уже ушло по соединению: там
// человек и так всё видит.
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

// incomingMessagePeer — чат, в который пришло чужое сообщение, или nil, если
// в пачке обновлений нет ничего, о чём стоит уведомлять.
//
// Уведомляем только о входящих сообщениях: об отметках о прочтении, правках и
// прочей служебной синхронизации человека будить незачем.
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
			// Служебные сообщения («вы добавлены в группу») не несут текста,
			// но о них уведомить стоит — человек должен узнать, что его добавили.
			if p := mtproto.FromPeer(msg.GetPeerId()); p != nil {
				peer = p
			}
		},
	})

	return peer
}
