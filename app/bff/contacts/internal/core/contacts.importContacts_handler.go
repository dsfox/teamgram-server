// Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
//  All rights reserved.
//
// Author: teamgramio (teamgram.io@gmail.com)
//

package core

import (
	"strings"

	"github.com/teamgram/proto/mtproto"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
)

// normalizePhone приводит номер к тому виду, в каком он лежит в базе:
// только цифры, без плюса, скобок и пробелов. Из телефонной книги номера
// приходят как угодно — «+7 999 123-45-67» должен найтись так же, как «79991234567».
func normalizePhone(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
}

// ContactsImportContacts
// contacts.importContacts#2c800be5 contacts:Vector<InputContact> = contacts.ImportedContacts;
//
// Клиент присылает телефонную книгу и ждёт, кто из неё уже зарегистрирован.
// В апстриме метод возвращал пустой ответ, поэтому список контактов всегда
// оставался пустым, сколько бы номеров ни было в книге.
func (c *ContactsCore) ContactsImportContacts(in *mtproto.TLContactsImportContacts) (*mtproto.Contacts_ImportedContacts, error) {
	var (
		imported = make([]*mtproto.ImportedContact, 0, len(in.GetContacts()))
		retry    = make([]int64, 0)
		found    = make([]int64, 0, len(in.GetContacts()))
		names    = make(map[int64]*mtproto.InputContact, len(in.GetContacts()))
	)

	for _, contact := range in.GetContacts() {
		if contact.GetPhone() == "" {
			continue
		}
		phoneContact := contact

		id, err := c.svcCtx.Dao.UserClient.UserGetUserIdByPhone(c.ctx, &userpb.TLUserGetUserIdByPhone{
			Phone: normalizePhone(phoneContact.GetPhone()),
		})
		if err != nil || id.GetV() == 0 {
			// Номера нет среди зарегистрированных — это обычное дело для телефонной книги.
			continue
		}
		if id.GetV() == c.MD.UserId {
			continue // сам себя в контакты не добавляем
		}

		found = append(found, id.GetV())
		names[id.GetV()] = phoneContact
		imported = append(imported, mtproto.MakeTLImportedContact(&mtproto.ImportedContact{
			UserId:   id.GetV(),
			ClientId: phoneContact.GetClientId(),
		}).To_ImportedContact())
	}

	if len(found) == 0 {
		return mtproto.MakeTLContactsImportedContacts(&mtproto.Contacts_ImportedContacts{
			Imported:       imported,
			PopularInvites: []*mtproto.PopularContact{},
			RetryContacts:  retry,
			Users:          []*mtproto.User{},
		}).To_Contacts_ImportedContacts(), nil
	}

	users, err := c.svcCtx.Dao.UserClient.UserGetMutableUsersV2(c.ctx, &userpb.TLUserGetMutableUsersV2{
		Id:      append([]int64{c.MD.UserId}, found...),
		Privacy: true,
		HasTo:   true,
		To:      found,
	})
	if err != nil {
		c.Logger.Errorf("contacts.importContacts - error: %v", err)
		return nil, err
	}

	for _, id := range found {
		contact := names[id]
		if _, err = c.svcCtx.Dao.UserClient.UserAddContact(c.ctx, &userpb.TLUserAddContact{
			UserId:                   c.MD.UserId,
			AddPhonePrivacyException: mtproto.BoolFalse,
			Id:                       id,
			FirstName:                contact.GetFirstName(),
			LastName:                 contact.GetLastName(),
			Phone:                    normalizePhone(contact.GetPhone()),
		}); err != nil {
			// Один неудачный контакт не должен ронять импорт всей книги:
			// клиент повторит его отдельно, если сочтёт нужным.
			c.Logger.Errorf("contacts.importContacts - добавление %d: %v", id, err)
			retry = append(retry, contact.GetClientId())
		}
	}

	return mtproto.MakeTLContactsImportedContacts(&mtproto.Contacts_ImportedContacts{
		Imported:       imported,
		PopularInvites: []*mtproto.PopularContact{},
		RetryContacts:  retry,
		Users:          users.GetUserListByIdList(c.MD.UserId, found...),
	}).To_Contacts_ImportedContacts(), nil
}
