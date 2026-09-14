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
	"github.com/teamgram/teamgram-server/app/service/biz/user/user"
)

// UserCheckPrivacy
// user.checkPrivacy flags:# user_id:int key_type:int peer_id:int is_contact:flags.0?true = Bool;
func (c *UserCore) UserCheckPrivacy(in *user.TLUserCheckPrivacy) (*mtproto.Bool, error) {
	rules, err := c.UserGetPrivacy(&user.TLUserGetPrivacy{
		UserId:  in.UserId,
		KeyType: in.KeyType,
	})

	if err != nil {
		return mtproto.BoolFalse, nil
	}

	// Answered from the rules rather than always yes: a call's p2p_allowed
	// rests on this (#14) - "who may connect to me directly", and a person
	// who chose nobody is handed the relay and nothing else.
	allowed := privacyAllows(in.GetUserId(), rules.GetDatas(), in.GetPeerId(), func(id, peer int64) bool {
		return c.svcCtx.Dao.GetUserContact(c.ctx, id, peer) != nil
	})
	return mtproto.ToBool(allowed), nil

}

// privacyAllows applies a person's rules for one key to one peer. No rules
// means everybody, as the phones assume. Rules about chat participants are
// not evaluated here - this service does not hold chat membership - and so
// count as "not a participant": they neither allow nor forbid.
func privacyAllows(selfId int64, rules []*mtproto.PrivacyRule, peerId int64, isContact func(id, peer int64) bool) bool {
	if len(rules) == 0 {
		return true
	}
	return mtproto.CheckPrivacyIsAllow(selfId, rules, peerId, isContact,
		func(int64, []int64) bool { return false })
}
