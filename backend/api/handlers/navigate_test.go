package handlers

import (
	"database/sql"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/pages"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The entry tables Navigate searches
var navigateTables = []string{
	"G_L_Entry", "Tax_Entry", "Customer_ledger_entry", "Vendor_ledger_entry", "Bank_Account_Ledger_Entry",
	"Check_Ledger_Entry", "Item_Ledger_Entry", "Phys_Inventory_Ledger_Entry", "Res_Ledger_Entry",
	"FA_Ledger_Entry", "Maintenance_Ledger_Entry", "Ins_Coverage_Ledger_Entry",
}

// Navigate (page 344) on a temporary source table: the list reads the lines its OnFindRecords
// fills from the page parameters — one per entry table with entries of the document, in the
// date filter, only tables the user may read; sort, search and /ids work on them; nothing is
// stored. Every line's entries page has the Navigate action (Ctrl+Alt+Q).
func TestNavigate(t *testing.T) {
	registry := pages.GetRegistry()
	if err := registry.LoadPages(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	// One entry of document D1 in every table, a second G/L entry, and document D2 on another date
	entryNo := 0
	insert := func(table, docNo, date string) {
		t.Helper()
		factory, ok := gtables.GetTableFactory(table)
		if !ok {
			t.Fatalf("table %s", table)
		}
		entryNo++
		rec := factory()
		rec.InitWithDBType(db, "TEST", database.DBTypeSQLite)
		rec.FromMap(map[string]interface{}{"entry_no": entryNo, "document_no": docNo, "posting_date": date})
		if !rec.Insert(false) {
			t.Fatalf("insert %s", table)
		}
	}
	for _, table := range navigateTables {
		insert(table, "D1", "2026-01-15")
	}
	insert("G_L_Entry", "D1", "2026-01-15")
	insert("G_L_Entry", "D2", "2026-03-01")

	var access *session.Session
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		sess := &session.Session{Company: "TEST", Language: "en-US"}
		if access != nil {
			sess = access
		}
		c.Locals("session", sess)
		fcodeunits.SetCurrentContext("U", "U", "TEST", "en-US")
		fcodeunits.SetCurrentAccess(sess)
		defer fcodeunits.ClearCurrentContext()
		return c.Next()
	})
	app.Get("/api/tables/:table/list", h.ListRecords)
	app.Get("/api/tables/:table/ids", h.GetRecordIDs)

	list := func(params, query string) []map[string]interface{} {
		t.Helper()
		status, out := getJSON(t, app, "/api/tables/Document_Entry/list?page_id=344&page_params="+url.QueryEscape(params)+query)
		if status != 200 {
			t.Fatalf("list %s: %d %v", params, status, out)
		}
		data, _ := out["data"].(map[string]interface{})
		raw, _ := data["records"].([]interface{})
		rows := make([]map[string]interface{}, 0, len(raw))
		for _, r := range raw {
			rows = append(rows, r.(map[string]interface{}))
		}
		return rows
	}

	rows := list(`{"document_no":"D1"}`, "")
	if len(rows) != len(navigateTables) {
		t.Fatalf("D1: %d lines, want %d: %v", len(rows), len(navigateTables), rows)
	}
	if rows[0]["table_name"] != "G/L Entry" || rows[0]["no_of_records"] != float64(2) || rows[0]["page_id"] != float64(20) ||
		rows[0]["document_no_filter"] != "D1" || rows[0]["posting_date_filter"] != "" {
		t.Errorf("G/L line: %v", rows[0])
	}
	for _, row := range rows {
		pageID := int(row["page_id"].(float64))
		p, err := registry.GetPage(pageID)
		if err != nil {
			t.Errorf("%v: page %d does not exist", row["table_name"], pageID)
			continue
		}
		found := false
		for _, a := range p.Page.Actions {
			if a.Name == "Navigate" && a.RunPage == 344 && a.Shortcut == "Ctrl+Alt+Q" &&
				a.RunPageParams["document_no"] == "document_no" && a.RunPageParams["posting_date"] == "posting_date" {
				found = true
			}
		}
		if !found {
			t.Errorf("page %d %s has no Navigate action", pageID, p.Page.Name)
		}
	}

	// The posting date filter; both filters blank find nothing
	if rows := list(`{"document_no":"D1","posting_date":"2026-02-01.."}`, ""); len(rows) != 0 {
		t.Errorf("D1 after February: %v", rows)
	}
	if rows := list(`{"posting_date":"2026-03-01"}`, ""); len(rows) != 1 || rows[0]["no_of_records"] != float64(1) || rows[0]["posting_date_filter"] != "2026-03-01" {
		t.Errorf("1 March: %v", rows)
	}
	if rows := list(`{}`, ""); len(rows) != 0 {
		t.Errorf("no filters: %v", rows)
	}

	// Sort and search on the temporary lines, and /ids
	if rows := list(`{"document_no":"D1"}`, "&sort_by=table_name&sort_order=desc"); len(rows) == 0 || rows[0]["table_name"] != "Vendor Ledger Entry" {
		t.Errorf("sorted: %v", rows)
	}
	if rows := list(`{"document_no":"D1"}`, "&search=check&search_fields="+url.QueryEscape(`["table_name"]`)); len(rows) != 1 {
		t.Errorf("search: %v", rows)
	}
	status, out := getJSON(t, app, "/api/tables/Document_Entry/ids?page_id=344&page_params="+url.QueryEscape(`{"document_no":"D1"}`))
	data, _ := out["data"].(map[string]interface{})
	if ids, _ := data["ids"].([]interface{}); status != 200 || len(ids) != len(navigateTables) {
		t.Errorf("ids: %d %v", status, out)
	}

	// Nothing reaches the Document Entry table in the database
	var stored int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "TEST$Document_Entry"`).Scan(&stored); err != nil || stored != 0 {
		t.Errorf("stored lines: %d %v", stored, err)
	}

	// A user who may not read Tax Entry does not see its line
	perms := map[string]session.TablePermission{"DOCUMENT_ENTRY": {Read: true}}
	for _, table := range navigateTables {
		if table != "Tax_Entry" {
			perms[strings.ToUpper(table)] = session.TablePermission{Read: true}
		}
	}
	access = &session.Session{Company: "TEST", Language: "en-US", HasRoles: true, Permissions: perms}
	rows = list(`{"document_no":"D1"}`, "")
	if len(rows) != len(navigateTables)-1 {
		t.Errorf("without Tax Entry: %d lines", len(rows))
	}
	for _, row := range rows {
		if row["table_id"] == float64(gtables.TaxEntryTableID) {
			t.Errorf("Tax Entry shown: %v", row)
		}
	}
}
