package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
	_ "github.com/mattn/go-sqlite3"
)

// autoFillCustomer is a customer whose business logic (autoFill) fills a sibling field
// from its OnValidate_No trigger.
type autoFillCustomer struct {
	gtables.Customer
}

func (t *autoFillCustomer) InitWithDBType(db database.Executor, company string, dbType database.DBType) {
	t.Customer.InitWithDBType(db, company, dbType)
	t.SetLogic(autoFill{})
}

type autoFill struct{}

func (autoFill) OnValidate_No(rec *gtables.Customer) error {
	rec.Name = types.NewText("Filled from " + rec.No.String())
	return nil
}

func init() {
	gtables.RegisterTableFactory("Test_AutoFill", 99001, func() ftables.Table {
		return &autoFillCustomer{}
	})
}

func newTablesTestApp(t *testing.T) *fiber.App {
	t.Helper()
	// Named shared-cache in-memory DB: handlers keep a result set open while running
	// a second query (e.g. Count), so more than one connection must see the same data
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.Customer{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatalf("create table: %v", err)
	}

	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/tables/:table/validate", h.ValidateField)
	app.Post("/api/tables/:table/init", h.InitRecord)
	app.Post("/api/tables/:table/insert", h.InsertRecord)
	app.Put("/api/tables/:table/modify/:id", h.ModifyRecord)
	app.Get("/api/tables/:table/list", h.ListRecords)
	app.Get("/api/tables/:table/ids", h.GetRecordIDs)
	app.Delete("/api/tables/:table/delete-many", h.DeleteRecords)
	app.Post("/api/tables/:table/copy", h.CopyRecords)
	app.Post("/api/tables/:table/insert-many", h.InsertRecords)
	return app
}

func postJSON(t *testing.T, app *fiber.App, url, body string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v (%s)", url, err, raw)
	}
	return out
}

func TestValidateFieldReturnsRecordWithSiblingsFilled(t *testing.T) {
	app := newTablesTestApp(t)

	out := postJSON(t, app, "/api/tables/Test_AutoFill/validate",
		`{"field":"no","value":"C1","record":{"no":"","name":"","city":"Oslo"}}`)

	if out["success"] != true {
		t.Fatalf("success = %v, error = %v", out["success"], out["error"])
	}
	data, _ := out["data"].(map[string]interface{})
	if data["name"] != "Filled from C1" {
		t.Errorf("data.name = %v, want trigger-filled %q", data["name"], "Filled from C1")
	}
	if data["no"] != "C1" {
		t.Errorf("data.no = %v, want validated value C1", data["no"])
	}
	if data["city"] != "Oslo" {
		t.Errorf("data.city = %v, want Oslo carried over from the in-progress record", data["city"])
	}
}

func TestValidateFieldReportsTriggerError(t *testing.T) {
	app := newTablesTestApp(t)

	// Customer.OnValidate_Gen_bus_posting_group rejects a group that does not exist
	out := postJSON(t, app, "/api/tables/Customer/validate", `{"field":"gen_bus_posting_group","value":"NONE","record":{}}`)
	if out["success"] != false || out["error"] == "" {
		t.Errorf("got success=%v error=%v, want a validation error", out["success"], out["error"])
	}
}

func TestInitRecordReturnsDefaults(t *testing.T) {
	app := newTablesTestApp(t)

	out := postJSON(t, app, "/api/tables/Payment_terms/init", `{}`)
	if out["success"] != true {
		t.Fatalf("success = %v, error = %v", out["success"], out["error"])
	}
	data, _ := out["data"].(map[string]interface{})
	if data["active"] != true {
		t.Errorf("data.active = %v, want YAML default true", data["active"])
	}
	if data["code"] != "" {
		t.Errorf("data.code = %v, want blank", data["code"])
	}
}

// A stale sibling value in the payload must not overwrite what another field's
// OnValidate trigger filled in.
func TestValidateChangedFieldsKeepsTriggerFilledSibling(t *testing.T) {
	table := &autoFillCustomer{}
	table.InitWithDBType(nil, "TEST", database.DBTypeSQLite)
	table.FromMap(map[string]interface{}{"no": "C1", "name": "Old"})

	data := map[string]interface{}{"no": "C2", "name": "Old", "city": "Oslo"}
	if err := validateChangedFields(table, "Test_AutoFill", data); err != nil {
		t.Fatalf("validateChangedFields: %v", err)
	}

	got := table.ToMap()
	if got["name"] != "Filled from C2" {
		t.Errorf("name = %v, want trigger-filled %q", got["name"], "Filled from C2")
	}
	if got["city"] != "Oslo" {
		t.Errorf("city = %v, want Oslo", got["city"])
	}
}

func TestValidateChangedFieldsReportsUnknownField(t *testing.T) {
	table := &autoFillCustomer{}
	table.InitWithDBType(nil, "TEST", database.DBTypeSQLite)

	if err := validateChangedFields(table, "Test_AutoFill", map[string]interface{}{"bogus": "x"}); err == nil {
		t.Error("validateChangedFields with unknown field = nil, want error")
	}
}

// The list page sends the whole init payload (every table field, including blank dates,
// FlowFields and nulls) with the user's edits on insert; that must insert cleanly.
func TestInitPayloadIsInsertable(t *testing.T) {
	app := newTablesTestApp(t)

	initOut := postJSON(t, app, "/api/tables/Customer/init", `{}`)
	record, _ := initOut["data"].(map[string]interface{})
	if record == nil {
		t.Fatalf("init returned no record: %v", initOut)
	}
	record["no"] = "c1"
	body, _ := json.Marshal(record)

	out := postJSON(t, app, "/api/tables/Customer/insert", string(body))
	if out["success"] != true {
		t.Fatalf("insert of init payload failed: %v", out["error"])
	}
	if data, _ := out["data"].(map[string]interface{}); data["no"] != "C1" {
		t.Errorf("inserted no = %v, want C1", data["no"])
	}
}

func getJSON(t *testing.T, app *fiber.App, target string) (int, map[string]interface{}) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", target, nil))
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v (%s)", target, err, raw)
	}
	return resp.StatusCode, out
}

// sort_by and filter field names from the query string must be real columns
// (they end up in SQL text); anything else is rejected before reaching SQL.
func TestListRejectsUnknownFieldNames(t *testing.T) {
	app := newTablesTestApp(t)
	postJSON(t, app, "/api/tables/Customer/insert", `{"no":"C1","name":"Acme"}`)

	injected := url.QueryEscape("no; DROP TABLE x --")
	badFilter := url.QueryEscape(`[{"field":"no) OR (1=1","expression":"x"}]`)
	for _, target := range []string{
		"/api/tables/Customer/list?sort_by=" + injected,
		"/api/tables/Customer/ids?sort_by=" + injected,
		"/api/tables/Customer/list?filters=" + badFilter,
	} {
		status, out := getJSON(t, app, target)
		if status != 400 || out["success"] != false {
			t.Errorf("GET %s = %d %v, want 400", target, status, out)
		}
	}

	goodFilter := url.QueryEscape(`[{"field":"no","expression":"C1"}]`)
	status, out := getJSON(t, app, "/api/tables/Customer/list?sort_by=name&filters="+goodFilter)
	if status != 200 || out["success"] != true {
		t.Fatalf("valid sort/filter = %d %v, want 200", status, out)
	}
	data, _ := out["data"].(map[string]interface{})
	if total, _ := data["total"].(float64); total != 1 {
		t.Errorf("total = %v, want 1", data["total"])
	}
}

// A failing OnInsert trigger is a business rule the user broke: the API returns its
// message with 400, not a generic "insert failed".
func TestInsertReturnsTriggerError(t *testing.T) {
	app := newTablesTestApp(t)

	out := postJSON(t, app, "/api/tables/Customer/insert", `{"no":"C1","name":"`+strings.Repeat("x", 60)+`"}`)
	if out["success"] != false {
		t.Fatalf("insert succeeded, want OnInsert to reject a 60-character name")
	}
	if msg, _ := out["error"].(string); !strings.Contains(msg, "Name cannot exceed 50 characters") {
		t.Errorf("error = %q, want the OnInsert validation message", msg)
	}
}

// The duplicate check must use the whole primary key. For composite keys it used only the
// first field (GetPrimaryKeyValue), so Get always failed, duplicates reached the database
// and came back as a generic "insert failed".
func TestInsertDuplicateCompositeKey(t *testing.T) {
	app := newTablesTestApp(t)
	db, _ := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	defer db.Close()
	if err := (&gtables.UserMember{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatalf("create table: %v", err)
	}
	// The related records must exist (table relations are checked on insert)
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.User{}, &gtables.UserRole{}, &gtables.Company{}} {
		if err := tbl.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	for _, stmt := range []string{
		`INSERT INTO "User" (user_id, user_name) VALUES ('HANS', 'Hans')`,
		`INSERT INTO "User_Role" (code, description) VALUES ('READER', 'Reader')`,
		`INSERT INTO "Company" (name) VALUES ('CRONUS'), ('OTHER')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	first := postJSON(t, app, "/api/tables/User_Member/insert", `{"user_id":"HANS","role_id":"READER","company":"CRONUS"}`)
	if first["success"] != true {
		t.Fatalf("first insert failed: %v", first["error"])
	}

	// Same user and role in another company is a different record
	other := postJSON(t, app, "/api/tables/User_Member/insert", `{"user_id":"HANS","role_id":"READER","company":"OTHER"}`)
	if other["success"] != true {
		t.Errorf("insert for another company failed: %v (must not be treated as a duplicate)", other["error"])
	}

	dup := postJSON(t, app, "/api/tables/User_Member/insert", `{"user_id":"HANS","role_id":"READER","company":"CRONUS"}`)
	if dup["success"] != false {
		t.Fatal("duplicate insert succeeded")
	}
	if msg, _ := dup["error"].(string); !strings.Contains(msg, "HANS,READER,CRONUS") {
		t.Errorf("error = %q, want a duplicate message naming the full key HANS,READER,CRONUS", msg)
	}
}

// A changed table relation field must point to an existing record on insert and modify
// (on-demand dropdowns accept typed keys and rely on this check).
func TestInsertRejectsUnknownRelationKey(t *testing.T) {
	app := newTablesTestApp(t)
	db, _ := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	defer db.Close()
	if err := (&gtables.PaymentTerms{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO "TEST$Payment Terms" (code, description, active) VALUES ('30 DAYS', 'Net 30', 1)`); err != nil {
		t.Fatal(err)
	}
	bad := postJSON(t, app, "/api/tables/Customer/insert", `{"no":"C1","name":"Acme","payment_terms_code":"NOPE"}`)
	if bad["success"] != false || !strings.Contains(fmt.Sprint(bad["error"]), "NOPE") {
		t.Errorf("insert with unknown payment terms = %v, want rejected naming NOPE", bad)
	}
	good := postJSON(t, app, "/api/tables/Customer/insert", `{"no":"C1","name":"Acme","payment_terms_code":"30 DAYS"}`)
	if good["success"] != true {
		t.Errorf("insert with existing payment terms failed: %v", good["error"])
	}
}

// Renaming a record to a key another record has fails with "already exists" (409), not the
// generic "Failed to modify", and changes nothing; a rename to a free key still works.
func TestRenameToExistingKey(t *testing.T) {
	app := newTablesTestApp(t)
	// The rename carries the new No. over to the ledger entries
	db, _ := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	defer db.Close()
	if err := (&gtables.CustomerLedgerEntry{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	for _, related := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.CommentLine{}, &gtables.ShipToAddress{}, &gtables.CustomerBankAccount{}, &gtables.VendorLedgerEntry{},
		&gtables.BankAccountLedgerEntry{}, &gtables.CheckLedgerEntry{}, &gtables.FALedgerEntry{}, &gtables.MaintenanceLedgerEntry{},
		&gtables.GenJournalTemplate{}, &gtables.GenJournalBatch{}, &gtables.GenJournalLine{}, &gtables.ItemJournalLine{}, &gtables.ItemLedgerEntry{},
		&gtables.SalesHeader{}, &gtables.SalesLine{}, &gtables.SalesShipmentHeader{}, &gtables.SalesShipmentLine{}, &gtables.SalesInvoiceHeader{},
		&gtables.SalesInvoiceLine{}, &gtables.SalesCrMemoHeader{}, &gtables.SalesCrMemoLine{}, &gtables.ReminderHeader{}, &gtables.ReminderLine{},
		&gtables.IssuedReminderHeader{}, &gtables.FinanceChargeMemoHeader{}, &gtables.FinanceChargeMemoLine{}, &gtables.IssuedFinChargeMemoHeader{}, &gtables.ReminderFinChargeEntry{}} {
		if err := related.CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	if err := (&gtables.GLEntry{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	for _, no := range []string{"C1", "C2"} {
		if out := postJSON(t, app, "/api/tables/Customer/insert", `{"no":"`+no+`","name":"Customer `+no+`"}`); out["success"] != true {
			t.Fatalf("insert %s: %v", no, out["error"])
		}
	}
	put := func(id, body string) (int, map[string]interface{}) {
		req := httptest.NewRequest("PUT", "/api/tables/Customer/modify/"+id, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		var out map[string]interface{}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, out
	}

	status, out := put("C1", `{"no":"C2"}`)
	if msg, _ := out["error"].(string); status != 409 || !strings.Contains(msg, "C2") || !strings.Contains(msg, "already exists") {
		t.Fatalf("rename to an existing key: %d %q, want 409 naming C2 as existing", status, msg)
	}
	if status, out := put("C1", `{"name":"Still here"}`); status != 200 {
		t.Fatalf("C1 gone after the refused rename: %d %v", status, out["error"])
	}

	if status, out := put("C1", `{"no":"C3"}`); status != 200 {
		t.Fatalf("rename to a free key: %d %v", status, out["error"])
	}
}

// /ids applies the list's filters, search and sort, so card navigation follows the list.
func TestRecordIDsFollowListQuery(t *testing.T) {
	app := newTablesTestApp(t)
	for _, c := range []struct{ no, name string }{{"C1", "Delta Sykkel"}, {"C2", "Alfa AS"}, {"C3", "Charlie Sykkel"}, {"C4", "Bravo AS"}} {
		if out := postJSON(t, app, "/api/tables/Customer/insert", `{"no":"`+c.no+`","name":"`+c.name+`"}`); out["success"] != true {
			t.Fatalf("insert %s: %v", c.no, out["error"])
		}
	}
	ids := func(query string) (int, string) {
		resp, err := app.Test(httptest.NewRequest("GET", "/api/tables/Customer/ids"+query, nil), -1)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		var out struct {
			Data struct {
				IDs []string `json:"ids"`
			} `json:"data"`
		}
		_ = json.Unmarshal(raw, &out)
		return resp.StatusCode, strings.Join(out.Data.IDs, ",")
	}
	for _, tc := range []struct{ query, want string }{
		{"", "C1,C2,C3,C4"},
		{"?sort_by=name", "C2,C4,C3,C1"},
		{"?sort_by=name&sort_order=desc", "C1,C3,C4,C2"},
		{"?filters=" + url.QueryEscape(`[{"field":"no","expression":"C2..C3"}]`), "C2,C3"},
		{"?search=sykkel&search_fields=" + url.QueryEscape(`["name"]`) + "&sort_by=name", "C3,C1"},
	} {
		if status, got := ids(tc.query); status != 200 || got != tc.want {
			t.Errorf("ids%s = %d %q, want %q", tc.query, status, got, tc.want)
		}
	}
	if status, _ := ids("?sort_by=password_hash"); status != 400 {
		t.Errorf("sort by an unknown/sensitive column: %d, want 400", status)
	}
}

// Trigger messages reach the user in the session's language, with the field's caption.
func TestValidateMessageInSessionLanguage(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.Customer{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: c.Get("X-Language")})
		return c.Next()
	})
	app.Post("/api/tables/:table/validate", h.ValidateField)

	for language, want := range map[string]string{"en-US": "Gen. Bus. Posting Group 'NONE' does not exist", "nb-NO": "Bedriftsbokføringsgruppe 'NONE' finnes ikke"} {
		req := httptest.NewRequest("POST", "/api/tables/Customer/validate", strings.NewReader(`{"field":"gen_bus_posting_group","value":"NONE","record":{"no":"C1"}}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Language", language)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		var out map[string]interface{}
		_ = json.Unmarshal(raw, &out)
		if msg, _ := out["error"].(string); msg != want {
			t.Errorf("%s: error %q, want %q", language, msg, want)
		}
	}
}
