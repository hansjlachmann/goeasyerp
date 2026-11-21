package handlers

import (
	"database/sql"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The main menu's text: none — the standard text; the lines of all languages; the user's own
// language's lines first (its Language by Translation Key), in line order, with their styles
func TestMainMenuText(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	for _, create := range []func() error{
		func() error {
			return (&gtables.MainMenuText{}).CreateTableWithDBType(db, "test", database.DBTypeSQLite)
		},
		func() error { return (&gtables.Language{}).CreateTableWithDBType(db, "", database.DBTypeSQLite) },
	} {
		if err := create(); err != nil {
			t.Fatal(err)
		}
	}
	h := NewMainMenuTextHandler(db, database.DBTypeSQLite)
	language := "nb-NO"
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{IsSuper: true, HasRoles: true, UserID: "HANS", Company: "test", Language: language})
		return c.Next()
	})
	app.Get("/api/main-menu-text", h.GetMainMenuText)
	lines := func() []interface{} {
		t.Helper()
		code, out := getJSON(t, app, "/api/main-menu-text")
		data, _ := out["data"].(map[string]interface{})
		got, ok := data["lines"].([]interface{})
		if code != 200 || !ok {
			t.Fatalf("%d %v", code, out)
		}
		return got
	}
	if got := lines(); len(got) != 0 {
		t.Fatalf("no lines: %v", got)
	}

	insert := func(lang string, no int, text string, style gtables.MainMenuTextStyle, bold bool) {
		t.Helper()
		rec := &gtables.MainMenuText{Language_code: types.NewCode(lang), Line_no: no, Text: types.NewText(text), Style: style, Bold: bold}
		rec.InitWithDBType(db, "test", database.DBTypeSQLite)
		if !rec.Insert(false) {
			t.Fatalf("insert %s %d", lang, no)
		}
	}
	insert("", 20000, "Second", 0, false)
	insert("", 10000, "Welcome to Example Ltd", 1, true)
	got := lines()
	first, _ := got[0].(map[string]interface{})
	if len(got) != 2 || first["text"] != "Welcome to Example Ltd" || first["style"] != "heading" || first["bold"] != true || first["italic"] != false {
		t.Fatalf("all languages: %v", got)
	}

	norsk := &gtables.Language{Code: "NOR", Name: "Norsk", Translation_key: "nb-NO"}
	norsk.InitWithDBType(db, "", database.DBTypeSQLite)
	if !norsk.Insert(false) {
		t.Fatal("language")
	}
	insert("NOR", 10000, "Velkommen", 2, false)
	got = lines()
	first, _ = got[0].(map[string]interface{})
	if len(got) != 1 || first["text"] != "Velkommen" || first["style"] != "large" {
		t.Errorf("the user's language: %v", got)
	}
	language = "en-US"
	if got = lines(); len(got) != 2 {
		t.Errorf("another language takes the lines of all languages: %v", got)
	}
}
