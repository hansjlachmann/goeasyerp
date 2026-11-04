// Package themerepo installs color themes from GitHub (Omarchy theme repositories) and
// refreshes them. Nothing from a repository is executed or passed on as typed: the theme
// file gives colors (parsed and normalized by backend/foundation/themes), the images are
// re-encoded, and everything is pinned to the commit it was read from.
package themerepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/themes"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// MaxInstalledThemes caps the themes installed from GitHub.
const MaxInstalledThemes = 50

// Errors of an install
var (
	ErrBusy         = errors.New("another theme is being installed, try again in a moment")
	ErrTooMany      = fmt.Errorf("at most %d themes can be installed", MaxInstalledThemes)
	ErrOwnerBlocked = errors.New("themes from this GitHub owner are not allowed here (THEME_ALLOWED_OWNERS)")
	ErrNotInstalled = errors.New("theme is not installed from GitHub")
	ErrBuiltin      = errors.New("built-in themes cannot be changed")
	ErrCodeTaken    = errors.New("the theme's code is taken by another repository")
)

// installing lets one install/refresh run at a time: each downloads and decodes up to
// 30 MB of images.
var installing sync.Mutex

// Result is what an install or refresh did.
type Result struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Mode      string `json:"mode"`
	Commit    string `json:"commit"`
	Unchanged bool   `json:"unchanged"` // refresh: the repository has no new commit
}

// Installer installs themes into the database.
type Installer struct {
	DB      *sql.DB
	DBType  database.DBType
	Fetcher *themes.Fetcher
	// AllowedOwners limits installs to these GitHub owners (lower case); empty = any
	AllowedOwners map[string]bool
}

// NewInstaller returns an installer for github.com; THEME_ALLOWED_OWNERS (comma
// separated) restricts the GitHub owners themes may come from.
func NewInstaller(db *sql.DB, dbType database.DBType) *Installer {
	in := &Installer{DB: db, DBType: dbType, Fetcher: themes.NewFetcher()}
	v := strings.TrimSpace(os.Getenv("THEME_ALLOWED_OWNERS"))
	if v != "" {
		in.AllowedOwners = map[string]bool{}
		for _, o := range strings.Split(v, ",") {
			o = strings.ToLower(strings.TrimSpace(o))
			if o != "" {
				in.AllowedOwners[o] = true
			}
		}
	}
	return in
}

// Install adds the theme of a GitHub repository URL to the user's themes. A repository that
// is installed already (by any user) is only added to the user's themes — the files are
// stored once — without downloading it again (Refresh updates it).
func (in *Installer) Install(ctx context.Context, repoURL, userID string) (Result, error) {
	repo, err := themes.ParseRepoURL(repoURL)
	if err != nil {
		return Result{}, err
	}
	if in.AllowedOwners != nil && !in.AllowedOwners[strings.ToLower(repo.Owner)] {
		return Result{}, ErrOwnerBlocked
	}
	if !installing.TryLock() {
		return Result{}, ErrBusy
	}
	defer installing.Unlock()

	code, existing, err := in.codeFor(repo)
	if err != nil {
		return Result{}, err
	}
	if existing {
		var rec gtables.Theme
		rec.InitWithDBType(in.DB, "", in.DBType)
		if !rec.Get(code) {
			return Result{}, ErrNotInstalled
		}
		err := tables.AddUserTheme(in.DB, in.DBType, userID, code)
		if err != nil {
			return Result{}, err
		}
		return Result{Code: code, Name: rec.Name.String(), Mode: modeName(rec.Mode), Commit: rec.Commit_sha.String(), Unchanged: true}, nil
	}
	if in.count() >= MaxInstalledThemes {
		return Result{}, ErrTooMany
	}
	return in.install(ctx, repo, code, userID, "")
}

// Refresh downloads the theme again from its repository (and branch); nothing changes when
// the repository has no new commit.
func (in *Installer) Refresh(ctx context.Context, code, userID string) (Result, error) {
	_, ok := themes.Builtin(code)
	if ok {
		return Result{}, ErrBuiltin
	}
	var rec gtables.Theme
	rec.InitWithDBType(in.DB, "", in.DBType)
	if !rec.Get(code) {
		return Result{}, ErrNotInstalled
	}
	repo, err := themes.ParseRepoURL(rec.Repo_url.String())
	if err != nil {
		return Result{}, err
	}
	if !installing.TryLock() {
		return Result{}, ErrBusy
	}
	defer installing.Unlock()
	return in.install(ctx, repo, code, userID, rec.Commit_sha.String())
}

func (in *Installer) install(ctx context.Context, repo themes.Repo, code, userID, currentCommit string) (Result, error) {
	d, err := in.Fetcher.Fetch(ctx, repo)
	if err != nil {
		return Result{}, err
	}
	name := displayName(repo.Name)
	if d.CommitSHA == currentCommit {
		var rec gtables.Theme
		rec.InitWithDBType(in.DB, "", in.DBType)
		_ = rec.Get(code)
		return Result{Code: code, Name: rec.Name.String(), Mode: modeName(rec.Mode), Commit: d.CommitSHA, Unchanged: true}, nil
	}

	palette, err := themes.ParsePalette(d.ColorsName, d.Colors)
	if err != nil {
		return Result{}, err
	}
	tokens := themes.Normalize(palette)
	tokensJSON, err := json.Marshal(tokens)
	if err != nil {
		return Result{}, err
	}

	// Images: re-encoded; an image that is not acceptable is skipped, not stored
	var assets []asset
	var thumbSource *themes.Image
	if d.Preview != nil {
		img, err := themes.ProcessImage(d.Preview, themes.PreviewWidth)
		if err == nil {
			assets = append(assets, asset{"preview", img})
			thumbSource = &img
		}
	}
	n := 0
	for _, raw := range d.Backgrounds {
		img, err := themes.ProcessImage(raw, themes.WallpaperWidth)
		if err != nil {
			continue
		}
		n++
		assets = append(assets, asset{fmt.Sprintf("bg-%d", n), img})
		tb, err := themes.Thumbnail(img)
		if err == nil {
			assets = append(assets, asset{fmt.Sprintf("tb-%d", n), tb}) // for the wallpaper picker
		}
		if thumbSource == nil {
			thumbSource = &img
		}
	}
	if thumbSource != nil {
		thumb, err := themes.Thumbnail(*thumbSource)
		if err == nil {
			assets = append(assets, asset{"thumb", thumb})
		}
	}

	tx, err := in.DB.Begin()
	if err != nil {
		return Result{}, err
	}
	err = in.save(tx, repo, code, name, userID, d.CommitSHA, string(tokensJSON), tokens.Mode, n, assets)
	if err != nil {
		_ = ftables.Rollback(tx)
		return Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return Result{}, err
	}
	return Result{Code: code, Name: name, Mode: tokens.Mode, Commit: d.CommitSHA}, nil
}

type asset struct {
	name string
	img  themes.Image
}

func (in *Installer) save(tx *sql.Tx, repo themes.Repo, code, name, userID, commit, tokensJSON, mode string, wallpapers int, assets []asset) error {
	now := types.NewDateTimeFromTime(time.Now())
	var rec gtables.Theme
	rec.InitWithDBType(tx, "", in.DBType)
	exists := rec.Get(code)
	if !exists {
		rec.Code = types.NewCode(code)
		rec.Installed_by = types.NewCode(userID)
		rec.Installed_at = now
	}
	rec.Name = types.NewText(name)
	rec.Mode = gtables.Theme_Mode.Dark
	if mode == "light" {
		rec.Mode = gtables.Theme_Mode.Light
	}
	rec.Repo_url = types.NewText(repo.URL())
	rec.Repo_ref = types.NewText(repo.Ref)
	rec.Commit_sha = types.NewText(commit)
	rec.Author = types.NewText(repo.Owner)
	rec.Tokens = types.NewText(tokensJSON)
	rec.Wallpapers = wallpapers
	rec.Updated_at = now
	ok := false
	if exists {
		ok = rec.Modify(true)
	} else {
		ok = rec.Insert(true)
	}
	if !ok {
		err := rec.TriggerError()
		if err != nil {
			return err
		}
		return fmt.Errorf("saving theme %s failed", code)
	}

	err := tables.AddUserTheme(tx, in.DBType, userID, code)
	if err != nil {
		return err
	}

	var old gtables.ThemeAsset
	old.InitWithDBType(tx, "", in.DBType)
	old.SetRange("theme_code", code)
	old.DeleteAll()
	for _, a := range assets {
		var rec gtables.ThemeAsset
		rec.InitWithDBType(tx, "", in.DBType)
		rec.Theme_code = types.NewCode(code)
		rec.Name = types.NewCode(a.name)
		rec.Content_type = types.NewText(a.img.ContentType)
		rec.Size = len(a.img.Data)
		rec.Width, rec.Height = a.img.Width, a.img.Height
		rec.Sha256 = types.NewText(a.img.SHA256)
		rec.Data = a.img.Data
		if !rec.Insert(true) {
			return fmt.Errorf("saving image %s of theme %s failed", a.name, code)
		}
	}
	return nil
}

// LinkInstallers adds every installed theme to the themes of the user who installed it,
// when it is not in anybody's themes (themes installed before themes were per user). Run
// at startup; does nothing when every theme has a user.
func LinkInstallers(db *sql.DB, dbType database.DBType) (int, error) {
	var rec gtables.Theme
	rec.InitWithDBType(db, "", dbType)
	type orphan struct{ code, user string }
	var orphans []orphan
	for found := rec.FindSet(); found; found = rec.Next() {
		var links gtables.UserTheme
		links.InitWithDBType(db, "", dbType)
		links.SetRange("theme_code", rec.Code.String())
		if links.Count() == 0 && rec.Installed_by.String() != "" {
			orphans = append(orphans, orphan{rec.Code.String(), rec.Installed_by.String()})
		}
	}
	for _, o := range orphans {
		err := tables.AddUserTheme(db, dbType, o.user, o.code)
		if err != nil {
			return 0, err
		}
	}
	return len(orphans), nil
}

// Remove takes an installed theme off the user's themes; the files are deleted when no user
// has the theme any more. removed is false when the user did not have it.
func (in *Installer) Remove(userID, code string) (removed bool, err error) {
	_, ok := themes.Builtin(code)
	if ok {
		return false, ErrBuiltin
	}
	tx, err := in.DB.Begin()
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			_ = ftables.Rollback(tx)
		}
	}()
	var link gtables.UserTheme
	link.InitWithDBType(tx, "", in.DBType)
	if !link.GetByPK(types.NewCode(userID), types.NewCode(code)) {
		_ = ftables.Rollback(tx)
		return false, nil
	}
	if !link.Delete(true) {
		return false, fmt.Errorf("removing theme %s failed", code)
	}
	var others gtables.UserTheme
	others.InitWithDBType(tx, "", in.DBType)
	others.SetRange("theme_code", code)
	if others.Count() == 0 {
		var rec gtables.Theme
		rec.InitWithDBType(tx, "", in.DBType)
		if rec.Get(code) && !rec.Delete(true) {
			return false, fmt.Errorf("deleting theme %s failed", code)
		}
	}
	err = ftables.Commit(tx)
	if err != nil {
		return false, err
	}
	return true, nil
}

var codeChars = regexp.MustCompile(`[^A-Z0-9]+`)

// codeFor returns the theme code of a repository: from its name ("omarchy-starsend-theme"
// → "STARSEND"), with the owner added when another repository has that code. existing is
// true when the repository is installed already.
func (in *Installer) codeFor(repo themes.Repo) (string, bool, error) {
	base := clip(strings.Trim(codeChars.ReplaceAllString(strings.ToUpper(shortName(repo.Name)), "-"), "-"), 40)
	if base == "" {
		base = "THEME"
	}
	owner := strings.Trim(codeChars.ReplaceAllString(strings.ToUpper(repo.Owner), "-"), "-")
	for _, code := range []string{base, clip(base+"-"+owner, 50)} {
		_, builtin := themes.Builtin(code)
		if builtin {
			continue
		}
		var rec gtables.Theme
		rec.InitWithDBType(in.DB, "", in.DBType)
		if !rec.Get(code) {
			return code, false, nil
		}
		if strings.EqualFold(rec.Repo_url.String(), repo.URL()) {
			return code, true, nil
		}
	}
	return "", false, ErrCodeTaken
}

func (in *Installer) count() int {
	var rec gtables.Theme
	rec.InitWithDBType(in.DB, "", in.DBType)
	return rec.Count()
}

// shortName strips the usual prefix and suffix of theme repositories.
func shortName(repoName string) string {
	n := strings.ToLower(repoName)
	n = strings.TrimPrefix(n, "omarchy-")
	n = strings.TrimSuffix(n, "-theme")
	n = strings.TrimSuffix(n, ".nvim")
	return n
}

// displayName turns "omarchy-super-game-bro-theme" into "Super Game Bro".
func displayName(repoName string) string {
	words := strings.FieldsFunc(shortName(repoName), func(r rune) bool { return r == '-' || r == '_' || r == '.' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return clip(strings.Join(words, " "), 100)
}

func clip(s string, n int) string {
	if len(s) > n {
		return strings.TrimRight(s[:n], "-")
	}
	return s
}

func modeName(m gtables.ThemeMode) string {
	if m == gtables.Theme_Mode.Light {
		return "light"
	}
	return "dark"
}
