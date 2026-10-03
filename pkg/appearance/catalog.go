// Package appearance is the chat themes and the name and profile colours the
// server offers (#23, #24), as tools/appearance.py wrote them into
// teamgramd/appearance/ (/app/appearance in the image).
//
// Everything here is colours: a theme carries no file, only an accent, the
// colours of one's own bubbles and a gradient behind them, for the day and for
// the night. The numbers are Android's own built-in palettes.
package appearance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/teamgram/proto/mtproto"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Where the image keeps the two files (Dockerfile: teamgramd/ becomes /app/).
const DefaultDir = "/app/appearance"

type Catalog struct {
	themesHash  int64
	themes      []themeEntry
	byEmoticon  map[string]bool
	coloursHash int32
	nameColours []int32
	profiles    []profileEntry
}

type themeEntry struct {
	Id       int64           `json:"id"`
	Emoticon string          `json:"emoticon"`
	Settings []settingsEntry `json:"settings"`
}

type settingsEntry struct {
	BaseTheme       string  `json:"base_theme"`
	AccentColor     int32   `json:"accent_color"`
	MessageColors   []int32 `json:"message_colors"`
	WallpaperColors []int32 `json:"wallpaper_colors"`
}

type colourSet struct {
	Palette []int32 `json:"palette"`
	Bg      []int32 `json:"bg"`
	Story   []int32 `json:"story"`
}

type profileEntry struct {
	ColorId    int32     `json:"color_id"`
	Colors     colourSet `json:"colors"`
	DarkColors colourSet `json:"dark_colors"`
}

func Load(dir string) (*Catalog, error) {
	var themes struct {
		Hash   int64        `json:"hash"`
		Themes []themeEntry `json:"themes"`
	}
	if err := readJSON(filepath.Join(dir, "chat-themes.json"), &themes); err != nil {
		return nil, err
	}
	var colours struct {
		Hash          int32          `json:"hash"`
		NameColors    []int32        `json:"name_colors"`
		ProfileColors []profileEntry `json:"profile_colors"`
	}
	if err := readJSON(filepath.Join(dir, "peer-colors.json"), &colours); err != nil {
		return nil, err
	}
	if themes.Hash == 0 || colours.Hash == 0 || len(themes.Themes) == 0 || len(colours.NameColors) == 0 {
		return nil, fmt.Errorf("%s: a list is empty or has no hash", dir)
	}
	c := &Catalog{
		themesHash:  themes.Hash,
		themes:      themes.Themes,
		byEmoticon:  map[string]bool{},
		coloursHash: colours.Hash,
		nameColours: colours.NameColors,
		profiles:    colours.ProfileColors,
	}
	for _, theme := range c.themes {
		// Android takes the first settings for the day and the second for
		// the night, and crashes on a theme with one (EmojiThemes.java).
		if len(theme.Settings) != 2 || theme.Id == 0 || theme.Emoticon == "" {
			return nil, fmt.Errorf("%s: theme %d %q needs an id, an emoticon and two settings", dir, theme.Id, theme.Emoticon)
		}
		for _, settings := range theme.Settings {
			if len(settings.WallpaperColors) != 4 || baseTheme(settings.BaseTheme) == nil {
				return nil, fmt.Errorf("%s: theme %q needs a known base theme and four background colours", dir, theme.Emoticon)
			}
		}
		c.byEmoticon[theme.Emoticon] = true
	}
	return c, nil
}

func readJSON(path string, into any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (c *Catalog) ThemesHash() int64  { return c.themesHash }
func (c *Catalog) ColoursHash() int32 { return c.coloursHash }

// Offers says whether this is one of the themes, matched as the exact string:
// iOS looks a theme up by the emoji exactly as it was sent.
func (c *Catalog) Offers(emoticon string) bool {
	return c.byEmoticon[emoticon]
}

// ChatThemes is what account.getChatThemes answers with.
func (c *Catalog) ChatThemes() []*mtproto.Theme {
	list := make([]*mtproto.Theme, 0, len(c.themes))
	for _, theme := range c.themes {
		settings := make([]*mtproto.ThemeSettings, 0, len(theme.Settings))
		for variant, s := range theme.Settings {
			settings = append(settings, s.themeSettings(theme.Id*10+int64(variant), variant == 1))
		}
		list = append(list, mtproto.MakeTLTheme(&mtproto.Theme{
			ForChat:    true,
			Id:         theme.Id,
			AccessHash: theme.Id ^ 0x1ce9,
			Slug:       "",
			Title:      "",
			Settings:   settings,
			Emoticon:   &wrapperspb.StringValue{Value: theme.Emoticon},
		}).To_Theme())
	}
	return list
}

// The wallpaper is a gradient with no file behind it, and it always carries
// its settings: Android reads settings.intensity off it without asking
// whether there are any (ChatActivity.java), and four colours, because a
// missing one blends towards black.
func (s settingsEntry) themeSettings(wallpaperId int64, dark bool) *mtproto.ThemeSettings {
	colour := func(i int) *wrapperspb.Int32Value { return &wrapperspb.Int32Value{Value: s.WallpaperColors[i]} }
	return mtproto.MakeTLThemeSettings(&mtproto.ThemeSettings{
		BaseTheme:     baseTheme(s.BaseTheme),
		AccentColor:   s.AccentColor,
		MessageColors: s.MessageColors,
		Wallpaper: mtproto.MakeTLWallPaperNoFile(&mtproto.WallPaper{
			Id:   wallpaperId,
			Dark: dark,
			Settings: mtproto.MakeTLWallPaperSettings(&mtproto.WallPaperSettings{
				BackgroundColor:       colour(0),
				SecondBackgroundColor: colour(1),
				ThirdBackgroundColor:  colour(2),
				FourthBackgroundColor: colour(3),
				// The second colour and the rotation share flags.4, so a
				// reader that sees the one reads the other too. Left out, it
				// read the next object's first four bytes as the rotation and
				// lost its place in everything after.
				Rotation: &wrapperspb.Int32Value{Value: 0},
			}).To_WallPaperSettings(),
		}).To_WallPaper(),
	}).To_ThemeSettings()
}

func baseTheme(name string) *mtproto.BaseTheme {
	switch name {
	case "classic":
		return mtproto.MakeTLBaseThemeClassic(nil).To_BaseTheme()
	case "day":
		return mtproto.MakeTLBaseThemeDay(nil).To_BaseTheme()
	case "night":
		return mtproto.MakeTLBaseThemeNight(nil).To_BaseTheme()
	case "tinted":
		return mtproto.MakeTLBaseThemeTinted(nil).To_BaseTheme()
	case "arctic":
		return mtproto.MakeTLBaseThemeArctic(nil).To_BaseTheme()
	}
	return nil
}

// NameColours lists the seven colours both clients carry, by number only.
func (c *Catalog) NameColours() []*mtproto.Help_PeerColorOption {
	list := make([]*mtproto.Help_PeerColorOption, 0, len(c.nameColours))
	for _, id := range c.nameColours {
		list = append(list, mtproto.MakeTLHelpPeerColorOption(&mtproto.Help_PeerColorOption{ColorId: id}).To_Help_PeerColorOption())
	}
	return list
}

// ProfileColours carries its colours: no client has these built in.
func (c *Catalog) ProfileColours() []*mtproto.Help_PeerColorOption {
	list := make([]*mtproto.Help_PeerColorOption, 0, len(c.profiles))
	for _, p := range c.profiles {
		list = append(list, mtproto.MakeTLHelpPeerColorOption(&mtproto.Help_PeerColorOption{
			ColorId:    p.ColorId,
			Colors:     p.Colors.profileSet(),
			DarkColors: p.DarkColors.profileSet(),
		}).To_Help_PeerColorOption())
	}
	return list
}

func (s colourSet) profileSet() *mtproto.Help_PeerColorSet {
	return mtproto.MakeTLHelpPeerColorProfileSet(&mtproto.Help_PeerColorSet{
		PaletteColors: s.Palette,
		BgColors:      s.Bg,
		StoryColors:   s.Story,
	}).To_Help_PeerColorSet()
}
