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
	// ice9: the port ICE9_ADDRESS means when it names only a host
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

// ice9: loadConfig reads the file on first use instead of panicking in init()
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

// ice9: ICE9_ADDRESS, when set, replaces dc_options with this server's own address
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

	// ice9: the version of the pack this client would get, so a phone learns of a new pack
	rValue.SetLangPackVersion(wrapperspb.Int32(langpack.Version(languageOf(c.MD.GetLangCode()), c.MD.GetClient())))

	return rValue.To_Config(), nil
}

// ice9: languageOf reduces "ru-RU" or "ru_ru" to the code a pack is filed under
func languageOf(clientLangCode string) string {
	code := strings.ToLower(clientLangCode)
	if i := strings.IndexAny(code, "-_"); i > 0 {
		code = code[:i]
	}
	return code
}

// ice9: suggestedLanguage offers Russian to ru, uk, kk and be phones, English to the rest
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
