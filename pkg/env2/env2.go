// Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
//  All rights reserved.
//
// Author: teamgramio (teamgram.io@gmail.com)
//

package env2

import (
	"flag"
)

var (
	MyAppName = "ice9"
	MyWebSite = "teamgram.net"
	// ice9: invite links on our host, repeated as the flag default; test_link_domain_gate.py (#87)
	TDotMe = "i.ice9.app"
	// MyWebClientSite = "web.teamgram.net"
	MyTgScheme = "tg"
	// IosAppStoreId   = ""
	// AndroidPackage  = "org.chatengine.messenger"

	// PredefinedUser - auto register
	PredefinedUser = false
	// PredefinedUser2
	// predefined2 - auto register
	PredefinedUser2 = false

	MyTgCDNHost = ""
)

func init() {
	flag.StringVar(&MyAppName, "app_name", "ice9", "app_name")
	flag.StringVar(&MyWebSite, "site_name", "teamgram.net", "site_name")
	flag.StringVar(&TDotMe, "t.me", "i.ice9.app", "t.me")
	// flag.StringVar(&MyWebClientSite, "webclient", "web.teamgram.net", "web.teamgram.net")
	flag.StringVar(&MyTgScheme, "tg", "tg", "tg")
	// flag.StringVar(&IosAppStoreId, "app_store_id", "", "")
	// flag.StringVar(&AndroidPackage, "android_package", "org.chatengine.messenger", "org.chatengine.messenger")
	flag.BoolVar(&PredefinedUser, "predefined", false, "predefined")
	flag.BoolVar(&PredefinedUser2, "predefined2", false, "predefined2")
	flag.StringVar(&MyTgCDNHost, "cdn", "", "cdn_host")
}

func IsTDotMe(me string) bool {
	switch me {
	case "teamgram.me":
		return true
	case "t.me":
		return true
	case TDotMe:
		return true
	}

	return false
}
