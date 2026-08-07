// Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
//  All rights reserved.
//
// Author: teamgramio (teamgram.io@gmail.com)
//

package core

import (
	"strings"

	"github.com/teamgram/proto/mtproto"
	"github.com/teamgram/teamgram-server/pkg/countries"
)

// HelpGetCountriesList
// help.getCountriesList#735787a8 lang_code:string hash:int = help.CountriesList;
//
// Upstream returns an empty list here ("license key required"), which leaves the
// country picker on the sign-in screen unable to open. The reference data lives
// in pkg/countries.
func (c *ConfigurationCore) HelpGetCountriesList(in *mtproto.TLHelpGetCountriesList) (*mtproto.Help_CountriesList, error) {
	russian := strings.HasPrefix(strings.ToLower(in.GetLangCode()), "ru")

	list := make([]*mtproto.Help_Country, 0, len(countries.All))
	for _, country := range countries.All {
		name := country.NameEN
		if russian {
			name = country.NameRU
		}

		list = append(list, mtproto.MakeTLHelpCountry(&mtproto.Help_Country{
			Hidden:      false,
			Iso2:        country.ISO2,
			DefaultName: country.NameEN,
			Name:        mtproto.MakeFlagsString(name),
			CountryCodes: []*mtproto.Help_CountryCode{
				mtproto.MakeTLHelpCountryCode(&mtproto.Help_CountryCode{
					CountryCode: country.CountryCode,
					Prefixes:    nil,
					Patterns:    country.Patterns,
				}).To_Help_CountryCode(),
			},
		}).To_Help_Country())
	}

	return mtproto.MakeTLHelpCountriesList(&mtproto.Help_CountriesList{
		Countries: list,
		Hash:      0,
	}).To_Help_CountriesList(), nil
}
