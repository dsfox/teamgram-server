// Copyright 2022 Teamgram Authors
//  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author: teamgramio (teamgram.io@gmail.com)
//

package core

import (
	"github.com/teamgram/proto/mtproto"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"
	"github.com/teamgram/teamgram-server/app/service/biz/dialog/dialog"
	"github.com/teamgram/teamgram-server/app/service/biz/message/message"
)

// MessagesDeleteHistory
// messages.deleteHistory#b08f922a flags:# just_clear:flags.0?true revoke:flags.1?true peer:InputPeer max_id:int min_date:flags.2?int max_date:flags.3?int = messages.AffectedHistory;
func (c *MessagesCore) MessagesDeleteHistory(in *mtproto.TLMessagesDeleteHistory) (*mtproto.Messages_AffectedHistory, error) {
	var (
		peer = mtproto.FromInputPeer2(c.MD.UserId, in.Peer)
	)

	if peer.IsChannel() {
		c.Logger.Errorf("messages.deleteHistory blocked, License key from https://teamgram.net required to unlock enterprise features.")
		return nil, mtproto.ErrEnterpriseIsBlocked
	}

	if !peer.IsChatOrUser() {
		err := mtproto.ErrPeerIdInvalid
		c.Logger.Errorf("messages.deleteHistory - error: %v", err)
		return nil, err
	}

	if in.GetMinDate() != nil || in.GetMaxDate() != nil {
		return c.deleteHistoryByDate(peer, in.GetMinDate().GetValue(), in.GetMaxDate().GetValue(), in.Revoke)
	}

	affectedHistory, err := c.svcCtx.Dao.MsgClient.MsgDeleteHistory(c.ctx, &msgpb.TLMsgDeleteHistory{
		UserId:    c.MD.UserId,
		AuthKeyId: c.MD.PermAuthKeyId,
		PeerType:  peer.PeerType,
		PeerId:    peer.PeerId,
		JustClear: in.GetJustClear(),
		Revoke:    in.Revoke,
		MaxId:     in.MaxId,
	})

	if err != nil {
		c.Logger.Errorf("messages.deleteHistory - error: %v", err)
		return nil, err
	}

	if !in.GetJustClear() {
		if peer.IsUser() {
			c.svcCtx.Dao.DialogDeleteDialog(c.ctx, &dialog.TLDialogDeleteDialog{
				UserId:   c.MD.UserId,
				PeerType: peer.PeerType,
				PeerId:   peer.PeerId,
			})
			if in.Revoke && !peer.IsSelf() {
				c.svcCtx.Dao.DialogDeleteDialog(c.ctx, &dialog.TLDialogDeleteDialog{
					UserId:   peer.PeerId,
					PeerType: peer.PeerType,
					PeerId:   c.MD.UserId,
				})
			}
		}
	}

	return affectedHistory, nil
}

// deleteHistoryByDate удаляет сообщения переписки за указанный период.
//
// Апстрим на этот запрос ничего не удалял, но отвечал успехом: пользователь
// считал переписку стёртой, а она оставалась на месте. Для мессенджера,
// который обещает приватность, это худший вид ошибки — молчаливый.
//
// Границы включительные; нулевая граница означает «без ограничения с этой стороны».
func (c *MessagesCore) deleteHistoryByDate(peer *mtproto.PeerUtil, minDate, maxDate int32, revoke bool) (*mtproto.Messages_AffectedHistory, error) {
	const pageSize = 100

	var (
		idList   []int32
		offsetId int32 = 0
	)

	// Идём по истории страницами от новых к старым, пока не выйдем за нижнюю границу.
	for {
		boxList, err := c.svcCtx.Dao.MessageClient.MessageGetHistoryMessages(c.ctx, &message.TLMessageGetHistoryMessages{
			UserId:   c.MD.UserId,
			PeerType: peer.PeerType,
			PeerId:   peer.PeerId,
			OffsetId: offsetId,
			Limit:    pageSize,
		})
		if err != nil {
			c.Logger.Errorf("messages.deleteHistory - чтение истории: %v", err)
			return nil, err
		}

		var (
			messages   []*mtproto.Message
			outOfRange bool
		)
		boxList.Visit(c.MD.UserId,
			func(messageList []*mtproto.Message) { messages = messageList },
			func(userIdList []int64) {},
			func(chatIdList []int64) {},
			func(channelIdList []int64) {})

		if len(messages) == 0 {
			break
		}

		for _, msg := range messages {
			date := msg.GetDate()
			if minDate > 0 && date < minDate {
				// история отсортирована от новых к старым: дальше только старее
				outOfRange = true
				break
			}
			if maxDate > 0 && date > maxDate {
				continue
			}
			idList = append(idList, msg.GetId())
			offsetId = msg.GetId()
		}

		if outOfRange || len(messages) < pageSize {
			break
		}
		if offsetId == 0 {
			// в странице не нашлось ни одного подходящего сообщения — сдвигаемся по последнему
			offsetId = messages[len(messages)-1].GetId()
		}
	}

	if len(idList) == 0 {
		return mtproto.MakeTLMessagesAffectedHistory(&mtproto.Messages_AffectedHistory{
			Pts:      c.svcCtx.Dao.IDGenClient2.CurrentPtsId(c.ctx, c.MD.UserId),
			PtsCount: 0,
			Offset:   0,
		}).To_Messages_AffectedHistory(), nil
	}

	// Удаление по списку идентификаторов не привязано к собеседнику: сообщения
	// уже найдены. Соседний messages.deleteMessages вызывает его так же.
	affected, err := c.svcCtx.Dao.MsgClient.MsgDeleteMessages(c.ctx, &msgpb.TLMsgDeleteMessages{
		UserId:    c.MD.UserId,
		AuthKeyId: c.MD.PermAuthKeyId,
		PeerType:  mtproto.PEER_EMPTY,
		PeerId:    0,
		Revoke:    revoke,
		Id:        idList,
	})
	if err != nil {
		c.Logger.Errorf("messages.deleteHistory - удаление: %v", err)
		return nil, err
	}

	return mtproto.MakeTLMessagesAffectedHistory(&mtproto.Messages_AffectedHistory{
		Pts:      affected.GetPts(),
		PtsCount: affected.GetPtsCount(),
		Offset:   0,
	}).To_Messages_AffectedHistory(), nil
}
