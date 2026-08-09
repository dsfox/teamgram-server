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

package code

import (
	"context"

	"github.com/teamgram/teamgram-server/pkg/code/attempt"
	"github.com/teamgram/teamgram-server/pkg/code/conf"
	"github.com/teamgram/teamgram-server/pkg/code/invite"
	"github.com/teamgram/teamgram-server/pkg/code/none"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyCodeInterface interface {
	SendSmsVerifyCode(ctx context.Context, phoneNumber, code, codeHash string) (string, error)
	VerifySmsCode(ctx context.Context, a attempt.Attempt) error
}

// NewVerifyCode picks how a sign-in code is checked.
//
// "none" is upstream's placeholder and compares the typed code against the
// constant 12345, ignoring the one the server generated - anyone who knew a
// phone number could sign in as its owner. It is kept only because removing a
// name from a config file is not the way to close a hole; it is never selected
// by default, and the deploy does not set it.
func NewVerifyCode(c *conf.SmsVerifyCodeConfig, store invite.Store) VerifyCodeInterface {
	if c == nil {
		c = new(conf.SmsVerifyCodeConfig)
	}

	switch c.Name {
	case "none":
		logx.Error("sign-in codes are NOT being checked: the 'none' verifier accepts 12345 " +
			"for any number. Remove Code.Name from the config to use invitations.")
		return none.New(c)
	}

	return invite.New(c, store)
}
