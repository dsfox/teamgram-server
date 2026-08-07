// Interface strings the server serves to the client.
//
// The client carries the English strings inside itself and fetches other
// languages from the server via langpack.getLangPack and langpack.getDifference.
// Untranslated keys fall back to English, so a translation can be filled in
// gradually — starting with what is seen most often.
package langpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/teamgram/proto/mtproto"
	"github.com/zeromicro/go-zero/core/logx"
)

// pluralForms are the plural forms; Russian has four of them plus "many".
// The keys match the fields of langPackStringPluralized.
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

// Dir is where translation files live; set at startup, next to the binary by default.
var Dir = "../langpack"

func load() {
	packs = make(map[string]*pack)
	for _, language := range Available {
		if language.Code == "en" {
			continue // English is built into the client
		}
		path := filepath.Join(Dir, language.Code+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			logx.Errorf("translation not read (%s): %v", path, err)
			continue
		}
		parsed := new(pack)
		if err = json.Unmarshal(data, parsed); err != nil {
			logx.Errorf("translation is corrupt (%s): %v", path, err)
			continue
		}
		packs[language.Code] = parsed
		logx.Infof("translation loaded: %s, %d strings", language.Code, len(parsed.Strings)+len(parsed.Plurals))
	}
}

// Difference returns the whole language pack: ours are small, and handing over
// everything at once is cheaper than keeping a version history.
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

// Strings returns only the requested keys.
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
		// The client expects an answer per key: a gap reads as "not loaded yet"
		list = append(list, mtproto.MakeTLLangPackStringDeleted(&mtproto.LangPackString{
			Key: key,
		}).To_LangPackString())
	}
	return list
}

// Loaded reports whether a translation exists for the language — that is what
// puts it into the picker: offering a language without strings is pointless.
func Loaded(langCode string) bool {
	loadOnce.Do(load)
	_, ok := packs[langCode]
	return ok
}
