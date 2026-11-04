package themes

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestParseColor(t *testing.T) {
	good := map[string]string{
		"#1a1B26":            "#1a1b26",
		"1a1b26":             "#1a1b26",
		"0x1a1b26":           "#1a1b26",
		"#abc":               "#aabbcc",
		"#1a1b26ff":          "#1a1b26",
		"rgb(10, 20, 30)":    "#0a141e",
		"rgba(10,20,30,0.5)": "#0a141e",
		"rgba(f0913fee)":     "#f0913f",
		"  #ffffff ":         "#ffffff",
	}
	for in, want := range good {
		c, err := ParseColor(in)
		if err != nil || c.Hex() != want {
			t.Errorf("ParseColor(%q) = %v, %v; want %s", in, c.Hex(), err, want)
		}
	}
	bad := []string{
		"", "red", "#12345", "#1234567", "rgb(300, 0, 0)", "rgb(1,2)", "url(http://x/y.png)",
		"#fff; background: url(x)", "expression(alert(1))", "var(--x)",
		"rgba(f0913fee) rgba(f0913f55) 45deg", strings.Repeat("a", 100),
	}
	for _, in := range bad {
		if c, err := ParseColor(in); err == nil {
			t.Errorf("ParseColor(%q) = %s, want error", in, c.Hex())
		}
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParsePaletteDialects(t *testing.T) {
	cases := []struct {
		fixture, name                        string
		mode, bg, fg, accent, sel, muted, dk string
	}{
		{"new-colors.toml", "colors.toml", "dark", "#05090a", "#c3d2d0", "#e0913f", "#1c2f33", "#617877", "#010304"},
		{"old-colors.toml", "colors.toml", "", "#000000", "#ffffff", "#ff7edb", "#614d85", "#495495", "#262335"},
		{"alacritty.toml", "alacritty.toml", "", "#214130", "#88a827", "#88a827", "#88a827", "#416828", "#416828"},
	}
	for _, c := range cases {
		p, err := ParsePalette(c.name, readFixture(t, c.fixture))
		if err != nil {
			t.Fatalf("%s: %v", c.fixture, err)
		}
		hex := func(x *RGB) string {
			if x == nil {
				return ""
			}
			return x.Hex()
		}
		got := []string{p.Mode, p.Background.Hex(), p.Foreground.Hex(), hex(p.Accent), hex(p.Selection), hex(p.Muted), hex(p.Darker)}
		want := []string{c.mode, c.bg, c.fg, c.accent, c.sel, c.muted, c.dk}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: field %d = %q, want %q", c.fixture, i, got[i], want[i])
			}
		}
		if mode := p.ResolveMode(); mode != "dark" {
			t.Errorf("%s: mode %s, want dark", c.fixture, mode)
		}
	}
}

func TestParsePaletteRejects(t *testing.T) {
	cases := map[string]string{
		"no colors":     `accent = "#ff0000"`,
		"bad bg":        "background = \"url(x)\"\nforeground = \"#ffffff\"",
		"numeric bg":    "background = 12\nforeground = \"#ffffff\"",
		"invalid toml":  "background = \"#000000\nforeground",
		"css injection": "background = \"#000; } body { display:none\"\nforeground = \"#ffffff\"",
	}
	for name, src := range cases {
		if _, err := ParsePalette("colors.toml", []byte(src)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	big := "background = \"#000000\"\nforeground = \"#ffffff\"\n# " + strings.Repeat("x", MaxThemeFileSize)
	if _, err := ParsePalette("colors.toml", []byte(big)); err == nil {
		t.Error("oversized file accepted")
	}
	// An invalid optional color is skipped, not passed on
	p, err := ParsePalette("colors.toml", []byte("background = \"#000000\"\nforeground = \"#ffffff\"\naccent = \"url(x)\""))
	if err != nil || p.Accent != nil {
		t.Errorf("invalid accent: %v, %v", p.Accent, err)
	}
}

var hexRe = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// checkTokens verifies the guarantees every theme gives: valid colors, ordered ramps and
// readable text.
func checkTokens(t *testing.T, name string, tk Tokens) {
	t.Helper()
	vals := append(tk.Gray[:], tk.Accent[:]...)
	v := tk.Vars
	vals = append(vals, v.PageBg, v.Surface, v.Fg, v.Heading, v.Appbar, v.AppbarFg, v.RowSelected,
		v.RowSelectedHover, v.TextSelBg, v.TextSelFg, v.ScrollTrack, v.ScrollThumb, v.ScrollThumbHover)
	for _, h := range vals {
		if !hexRe.MatchString(h) {
			t.Errorf("%s: invalid color %q", name, h)
		}
	}
	if tk.Mode != "light" && tk.Mode != "dark" {
		t.Errorf("%s: mode %q", name, tk.Mode)
	}
	// Gray ramp goes from light (50) to dark (950)
	for i := 1; i < len(tk.Gray); i++ {
		if MustColor(tk.Gray[i]).ToLab().L > MustColor(tk.Gray[i-1]).ToLab().L+0.02 {
			t.Errorf("%s: gray %d lighter than %d (%s, %s)", name, Steps[i], Steps[i-1], tk.Gray[i], tk.Gray[i-1])
		}
	}
	min := func(what string, a, b string, want float64) {
		if c := Contrast(MustColor(a), MustColor(b)); c < want-0.05 {
			t.Errorf("%s: %s contrast %.2f < %.1f (%s on %s)", name, what, c, want, a, b)
		}
	}
	min("text", v.Fg, v.PageBg, minText)
	min("app bar", v.AppbarFg, v.Appbar, minSecondary)
	min("selected text", v.TextSelFg, v.TextSelBg, minSecondary)
	if tk.Mode == "dark" {
		min("secondary text", tk.Gray[4], v.PageBg, minSecondary)
		min("link", tk.Accent[4], v.PageBg, minSecondary)
		min("selected row", v.Fg, v.RowSelected, minSecondary)
	} else {
		min("secondary text", tk.Gray[6], v.PageBg, minSecondary)
		min("link", tk.Accent[6], v.PageBg, minSecondary)
		min("selected row", v.Fg, v.RowSelected, minSecondary)
	}
}

func TestBuiltins(t *testing.T) {
	seen := map[string]bool{}
	modes := map[string]int{}
	for _, th := range Builtins() {
		if seen[th.Code] {
			t.Errorf("duplicate code %s", th.Code)
		}
		seen[th.Code] = true
		modes[th.Mode]++
		if th.Mode != th.Tokens.Mode {
			t.Errorf("%s: mode %s, tokens %s", th.Code, th.Mode, th.Tokens.Mode)
		}
		if th.Code == DefaultLight || th.Code == DefaultDark {
			continue // values as they are (app.css), checked against app.css below
		}
		checkTokens(t, th.Code, th.Tokens)
	}
	if !seen[DefaultLight] || !seen[DefaultDark] || modes["light"] < 2 || modes["dark"] < 2 {
		t.Errorf("built-ins: %v %v", seen, modes)
	}
}

func TestNormalizeFixturesAndHostilePalettes(t *testing.T) {
	for _, f := range []struct{ fixture, name string }{
		{"new-colors.toml", "colors.toml"}, {"old-colors.toml", "colors.toml"}, {"alacritty.toml", "alacritty.toml"},
	} {
		p, err := ParsePalette(f.name, readFixture(t, f.fixture))
		if err != nil {
			t.Fatal(err)
		}
		checkTokens(t, f.fixture, Normalize(p))
	}
	gray := MustColor("#777777")
	hostile := map[string]Palette{
		"text = background":      {Background: gray, Foreground: gray},
		"dark text on dark":      {Mode: "dark", Background: MustColor("#101010"), Foreground: MustColor("#202020")},
		"light text on light":    {Mode: "light", Background: MustColor("#f0f0f0"), Foreground: MustColor("#e0e0e0")},
		"accent = background":    {Background: MustColor("#202020"), Foreground: MustColor("#eeeeee"), Accent: ptr(MustColor("#202020"))},
		"selection = foreground": {Background: MustColor("#202020"), Foreground: MustColor("#eeeeee"), Selection: ptr(MustColor("#eeeeee"))},
		"declared light on dark": {Mode: "light", Background: MustColor("#000000"), Foreground: MustColor("#ffffff")},
	}
	for name, p := range hostile {
		checkTokens(t, name, Normalize(p))
	}
}

func ptr(c RGB) *RGB { return &c }

// The default themes (GoEasyERP) must be exactly the defaults in app.css: choosing them
// changes nothing; the brand's amber is accent-500, slate the text and app bar.
func TestDefaultTokensMatchAppCSS(t *testing.T) {
	css, err := os.ReadFile("../../../frontend/src/app.css")
	if err != nil {
		t.Skip("app.css not found:", err)
	}
	root := string(css)
	root = root[strings.Index(root, ":root {"):]
	root = root[:strings.Index(root, "}")]
	triplet := func(name string) string {
		m := regexp.MustCompile(`--` + regexp.QuoteMeta(name) + `:\s*(\d+) (\d+) (\d+);`).FindStringSubmatch(root)
		if m == nil {
			t.Fatalf("--%s not in app.css :root", name)
		}
		return m[1] + " " + m[2] + " " + m[3]
	}
	asTriplet := func(h string) string {
		c := MustColor(h)
		return strings.Join([]string{itoa(c.R), itoa(c.G), itoa(c.B)}, " ")
	}
	light := goEasyERPLightTokens()
	for i, step := range Steps {
		if got, want := asTriplet(light.Gray[i]), triplet("gray-"+itoa(step)); got != want {
			t.Errorf("gray-%d: tokens %s, app.css %s", step, got, want)
		}
		if got, want := asTriplet(light.Accent[i]), triplet("accent-"+itoa(step)); got != want {
			t.Errorf("accent-%d: tokens %s, app.css %s", step, got, want)
		}
	}
	for name, h := range map[string]string{
		"page-bg": light.Vars.PageBg, "surface": light.Vars.Surface, "fg": light.Vars.Fg,
		"heading": light.Vars.Heading, "appbar": light.Vars.Appbar, "appbar-fg": light.Vars.AppbarFg,
		"text-sel-bg": light.Vars.TextSelBg, "text-sel-fg": light.Vars.TextSelFg,
		"scroll-track": light.Vars.ScrollTrack, "scroll-thumb": light.Vars.ScrollThumb,
		"scroll-thumb-hover": light.Vars.ScrollThumbHover,
	} {
		if got, want := asTriplet(h), triplet(name); got != want {
			t.Errorf("--%s: tokens %s, app.css %s", name, got, want)
		}
	}
	// the brand colours, and links (accent-600) and secondary text (gray-600) readable on white
	// and on the off-white page
	if light.Accent[5] != "#f2a900" || light.Vars.Fg != "#1f2933" || light.Vars.Appbar != "#1f2933" || light.Vars.PageBg != "#f5f7f8" || light.Gray[4] != "#8a96a3" {
		t.Errorf("not the brand colours: accent-500 %s, fg %s, appbar %s, page %s, gray-400 %s",
			light.Accent[5], light.Vars.Fg, light.Vars.Appbar, light.Vars.PageBg, light.Gray[4])
	}
	for _, bg := range []string{"#ffffff", light.Vars.PageBg} {
		for name, c := range map[string]string{"accent-600": light.Accent[6], "gray-600": light.Gray[6]} {
			if Contrast(MustColor(c), MustColor(bg)) < 4.5 {
				t.Errorf("%s %s on %s: contrast below 4.5", name, c, bg)
			}
		}
	}
	if !strings.Contains(string(css), "--row-selected: 68 80 92;") || goEasyERPDarkTokens().Vars.RowSelected != "#44505c" {
		t.Error("dark selected row differs from app.css")
	}
}

func itoa[T ~int | ~uint8](v T) string { return strconv.Itoa(int(v)) }
