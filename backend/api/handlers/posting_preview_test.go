package handlers

import (
	"database/sql"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/postingpreview"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"

	// page 115's trigger (OnFindRecords)
	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/pages"
)

// A preview's entries are listed from the preview (an entries page with the page parameter
// preview) and Posting Preview lists its tables; nothing is in the database; another user's
// preview is refused
func TestPostingPreviewLists(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, rec := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.GLEntry{}, &gtables.DocumentEntry{}} {
		if err := rec.CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := postingpreview.Run(db, "TEST", database.DBTypeSQLite, "ANNA", func(src ftables.Source) error {
		for i, amount := range []string{"40", "-40"} {
			glEntry := gtables.NewGLEntry(src)
			glEntry.Entry_no = i + 1
			glEntry.Gl_account_no = "1210"
			glEntry.Amount = types.MustDecimal(amount)
			if err := glEntry.InsertOrError(false); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	appAs := func(userID string) *fiber.App {
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("session", &session.Session{UserID: userID, Company: "TEST", Language: "en-US"})
			return c.Next()
		})
		app.Use(middleware.UserContext()) // the page trigger runs as the user
		h := NewTablesHandler(db)
		app.Get("/api/tables/:table/list", h.ListRecords)
		return app
	}
	params := url.QueryEscape(`{"preview":"` + preview.ID + `"}`)

	status, out := getJSON(t, appAs("ANNA"), "/api/tables/G_L_Entry/list?page_params="+params)
	if status != 200 || out["data"].(map[string]interface{})["total"].(float64) != 2 {
		t.Errorf("previewed G/L entries: %d %v", status, out)
	}
	status, out = getJSON(t, appAs("ANNA"), "/api/tables/Document_Entry/list?page_id=115&page_params="+params)
	if status != 200 || out["data"].(map[string]interface{})["total"].(float64) != 1 {
		t.Errorf("Posting Preview lines: %d %v", status, out)
	}
	status, out = getJSON(t, appAs("ANNA"), "/api/tables/G_L_Entry/list")
	if status != 200 || out["data"].(map[string]interface{})["total"].(float64) != 0 {
		t.Errorf("the database has no entries: %d %v", status, out)
	}
	status, out = getJSON(t, appAs("BERT"), "/api/tables/G_L_Entry/list?page_params="+params)
	if status == 200 || !strings.Contains(out["error"].(string), "no longer available") {
		t.Errorf("another user's preview: %d %v", status, out)
	}
}
