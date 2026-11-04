package themes

import (
	"embed"
	"fmt"
	"sync"
)

// Theme is a color theme the user can choose.
type Theme struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Mode    string `json:"mode"`   // "light" or "dark"
	Source  string `json:"source"` // "builtin" or "github"
	Author  string `json:"author,omitempty"`
	RepoURL string `json:"repo_url,omitempty"`
	// Version is the commit an installed theme is pinned to; asset URLs carry it, so a
	// refreshed theme's images are not taken from the browser cache
	Version    string `json:"version,omitempty"`
	Wallpapers int    `json:"wallpapers"` // number of wallpapers (assets bg-1 … bg-n)
	Tokens     Tokens `json:"tokens"`
}

// The default themes: the GoEasyERP brand colours, the defaults in app.css; the classic ERP
// palette (teal) stays as Classic Light/Dark.
const (
	DefaultLight = "GOEASYERP-LIGHT"
	DefaultDark  = "GOEASYERP-DARK"
	ClassicLight = "CLASSIC-LIGHT"
	ClassicDark  = "CLASSIC-DARK"
)

//go:embed builtin/*.toml
var builtinFS embed.FS

// Built-in themes besides GoEasyERP and Classic: Omarchy-style colors.toml files in builtin/,
// normalized like a theme installed from GitHub.
var builtinFiles = []struct{ code, name, file string }{
	{"TOKYO-NIGHT", "Tokyo Night", "tokyo-night.toml"},
	{"NORD", "Nord", "nord.toml"},
	{"GRUVBOX", "Gruvbox", "gruvbox.toml"},
	{"CATPPUCCIN-LATTE", "Catppuccin Latte", "catppuccin-latte.toml"},
	{"SOLARIZED-LIGHT", "Solarized Light", "solarized-light.toml"},
}

var (
	builtinOnce   sync.Once
	builtinThemes []Theme
)

// Builtins returns the built-in themes: GoEasyERP Light/Dark, Classic Light/Dark, then the
// others.
func Builtins() []Theme {
	builtinOnce.Do(func() {
		builtinThemes = []Theme{
			{Code: DefaultLight, Name: "GoEasyERP Light", Mode: "light", Source: "builtin", Tokens: goEasyERPLightTokens()},
			{Code: DefaultDark, Name: "GoEasyERP Dark", Mode: "dark", Source: "builtin", Tokens: goEasyERPDarkTokens()},
			{Code: ClassicLight, Name: "Classic Light", Mode: "light", Source: "builtin", Tokens: classicLightTokens()},
			{Code: ClassicDark, Name: "Classic Dark", Mode: "dark", Source: "builtin", Tokens: classicDarkTokens()},
		}
		for _, b := range builtinFiles {
			data, err := builtinFS.ReadFile("builtin/" + b.file)
			if err != nil {
				panic(fmt.Sprintf("built-in theme %s: %v", b.file, err))
			}
			p, err := ParsePalette("colors.toml", data)
			if err != nil {
				panic(fmt.Sprintf("built-in theme %s: %v", b.file, err))
			}
			t := Normalize(p)
			builtinThemes = append(builtinThemes, Theme{Code: b.code, Name: b.name, Mode: t.Mode, Source: "builtin", Tokens: t})
		}
	})
	return builtinThemes
}

// Builtin returns the built-in theme with the code.
func Builtin(code string) (Theme, bool) {
	for _, t := range Builtins() {
		if t.Code == code {
			return t, true
		}
	}
	return Theme{}, false
}
