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

// normalizePhone brings a number to the shape stored in the database: digits
// only, without a plus, brackets or spaces. Numbers come from the address book
// in any form — "+7 999 123-45-67" must be found just like "79991234567".
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
// The client sends the address book and expects to learn who is already
// registered. Upstream returned an empty answer, so the contact list stayed
// empty no matter how many numbers the book held.
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
			// The number is not registered, which is normal for an address book.
			continue
		}
		if id.GetV() == c.MD.UserId {
			continue // never add yourself to the contacts
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
			// A single failed contact must not sink the whole import: the client
			// will retry it separately if it sees fit.
			c.Logger.Errorf("contacts.importContacts - adding %d: %v", id, err)
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
