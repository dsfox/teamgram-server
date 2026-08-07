// Package langpack — языки интерфейса, о которых сервер сообщает клиенту.
//
// Клиент спрашивает список методом langpack.getLanguages. В открытой версии
// teamgram ответ пустой, и экран выбора языка висит в загрузке навсегда.
//
// Здесь перечислены языки, строки которых есть в самом приложении. Языки,
// которых нет в клиенте, добавлять сюда нельзя: сервер должен будет отдать
// для них перевод (langpack.getLangPack), а отдавать нечего — выбрав такой
// язык, пользователь получит интерфейс из пустых строк.
package langpack

import "github.com/teamgram/proto/mtproto"

type Language struct {
	Code       string
	Name       string
	NativeName string
	PluralCode string
}

// Available — языки, встроенные в клиент.
var Available = []Language{
	{Code: "en", Name: "English", NativeName: "English", PluralCode: "en"},
	{Code: "ru", Name: "Russian", NativeName: "Русский", PluralCode: "ru"},
}

// Languages возвращает список для langpack.getLanguages.
func Languages() []*mtproto.LangPackLanguage {
	list := make([]*mtproto.LangPackLanguage, 0, len(Available))
	for _, language := range Available {
		// Язык без строк показывать нельзя: выбрав его, пользователь получит
		// интерфейс из пустых надписей. Английский встроен в клиент.
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
