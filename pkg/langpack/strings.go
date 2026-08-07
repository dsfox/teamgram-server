// Строки интерфейса, которые сервер отдаёт клиенту.
//
// Клиент содержит английские строки внутри себя, а остальные языки берёт
// с сервера методами langpack.getLangPack и langpack.getDifference.
// Непереведённые ключи клиент показывает по-английски, поэтому перевод
// можно наполнять постепенно — сначала то, что видно чаще всего.
package langpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/teamgram/proto/mtproto"
	"github.com/zeromicro/go-zero/core/logx"
)

// pluralForms — формы множественного числа; в русском их четыре плюс «много».
// Ключи совпадают с полями langPackStringPluralized.
type pluralForms struct {
	Zero  string `json:"zero,omitempty"`
	One   string `json:"one,omitempty"`
	Two   string `json:"two,omitempty"`
	Few   string `json:"few,omitempty"`
	Many  string `json:"many,omitempty"`
	Other string `json:"other,omitempty"`
}

type pack struct {
	Version int32                  `json:"version"`
	Strings map[string]string      `json:"strings"`
	Plurals map[string]pluralForms `json:"plurals"`
}

var (
	loadOnce sync.Once
	packs    map[string]*pack
)

// Dir — где лежат файлы перевода; задаётся при старте, по умолчанию рядом с бинарником.
var Dir = "../langpack"

func load() {
	packs = make(map[string]*pack)
	for _, language := range Available {
		if language.Code == "en" {
			continue // английский встроен в клиент
		}
		path := filepath.Join(Dir, language.Code+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			logx.Errorf("перевод не прочитан (%s): %v", path, err)
			continue
		}
		parsed := new(pack)
		if err = json.Unmarshal(data, parsed); err != nil {
			logx.Errorf("перевод повреждён (%s): %v", path, err)
			continue
		}
		packs[language.Code] = parsed
		logx.Infof("перевод загружен: %s, строк %d", language.Code, len(parsed.Strings)+len(parsed.Plurals))
	}
}

// Difference возвращает строки языка целиком: пакеты у нас небольшие,
// и отдать всё разом дешевле, чем вести историю версий.
func Difference(langCode string) *mtproto.LangPackDifference {
	loadOnce.Do(load)

	loaded, ok := packs[langCode]
	if !ok {
		return mtproto.MakeTLLangPackDifference(&mtproto.LangPackDifference{
			LangCode: langCode,
			Strings:  []*mtproto.LangPackString{},
		}).To_LangPackDifference()
	}

	list := make([]*mtproto.LangPackString, 0, len(loaded.Strings)+len(loaded.Plurals))
	for key, value := range loaded.Strings {
		list = append(list, mtproto.MakeTLLangPackString(&mtproto.LangPackString{
			Key:   key,
			Value: value,
		}).To_LangPackString())
	}
	for key, forms := range loaded.Plurals {
		list = append(list, mtproto.MakeTLLangPackStringPluralized(&mtproto.LangPackString{
			Key:        key,
			ZeroValue:  mtproto.MakeFlagsString(forms.Zero),
			OneValue:   mtproto.MakeFlagsString(forms.One),
			TwoValue:   mtproto.MakeFlagsString(forms.Two),
			FewValue:   mtproto.MakeFlagsString(forms.Few),
			ManyValue:  mtproto.MakeFlagsString(forms.Many),
			OtherValue: forms.Other,
		}).To_LangPackString())
	}

	return mtproto.MakeTLLangPackDifference(&mtproto.LangPackDifference{
		LangCode:    langCode,
		FromVersion: 0,
		Version:     loaded.Version,
		Strings:     list,
	}).To_LangPackDifference()
}

// Strings возвращает только запрошенные ключи.
func Strings(langCode string, keys []string) []*mtproto.LangPackString {
	loadOnce.Do(load)

	list := make([]*mtproto.LangPackString, 0, len(keys))
	loaded, ok := packs[langCode]
	if !ok {
		return list
	}

	for _, key := range keys {
		if value, found := loaded.Strings[key]; found {
			list = append(list, mtproto.MakeTLLangPackString(&mtproto.LangPackString{
				Key:   key,
				Value: value,
			}).To_LangPackString())
			continue
		}
		if forms, found := loaded.Plurals[key]; found {
			list = append(list, mtproto.MakeTLLangPackStringPluralized(&mtproto.LangPackString{
				Key:        key,
				ZeroValue:  mtproto.MakeFlagsString(forms.Zero),
				OneValue:   mtproto.MakeFlagsString(forms.One),
				TwoValue:   mtproto.MakeFlagsString(forms.Two),
				FewValue:   mtproto.MakeFlagsString(forms.Few),
				ManyValue:  mtproto.MakeFlagsString(forms.Many),
				OtherValue: forms.Other,
			}).To_LangPackString())
			continue
		}
		// Клиент ждёт ответ на каждый ключ: пропуск он трактует как «ещё не загружено»
		list = append(list, mtproto.MakeTLLangPackStringDeleted(&mtproto.LangPackString{
			Key: key,
		}).To_LangPackString())
	}
	return list
}

// Loaded сообщает, есть ли перевод для языка — по этому признаку язык
// попадает в список выбора: показывать язык без строк бессмысленно.
func Loaded(langCode string) bool {
	loadOnce.Do(load)
	_, ok := packs[langCode]
	return ok
}
