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

// deleteHistoryByDate removes the conversation messages within a period.
//
// Upstream deleted nothing for this request yet answered with success: the user
// believed the conversation was wiped while it stayed in place. For a messenger
// that promises privacy this is the worst kind of bug — a silent one.
//
// Both bounds are inclusive; a zero bound means "unbounded on that side".
func (c *MessagesCore) deleteHistoryByDate(peer *mtproto.PeerUtil, minDate, maxDate int32, revoke bool) (*mtproto.Messages_AffectedHistory, error) {
	const pageSize = 100

	var (
		idList   []int32
		offsetId int32 = 0
	)

	// Walk the history page by page from new to old until we pass the lower bound.
	for {
		boxList, err := c.svcCtx.Dao.MessageClient.MessageGetHistoryMessages(c.ctx, &message.TLMessageGetHistoryMessages{
			UserId:   c.MD.UserId,
			PeerType: peer.PeerType,
			PeerId:   peer.PeerId,
			OffsetId: offsetId,
			Limit:    pageSize,
		})
		if err != nil {
			c.Logger.Errorf("messages.deleteHistory - reading history: %v", err)
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
				// history is sorted new to old: everything further back is older
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
			// no suitable message on this page: move on from the last one
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

	// Deleting by a list of ids is not tied to a peer: the messages are already
	// found. The neighbouring messages.deleteMessages calls it the same way.
	affected, err := c.svcCtx.Dao.MsgClient.MsgDeleteMessages(c.ctx, &msgpb.TLMsgDeleteMessages{
		UserId:    c.MD.UserId,
		AuthKeyId: c.MD.PermAuthKeyId,
		PeerType:  mtproto.PEER_EMPTY,
		PeerId:    0,
		Revoke:    revoke,
		Id:        idList,
	})
	if err != nil {
		c.Logger.Errorf("messages.deleteHistory - deleting: %v", err)
		return nil, err
	}

	return mtproto.MakeTLMessagesAffectedHistory(&mtproto.Messages_AffectedHistory{
		Pts:      affected.GetPts(),
		PtsCount: affected.GetPtsCount(),
		Offset:   0,
	}).To_Messages_AffectedHistory(), nil
}
