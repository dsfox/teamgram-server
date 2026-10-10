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
	"github.com/teamgram/proto/mtproto"
	userpb "github.com/teamgram/teamgram-server/app/service/biz/user/user"
)

// ContactsSearch
// contacts.search#11f812d8 q:string limit:int = contacts.Found;
func (c *ContactsCore) ContactsSearch(in *mtproto.TLContactsSearch) (*mtproto.Contacts_Found, error) {
	var (
		limit = in.GetLimit()
	)

	if limit > 50 {
		limit = 50
	}
	if limit == 0 {
		limit = 50
	}

	q := in.Q

	if q == "" {
		err := mtproto.ErrSearchQueryEmpty
		c.Logger.Errorf("contacts.search - error: %v", err)
		return nil, err
	}

	if q[0] == '@' {
		q = q[1:]
	}

	if len(q) < 3 {
		err := mtproto.ErrQueryTooShort
		c.Logger.Errorf("contacts.search - error: %v", err)
		return nil, err
	}

	var (
		idHelper = mtproto.NewIDListHelper(c.MD.UserId)
	)

	found := mtproto.MakeTLContactsFound(&mtproto.Contacts_Found{
		MyResults: []*mtproto.Peer{},
		Results:   []*mtproto.Peer{},
		Users:     []*mtproto.User{},
		Chats:     []*mtproto.Chat{},
	}).To_Contacts_Found()

	// TODO(@benqi):
	// This method will exclude the current user's contacts from the search results. It is assumed that searches among the user's contacts can be handled locally by the client.
	//

	// Check query string and limit
	// No directory (the owner's decision of 5 October): a stranger is found by
	// the exact username they gave out, or by their number - never by a name
	// or part of one. Upstream searched every account by username prefix and
	// by first and last name, while the App Store listing said strangers
	// cannot look you up. The person's own contacts are searched on the phone.
	if limit > 0 {
		peer, err := c.svcCtx.Dao.UserClient.UserResolveUsername(c.ctx, &userpb.TLUserResolveUsername{
			Username: q,
		})
		if err != nil {
			c.Logger.Infof("contacts.search - %d: no username %q (%v)", c.MD.UserId, q, err)
		} else if peer.GetPredicateName() == mtproto.Predicate_peerUser && peer.GetUserId() != c.MD.UserId {
			// A username the server gave out is for mentions, not a way in:
			// only one the person chose finds them (#239).
			given, err := c.svcCtx.Usernames.IsGenerated(c.ctx, peer.GetUserId(), q)
			if err != nil {
				c.Logger.Errorf("contacts.search - cannot tell whether %q was given: %v", q, err)
			}
			if err == nil && !given {
				idHelper.PickByPeer(peer)
			}
		}
	}

	idHelper.Visit(
		func(userIdList []int64) {
			users, _ := c.svcCtx.Dao.UserClient.UserGetMutableUsers(c.ctx,
				&userpb.TLUserGetMutableUsers{
					Id: userIdList,
				})

			users.Visit(func(it *mtproto.ImmutableUser) {
				if it.Deleted() {
					return
				}

				peer := mtproto.MakeTLPeerUser(&mtproto.Peer{
					UserId: it.Id(),
				})
				if ok, _ := it.CheckContact(c.MD.UserId); ok {
					found.MyResults = append(found.MyResults, peer.To_Peer())
				} else {
					found.Results = append(found.Results, peer.To_Peer())
				}
			})

			found.Users = users.GetUserListByIdList(c.MD.UserId, userIdList...)
		},
		func(chatIdList []int64) {
		},
		func(channelIdList []int64) {
			if c.svcCtx.Plugin != nil {
				chats := c.svcCtx.Plugin.GetChannelListByIdList(c.ctx, c.MD.UserId, channelIdList...)
				for _, ch := range chats {
					if ch.PredicateName == mtproto.Predicate_chatEmpty {
						continue
					}
					found.Chats = append(found.Chats, ch)
					found.Results = append(found.Results, mtproto.MakePeerChannel(ch.GetId()))
				}
			} else {
				c.Logger.Infof("contacts.search blocked, License key from https://teamgram.net required to unlock enterprise features.")
			}
		})

	return found, nil
}
