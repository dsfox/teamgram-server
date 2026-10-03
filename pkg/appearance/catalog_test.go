package appearance

import (
	"testing"

	"github.com/teamgram/proto/mtproto"
)

const shipped = "../../teamgramd/appearance"

func loadShipped(t *testing.T) *Catalog {
	t.Helper()
	c, err := Load(shipped)
	if err != nil {
		t.Fatalf("the shipped catalog does not load: %v", err)
	}
	return c
}

// Android reads [0] as the day and [1] as the night and crashes with fewer;
// iOS reads the accent's top byte as alpha, so a 24-bit accent is invisible.
func TestEveryThemeHasADayAndANightTheClientsCanDraw(t *testing.T) {
	c := loadShipped(t)
	themes := c.ChatThemes()
	if len(themes) != 7 {
		t.Fatalf("seven themes, got %d", len(themes))
	}
	seen := map[string]bool{}
	for _, theme := range themes {
		e := theme.Emoticon.GetValue()
		if e == "" || seen[e] || e == "❌" || e == "🎨" || e == "🏠" {
			t.Fatalf("%q: an emoji of its own, not one the clients use themselves", e)
		}
		seen[e] = true
		if !c.Offers(e) {
			t.Fatalf("%q is listed and not offered", e)
		}
		if theme.Id == 0 || theme.AccessHash == 0 || theme.Document != nil {
			t.Fatalf("%q: an id, an access hash and no file: %v", e, theme)
		}
		if len(theme.Settings) != 2 {
			t.Fatalf("%q: a day and a night, got %d", e, len(theme.Settings))
		}
		day, night := theme.Settings[0], theme.Settings[1]
		if day.BaseTheme.GetPredicateName() != mtproto.Predicate_baseThemeClassic ||
			night.BaseTheme.GetPredicateName() != mtproto.Predicate_baseThemeTinted {
			t.Fatalf("%q: a light base first and a dark one second: %s, %s", e,
				day.BaseTheme.GetPredicateName(), night.BaseTheme.GetPredicateName())
		}
		for _, s := range theme.Settings {
			if uint32(s.AccentColor)>>24 != 0xFF {
				t.Fatalf("%q: the accent %08x has no alpha, and iOS would draw nothing", e, uint32(s.AccentColor))
			}
			w := s.Wallpaper
			if w.GetPredicateName() != mtproto.Predicate_wallPaperNoFile || w.Id == 0 || w.Settings == nil {
				t.Fatalf("%q: a gradient with no file, an id and its settings: %v", e, w)
			}
			if w.Settings.FourthBackgroundColor == nil || w.Settings.Emoticon != nil {
				t.Fatalf("%q: four colours and no emoticon of its own: %v", e, w.Settings)
			}
			if w.Settings.SecondBackgroundColor != nil && w.Settings.Rotation == nil {
				t.Fatalf("%q: a second colour sets flags.4, and a reader then reads a rotation too", e)
			}
		}
	}
	if c.Offers("🤡") || c.Offers("") {
		t.Fatal("offers exactly the list")
	}
}

func TestTheAnswersEncodeForTheClientsLayers(t *testing.T) {
	c := loadShipped(t)
	answers := []interface {
		Encode(*mtproto.EncodeBuf, int32) error
	}{
		mtproto.MakeTLAccountThemes(&mtproto.Account_Themes{Hash: c.ThemesHash(), Themes: c.ChatThemes()}),
		mtproto.MakeTLAccountThemes(&mtproto.Account_Themes{Hash: c.AppThemesHash(), Themes: c.AppThemes()}),
		mtproto.MakeTLHelpPeerColors(&mtproto.Help_PeerColors{Hash: c.ColoursHash(), Colors: c.NameColours()}),
		mtproto.MakeTLHelpPeerColors(&mtproto.Help_PeerColors{Hash: c.ColoursHash(), Colors: c.ProfileColours()}),
	}
	for _, layer := range []int32{214, 228} {
		for i, answer := range answers {
			x := mtproto.NewEncodeBuf(4096)
			if err := answer.Encode(x, layer); err != nil || x.GetOffset() == 0 {
				t.Fatalf("answer %d does not encode at layer %d: %v", i, layer, err)
			}
		}
	}
}

// The name colours are the clients' own seven, by number; the profile colours
// carry theirs, two of each kind, in both lights: Android cuts any other count
// into the wrong slots and reads a missing dark set as black.
func TestNameColoursAreNumbersAndProfileColoursAreWhole(t *testing.T) {
	c := loadShipped(t)
	names := c.NameColours()
	if len(names) != 7 {
		t.Fatalf("seven name colours, got %d", len(names))
	}
	for i, n := range names {
		if n.ColorId != int32(i) || n.Colors != nil {
			t.Fatalf("name colour %d: the number only: %v", i, n)
		}
	}
	profiles := c.ProfileColours()
	if len(profiles)%8 != 0 || len(profiles) == 0 {
		t.Fatalf("Android draws eight to a row, got %d", len(profiles))
	}
	for _, p := range profiles {
		for _, set := range []*mtproto.Help_PeerColorSet{p.Colors, p.DarkColors} {
			if set.GetPredicateName() != mtproto.Predicate_help_peerColorProfileSet ||
				len(set.PaletteColors) != 2 || len(set.BgColors) != 2 || len(set.StoryColors) != 2 {
				t.Fatalf("profile colour %d: two of each, day and night: %v", p.ColorId, set)
			}
		}
	}
	if c.ThemesHash() == 0 || c.ColoursHash() == 0 {
		t.Fatal("zero is what a client holding nothing asks with")
	}
}

// Android's Chat Settings picker draws the default themes of account.getThemes,
// takes settings 0..3 as "Blue", "Day", "Night" and "Dark Blue", and leaves out
// a theme with fewer than four (#223).
func TestAppThemesAreDefaultAndCarryTheFourBasesInOrder(t *testing.T) {
	c := loadShipped(t)
	apps := c.AppThemes()
	if len(apps) != 7 || c.AppThemesHash() == 0 || c.AppThemesHash() == c.ThemesHash() {
		t.Fatalf("seven app themes with a hash of their own, got %d and %d", len(apps), c.AppThemesHash())
	}
	chatIds := map[int64]bool{}
	for _, theme := range c.ChatThemes() {
		chatIds[theme.Id] = true
	}
	bases := []string{mtproto.Predicate_baseThemeClassic, mtproto.Predicate_baseThemeDay,
		mtproto.Predicate_baseThemeNight, mtproto.Predicate_baseThemeTinted}
	for _, theme := range apps {
		e := theme.Emoticon.GetValue()
		if !theme.Default || theme.ForChat || theme.Id == 0 || chatIds[theme.Id] {
			t.Fatalf("%q: a default app theme with an id no chat theme has: %v", e, theme)
		}
		if len(theme.Settings) != 4 {
			t.Fatalf("%q: four settings, got %d", e, len(theme.Settings))
		}
		for i, s := range theme.Settings {
			if s.BaseTheme.GetPredicateName() != bases[i] {
				t.Fatalf("%q: settings %d is %s, Android reads it as %s", e, i, s.BaseTheme.GetPredicateName(), bases[i])
			}
			if uint32(s.AccentColor)>>24 != 0xFF || s.Wallpaper.GetSettings().GetFourthBackgroundColor() == nil {
				t.Fatalf("%q %s: an opaque accent and four colours behind", e, bases[i])
			}
			if dark := i >= 2; s.Wallpaper.Dark != dark {
				t.Fatalf("%q %s: the wallpaper says dark=%v", e, bases[i], s.Wallpaper.Dark)
			}
		}
	}
}

func TestAnAppThemeOutOfOrderIsRefused(t *testing.T) {
	inOrder := []settingsEntry{{BaseTheme: "classic"}, {BaseTheme: "day"}, {BaseTheme: "night"}, {BaseTheme: "tinted"}}
	if !appBasesInOrder(inOrder) {
		t.Fatal("the four bases in order are refused")
	}
	swapped := []settingsEntry{{BaseTheme: "classic"}, {BaseTheme: "night"}, {BaseTheme: "day"}, {BaseTheme: "tinted"}}
	if appBasesInOrder(swapped) || appBasesInOrder(inOrder[:2]) {
		t.Fatal("a theme Android would read wrongly, or leave out, is let through")
	}
}
