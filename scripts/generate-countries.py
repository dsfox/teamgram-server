"""Generates the country list for help.getCountriesList.

In the open teamgram build this method is a stub returning an empty list
("license key required"), which keeps the country picker from opening at sign-in.
The data comes from phonenumbers (dialling codes and number patterns) and
pycountry (names, Russian ones included).

Usage: python3 server/scripts/generate-countries.py > server/pkg/countries/countries.go
"""
import gettext
import json

import phonenumbers
import pycountry
from phonenumbers import phonemetadata

HEADER = '''// Package countries is the country reference data for help.getCountriesList.
//
// Generated file: server/scripts/generate-countries.py
// Sources: the phonenumbers library (codes and number lengths) and pycountry (names).
package countries

// Country is a country in the shape the client expects on the sign-in screen.
type Country struct {
	ISO2        string   // country code, for example RS
	NameEN      string   // name in English
	NameRU      string   // name in Russian
	CountryCode string   // phone code without the plus, for example 381
	Patterns    []string // number patterns: X stands for any digit
}

// All holds every country, sorted by the English name.
var All = []Country{
'''

FOOTER = "}\n"


def patterns_for(region: str) -> list:
    """Patterns like 'XX XXX XXXX': the client uses them to hint the number format."""
    metadata = phonemetadata.PhoneMetadata.metadata_for_region(region)
    if metadata is None or metadata.mobile is None:
        return []
    lengths = sorted(set(metadata.mobile.possible_length or []))
    return ["X" * length for length in lengths if length]


def main():
    russian = gettext.translation("iso3166-1", pycountry.LOCALES_DIR, languages=["ru"])

    rows = []
    for region in sorted(phonenumbers.SUPPORTED_REGIONS):
        country = pycountry.countries.get(alpha_2=region)
        if country is None:
            continue
        name_en = getattr(country, "common_name", country.name)
        name_ru = russian.gettext(country.name)
        code = phonenumbers.country_code_for_region(region)
        if not code:
            continue
        patterns = ", ".join(json.dumps(p) for p in patterns_for(region))
        # via json.dumps, otherwise apostrophes in names (Côte d'Ivoire) break the literal
        rows.append((name_en, f"\t{{ISO2: {json.dumps(region)}, NameEN: {json.dumps(name_en)}, "
                              f"NameRU: {json.dumps(name_ru, ensure_ascii=False)}, "
                              f"CountryCode: {json.dumps(str(code))}, "
                              f"Patterns: []string{{{patterns}}}}},"))

    print(HEADER, end="")
    for _, row in sorted(rows):
        print(row)
    print(FOOTER, end="")


if __name__ == "__main__":
    main()
