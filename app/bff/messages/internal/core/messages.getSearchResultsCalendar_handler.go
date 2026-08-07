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
)

// MessagesGetSearchResultsCalendar
// messages.getSearchResultsCalendar#49f0bde9 peer:InputPeer filter:MessagesFilter offset_id:int offset_date:int = messages.SearchResultsCalendar;
func (c *MessagesCore) MessagesGetSearchResultsCalendar(in *mtproto.TLMessagesGetSearchResultsCalendar) (*mtproto.Messages_SearchResultsCalendar, error) {
	// The calendar shows which days had messages in a conversation. We do not
	// compute that breakdown, yet answering with an error is not an option: the
	// client raises an alert. An empty answer is accepted calmly — the calendar
	// simply opens without marks.
	return mtproto.MakeTLMessagesSearchResultsCalendar(&mtproto.Messages_SearchResultsCalendar{
		Inexact:        false,
		Count:          0,
		MinDate:        0,
		MinMsgId:       0,
		OffsetIdOffset: nil,
		Periods:        []*mtproto.SearchResultsCalendarPeriod{},
		Messages:       []*mtproto.Message{},
		Chats:          []*mtproto.Chat{},
		Users:          []*mtproto.User{},
	}).To_Messages_SearchResultsCalendar(), nil
}
