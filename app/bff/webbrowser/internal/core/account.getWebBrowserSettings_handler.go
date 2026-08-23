// Copyright (c) 2026 The Teamgram Authors (https://teamgram.net).
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

package core

import (
	"github.com/teamgram/proto/mtproto"
)

// AccountGetWebBrowserSettings
// account.getWebBrowserSettings#56655768 hash:long = account.WebBrowserSettings;
func (c *WebBrowserCore) AccountGetWebBrowserSettings(in *mtproto.TLAccountGetWebBrowserSettings) (*mtproto.Account_WebBrowserSettings, error) {
	// Nothing is stored, and the honest answer to "which domains open outside
	// the app" is "none" rather than an error. It used to be an error, and the
	// clients from 12.9 on ask this at startup: the answer they got was
	// ERR_ENTERPRISE_IS_BLOCKED, which they back off from and retry, with the
	// rest of their service queue waiting behind it. Found while carrying the
	// clients forward - see ice9 #57.
	c.Logger.Infof("account.getWebBrowserSettings: nothing is stored, answering with no exceptions")

	return mtproto.MakeTLAccountWebBrowserSettings(&mtproto.Account_WebBrowserSettings{
		OpenExternalBrowser: false,
		DisplayCloseButton:  true,
		ExternalExceptions:  []*mtproto.WebDomainException{},
		InappExceptions:     []*mtproto.WebDomainException{},
		Hash:                0,
	}).To_Account_WebBrowserSettings(), nil
}
