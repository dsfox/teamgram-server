/*
 * Created from 'scheme.tl' by 'mtprotoc'
 *
 * Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
 *  All rights reserved.
 *
 * Author: teamgramio (teamgram.io@gmail.com)
 */

package core

import (
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/service/biz/updates/updates"
)

// UpdatesGetStateV2
// updates.getStateV2 auth_key_id:long user_id:long = updates.State;
func (c *UpdatesCore) UpdatesGetStateV2(in *updates.TLUpdatesGetStateV2) (*mtproto.Updates_State, error) {
	pts := c.svcCtx.Dao.IDGenClient2.CurrentPtsId(c.ctx, in.UserId)
	if pts == 0 {
		pts = c.svcCtx.Dao.IDGenClient2.NextPtsId(c.ctx, in.UserId)
	}

	// Upstream returned -1 when no updates had happened yet. To the client that
	// is not "empty" but a non-existent number: every arriving update falls
	// outside the known sequence, the client sees a gap and goes resynchronising
	// — round and round. From outside it looks like an endless "Updating" with
	// empty chats. It only hits those who have received nothing so far, that is,
	// exactly the new users.
	seq := c.svcCtx.Dao.IDGenClient2.CurrentSeqId(c.ctx, in.AuthKeyId)
	qts := c.svcCtx.Dao.IDGenClient2.CurrentQtsId(c.ctx, in.AuthKeyId)
	return mtproto.MakeTLUpdatesState(&mtproto.Updates_State{
		Pts:         pts,
		Qts:         qts,
		Seq:         seq,
		Date:        int32(time.Now().Unix()), // TODO(@benqi): do.Date2???
		UnreadCount: 0,
	}).To_Updates_State(), nil
}
