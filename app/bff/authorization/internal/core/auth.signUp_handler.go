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
	"math/rand"

	"github.com/teamgram/marmota/pkg/threading2"
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/proto/mtproto/crypto"
	"github.com/teamgram/teamgram-server/app/bff/authorization/internal/logic"
	"github.com/teamgram/teamgram-server/app/bff/authorization/model"
	msgpb "github.com/teamgram/teamgram-server/app/messenger/msg/msg/msg"
	"github.com/teamgram/teamgram-server/app/service/authsession/authsession"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
)

/*
  Android client auth.signUp#1b067634, handler error
	if (error.text.contains("PHONE_NUMBER_INVALID")) {
		needShowAlert(LocaleController.getString("AppName", R.string.AppName), LocaleController.getString("InvalidPhoneNumber", R.string.InvalidPhoneNumber));
	} else if (error.text.contains("PHONE_CODE_EMPTY") || error.text.contains("PHONE_CODE_INVALID")) {
		needShowAlert(LocaleController.getString("AppName", R.string.AppName), LocaleController.getString("InvalidCode", R.string.InvalidCode));
	} else if (error.text.contains("PHONE_CODE_EXPIRED")) {
		needShowAlert(LocaleController.getString("AppName", R.string.AppName), LocaleController.getString("CodeExpired", R.string.CodeExpired));
	} else if (error.text.contains("FIRSTNAME_INVALID")) {
		needShowAlert(LocaleController.getString("AppName", R.string.AppName), LocaleController.getString("InvalidFirstName", R.string.InvalidFirstName));
	} else if (error.text.contains("LASTNAME_INVALID")) {
		needShowAlert(LocaleController.getString("AppName", R.string.AppName), LocaleController.getString("InvalidLastName", R.string.InvalidLastName));
	} else {
		needShowAlert(LocaleController.getString("AppName", R.string.AppName), error.text);
	}

*/

// AuthSignUp
// auth.signUp#80eee427 phone_number:string phone_code_hash:string first_name:string last_name:string = auth.Authorization;
func (c *AuthorizationCore) AuthSignUp(in *mtproto.TLAuthSignUp) (*mtproto.Auth_Authorization, error) {
	if c.svcCtx.Plugin != nil {
		c.svcCtx.Plugin.OnAuthAction(c.ctx,
			c.MD.PermAuthKeyId,
			c.MD.ClientMsgId,
			c.MD.ClientAddr,
			in.PhoneNumber,
			logic.GetActionType(in),
			"auth.signUp")
	}

	// 1. check phone_code empty
	var (
		phoneCode *string = nil
	)

	// 3. check number
	// 3.1. empty
	if in.PhoneNumber == "" {
		c.Logger.Errorf("check phone_number error - empty")
		err := mtproto.ErrPhoneNumberInvalid
		return nil, err
	}

	// 3.2. check phone_number
	// 客户端发送的手机号格式为: "+86 111 1111 1111"，归一化
	// We need getRegionCode from phone_number
	reginCode, phoneNumber, err := checkPhoneNumberInvalid(in.PhoneNumber)
	if err != nil {
		c.Logger.Errorf("check phone_number error - %v", err)
		err = mtproto.ErrPhoneNumberInvalid
		return nil, err
	}

	if in.PhoneCodeHash == "" {
		c.Logger.Errorf("check phone_code_hash error - empty")
		err = mtproto.ErrPhoneCodeHashEmpty
		return nil, err
	}

	// TODO(@benqi): register name ruler
	// check first name invalid
	if in.FirstName == "" {
		c.Logger.Errorf("check first_name error - empty")
		err = mtproto.ErrFirstnameInvalid
		return nil, err
	}

	// TODO(@benqi): PHONE_NUMBER_FLOOD
	// <string name="PhoneNumberFlood">Sorry, you have deleted and re-created your account too many times recently.
	//    Please wait for a few days before signing up again.</string>
	//

	// Asked again for an account this session already has: answer with it.
	//
	// The transport resends a request it thinks went unanswered, and the client
	// resends when somebody presses the button twice. The first call registers
	// the person, binds this key to them and - correctly - forgets the code,
	// which the second call then cannot find. It answered PHONE_CODE_EXPIRED,
	// and on screen that is "Code expired, please start over" over an account
	// that exists, is bound and is signed in. Two people hit it within an hour
	// of each other, one of them on the first step of a checklist.
	//
	// Registering is not repeatable, but saying who was registered is.
	if already := c.alreadySignedUp(phoneNumber); already != nil {
		c.Logger.Infof("auth.signUp - this session is already %d, answering with it", already.Id())
		return mtproto.MakeTLAuthAuthorization(&mtproto.Auth_Authorization{
			SetupPasswordRequired: false,
			OtherwiseReloginDays:  nil,
			TmpSessions:           nil,
			FutureAuthToken:       nil,
			User:                  already.ToSelfUser(),
		}).To_Auth_Authorization(), nil
	}

	////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	var (
		codeData *model.PhoneCodeTransaction
	)
	// phoneRegistered := auth.CheckPhoneNumberExist(phoneNumber)
	codeData, err = c.svcCtx.AuthLogic.DoAuthSignUp(c.ctx, c.MD.PermAuthKeyId, phoneNumber, phoneCode, in.PhoneCodeHash)
	if err != nil {
		c.Logger.Errorf(err.Error())
		return nil, err
	} else {
		_ = codeData
	}

	var (
		user *mtproto.ImmutableUser
	)

	key := crypto.CreateAuthKey()
	_, err = c.svcCtx.Dao.AuthsessionClient.AuthsessionSetAuthKey(c.ctx, &authsession.TLAuthsessionSetAuthKey{
		AuthKey: &mtproto.AuthKeyInfo{
			AuthKeyId:          key.AuthKeyId(),
			AuthKey:            key.AuthKey(),
			AuthKeyType:        mtproto.AuthKeyTypePerm,
			PermAuthKeyId:      key.AuthKeyId(),
			TempAuthKeyId:      0,
			MediaTempAuthKeyId: 0,
		},
		FutureSalt: nil,
	})

	if err != nil {
		c.Logger.Errorf("create user secret key error")
		return nil, err
	}

	var (
		firstName = in.FirstName
		lastName  = in.LastName
	)

	// Create new user
	if user, err = c.svcCtx.UserClient.UserCreateNewUser(c.ctx, &userpb.TLUserCreateNewUser{
		SecretKeyId: key.AuthKeyId(),
		Phone:       phoneNumber,
		CountryCode: reginCode,
		FirstName:   firstName,
		LastName:    lastName,
	}); err != nil {
		c.Logger.Errorf("createNewUser error: %v", err)
		return nil, err
	}

	// TODO(@benqi): remove to createNewUser
	// user.Self = true

	// bind auth_key and user_id
	_, err = c.svcCtx.Dao.AuthsessionClient.AuthsessionBindAuthKeyUser(c.ctx, &authsession.TLAuthsessionBindAuthKeyUser{
		AuthKeyId: c.MD.PermAuthKeyId,
		UserId:    user.User.Id,
	})
	if err != nil {
		c.Logger.Errorf("bindAuthKeyUser error: %v", err)
		err = mtproto.ErrInternalServerError
		return nil, err
	}

	return threading2.WrapperGoFunc(
		c.ctx,
		mtproto.MakeTLAuthAuthorization(&mtproto.Auth_Authorization{
			SetupPasswordRequired: false,
			OtherwiseReloginDays:  nil,
			TmpSessions:           nil,
			FutureAuthToken:       nil,
			User:                  user.ToSelfUser(),
		}).To_Auth_Authorization(),
		func(ctx context.Context) {
			// on event
			c.svcCtx.AuthLogic.DeletePhoneCode(ctx, c.MD.PermAuthKeyId, phoneNumber, in.PhoneCodeHash)
			c.pushSignInMessage(ctx, user.Id(), codeData.PhoneCode)
			c.ensureRecoveryPhrase(ctx, user.Id(), phoneNumber)
			c.onContactSignUp(ctx, c.MD.PermAuthKeyId, user.Id(), phoneNumber)
			// Who came in on whose invitation: the id exists only now (#47).
			if err := c.svcCtx.Invitations.Adopted(ctx, phoneNumber, user.Id()); err != nil {
				c.Logger.Errorf("auth.signUp - %v", err)
			}
			// And where the invitation leads: a code minted from a group puts
			// the person into it (#164).
			if chatId, inviterId, err := c.svcCtx.Invitations.Landing(ctx, phoneNumber, user.Id()); err != nil {
				c.Logger.Errorf("auth.signUp - %v", err)
			} else if chatId != 0 {
				c.landInChat(ctx, chatId, inviterId, user.Id())
			}
		},
	).(*mtproto.Auth_Authorization), nil
}

func (c *AuthorizationCore) onContactSignUp(ctx context.Context, authKeyId, userId int64, phone string) {
	importers, _ := c.svcCtx.Dao.UserClient.UserGetImportersByPhone(ctx, &userpb.TLUserGetImportersByPhone{
		Phone: phone,
	})

	for _, c2 := range importers.GetDatas() {
		c.Logger.Infof("importer: %v", c2)
		v, _ := c.svcCtx.Dao.UserClient.UserGetContactSignUpNotification(ctx, &userpb.TLUserGetContactSignUpNotification{
			UserId: c2.ClientId,
		})

		_ = authKeyId
		if mtproto.FromBool(v) {
			c.svcCtx.Dao.MsgClient.MsgPushUserMessage(
				context.Background(),
				&msgpb.TLMsgPushUserMessage{
					UserId:    userId,
					AuthKeyId: 0,
					PeerType:  mtproto.PEER_USER,
					PeerId:    c2.ClientId,
					PushType:  1,
					Message: msgpb.MakeTLOutboxMessage(&msgpb.OutboxMessage{
						NoWebpage:    false,
						Background:   false,
						RandomId:     rand.Int63(),
						Message:      mtproto.MakeContactSignUpMessage(userId, c2.ClientId),
						ScheduleDate: nil,
					}).To_OutboxMessage(),
				})
		} else {
			c.Logger.Infof("not setting contactSignUpNotification")
		}
	}
	c.svcCtx.Dao.UserClient.UserDeleteImportersByPhone(ctx, &userpb.TLUserDeleteImportersByPhone{
		Phone: phone,
	})
}

// alreadySignedUp is the account this session already belongs to, when the
// number being registered is that account's own. Nil when the session belongs
// to nobody, which is the ordinary case for a registration.
//
// It exists because registering is not repeatable and being told who was
// registered is. The transport resends; people press twice. Only a match on the
// phone number counts: a session belonging to somebody else being handed a
// different number is not a repeat, it is a mistake, and it must go down the
// path that refuses it.
func (c *AuthorizationCore) alreadySignedUp(phoneNumber string) *mtproto.ImmutableUser {
	if c.MD == nil || c.MD.UserId == 0 {
		return nil
	}

	user, err := c.svcCtx.Dao.UserClient.UserGetImmutableUserByPhone(c.ctx, &userpb.TLUserGetImmutableUserByPhone{
		Phone: phoneNumber,
	})
	if err != nil || user == nil {
		return nil
	}
	if user.Id() != c.MD.UserId {
		return nil
	}
	return user
}
