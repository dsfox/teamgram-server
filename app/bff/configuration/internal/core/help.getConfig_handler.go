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
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/langpack"

	"github.com/zeromicro/go-zero/core/jsonx"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	configFile = "./config.json"
	// What ICE9_ADDRESS means when it names a host and no port. The clients
	// dial this one; see clients' seed lists and deploy/production.
	defaultPort = "10443"
	// date = 1509066502,    2017/10/27 09:08:22
	// expires = 1509070295, 2017/10/27 10:11:35
	expiresTimeout = 3600 // 超时时间设置为3600秒

	// support user: @benqi
	// SUPPORT_USER_ID = 2
)

var (
	config     mtproto.TLConfig
	configOnce sync.Once
)

// The file is read on first use rather than at import. It used to panic in
// init(), which meant the package could not be imported anywhere the file was
// not sitting in the working directory - a test in this package could not run
// at all, and the rule below is exactly the kind of thing that wants one.
//
// A missing config is still fatal for the server, and says so; it simply says it
// when somebody asks for the config rather than when the binary loads.
func loadConfig() {
	configData, err := os.ReadFile(configFile)
	if err != nil {
		logx.Errorf("config not read (%s): the client will be given an empty one: %v", configFile, err)
		return
	}

	if err = jsonx.Unmarshal(configData, &config); err != nil {
		logx.Errorf("config is corrupt (%s): %v", configFile, err)
	}

	adoptAddressFromEnvironment()
}

// The address this server answers on, which is not ours to know at build time.
//
// dc_options is the list a client keeps and dials from then on: whatever stands
// here replaces what the phone was seeded with. So a server put up by somebody
// else must say its own address, or its people would be handed ours and quietly
// end up on our machine. ICE9_ADDRESS is how install.sh says it, and our own
// deploy says it the same way rather than editing config.json.
//
// Empty means "use the file", which is what the local stand does.
func adoptAddressFromEnvironment() {
	address := strings.TrimSpace(os.Getenv("ICE9_ADDRESS"))
	if address == "" {
		return
	}

	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		host, portText = address, defaultPort
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		logx.Errorf("ICE9_ADDRESS (%s) names no usable port: keeping %s", address, configFile)
		return
	}

	config.SetDcOptions([]*mtproto.DcOption{
		mtproto.MakeTLDcOption(&mtproto.DcOption{
			Id:        config.GetThisDc(),
			IpAddress: host,
			Port:      int32(port),
			Static:    true,
		}).To_DcOption(),
	})
	logx.Infof("help.getConfig will hand out %s:%d, from ICE9_ADDRESS", host, port)
}

// HelpGetConfig
// help.getConfig#c4f9186b = Config;
func (c *ConfigurationCore) HelpGetConfig(in *mtproto.TLHelpGetConfig) (*mtproto.Config, error) {
	_ = in

	configOnce.Do(loadConfig)

	rValue, _ := proto.Clone(&config).(*mtproto.TLConfig)
	now := int32(time.Now().Unix())
	rValue.SetDate(now)
	rValue.SetExpires(now + expiresTimeout)
	rValue.SetSuggestedLangCode(wrapperspb.String(suggestedLanguage(c.MD.GetLangCode())))

	// The version of the pack this client would be given, rather than the number
	// the file inherited from upstream. A client asks for a newer pack only when
	// this is larger than the version it holds, so a constant here means a phone
	// that already has a pack never learns of a new one - which is what ours did:
	// every string we changed reached fresh installs and nobody else.
	rValue.SetLangPackVersion(wrapperspb.Int32(langpack.Version(languageOf(c.MD.GetLangCode()), c.MD.GetClient())))

	return rValue.To_Config(), nil
}

// languageOf reduces what the client reports - "ru-RU", "ru_ru" - to the code a
// pack is filed under.
func languageOf(clientLangCode string) string {
	code := strings.ToLower(clientLangCode)
	if i := strings.IndexAny(code, "-_"); i > 0 {
		code = code[:i]
	}
	return code
}

// We offer two languages, and this decides which one a phone is offered before
// anybody chooses. Russian where it is what people read, English everywhere
// else. The decision lives here rather than in each client so that both make it
// the same way - and because the file this config comes from suggested
// "classic-zh-cn", inherited from upstream, to everyone who asked.
//
// The client reports the language its system is set to; that is the only signal
// we have, and it is the right one - a phone set to Russian is not ambiguous.
func suggestedLanguage(clientLangCode string) string {
	code := strings.ToLower(clientLangCode)
	if i := strings.IndexAny(code, "-_"); i > 0 {
		code = code[:i]
	}
	switch code {
	case "ru", "uk", "kk", "be":
		return "ru"
	default:
		return "en"
	}
}
