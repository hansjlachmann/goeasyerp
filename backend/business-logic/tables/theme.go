package tables

import (
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/themes"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// themeLogic is the business logic of table 901 Theme: a color theme installed from
// GitHub (business-logic/themerepo). The files are stored once; each user sees the themes
// they added (User_Theme). Built-in themes (backend/foundation/themes) are not stored and
// every user has them.
type themeLogic struct{}

func init() { gtables.RegisterThemeLogic(themeLogic{}) }

func (themeLogic) OnInsert(rec *gtables.Theme) error {
	return checkThemeTokens(rec)
}

func (themeLogic) OnModify(rec *gtables.Theme) error {
	return checkThemeTokens(rec)
}

// OnDelete deletes the theme's images and removes it from every user's themes. Users who
// chose the theme fall back to the default theme of its mode (handlers/themes.go).
func (themeLogic) OnDelete(rec *gtables.Theme) error {
	var links gtables.UserTheme
	links.InitWithDBType(rec.GetDB(), "", rec.GetDBType())
	links.SetRange("theme_code", rec.Code.String())
	links.DeleteAll()

	var assets gtables.ThemeAsset
	assets.InitWithDBType(rec.GetDB(), "", rec.GetDBType())
	assets.SetRange("theme_code", rec.Code.String())
	assets.DeleteAll()
	return nil
}

// checkThemeTokens: the stored tokens must be valid theme tokens
func checkThemeTokens(rec *gtables.Theme) error {
	_, err := themes.ParseTokens(rec.Tokens.String())
	if err != nil {
		return apperrors.FieldFormat(gtables.ThemeTableName, "tokens", "JSON")
	}
	return nil
}

// toTheme returns the stored theme as the API sends it.
func toTheme(rec *gtables.Theme) (themes.Theme, error) {
	tokens, err := themes.ParseTokens(rec.Tokens.String())
	if err != nil {
		return themes.Theme{}, err
	}
	return themes.Theme{
		Code: rec.Code.String(), Name: rec.Name.String(), Mode: tokens.Mode, Source: "github",
		Author: rec.Author.String(), RepoURL: rec.Repo_url.String(), Tokens: tokens,
		Wallpapers: rec.Wallpapers, Version: rec.Commit_sha.String(),
	}, nil
}

// UserThemes returns the installed themes the user has added (in code order).
func UserThemes(db database.Executor, dbType database.DBType, userID string) []themes.Theme {
	var links gtables.UserTheme
	links.InitWithDBType(db, "", dbType)
	links.SetRange("user_id", userID)
	var codes []string
	for found := links.FindSet(); found; found = links.Next() {
		codes = append(codes, links.Theme_code.String())
	}
	var out []themes.Theme
	for _, code := range codes {
		th, ok := ThemeByCode(db, dbType, code)
		if ok && th.Source == "github" {
			out = append(out, th)
		}
	}
	return out
}

// UserHasTheme reports whether the user has added the installed theme.
func UserHasTheme(db database.Executor, dbType database.DBType, userID, code string) bool {
	var link gtables.UserTheme
	link.InitWithDBType(db, "", dbType)
	return link.GetByPK(types.NewCode(userID), types.NewCode(code))
}

// AddUserTheme adds an installed theme to the user's themes (nothing when it is there).
func AddUserTheme(db database.Executor, dbType database.DBType, userID, code string) error {
	if UserHasTheme(db, dbType, userID, code) {
		return nil
	}
	var link gtables.UserTheme
	link.InitWithDBType(db, "", dbType)
	link.User_id = types.NewCode(userID)
	link.Theme_code = types.NewCode(code)
	link.Added_at = types.NewDateTimeFromTime(time.Now())
	return link.InsertOrError(true)
}

// UserThemeByCode finds a theme the user can choose: built-in, or installed and added by
// the user.
func UserThemeByCode(db database.Executor, dbType database.DBType, userID, code string) (themes.Theme, bool) {
	th, ok := themes.Builtin(code)
	if ok {
		return th, true
	}
	if db == nil || !UserHasTheme(db, dbType, userID, code) {
		return themes.Theme{}, false
	}
	return ThemeByCode(db, dbType, code)
}

// ThemeByCode finds a theme by code: built-in or installed (by any user).
func ThemeByCode(db database.Executor, dbType database.DBType, code string) (themes.Theme, bool) {
	th, ok := themes.Builtin(code)
	if ok {
		return th, true
	}
	if db == nil || code == "" {
		return themes.Theme{}, false
	}
	var rec gtables.Theme
	rec.InitWithDBType(db, "", dbType)
	if !rec.Get(code) {
		return themes.Theme{}, false
	}
	th, err := toTheme(&rec)
	return th, err == nil
}
