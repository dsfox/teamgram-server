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

// AuthCheckPassword
// auth.checkPassword#d18b4d16 password:InputCheckPasswordSRP = auth.Authorization;
func (c *AuthorizationCore) AuthCheckPassword(in *mtproto.TLAuthCheckPassword) (*mtproto.Auth_Authorization, error) {
	// Password verification is not implemented: upstream grants authorization
	// WITHOUT checking the password. Until two-factor protection is written,
	// password sign-in must stay closed — otherwise enabling this branch would
	// make any password work. The branch is currently unreachable
	// (account.getPassword always reports no password), but leaving it open is
	// not acceptable.
	err := mtproto.ErrPasswordHashInvalid
	c.Logger.Errorf("auth.checkPassword: password verification is not implemented, sign-in rejected")
	return nil, err
}
