// Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
//  All rights reserved.
//
// Author: teamgramio (teamgram.io@gmail.com)
//

package core

import (
	"github.com/teamgram/proto/mtproto"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
)

// ContactsAcceptContact
// contacts.acceptContact#f831a20f id:InputUser = Updates;
//
// Так работает кнопка «Поделиться номером» в плашке над перепиской с незнакомцем.
// Апстрим возвращал пустой ответ: кнопка нажималась, но ничего не происходило.
// По сути это добавление в контакты — имя берётся из профиля собеседника,
// потому что вводить его пользователю здесь негде.
func (c *ContactsCore) ContactsAcceptContact(in *mtproto.TLContactsAcceptContact) (*mtproto.Updates, error) {
	id := mtproto.FromInputUser(c.MD.UserId, in.Id)
	if !id.IsUser() || id.IsSelf() || id.PeerId == c.MD.UserId {
		err := mtproto.ErrContactIdInvalid
		c.Logger.Errorf("contacts.acceptContact - error: %v", err)
		return nil, err
	}

	users, err := c.svcCtx.Dao.UserClient.UserGetMutableUsersV2(c.ctx, &userpb.TLUserGetMutableUsersV2{
		Id:      []int64{c.MD.UserId, id.PeerId},
		Privacy: true,
		HasTo:   true,
		To:      []int64{id.PeerId},
	})
	if err != nil || !users.CheckExistUser(c.MD.UserId, id.PeerId) {
		c.Logger.Errorf("contacts.acceptContact - error: %v", err)
		return nil, mtproto.ErrContactIdInvalid
	}

	contact, _ := users.GetImmutableUser(id.PeerId)
	changeMutual, err := c.svcCtx.Dao.UserClient.UserAddContact(c.ctx, &userpb.TLUserAddContact{
		UserId:                   c.MD.UserId,
		AddPhonePrivacyException: mtproto.BoolTrue,
		Id:                       id.PeerId,
		FirstName:                contact.FirstName(),
		LastName:                 contact.LastName(),
		Phone:                    contact.Phone(),
	})
	if err != nil {
		c.Logger.Errorf("contacts.acceptContact - error: %v", err)
		return nil, mtproto.ErrContactIdInvalid
	}

	cUser, _ := users.GetUnsafeUser(c.MD.UserId, id.PeerId)
	cUser.Contact = true
	cUser.MutualContact = mtproto.FromBool(changeMutual)
	me, _ := users.GetUnsafeUserSelf(c.MD.UserId)

	// Плашка над перепиской должна исчезнуть — сообщаем клиенту, что предлагать
	// больше нечего.
	return mtproto.MakeUpdatesByUpdatesUsers(
		[]*mtproto.User{me, cUser},
		mtproto.MakeTLUpdatePeerSettings(&mtproto.Update{
			Peer_PEER: id.ToPeer(),
			Settings: mtproto.MakeTLPeerSettings(&mtproto.PeerSettings{
				ReportSpam:   false,
				AddContact:   false,
				BlockContact: false,
				ShareContact: false,
			}).To_PeerSettings(),
		}).To_Update()), nil
}
