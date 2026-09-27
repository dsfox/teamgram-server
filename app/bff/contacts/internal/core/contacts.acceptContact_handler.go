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
// This is what the "Share my phone number" button does in the bar above a chat
// with a stranger. Upstream returned an empty answer: the button could be
// pressed but nothing happened. In essence it adds a contact — the name comes
// from the peer's profile because there is nowhere for the user to type one.
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

	// CheckExistUser above says both are there, and each lookup is still
	// asked: an answer thrown away as `_` is a nil the line after it dereferences.
	contact, ok := users.GetImmutableUser(id.PeerId)
	if !ok {
		c.Logger.Errorf("contacts.acceptContact - %d is not among the users just read", id.PeerId)
		return nil, mtproto.ErrContactIdInvalid
	}
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

	cUser, err := users.GetUnsafeUser(c.MD.UserId, id.PeerId)
	if err != nil {
		c.Logger.Errorf("contacts.acceptContact - cannot read %d back: %v", id.PeerId, err)
		return nil, mtproto.ErrContactIdInvalid
	}
	cUser.Contact = true
	cUser.MutualContact = mtproto.FromBool(changeMutual)
	me, err := users.GetUnsafeUserSelf(c.MD.UserId)
	if err != nil {
		c.Logger.Errorf("contacts.acceptContact - cannot read %d back: %v", c.MD.UserId, err)
		return nil, mtproto.ErrContactIdInvalid
	}

	// The bar above the chat should disappear, so tell the client there is
	// nothing left to offer.
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
