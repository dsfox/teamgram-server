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
	"github.com/teamgram/teamgram-server/app/service/biz/message/message"
)

// MessagesReadMentions
// messages.readMentions#f0189d3 flags:# peer:InputPeer top_msg_id:flags.0?int = messages.AffectedHistory;
//
// Отмечает упоминания в переписке прочитанными. Апстрим отвечал ошибкой, и значок
// упоминаний в группе не сбрасывался никогда — висел, даже когда всё прочитано.
// Отметка выполняется тем же путём, что и для остальных непрочитанных вложений.
func (c *MessagesCore) MessagesReadMentions(in *mtproto.TLMessagesReadMentions) (*mtproto.Messages_AffectedHistory, error) {
	peer := mtproto.FromInputPeer2(c.MD.UserId, in.GetPeer())
	if !peer.IsChatOrUser() {
		err := mtproto.ErrPeerIdInvalid
		c.Logger.Errorf("messages.readMentions - error: %v", err)
		return nil, err
	}

	const pageSize = 100

	boxList, err := c.svcCtx.Dao.MessageClient.MessageGetUnreadMentions(c.ctx, &message.TLMessageGetUnreadMentions{
		UserId:   c.MD.UserId,
		PeerType: peer.PeerType,
		PeerId:   peer.PeerId,
		Limit:    pageSize,
	})
	if err != nil {
		c.Logger.Errorf("messages.readMentions - error: %v", err)
		return nil, err
	}

	contents := make([]*msgpb.ContentMessage, 0, boxList.Length())
	for _, box := range boxList.GetDatas() {
		contents = append(contents, &msgpb.ContentMessage{
			Id:              box.MessageId,
			SendUserId:      box.SenderUserId,
			DialogMessageId: box.DialogMessageId,
			Mentioned:       true,
		})
	}

	if len(contents) == 0 {
		return mtproto.MakeTLMessagesAffectedHistory(&mtproto.Messages_AffectedHistory{
			Pts:      c.svcCtx.Dao.IDGenClient2.CurrentPtsId(c.ctx, c.MD.UserId),
			PtsCount: 0,
			Offset:   0,
		}).To_Messages_AffectedHistory(), nil
	}

	affected, err := c.svcCtx.Dao.MsgClient.MsgReadMessageContents(c.ctx, &msgpb.TLMsgReadMessageContents{
		UserId:    c.MD.UserId,
		AuthKeyId: c.MD.PermAuthKeyId,
		PeerType:  peer.PeerType,
		PeerId:    peer.PeerId,
		Id:        contents,
	})
	if err != nil {
		c.Logger.Errorf("messages.readMentions - error: %v", err)
		return nil, err
	}

	return mtproto.MakeTLMessagesAffectedHistory(&mtproto.Messages_AffectedHistory{
		Pts:      affected.GetPts(),
		PtsCount: affected.GetPtsCount(),
		// Ненулевое смещение просит клиента повторить вызов: за один заход
		// отмечаем страницу, остальное он дочитает следующим запросом.
		Offset: 0,
	}).To_Messages_AffectedHistory(), nil
}
