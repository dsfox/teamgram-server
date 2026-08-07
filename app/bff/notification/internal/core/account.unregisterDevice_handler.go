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
)

// AccountUnregisterDevice
// account.unregisterDevice#6a0d3206 token_type:int token:string other_uids:Vector<long> = Bool;
//
// The app calls this on sign-out. Forgetting the token is mandatory: otherwise
// notifications keep reaching a person who has left.
func (c *NotificationCore) AccountUnregisterDevice(in *mtproto.TLAccountUnregisterDevice) (*mtproto.Bool, error) {
	if c.svcCtx.Dao.Devices == nil {
		return mtproto.BoolTrue, nil
	}

	err := c.svcCtx.Dao.Devices.Unregister(
		c.ctx,
		c.MD.PermAuthKeyId,
		c.MD.UserId,
		in.GetTokenType(),
		in.GetToken())
	if err != nil {
		c.Logger.Errorf("account.unregisterDevice - error: %v", err)
		return mtproto.BoolFalse, nil
	}

	c.Logger.Infof("account.unregisterDevice - user: %d, type: %d", c.MD.UserId, in.GetTokenType())

	return mtproto.BoolTrue, nil
}
