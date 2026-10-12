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

	// ice9: answered from the rules, not always yes: a call's p2p_allowed rests on it (#14)
	allowed := privacyAllows(in.GetUserId(), rules.GetDatas(), in.GetPeerId(), func(id, peer int64) bool {
		return c.svcCtx.Dao.GetUserContact(c.ctx, id, peer) != nil
	})
	return mtproto.ToBool(allowed), nil

}

// ice9: one key's rules for one peer; none allow everybody, chat-participant rules are not evaluated
func privacyAllows(selfId int64, rules []*mtproto.PrivacyRule, peerId int64, isContact func(id, peer int64) bool) bool {
	if len(rules) == 0 {
		return true
	}
	return mtproto.CheckPrivacyIsAllow(selfId, rules, peerId, isContact,
		func(int64, []int64) bool { return false })
}
