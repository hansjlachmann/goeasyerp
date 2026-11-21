package handlers

import (
	"database/sql"
	"net/url"
	"sort"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A lookup shows a related table's rows only to a user who may read that table: without Read
// the dropdown has no rows and takes the typed text, and the on-demand rows are refused (S23)
func TestLookupsNeedReadOnTheRelatedTable(t *testing.T) {
	clerk := func(perms map[string]session.TablePermission) *session.Session {
		s := &session.Session{UserID: "CLERK", Company: "TEST", Language: "en-US"}
		s.SetPermissions(true, false, perms)
		return s
	}
	customerOnly := clerk(map[string]session.TablePermission{"CUSTOMER": {Read: true}})
	both := clerk(map[string]session.TablePermission{"CUSTOMER": {Read: true}, "PAYMENT_TERMS": {Read: true}})

	_, out := getJSON(t, newLookupTestAppAs(t, 5, customerOnly), "/api/tables/Customer/list")
	denied := lookupOf(t, out)
	if rows, _ := denied["rows"].([]interface{}); len(rows) != 0 || denied["free_text"] != true || denied["new_page_id"] != nil {
		t.Errorf("without Read on Payment Terms: %v, want no rows, free text, no New", denied)
	}
	_, out = getJSON(t, newLookupTestAppAs(t, 5, both), "/api/tables/Customer/list")
	if rows, _ := lookupOf(t, out)["rows"].([]interface{}); len(rows) != 5 {
		t.Errorf("with Read on Payment Terms: %d rows, want 5", len(rows))
	}

	app := newLookupTestAppAs(t, 250, customerOnly)
	_, out = getJSON(t, app, "/api/tables/Customer/list")
	if big := lookupOf(t, out); big["lazy_url"] != nil || big["total"] != nil {
		t.Errorf("without Read on a large related table: %v, want no lazy URL or total", big)
	}
	if status, _ := getJSON(t, app, "/api/tables/Customer/lookup/payment_terms_code?search=PT"); status != 403 {
		t.Errorf("on-demand rows without Read on Payment Terms: %d, want 403", status)
	}
	if status, _ := getJSON(t, newLookupTestAppAs(t, 250, both), "/api/tables/Customer/lookup/payment_terms_code?search=PT"); status != 200 {
		t.Errorf("on-demand rows with Read on Payment Terms: %d, want 200", status)
	}
}

// The on-demand rows of a relation with a FIELD filter (an item's variants) are those of the
// record's value the dropdown sends, not the whole table
func TestLookupRowsRecordFilters(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.BOMJournalLine{}, &gtables.ItemVariant{}} {
		if err := tbl.CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range [][2]string{{"1000", "A"}, {"1000", "B"}, {"2000", "C"}} {
		var variant gtables.ItemVariant
		variant.InitWithDBType(db, "TEST", database.DBTypeSQLite)
		variant.Item_no = types.NewCode(v[0])
		variant.Code = types.NewCode(v[1])
		if !variant.Insert(false) {
			t.Fatalf("insert variant %v", v)
		}
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{IsSuper: true, HasRoles: true, Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/tables/:table/lookup/:field", h.LookupRows)
	codes := func(query string) []string {
		t.Helper()
		status, out := getJSON(t, app, "/api/tables/BOM_Journal_Line/lookup/variant_code"+query)
		if status != 200 {
			t.Fatalf("%s: %d", query, status)
		}
		data, _ := out["data"].(map[string]interface{})
		rows, _ := data["rows"].([]interface{})
		var got []string
		for _, r := range rows {
			got = append(got, r.(map[string]interface{})["_key"].(string))
		}
		sort.Strings(got)
		return got
	}
	for query, want := range map[string]string{
		"?record_item_no=1000":                            "A,B",
		"?record_item_no=2000":                            "C",
		"?record_item_no=" + url.QueryEscape("*"):         "",
		"?record_item_no=" + url.QueryEscape("1000|2000"): "",
		"": "",
	} {
		got := codes(query)
		joined := ""
		for i, g := range got {
			if i > 0 {
				joined += ","
			}
			joined += g
		}
		if joined != want {
			t.Errorf("%q: variants %q, want %q", query, joined, want)
		}
	}
}
