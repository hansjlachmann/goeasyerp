package handlers

import (
	"database/sql"
	"fmt"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"net/url"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// newLookupTestApp has Customer and Payment Terms with n payment terms.
func newLookupTestApp(t *testing.T, n int) *fiber.App {
	t.Helper()
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", t.Name(), n))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.Customer{}, &gtables.PaymentTerms{}} {
		if err := tbl.CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	for i := range n {
		var pt gtables.PaymentTerms
		pt.InitWithDBType(db, "TEST", database.DBTypeSQLite)
		pt.Code = types.NewCode(fmt.Sprintf("PT%03d", i))
		pt.Description = types.NewText(fmt.Sprintf("Terms number %d", i))
		pt.Active = true
		if !pt.Insert(true) {
			t.Fatalf("insert payment terms %d", i)
		}
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/tables/:table/list", h.ListRecords)
	app.Get("/api/tables/:table/options", h.GetOptions)
	app.Get("/api/tables/:table/lookup/:field", h.LookupRows)
	return app
}

func lookupOf(t *testing.T, out map[string]interface{}) map[string]interface{} {
	t.Helper()
	captions, _ := out["captions"].(map[string]interface{})
	if captions == nil {
		data, _ := out["data"].(map[string]interface{})
		captions = data
	}
	lookups, _ := captions["lookups"].(map[string]interface{})
	l, _ := lookups["payment_terms_code"].(map[string]interface{})
	return l
}

// Small related tables are sent along; large ones only as a lazy URL.
func TestLookupsInlineOrLazy(t *testing.T) {
	_, out := getJSON(t, newLookupTestApp(t, 5), "/api/tables/Customer/list")
	small := lookupOf(t, out)
	if rows, _ := small["rows"].([]interface{}); len(rows) != 5 || small["lazy_url"] != nil {
		t.Errorf("5 payment terms: rows %d lazy_url %v, want 5 inline rows", len(rows), small["lazy_url"])
	}

	app := newLookupTestApp(t, 250)
	for _, target := range []string{"/api/tables/Customer/list", "/api/tables/Customer/options"} {
		_, out := getJSON(t, app, target)
		big := lookupOf(t, out)
		if big["lazy_url"] != "/api/tables/Customer/lookup/payment_terms_code" || big["rows"] != nil || big["simple"] != nil {
			t.Errorf("%s: 250 payment terms: %v, want lazy_url and no rows", target, big)
		}
		if total, _ := big["total"].(float64); total != 250 {
			t.Errorf("%s: total = %v", target, big["total"])
		}
	}
}

func TestLookupRowsEndpoint(t *testing.T) {
	app := newLookupTestApp(t, 250)
	rowsOf := func(target string) ([]string, int) {
		status, out := getJSON(t, app, target)
		if status != 200 {
			t.Fatalf("GET %s = %d %v", target, status, out)
		}
		data := out["data"].(map[string]interface{})
		var keys []string
		for _, r := range data["rows"].([]interface{}) {
			row := r.(map[string]interface{})
			if row["code"] != row["_key"] || row["description"] == nil {
				t.Fatalf("row %v lacks key/columns", row)
			}
			keys = append(keys, row["_key"].(string))
		}
		return keys, int(data["total"].(float64))
	}

	// First page, ordered by key
	keys, total := rowsOf("/api/tables/Customer/lookup/payment_terms_code")
	if len(keys) != 50 || keys[0] != "PT000" || total != 250 {
		t.Errorf("first page: %d rows from %v, total %d", len(keys), keys[:1], total)
	}
	// Search over key and lookup columns, case-insensitive
	keys, total = rowsOf("/api/tables/Customer/lookup/payment_terms_code?search=" + url.QueryEscape("number 12"))
	if total != 11 || keys[0] != "PT012" {
		t.Errorf("search 'number 12': %v total %d, want PT012, PT120..PT129", keys, total)
	}
	keys, _ = rowsOf("/api/tables/Customer/lookup/payment_terms_code?search=pt24")
	if fmt.Sprint(keys) != "[PT240 PT241 PT242 PT243 PT244 PT245 PT246 PT247 PT248 PT249]" {
		t.Errorf("search pt24: %v", keys)
	}
	// Exact key (any case)
	keys, _ = rowsOf("/api/tables/Customer/lookup/payment_terms_code?key=pt007")
	if fmt.Sprint(keys) != "[PT007]" {
		t.Errorf("key pt007: %v", keys)
	}
	keys, _ = rowsOf("/api/tables/Customer/lookup/payment_terms_code?key=NOPE")
	if len(keys) != 0 {
		t.Errorf("key NOPE: %v", keys)
	}
	// Paging
	keys, _ = rowsOf("/api/tables/Customer/lookup/payment_terms_code?offset=240")
	if len(keys) != 10 || keys[0] != "PT240" {
		t.Errorf("offset 240: %v", keys)
	}
	// Not a relation field
	if status, _ := getJSON(t, app, "/api/tables/Customer/lookup/name"); status != 404 {
		t.Errorf("lookup on a field without relation = %d, want 404", status)
	}
}
