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
	"encoding/hex"
	"strings"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/devices"
)

// AccountRegisterDevice
// account.registerDevice#ec86017a flags:# no_muted:flags.0?true token_type:int token:string app_sandbox:Bool secret:bytes other_uids:Vector<long> = Bool;
//
// The app calls this on every launch: Apple hands out the device token anew and
// rotates it from time to time. The row is overwritten per authorization key —
// one device, one token.
func (c *NotificationCore) AccountRegisterDevice(in *mtproto.TLAccountRegisterDevice) (*mtproto.Bool, error) {
	if c.svcCtx.Dao.Devices == nil {
		c.Logger.Errorf("account.registerDevice - notifications disabled: no database configured")
		return mtproto.BoolFalse, nil
	}

	token := strings.TrimSpace(in.GetToken())
	if token == "" {
		c.Logger.Errorf("account.registerDevice - empty token, type %d", in.GetTokenType())
		return mtproto.BoolFalse, nil
	}

	err := c.svcCtx.Dao.Devices.Register(c.ctx, &devices.DeviceDO{
		AuthKeyId:  c.MD.PermAuthKeyId,
		UserId:     c.MD.UserId,
		TokenType:  in.GetTokenType(),
		Token:      token,
		NoMuted:    in.GetNoMuted(),
		AppSandbox: mtproto.FromBool(in.GetAppSandbox()),
		// The secret is only needed to encrypt text inside a notification. We do
		// not send the text, yet we keep the secret — otherwise adding an app
		// extension later would force every device to register anew.
		Secret:    hex.EncodeToString(in.GetSecret()),
		OtherUids: devices.JoinUids(in.GetOtherUids()),
	})
	if err != nil {
		c.Logger.Errorf("account.registerDevice - error: %v", err)
		return mtproto.BoolFalse, nil
	}

	c.Logger.Infof("account.registerDevice - user: %d, type: %d, sandbox: %v",
		c.MD.UserId, in.GetTokenType(), mtproto.FromBool(in.GetAppSandbox()))

	return mtproto.BoolTrue, nil
}
