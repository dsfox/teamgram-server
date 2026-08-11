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
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/rpc/metadata"
	"github.com/teamgram/teamgram-server/app/bff/authorization/internal/svc"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"
	"github.com/teamgram/teamgram-server/pkg/code/conf"
	"github.com/teamgram/teamgram-server/pkg/code/invite"
	"github.com/teamgram/teamgram-server/pkg/env2"
	"github.com/teamgram/teamgram-server/pkg/phonenumber"

	"github.com/zeromicro/go-zero/core/contextx"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
)

type AuthorizationCore struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	MD *metadata.RpcMetadata
}

func New(ctx context.Context, svcCtx *svc.ServiceContext) *AuthorizationCore {
	return &AuthorizationCore{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
		MD:     metadata.RpcMetadataFromIncoming(ctx),
	}
}

func checkPhoneNumberInvalid(phone string) (string, string, error) {
	// 3. check number
	// 3.1. empty
	if phone == "" {
		// log.Errorf("check phone_number error - empty")
		return "", "", mtproto.ErrPhoneNumberInvalid
	}

	phone = strings.ReplaceAll(phone, " ", "")
	if phone == "+42400" ||
		phone == "+424000" ||
		phone == "+424001" ||
		phone == "+42777" {
		return "", phone[1:], nil
	}

	// fragment
	if strings.HasPrefix(phone, "+888") {
		if len(phone) == 12 {
			// +888 0888 0080
			return "", phone[1:], nil
		} else {
			return "", "", mtproto.ErrPhoneNumberInvalid
		}
	} else if strings.HasPrefix(phone, "888") {
		if len(phone) == 11 {
			// +888 0888 0080
			return "", phone, nil
		} else {
			return "", "", mtproto.ErrPhoneNumberInvalid
		}
	}

	// 3.2. check phone_number
	// 客户端发送的手机号格式为: "+86 111 1111 1111"，归一化
	// We need getRegionCode from phone_number
	pNumber, err := phonenumber.MakePhoneNumberHelper(phone, "")
	if err != nil {
		// Strict validation rejects numbers absent from a country numbering plan.
		// For a server that never calls or texts anyone this is a pointless
		// obstacle: on a test stand the numbers are arbitrary anyway. So accept
		// any number in international format and treat country parsing as
		// optional.
		if digits, ok := plausiblePhoneNumber(phone); ok {
			return "", digits, nil
		}
		return "", "", mtproto.ErrPhoneNumberInvalid
	}

	return pNumber.GetRegionCode(), pNumber.GetNormalizeDigits(), nil
}

// plausiblePhoneNumber accepts a number that looks international: a plus and
// 8-15 digits (E.164 allows up to 15). It returns the number without the plus,
// which is how it is stored.
func plausiblePhoneNumber(phone string) (string, bool) {
	digits := strings.TrimPrefix(phone, "+")
	if len(digits) < 8 || len(digits) > 15 {
		return "", false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return digits, true
}

const (
	signInMessageTpl = `Login code: %s. Do not give this code to anyone, even if they say they are from %s!

This code can be used to log in to your %s account. We never ask it for anything else.

If you didn't request this code by trying to log in on another device, simply ignore this message.`

	// The recovery phrase is the answer to the question this service could not
	// answer at all: a phone is lost, there is nowhere to send a code, and there
	// is no SMS. So it is handed over in advance and kept by the person. It is
	// said plainly here that nobody can look it up afterwards, because that is
	// true and because a person who does not know it will not write it down.
)

// boldRanges marks the given words bold, wherever they ended up. The offsets
// used to be written by hand, which held only while the code was five digits
// long: making it eight moved the text under them and the emphasis landed in
// the middle of a word.
func boldRanges(message string, words ...string) []*mtproto.MessageEntity {
	entities := make([]*mtproto.MessageEntity, 0, len(words))
	for _, word := range words {
		at := strings.Index(message, word)
		if at < 0 {
			continue
		}
		entities = append(entities, mtproto.MakeTLMessageEntityBold(&mtproto.MessageEntity{
			Offset: int32(at),
			Length: int32(len(word)),
		}).To_MessageEntity())
	}
	return entities
}

// pushServiceMessage delivers text from the service account to one person.
func (c *AuthorizationCore) ensureRecoveryPhrase(ctx context.Context, userId int64, phoneNumber string) {
	store := c.svcCtx.Dao.Store()
	if store == nil {
		c.Logger.Errorf("recovery: no store, so %d has no way back", userId)
		return
	}

	// Delivered, not merely existing: a code minted by hand for somebody who
	// could not sign in may never have reached them, and an account whose way
	// back nobody knows must not look covered.
	if invite.HasDeliveredRecoveryPhrase(ctx, store, phoneNumber) {
		return
	}

	// Nothing is minted here any more, and nothing is sent.
	//
	// The phrase is made on the device and only a one-way derivation of it ever
	// reaches this server, through mls.setRecoverySecret. What used to happen
	// instead left every phrase sitting in the message table in plain text - a
	// phrase signs in without a code, so that was a copy of every key in one
	// place, and it made the encrypted history backup meaningless because its
	// key comes from the same words.
	//
	// An account whose device has not registered one yet has no way back, which
	// is why this says so out loud: it is the only sign that a client is too old
	// or that the call failed.
	c.Logger.Infof("recovery: %d has no way back yet - waiting for the device to register one", userId)
}

func (c *AuthorizationCore) pushSignInMessage(ctx context.Context, signInUserId int64, code string) {
	// Delivered at once, not after a pause. The client asks for its dialog list
	// once, right after signing in, and then lives on its local copy: a chat
	// that does not exist yet at that moment never appears at all - not after a
	// relaunch, not after a force quit - while its unread count still reaches
	// the badge. Two seconds of delay cost a chat that is invisible forever.
	//
	// The context is detached because the request that started this returns
	// immediately and would cancel the delivery with it.
	ctx = contextx.ValueOnlyFrom(ctx)
	threading.GoSafe(func() {
		message := mtproto.MakeTLMessage(&mtproto.Message{
			Out:     true,
			Date:    int32(time.Now().Unix()),
			FromId:  mtproto.MakePeerUser(777000),
			PeerId:  mtproto.MakeTLPeerUser(&mtproto.Peer{UserId: signInUserId}).To_Peer(),
			Message: fmt.Sprintf(signInMessageTpl, code, env2.MyAppName, env2.MyAppName),
			// Found in the text rather than counted out by hand: the offsets
			// were written for a five-digit code and the code is eight now.
			Entities: boldRanges(
				fmt.Sprintf(signInMessageTpl, code, env2.MyAppName, env2.MyAppName),
				"Login code:", "not"),
		}).To_Message()

		if len(c.svcCtx.Config.SignInMessage) > 0 {
			builder := conf.ToMessageBuildHelper(
				c.svcCtx.Config.SignInMessage,
				map[string]interface{}{
					"code":     code,
					"app_name": env2.MyAppName,
				})
			message.Message, message.Entities = mtproto.MakeTextAndMessageEntities(builder)
		}

		c.svcCtx.Dao.MsgClient.MsgPushUserMessage(
			ctx,
			&msgpb.TLMsgPushUserMessage{
				UserId:    777000,
				AuthKeyId: 0,
				PeerType:  mtproto.PEER_USER,
				PeerId:    signInUserId,
				PushType:  1,
				Message: msgpb.MakeTLOutboxMessage(&msgpb.OutboxMessage{
					NoWebpage:    false,
					Background:   false,
					RandomId:     rand.Int63(),
					Message:      message,
					ScheduleDate: nil,
				}).To_OutboxMessage(),
			})
	})
}
