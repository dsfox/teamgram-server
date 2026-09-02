// Copyright 2025 Teamgram Authors
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

// UsersGetRequirementsToContact
// users.getRequirementsToContact#d89a83a3 id:Vector<InputUser> = Vector<RequirementToContact>;
//
// Asked whenever a contact list or a profile is drawn. Nobody here needs
// anything to be written to - there is no premium and there are no paid
// messages - so the answer is "nothing" for each person asked about. One
// item per id, in order: both clients walk the answer by index against the
// list they sent, and refusing instead had them asking again behind a
// stalled queue (#159).
func (c *PrivacySettingsCore) UsersGetRequirementsToContact(in *mtproto.TLUsersGetRequirementsToContact) (*mtproto.Vector_RequirementToContact, error) {
	nothing := make([]*mtproto.RequirementToContact, 0, len(in.GetId()))
	for range in.GetId() {
		nothing = append(nothing, mtproto.MakeTLRequirementToContactEmpty(nil).To_RequirementToContact())
	}
	return &mtproto.Vector_RequirementToContact{Datas: nothing}, nil
}
