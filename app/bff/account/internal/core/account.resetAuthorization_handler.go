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

// AccountResetAuthorization
// account.resetAuthorization#df77f3bc hash:long = Bool;
func (c *AccountCore) AccountResetAuthorization(in *mtproto.TLAccountResetAuthorization) (*mtproto.Bool, error) {
	if in.Hash == 0 {
		c.Logger.Errorf("account.resetAuthorization#df77f3bc - hash is 0")
		return mtproto.BoolFalse, nil
	}

	tKeyIdList, err := c.svcCtx.Dao.AuthsessionClient.AuthsessionResetAuthorization(c.ctx, &authsession.TLAuthsessionResetAuthorization{
		UserId:    c.MD.UserId,
		AuthKeyId: c.MD.PermAuthKeyId,
		Hash:      in.Hash,
	})

	if err != nil {
		c.Logger.Errorf("account.resetAuthorization#df77f3bc - error: %v", err)
		return nil, err
	}

	for _, id := range tKeyIdList.Datas {
		// notify kill session
		c.svcCtx.Dao.SyncClient.SyncUpdatesMe(
			c.ctx,
			&sync.TLSyncUpdatesMe{
				UserId:        c.MD.UserId,
				PermAuthKeyId: id,
				ServerId:      nil,
				AuthKeyId:     nil,
				SessionId:     nil,
				Updates:       mtproto.MakeTLUpdatesTooLong(nil).To_Updates(),
			})

		c.svcCtx.Dao.SyncClient.SyncUpdatesMe(
			c.ctx,
			&sync.TLSyncUpdatesMe{
				UserId:        c.MD.UserId,
				PermAuthKeyId: id,
				ServerId:      nil,
				AuthKeyId:     nil,
				SessionId:     nil,
				Updates: mtproto.MakeTLUpdateAccountResetAuthorization(&mtproto.Updates{
					UserId:    c.MD.UserId,
					AuthKeyId: id,
				}).To_Updates(),
			})
	}

	// And the phones that are still here, told at once rather than left to ask.
	//
	// A device that has been signed out stops being able to publish, and the
	// server throws away what it had - but the leaf it holds in every
	// conversation stays, and a leaf is what reading is. Only another phone of
	// the same person can take it out, and it only knows to when it next asks
	// how many devices this account has. On its own rhythm that is four minutes
	// on Android, and until then the phone in the drawer opens everything said
	// (#121).
	//
	// updatesTooLong rather than a new kind of update. It means "you may have
	// missed something, catch up", which is exactly what happened, and both
	// clients already act on it - and for a client of this fork catching up
	// includes asking how many devices it has. Inventing a wire type for this
	// would be a predicate, a wrapper and three registries in the forked proto,
	// and the same again in each client's generated code, to say something the
	// clients can already be told.
	//
	// Not to the phone that did the terminating: it asks at once, on both
	// clients, at the moment the button is pressed.
	c.svcCtx.Dao.SyncClient.SyncUpdatesNotMe(
		c.ctx,
		&sync.TLSyncUpdatesNotMe{
			UserId:        c.MD.UserId,
			PermAuthKeyId: c.MD.PermAuthKeyId,
			Updates:       mtproto.MakeTLUpdatesTooLong(nil).To_Updates(),
		})

	return mtproto.BoolTrue, nil
}
