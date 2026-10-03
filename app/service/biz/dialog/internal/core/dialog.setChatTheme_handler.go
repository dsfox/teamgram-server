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
	"context"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/app/service/biz/dialog/dialog"
)

// DialogSetChatTheme
// dialog.setChatTheme user_id:long peer_type:int peer_id:long theme_emoticon:string = Bool;
//
// The theme of a chat between two, written for both of them (#23). Through
// the dialog cache, as setChatWallpaper writes: getFullUser reads the dialog
// through it, and a write beside the cache left both phones reading the
// theme they had before.
func (c *DialogCore) DialogSetChatTheme(in *dialog.TLDialogSetChatTheme) (*mtproto.Bool, error) {
	_, _, err := c.svcCtx.Dao.CachedConn.Exec(
		c.ctx,
		func(ctx context.Context, conn *sqlx.DB) (int64, int64, error) {
			var written int64
			for _, side := range [][2]int64{{in.UserId, in.PeerId}, {in.PeerId, in.UserId}} {
				n, err := c.svcCtx.Dao.DialogsDAO.UpdateCustomMap(
					ctx,
					map[string]interface{}{
						"theme_emoticon": in.ThemeEmoticon,
					},
					side[0],
					in.PeerType,
					side[1])
				if err != nil {
					return 0, written, err
				}
				written += n
			}
			return 0, written, nil
		},
		dialog.GetDialogCacheKey(in.UserId, mtproto.MakePeerDialogId(in.PeerType, in.PeerId)),
		dialog.GetDialogCacheKey(in.PeerId, mtproto.MakePeerDialogId(in.PeerType, in.UserId)))
	if err != nil {
		c.Logger.Errorf("dialog.setChatTheme - %v: %v", in, err)
		return nil, err
	}

	return mtproto.BoolTrue, nil
}
