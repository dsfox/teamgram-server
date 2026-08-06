"""Генерирует список стран для help.getCountriesList.

В открытой версии teamgram этот метод — заглушка, отдающая пустой список
(«требуется лицензионный ключ»), из-за чего в клиенте не открывается выбор
страны при входе. Данные берём из phonenumbers (телефонные коды и шаблоны
номеров) и pycountry (названия, включая русские).

Запуск: python3 server/scripts/generate-countries.py > server/pkg/countries/countries.go
"""
import gettext
import json

import phonenumbers
import pycountry
from phonenumbers import phonemetadata

HEADER = '''// Package countries — справочник стран для help.getCountriesList.
//
// Файл сгенерирован: server/scripts/generate-countries.py
// Данные: библиотеки phonenumbers (коды и длины номеров) и pycountry (названия).
package countries

// Country — страна в том виде, в каком её ждёт клиент на экране входа.
type Country struct {
	ISO2        string   // код страны, например RS
	NameEN      string   // название по-английски
	NameRU      string   // название по-русски
	CountryCode string   // телефонный код без плюса, например 381
	Patterns    []string // шаблоны номера: X — любая цифра
}

// All — все страны, отсортированы по английскому названию.
var All = []Country{
'''

FOOTER = "}\n"


def patterns_for(region: str) -> list:
    """Шаблоны вида 'XX XXX XXXX' — по ним клиент подсказывает формат номера."""
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
        # через json.dumps, иначе апострофы в названиях (Côte d'Ivoire) ломают литерал
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
