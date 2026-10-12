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
	"time"

	"github.com/teamgram/proto/mtproto"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
)

// ContactsResolvePhone
// contacts.resolvePhone#8af94344 phone:string = contacts.ResolvedPeer;
func (c *UsersCore) ContactsResolvePhone(in *mtproto.TLContactsResolvePhone) (*mtproto.Contacts_ResolvedPeer, error) {
	// ice9: sixty lookups an hour per account, too few to walk the numbers (#170)
	ok, wait, err := lookupAllowed(c.ctx, c.svcCtx.Dao.KV, c.MD.UserId, time.Now())
	if err != nil {
		c.Logger.Errorf("contacts.resolvePhone - cannot count lookups, letting this one through: %v", err)
	}
	if !ok {
		c.Logger.Infof("contacts.resolvePhone - refused: user %d is over %d lookups this hour", c.MD.UserId, lookupsPerHour)
		return nil, mtproto.NewErrFloodWaitX(wait)
	}
	id, err := c.svcCtx.Dao.UserClient.UserGetUserIdByPhone(c.ctx, &userpb.TLUserGetUserIdByPhone{
		Phone: in.GetPhone(),
	})
	if err != nil || id.GetV() == 0 {
		// ice9: nobody on that number is PHONE_NOT_OCCUPIED, not INTERNAL_SERVER_ERROR
		c.Logger.Infof("contacts.resolvePhone - nobody on %s", in.GetPhone())
		return nil, mtproto.ErrPhoneNotOccupied
	}

	// ice9: a number finds its owner unless they said otherwise, as Telegram's default
	var (
		allow = true
	)

	contactList, err := c.svcCtx.Dao.UserClient.UserGetMutableUsersV2(c.ctx, &userpb.TLUserGetMutableUsersV2{
		Id:      []int64{id.GetV(), c.MD.UserId},
		Privacy: true,
		HasTo:   true,
		To:      []int64{c.MD.UserId},
	})
	if err != nil {
		c.Logger.Errorf("contacts.resolvePhone - error: %v", err)
		return nil, mtproto.ErrPhoneNotOccupied
	}

	me, _ := contactList.GetImmutableUser(c.MD.UserId)
	resolved, _ := contactList.GetImmutableUser(id.GetV())

	if me == nil || resolved == nil {
		// ice9: a pair that cannot be read is PHONE_NOT_OCCUPIED too, so the client offers an invite
		c.Logger.Errorf("contacts.resolvePhone - %s is user %d, but the pair could not be read", in.GetPhone(), id.GetV())
		return nil, mtproto.ErrPhoneNotOccupied
	}

	rules, _ := c.svcCtx.Dao.UserClient.UserGetPrivacy(c.ctx, &userpb.TLUserGetPrivacy{
		UserId:  id.GetV(),
		KeyType: mtproto.ADDED_BY_PHONE,
	})
	if rules != nil && len(rules.Datas) > 0 {
		allow = mtproto.CheckPrivacyIsAllow(
			c.MD.UserId,
			rules.Datas,
			id.GetV(),
			func(id, checkId int64) bool {
				contact, _ := resolved.CheckContact(checkId)
				return contact
			},
			func(checkId int64, idList []int64) bool {
				return false
			})
	}
	if !allow {
		c.Logger.Errorf("contacts.resolvePhone - error: %v", err)
		return nil, mtproto.ErrPhoneNotOccupied
	}

	return mtproto.MakeTLContactsResolvedPeer(&mtproto.Contacts_ResolvedPeer{
		Peer:  mtproto.MakePeerUser(id.GetV()),
		Chats: []*mtproto.Chat{},
		Users: []*mtproto.User{resolved.ToUnsafeUser(me)},
	}).To_Contacts_ResolvedPeer(), nil
}
