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

	// Апстрим отдавал -1, когда обновлений ещё не было. Для клиента это не
	// «пусто», а несуществующий номер: любое пришедшее обновление оказывается
	// за пределами известной последовательности, клиент видит пропуск и уходит
	// досинхронизироваться — и так по кругу. Снаружи это вечное «Updating» с
	// пустыми чатами. Проявляется только у тех, кому ещё ничего не приходило,
	// то есть ровно у новых пользователей.
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
