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
// Апстрим отдавал пустой список, поэтому экран «Активные сеансы» всегда был пуст:
// пользователь не видел, с каких устройств зашёл, и не мог завершить чужой сеанс.
// Сами данные есть — сервис authsession умеет их отдавать, не хватало вызова.
func (c *AccountCore) AccountGetAuthorizations(in *mtproto.TLAccountGetAuthorizations) (*mtproto.Account_Authorizations, error) {
	authorizations, err := c.svcCtx.Dao.AuthsessionClient.AuthsessionGetAuthorizations(c.ctx, &authsession.TLAuthsessionGetAuthorizations{
		UserId: c.MD.UserId,
		// текущий сеанс клиент показывает отдельно, из списка его исключаем
		ExcludeAuthKeyId: c.MD.PermAuthKeyId,
	})
	if err != nil {
		c.Logger.Errorf("account.getAuthorizations - error: %v", err)
		return nil, err
	}

	return authorizations, nil
}
