// Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
//  All rights reserved.
//
// Author: teamgramio (teamgram.io@gmail.com)
//

package core

import (
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/service/authsession/authsession"
)

// AccountGetAuthorizations
// account.getAuthorizations#e320c158 = account.Authorizations;
//
// Upstream returned an empty list, so the "Active sessions" screen was always
// blank: the user could not see which devices were signed in nor terminate a
// foreign session. The data exists — the authsession service can serve it, only
// the call was missing.
func (c *AccountCore) AccountGetAuthorizations(in *mtproto.TLAccountGetAuthorizations) (*mtproto.Account_Authorizations, error) {
	authorizations, err := c.svcCtx.Dao.AuthsessionClient.AuthsessionGetAuthorizations(c.ctx, &authsession.TLAuthsessionGetAuthorizations{
		UserId: c.MD.UserId,
		// the client shows the current session separately, so exclude it here
		ExcludeAuthKeyId: c.MD.PermAuthKeyId,
	})
	if err != nil {
		c.Logger.Errorf("account.getAuthorizations - error: %v", err)
		return nil, err
	}

	return authorizations, nil
}
