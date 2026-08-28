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
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/service/authsession/authsession"
	"github.com/teamgram/teamgram-server/pkg/mls"
)

// AuthsessionResetAuthorization
// authsession.resetAuthorization user_id:long auth_key_id:long hash:long = Vector<long>;
func (c *AuthsessionCore) AuthsessionResetAuthorization(in *authsession.TLAuthsessionResetAuthorization) (*authsession.Vector_Long, error) {
	var (
		excludeKeyId = in.AuthKeyId
	)

	if excludeKeyId != 0 {
		myKeyData, err := c.svcCtx.Dao.QueryAuthKeyV2(c.ctx, in.AuthKeyId)
		if err != nil {
			c.Logger.Errorf("session.getAuthorizations - error: %v", err)
			return nil, err
		} else if myKeyData == nil {
			c.Logger.Errorf("session.getAuthorizations - error: %v", err)
			err = mtproto.ErrAuthKeyInvalid
			return nil, err
		} else {
			excludeKeyId = myKeyData.PermAuthKeyId
		}
	}

	keyIdList := c.svcCtx.Dao.ResetAuthorization(c.ctx, in.UserId, excludeKeyId, in.Hash)
	// log.Debugf("keyIdList: %v", keyIdList)

	// A terminated session is a device that is gone, and what it published has
	// to go with it. This is the path both "terminate this one" and "terminate
	// all the others" take; signing out takes another, and does the same there.
	//
	// Left behind, those key packages are counted for ever, and the phone
	// somebody actually holds reads the count as devices of its own still to be
	// let into every conversation - so it adds leaves nobody is behind (#41).
	for _, keyId := range keyIdList {
		mls.ForgetDevice(c.ctx, c.svcCtx.Dao.DB, in.UserId, keyId)
	}

	keyIdL2ist := make([]int64, 0, len(keyIdList))
	for _, keyId := range keyIdList {
		keyData, _ := c.svcCtx.Dao.QueryAuthKeyV2(c.ctx, keyId)
		if keyData != nil {
			if keyData.TempAuthKeyId != 0 {
				keyIdL2ist = append(keyIdL2ist, keyData.TempAuthKeyId)
			} else {
				keyIdL2ist = append(keyIdL2ist, keyId)
			}
		}
	}

	return &authsession.Vector_Long{
		Datas: keyIdList,
	}, nil
}
