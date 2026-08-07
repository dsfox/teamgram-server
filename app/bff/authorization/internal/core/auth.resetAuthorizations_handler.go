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
	"github.com/teamgram/teamgram-server/app/messenger/sync/sync"
	"github.com/teamgram/teamgram-server/app/service/authsession/authsession"
)

// AuthResetAuthorizations
// auth.resetAuthorizations#9fab0d1a = Bool;
// Terminates every session except the current one. Upstream answered with an
// error, so the "Terminate all other sessions" button raised an alert and the
// only way out was one by one. A zero hash means "all but the given key" to the
// service.
func (c *AuthorizationCore) AuthResetAuthorizations(in *mtproto.TLAuthResetAuthorizations) (*mtproto.Bool, error) {
	keyIdList, err := c.svcCtx.AuthsessionClient.AuthsessionResetAuthorization(c.ctx, &authsession.TLAuthsessionResetAuthorization{
		UserId:    c.MD.UserId,
		AuthKeyId: c.MD.PermAuthKeyId,
		Hash:      0,
	})
	if err != nil {
		c.Logger.Errorf("auth.resetAuthorizations - error: %v", err)
		return nil, err
	}

	// Tell every terminated session it is no longer valid, otherwise the device
	// keeps working until its next reconnect. Same order as in
	// account.resetAuthorization for a single session.
	for _, keyId := range keyIdList.GetDatas() {
		c.svcCtx.SyncClient.SyncUpdatesMe(c.ctx, &sync.TLSyncUpdatesMe{
			UserId:        c.MD.UserId,
			PermAuthKeyId: keyId,
			Updates:       mtproto.MakeTLUpdatesTooLong(nil).To_Updates(),
		})
		c.svcCtx.SyncClient.SyncUpdatesMe(c.ctx, &sync.TLSyncUpdatesMe{
			UserId:        c.MD.UserId,
			PermAuthKeyId: keyId,
			Updates: mtproto.MakeTLUpdateAccountResetAuthorization(&mtproto.Updates{
				UserId:    c.MD.UserId,
				AuthKeyId: keyId,
			}).To_Updates(),
		})
	}

	return mtproto.BoolTrue, nil
}
