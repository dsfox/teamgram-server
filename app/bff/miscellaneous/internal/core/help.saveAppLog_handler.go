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

// HelpSaveAppLog
// help.saveAppLog#6f02f748 events:Vector<InputAppEvent> = Bool;
func (c *MiscellaneousCore) HelpSaveAppLog(in *mtproto.TLHelpSaveAppLog) (*mtproto.Bool, error) {
	// The client offers us its own telemetry; we do not collect any. Accepting
	// and dropping it is the honest answer - refusing only makes the client try
	// again, and its queue waits behind the retry.
	return mtproto.BoolTrue, nil
}
