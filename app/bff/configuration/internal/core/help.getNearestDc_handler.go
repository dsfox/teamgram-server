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

// HelpGetNearestDc
// help.getNearestDc#1fb33026 = NearestDc;
func (c *ConfigurationCore) HelpGetNearestDc(in *mtproto.TLHelpGetNearestDc) (*mtproto.NearestDc, error) {
	_ = in

	// There is one datacentre and this is it. Upstream uses this to send a
	// client to the closest of many; with one, the answer is always the same
	// one and always correct.

	rValue := mtproto.MakeTLNearestDc(&mtproto.NearestDc{
		// Empty rather than upstream's "CN". This field is where the client is
		// told which country it appears to be in, and it preselects the dialling
		// code on the sign-in screen - so a wrong value is a person in Moscow
		// being offered +86. We do not look anybody up, so we do not know, and
		// an empty answer says exactly that.
		//
		// It never preselected China in practice: setCountry looks the value up
		// by code and then matches it against country *names*, which cannot
		// agree, so nothing happened. Checked on a fresh install - the field is
		// blank. That is upstream's mistake protecting us from upstream's data,
		// which is not a thing to leave standing.
		Country:   "",
		ThisDc:    1,
		NearestDc: 1,
	}).To_NearestDc()

	return rValue, nil
}
