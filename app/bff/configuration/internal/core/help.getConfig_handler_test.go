package core

import "testing"

// Which language a phone is offered before anybody chooses one. The rule lives
// on the server so that both clients make the same decision, and it is worth a
// test because the answer is a product decision rather than a technical one:
// Russian where it is what people read, English everywhere else.
//
// The config file this handler starts from suggested "classic-zh-cn" to
// everyone, inherited from upstream, until this was written.
func TestSuggestedLanguage(t *testing.T) {
	cases := []struct {
		client string
		want   string
	}{
		{"ru", "ru"},
		{"ru-RU", "ru"},
		{"ru_RU", "ru"},
		{"RU", "ru"},
		{"uk", "ru"},
		{"uk-UA", "ru"},
		{"kk", "ru"},
		{"kk-KZ", "ru"},
		{"be", "ru"},
		{"be-BY", "ru"},

		{"en", "en"},
		{"en-US", "en"},
		{"de", "en"},
		{"fr-FR", "en"},
		{"zh-CN", "en"},
		{"", "en"},
	}

	for _, c := range cases {
		if got := suggestedLanguage(c.client); got != c.want {
			t.Errorf("a client reporting %q is offered %q, expected %q", c.client, got, c.want)
		}
	}
}
