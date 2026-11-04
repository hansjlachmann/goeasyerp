package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
	"github.com/hansjlachmann/openerp/backend/foundation/themes"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func newThemesTestApp(t *testing.T, userID string) *fiber.App {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.UserPersonalization{}, &gtables.Theme{}, &gtables.ThemeAsset{}, &gtables.UserTheme{}} {
		if err := tbl.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	h := NewThemesHandler(db, database.DBTypeSQLite)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{UserID: userID, Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/themes", h.ListThemes)
	app.Get("/api/appearance", h.GetAppearance)
	app.Put("/api/appearance", h.SaveAppearance)
	app.Get("/api/themes/:code/assets/:name", h.GetThemeAsset)
	app.Delete("/api/themes/:code", h.DeleteTheme)
	return app
}

func putJSON(t *testing.T, app *fiber.App, url, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest("PUT", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v (%s)", url, err, raw)
	}
	return resp.StatusCode, out
}

func appearanceSettingsOf(t *testing.T, out map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, _ := out["data"].(map[string]interface{})
	s, _ := data["settings"].(map[string]interface{})
	if s == nil {
		t.Fatalf("no settings in %v", out)
	}
	return s
}

func TestAppearanceDefaultsSaveAndChecks(t *testing.T) {
	app := newThemesTestApp(t, "ANNA")

	// A user without a record gets GoEasyERP Light/Dark, dark mode, no wallpaper
	_, out := getJSON(t, app, "/api/appearance")
	s := appearanceSettingsOf(t, out)
	if s["light_theme"] != "GOEASYERP-LIGHT" || s["dark_theme"] != "GOEASYERP-DARK" || s["mode"] != "dark" || s["wallpaper"] != "" || out["data"].(map[string]interface{})["saved"] != false {
		t.Fatalf("defaults = %v", s)
	}
	dark := out["data"].(map[string]interface{})["dark"].(map[string]interface{})
	if dark["code"] != "GOEASYERP-DARK" || dark["tokens"] == nil {
		t.Errorf("dark slot = %v", dark)
	}

	// Choose Tokyo Night for dark mode and switch to light mode
	code, out := putJSON(t, app, "/api/appearance", `{"dark_theme":"TOKYO-NIGHT","density":"compact"}`)
	if code != 200 {
		t.Fatalf("save = %d %v", code, out)
	}
	if _, out = putJSON(t, app, "/api/appearance", `{"mode":"light"}`); out["success"] != true {
		t.Fatalf("mode = %v", out)
	}
	// Only the fields sent change
	_, out = getJSON(t, app, "/api/appearance")
	s = appearanceSettingsOf(t, out)
	if s["dark_theme"] != "TOKYO-NIGHT" || s["mode"] != "light" || s["density"] != "compact" || s["light_theme"] != "GOEASYERP-LIGHT" {
		t.Errorf("after save = %v", s)
	}

	// The table's triggers refuse bad values; nothing changes
	bad := map[string]string{
		`{"dark_theme":"NO-SUCH-THEME"}`:    "does not exist",
		`{"dark_theme":"CATPPUCCIN-LATTE"}`: "other mode",
		`{"light_theme":"NORD"}`:            "other mode",
		`{"panel_opacity":40}`:              "between 75 and 100",
		`{"wallpaper_opacity":101}`:         "between 0 and 100",
		`{"font_size":300}`:                 "between 80 and 150",
		`{"mode":"sepia"}`:                  "",
		`{"font":"comic_sans"}`:             "",
		`{"wallpaper":"../../etc/passwd"}`:  "format",
		`{"wallpaper":"bg-9"}`:              "format",
	}
	for body, want := range bad {
		code, out := putJSON(t, app, "/api/appearance", body)
		msg, _ := out["error"].(string)
		if code != 400 || out["success"] != false || !strings.Contains(msg, want) {
			t.Errorf("%s = %d %q, want 400 containing %q", body, code, msg, want)
		}
	}
	_, out = getJSON(t, app, "/api/appearance")
	if s2 := appearanceSettingsOf(t, out); s2["dark_theme"] != "TOKYO-NIGHT" || s2["panel_opacity"] != float64(92) {
		t.Errorf("after refused saves = %v", s2)
	}

	// Another user still has the defaults
	other := newThemesTestApp(t, "BJORN")
	_, out = getJSON(t, other, "/api/appearance")
	if s := appearanceSettingsOf(t, out); s["dark_theme"] != "GOEASYERP-DARK" {
		t.Errorf("other user = %v", s)
	}
}

func TestListThemes(t *testing.T) {
	app := newThemesTestApp(t, "ANNA")
	_, out := getJSON(t, app, "/api/themes")
	data, _ := out["data"].(map[string]interface{})
	list, _ := data["themes"].([]interface{})
	if data["can_install"] != true || data["can_delete"] != true {
		t.Errorf("permissions = %v", data)
	}
	codes := map[string]bool{}
	for _, item := range list {
		codes[item.(map[string]interface{})["code"].(string)] = true
	}
	for _, c := range []string{"GOEASYERP-LIGHT", "GOEASYERP-DARK", "CLASSIC-LIGHT", "CLASSIC-DARK", "TOKYO-NIGHT", "NORD", "CATPPUCCIN-LATTE"} {
		if !codes[c] {
			t.Errorf("theme %s missing from %v", c, codes)
		}
	}
}

// Installed themes are per user: a user sees, chooses and gets the images of only the
// themes they added
func TestInstalledThemesArePerUser(t *testing.T) {
	admin := newThemesTestApp(t, "ADMIN")
	hans := newThemesTestApp(t, "HANS")

	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	nord, _ := themes.Builtin("NORD")
	tok, _ := json.Marshal(nord.Tokens)
	var th gtables.Theme
	th.InitWithDBType(db, "", database.DBTypeSQLite)
	th.Code, th.Name, th.Tokens, th.Installed_by = types.NewCode("DUNE"), types.NewText("Dune"), types.NewText(string(tok)), types.NewCode("ADMIN")
	th.Repo_url = types.NewText("https://github.com/o/omarchy-dune-theme")
	if !th.Insert(true) {
		t.Fatalf("insert theme: %v", th.TriggerError())
	}
	var asset gtables.ThemeAsset
	asset.InitWithDBType(db, "", database.DBTypeSQLite)
	asset.Theme_code, asset.Name, asset.Content_type, asset.Data = types.NewCode("DUNE"), types.NewCode("bg-1"), types.NewText("image/jpeg"), []byte{0xff, 0xd8, 0xff}
	if !asset.Insert(true) {
		t.Fatal("insert asset")
	}
	if err := tables.AddUserTheme(db, database.DBTypeSQLite, "ADMIN", "DUNE"); err != nil {
		t.Fatal(err)
	}

	listed := func(app *fiber.App) bool {
		_, out := getJSON(t, app, "/api/themes")
		for _, item := range out["data"].(map[string]interface{})["themes"].([]interface{}) {
			if item.(map[string]interface{})["code"] == "DUNE" {
				return true
			}
		}
		return false
	}
	if !listed(admin) || listed(hans) {
		t.Errorf("listed: ADMIN %v, HANS %v (want true, false)", listed(admin), listed(hans))
	}
	status := func(app *fiber.App) int {
		resp, _ := app.Test(httptest.NewRequest("GET", "/api/themes/DUNE/assets/bg-1", nil))
		return resp.StatusCode
	}
	if status(admin) != 200 || status(hans) != 404 {
		t.Errorf("asset: ADMIN %d, HANS %d (want 200, 404)", status(admin), status(hans))
	}
	if code, out := putJSON(t, hans, "/api/appearance", `{"dark_theme":"DUNE"}`); code != 400 {
		t.Errorf("HANS chose ADMIN's theme: %d %v", code, out)
	}
	if code, _ := putJSON(t, admin, "/api/appearance", `{"dark_theme":"DUNE"}`); code != 200 {
		t.Errorf("ADMIN could not choose his theme: %d", code)
	}
	// HANS cannot remove ADMIN's theme
	resp, _ := hans.Test(httptest.NewRequest("DELETE", "/api/themes/DUNE", nil))
	if resp.StatusCode != 404 || !listed(admin) {
		t.Errorf("HANS remove = %d, ADMIN still has it %v", resp.StatusCode, listed(admin))
	}
}

// Every user may manage their own themes, also a user whose roles give no Theme permission
func TestThemesWithoutThemePermission(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.UserPersonalization{}, &gtables.Theme{}, &gtables.ThemeAsset{}, &gtables.UserTheme{}} {
		if err := tbl.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	h := NewThemesHandler(db, database.DBTypeSQLite)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		sess := &session.Session{UserID: "CLERK", Company: "TEST", Language: "en-US"}
		sess.SetPermissions(true, false, map[string]session.TablePermission{"CUSTOMER": {Read: true}})
		c.Locals("session", sess)
		return c.Next()
	})
	app.Get("/api/themes", h.ListThemes)
	app.Post("/api/themes/install", h.InstallTheme)
	app.Delete("/api/themes/:code", h.DeleteTheme)

	_, out := getJSON(t, app, "/api/themes")
	if data := out["data"].(map[string]interface{}); data["can_install"] != true {
		t.Errorf("can_install = %v", data["can_install"])
	}
	// Reaches the installer (a bad URL is refused there, not by a permission check)
	if out := postJSON(t, app, "/api/themes/install", `{"url":"https://example.com/a/b"}`); !strings.Contains(fmt.Sprint(out["error"]), "GitHub") {
		t.Errorf("install = %v", out)
	}
	resp, _ := app.Test(httptest.NewRequest("DELETE", "/api/themes/NOPE", nil))
	if resp.StatusCode != 404 {
		t.Errorf("remove = %d, want 404 (not 403)", resp.StatusCode)
	}
}
