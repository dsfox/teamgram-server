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

// Platforms name the same sentence differently: the iOS pack calls it
// Common.Cancel and the Android one calls it Cancel. Serving one to the other is
// not a partial translation, it is none at all - eleven thousand strings arrive
// and the client recognises no key in them, so the interface stays English and
// looks as though choosing a language did nothing.
//
// The file for iOS keeps its plain name because it was there first;
// anything else is <code>.<platform>.json.
func packPath(langCode, platform string) string {
	if platform == "" || platform == "ios" {
		return filepath.Join(Dir, langCode+".json")
	}
	return filepath.Join(Dir, langCode+"."+platform+".json")
}

// platforms we carry a pack for; anything else is served the iOS one, which is
// what the older clients asked for by not asking.
var platforms = []string{"ios", "android"}

func load() {
	packs = make(map[string]*pack)
	for _, language := range Available {
		if language.Code == "en" {
			continue // English is built into the client
		}
		for _, platform := range platforms {
			path := packPath(language.Code, platform)
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
			packs[key(language.Code, platform)] = parsed
			logx.Infof("translation loaded: %s/%s, %d strings",
				language.Code, platform, len(parsed.Strings)+len(parsed.Plurals))
		}
	}
}

// key is how a pack is looked up: language and platform together, because the
// same language differs between them.
func key(langCode, platform string) string {
	if platform != "android" {
		platform = "ios"
	}
	return langCode + "/" + platform
}

// resetForTest makes the loader read again. The packs are read once per process,
// which is right for a server and inconvenient for a test that wants to point
// Dir somewhere else.
func resetForTest() {
	loadOnce = sync.Once{}
	packs = nil
}

// Difference answers langpack.getDifference and langpack.getLangPack.
//
// fromVersion is what the client already has, and it matters: a client that is
// up to date must be told so, with an empty answer. Sending it the whole pack
// again does not update anything, so it asks again - and again. That is what
// happened: four requests a second, each answered with eleven thousand strings,
// for as long as the app was open, with everything else the client wanted to do
// queued behind it. A message that had already arrived by push took the best
// part of a minute to appear in the chat.
//
// We keep no history, so any client that is behind gets the whole pack. That is
// honest and cheap; what is not allowed is handing it over to a client that
// asked whether anything had changed.
// Version is what help.getConfig has to advertise: the version of the pack this
// client would be given.
//
// It matters more than it looks. A client only asks for a newer pack when the
// config says there is one - it compares the version it holds against the one
// help.getConfig names, and asks for nothing if that is not larger. So a config
// naming any number unrelated to the packs means a phone that already has a pack
// never learns of a new one, for ever. Ours named a constant inherited from
// upstream, 262834, while the packs were in the sixty millions: every phone
// updated once, on the launch that fetched its first pack, and never again.
// Fresh installs hid it, because a phone with no pack at all asks regardless.
//
// Zero for a language we carry no pack for, English included, which is also what
// such a client holds - so it asks for nothing, correctly.
func Version(langCode, platform string) int32 {
	loadOnce.Do(load)

	loaded, ok := packs[key(langCode, platform)]
	if !ok {
		return 0
	}
	return loaded.Version
}

func Difference(langCode, platform string, fromVersion int32) *mtproto.LangPackDifference {
	loadOnce.Do(load)

	loaded, ok := packs[key(langCode, platform)]
	if !ok {
		return mtproto.MakeTLLangPackDifference(&mtproto.LangPackDifference{
			LangCode: langCode,
			Strings:  []*mtproto.LangPackString{},
		}).To_LangPackDifference()
	}

	if fromVersion >= loaded.Version {
		// Nothing new. The client stops asking.
		return mtproto.MakeTLLangPackDifference(&mtproto.LangPackDifference{
			LangCode:    langCode,
			FromVersion: fromVersion,
			Version:     loaded.Version,
			Strings:     []*mtproto.LangPackString{},
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
		FromVersion: fromVersion,
		Version:     loaded.Version,
		Strings:     list,
	}).To_LangPackDifference()
}

// Strings returns only the requested keys.
func Strings(langCode, platform string, keys []string) []*mtproto.LangPackString {
	loadOnce.Do(load)

	list := make([]*mtproto.LangPackString, 0, len(keys))
	loaded, ok := packs[key(langCode, platform)]
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
// A language is offered when we can serve it to somebody: a pack for either
// platform is enough to put it in the picker.
func Loaded(langCode string) bool {
	loadOnce.Do(load)
	for _, platform := range platforms {
		if _, ok := packs[key(langCode, platform)]; ok {
			return true
		}
	}
	return false
}
