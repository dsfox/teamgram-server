// Package langpack lists the interface languages the server advertises.
//
// The client asks for the list via langpack.getLanguages. In the open teamgram
// build the answer is empty and the language screen spins forever.
//
// Listed here are the languages whose strings exist in the app itself. Adding a
// language the client does not carry is not allowed: the server would have to
// serve its translation (langpack.getLangPack) and there is nothing to serve —
// picking such a language leaves the user with a blank interface.
package langpack

import "github.com/teamgram/proto/mtproto"

type Language struct {
	Code       string
	Name       string
	NativeName string
	PluralCode string
}

// Available lists the languages built into the client.
var Available = []Language{
	{Code: "en", Name: "English", NativeName: "English", PluralCode: "en"},
	{Code: "ru", Name: "Russian", NativeName: "Русский", PluralCode: "ru"},
}

// Language returns one entry for langpack.getLanguage. The client asks for it
// before it downloads anything: without an answer the language picker spins
// forever and the switch never happens, which is how a missing method looks
// from the outside.
func LanguageByCode(code string) *mtproto.LangPackLanguage {
	for _, l := range Languages() {
		if l.GetLangCode() == code {
			return l
		}
	}
	return nil
}

// Languages returns the list for langpack.getLanguages.
func Languages() []*mtproto.LangPackLanguage {
	list := make([]*mtproto.LangPackLanguage, 0, len(Available))
	for _, language := range Available {
		// A language without strings must not be offered: choosing it leaves the
		// user with blank labels. English is built into the client.
		if language.Code != "en" && !Loaded(language.Code) {
			continue
		}
		list = append(list, mtproto.MakeTLLangPackLanguage(&mtproto.LangPackLanguage{
			Official:        true,
			LangCode:        language.Code,
			Name:            language.Name,
			NativeName:      language.NativeName,
			PluralCode:      language.PluralCode,
			BaseLangCode:    nil,
			StringsCount:    0,
			TranslatedCount: 0,
			TranslationsUrl: "",
		}).To_LangPackLanguage())
	}
	return list
}
