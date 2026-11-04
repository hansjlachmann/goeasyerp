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
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Read-only fields (table YAML editable: false) are not written from a page: insert and modify
// keep their stored value, validate refuses them; validation min/max is checked on writes.
func TestReadOnlyAndRangeFields(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.Currency{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/tables/:table/insert", h.InsertRecord)
	app.Put("/api/tables/:table/modify/:id", h.ModifyRecord)
	app.Post("/api/tables/:table/validate", h.ValidateField)

	stored := func(column string) string {
		var v sql.NullString
		if err := db.QueryRow(`SELECT ` + column + ` FROM "TEST$Currency" WHERE code = 'EUR'`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v.String
	}

	if status, out := sendJSON(t, app, "POST", "/api/tables/Currency/insert", `{"code":"EUR","description":"Euro","currency_factor":"7.5","last_date_modified":"2026-01-01"}`); status != 200 {
		t.Fatalf("insert: %d %v", status, out["error"])
	}
	if f := stored("currency_factor"); f != "" && f != "0" {
		t.Errorf("insert set the read-only currency_factor to %q", f)
	}
	if d := stored("last_date_modified"); d != "" && !strings.HasPrefix(d, "0001") {
		t.Errorf("insert set the read-only last_date_modified to %q", d)
	}

	if _, err := db.Exec(`UPDATE "TEST$Currency" SET currency_factor = 7.45 WHERE code = 'EUR'`); err != nil {
		t.Fatal(err)
	}
	if status, out := sendJSON(t, app, "PUT", "/api/tables/Currency/modify/EUR", `{"code":"EUR","description":"Euro (EMU)","currency_factor":"1"}`); status != 200 {
		t.Fatalf("modify: %d %v", status, out["error"])
	}
	if f := stored("currency_factor"); f != "7.45" {
		t.Errorf("modify changed the read-only currency_factor to %q", f)
	}
	if d := stored("description"); d != "Euro (EMU)" {
		t.Errorf("modify did not save description: %q", d)
	}

	status, out := sendJSON(t, app, "POST", "/api/tables/Currency/validate", `{"field":"currency_factor","value":"2","record":{"code":"EUR"}}`)
	if status != 400 || out["error"] != "Currency Factor cannot be changed." {
		t.Errorf("validate read-only field: %d %v", status, out["error"])
	}

	status, out = sendJSON(t, app, "PUT", "/api/tables/Currency/modify/EUR", `{"code":"EUR","amount_rounding_precision":"-0.01"}`)
	if status == 200 || !strings.Contains(out["error"].(string), "must not be less than 0") {
		t.Errorf("negative precision: %d %v", status, out["error"])
	}
}

// The page metadata carries the table's field properties: read-only fields are not editable,
// decimals have their places, cards get the data caption fields.
func TestPageFieldProperties(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/pages/:id", NewPagesHandler().GetPage)

	get := func(id string) pages.PageDefinition {
		resp, err := app.Test(httptest.NewRequest("GET", "/api/pages/"+id, nil))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		var out struct {
			Data pages.PageDefinition `json:"data"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode: %v (%s)", err, raw)
		}
		return out.Data
	}

	fields := map[string]pages.Field{}
	for _, f := range get("5").Page.Layout.Repeater.Fields {
		fields[f.Source] = f
	}
	for _, name := range []string{"currency_factor", "last_date_modified"} {
		if e := fields[name].Editable; e == nil || *e {
			t.Errorf("%s editable = %v, want false", name, e)
		}
	}
	if e := fields["description"].Editable; e != nil && !*e {
		t.Error("description not editable")
	}
	if p := fields["amount_rounding_precision"].DecimalPlaces; p != "2:5" {
		t.Errorf("amount_rounding_precision decimal_places = %q", p)
	}
	for id, want := range map[string]string{"494": "code,description", "495": "code", "21": "no,name"} {
		if c := get(id).Page.DataCaptionFields; strings.Join(c, ",") != want {
			t.Errorf("page %s data_caption_fields = %v, want %s (none: the card shows its key)", id, c, want)
		}
	}
	if c := get("5").Page.DataCaptionFields; len(c) != 0 {
		t.Errorf("list page 5 data_caption_fields = %v", c)
	}
	for _, s := range get("495").Page.Layout.Sections {
		for _, f := range s.Fields {
			if e := f.Editable; (f.Source == "last_date_modified" || f.Source == "last_date_adjusted") != (e != nil && !*e) {
				t.Errorf("card 495 %s editable = %v", f.Source, e)
			}
		}
	}
}

// A setup table's card reads its single record, created when missing; a read-only key field
// (Exchange Rates' currency, from the list's filter) is kept on insert and never renamed.
func TestSetupCardAndReadOnlyKey(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.GeneralLedgerSetup{}, &gtables.Currency{}, &gtables.CurrencyExchangeRate{}} {
		if err := tbl.CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/tables/:table/card", h.GetRecord)
	app.Put("/api/tables/:table/modify", h.ModifyRecord)
	app.Post("/api/tables/:table/insert", h.InsertRecord)
	app.Put("/api/tables/:table/modify/:id", h.ModifyRecord)

	status, out := getJSON(t, app, "/api/tables/General_Ledger_Setup/card")
	if status != 200 || out["data"].(map[string]interface{})["amount_decimal_places"] != "2:2" {
		t.Fatalf("setup card: %d %v", status, out)
	}
	if status, out := sendJSON(t, app, "PUT", "/api/tables/General_Ledger_Setup/modify", `{"lcy_code":"NOK"}`); status != 200 {
		t.Fatalf("modify setup: %d %v", status, out["error"])
	}
	if _, out := getJSON(t, app, "/api/tables/General_Ledger_Setup/card"); out["data"].(map[string]interface{})["lcy_code"] != "NOK" {
		t.Errorf("setup after modify: %v", out["data"])
	}

	if status, out := sendJSON(t, app, "POST", "/api/tables/Currency/insert", `{"code":"EUR"}`); status != 200 {
		t.Fatalf("insert currency: %d %v", status, out["error"])
	}
	if status, out := sendJSON(t, app, "POST", "/api/tables/Currency_Exchange_Rate/insert", `{"currency_code":"EUR","starting_date":"2026-01-01","exchange_rate_amount":"100","relational_exch_rate_amount":"1150"}`); status != 200 {
		t.Fatalf("insert rate with the read-only key from the filter: %d %v", status, out["error"])
	}
	if status, out := sendJSON(t, app, "PUT", "/api/tables/Currency_Exchange_Rate/modify/EUR,2026-01-01", `{"currency_code":"USD","exchange_rate_amount":"200"}`); status != 200 {
		t.Fatalf("modify rate: %d %v", status, out["error"])
	}
	var code, amount string
	if err := db.QueryRow(`SELECT currency_code, exchange_rate_amount FROM "TEST$Currency_Exchange_Rate"`).Scan(&code, &amount); err != nil || code != "EUR" || amount != "200" {
		t.Errorf("after modify: %s %s %v — want the key kept (EUR) and the amount changed", code, amount, err)
	}
}

// /init with the page's OnNewRecord triggers: a new G/L account takes the Income/Balance of the
// account it is inserted below; Totaling (a lookup-only relation) takes a filter of accounts
func TestNewRecordTriggerAndLookupOnlyRelation(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.GLAccount{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/tables/:table/init", h.InitRecord)
	app.Post("/api/tables/:table/insert", h.InsertRecord)

	if _, out := sendJSON(t, app, "POST", "/api/tables/G_L_Account/init", ``); out["data"].(map[string]interface{})["income_balance"] != float64(0) {
		t.Errorf("init without a page: %v", out["data"])
	}
	status, out := sendJSON(t, app, "POST", "/api/tables/G_L_Account/init", `{"page_id":16,"x_rec":{"no":"1000","income_balance":1},"below_x_rec":true}`)
	if status != 200 || out["data"].(map[string]interface{})["income_balance"] != float64(1) {
		t.Errorf("init below a balance sheet account: %d %v", status, out)
	}

	if status, out := sendJSON(t, app, "POST", "/api/tables/G_L_Account/insert", `{"no":"1999","account_type":4,"totaling":"1000..1998"}`); status != 200 {
		t.Errorf("Totaling with a filter refused: %d %v", status, out["error"])
	}
}

// A lookup's New opens the related table's list page: its lookup_page_id, else its list page
func TestLookupPage(t *testing.T) {
	for table, want := range map[string]int{"G_L_Account": 18, "Dimension_1": 12, "Currency": 5, "Payment_terms": 4, "Nothing": 0} {
		if got := pages.GetRegistry().LookupPage(table); got != want {
			t.Errorf("%s: page %d, want %d", table, got, want)
		}
	}
}

// A lookup offers New only where its page can enter a record: not for the Object table (a
// system table, page 908 read-only) nor for posted tables
func TestNewRecordPage(t *testing.T) {
	for table, want := range map[string]int{"G_L_Account": 18, "Currency": 5, "Object": 0, "Customer_ledger_entry": 0} {
		if got := pages.GetRegistry().NewRecordPage(table); got != want {
			t.Errorf("%s: New opens page %d, want %d", table, got, want)
		}
	}
}

// A conditional relation (Bal. Account No. by Bal. Account Type): the API checks the table of
// the record's type, sends one lookup per type, and the G/L Account totals of a list include
// total accounts (their Totaling) and the list's date filter
func TestConditionalRelationAndGLTotals(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.GLAccount{}, &gtables.GLEntry{}, &gtables.Customer{}, &gtables.CustomerLedgerEntry{}} {
		if err := tbl.CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/tables/:table/insert", h.InsertRecord)
	app.Get("/api/tables/:table/list", h.ListRecords)
	app.Get("/api/tables/:table/lookup/:field", h.LookupRows)

	for _, body := range []string{
		`{"no":"1000","name":"Bank"}`, `{"no":"2000","name":"Sales"}`,
		`{"no":"2999","name":"Total","account_type":2,"totaling":"1000..2000"}`,
	} {
		if status, out := sendJSON(t, app, "POST", "/api/tables/G_L_Account/insert", body); status != 200 {
			t.Fatalf("insert account %s: %d %v", body, status, out["error"])
		}
	}
	if status, out := sendJSON(t, app, "POST", "/api/tables/Customer/insert", `{"no":"C1","name":"Acme"}`); status != 200 {
		t.Fatalf("insert customer: %d %v", status, out["error"])
	}

	// Bal. Account No.: a G/L account for type G/L Account, a customer for type Customer, a
	// vendor for type Vendor; any value for a type whose table does not exist yet (Employee).
	// G/L entries are posted (the table API does not insert them): checked on the record.
	src := ftables.DBSource(db, "TEST", database.DBTypeSQLite)
	type entrySpec struct {
		entryNo int
		account string
		date    string
		amount  string
		balType gtables.GLEntryBal_account_type
		balNo   string
	}
	for spec, ok := range map[entrySpec]bool{
		{1, "1000", "2026-01-10", "100", 0, "2000"}: true,
		{2, "2000", "2026-02-10", "-100", 1, "C1"}:  true,
		{3, "2000", "2026-02-10", "0", 1, "2000"}:   false,
		{4, "2000", "2026-02-10", "0", 2, "V9"}:     false,
		{5, "2000", "2026-02-10", "0", 5, "E9"}:     true,  // employees: no table yet
		{6, "2000", "2026-02-10", "0", 0, "2999"}:   false, // a total account: posting accounts only
	} {
		glEntry := gtables.NewGLEntry(src)
		glEntry.Entry_no = spec.entryNo
		glEntry.Gl_account_no = types.Code(spec.account)
		glEntry.Posting_date = types.MustDate(spec.date)
		glEntry.Amount = types.MustDecimal(spec.amount)
		glEntry.Bal_account_type = spec.balType
		err := glEntry.Validate("bal_account_no", types.Code(spec.balNo))
		if err == nil {
			err = glEntry.InsertOrError(false)
		}
		if (err == nil) != ok {
			t.Errorf("%+v: %v, want ok=%v", spec, err, ok)
		}
	}

	// Lookups: one variant per type with a table (G/L Account, Customer, Vendor, Bank Account, Fixed Asset)
	_, out := getJSON(t, app, "/api/tables/G_L_Entry/list")
	lookup := out["captions"].(map[string]interface{})["lookups"].(map[string]interface{})["bal_account_no"].(map[string]interface{})
	variants := lookup["variants"].(map[string]interface{})
	if lookup["variant_field"] != "bal_account_type" || len(variants) != 5 || variants["1"].(map[string]interface{})["rows"].([]interface{})[0].(map[string]interface{})["_key"] != "C1" {
		t.Errorf("bal_account_no lookup = %v", lookup)
	}
	// The G/L Account variant shows the posting accounts only (classic WHERE Account Type=CONST(Posting))
	if filters := fmt.Sprint(variants["0"].(map[string]interface{})["filters"]); filters != "[map[column:account_type const:0]]" {
		t.Errorf("G/L Account variant filters = %s", filters)
	}
	if status, out := getJSON(t, app, "/api/tables/G_L_Entry/lookup/bal_account_no?variant=0"); status != 200 || len(out["data"].(map[string]interface{})["rows"].([]interface{})) != 2 {
		t.Errorf("lazy lookup of variant 0 (posting accounts 1000, 2000): %d %v", status, out)
	}
	if status, out := getJSON(t, app, "/api/tables/G_L_Entry/lookup/bal_account_no?variant=1"); status != 200 || out["data"].(map[string]interface{})["rows"].([]interface{})[0].(map[string]interface{})["_key"] != "C1" {
		t.Errorf("lazy lookup of variant 1: %d %v", status, out)
	}

	// Totals in a list: the total account sums its range; a date filter narrows them
	totals := func(query string) map[string]string {
		_, out := getJSON(t, app, "/api/tables/G_L_Account/list"+query)
		got := map[string]string{}
		for _, r := range out["data"].(map[string]interface{})["records"].([]interface{}) {
			rec := r.(map[string]interface{})
			got[rec["no"].(string)] = fmt.Sprint(rec["net_change"])
		}
		return got
	}
	if got := totals(""); got["1000"] != "100" || got["2000"] != "-100" || got["2999"] != "0" {
		t.Errorf("net change = %v", got)
	}
	if got := totals("?flow_filters=" + url.QueryEscape(`[{"field":"date_filter","expression":"2026-01-01..2026-01-31"}]`)); got["2999"] != "100" || got["2000"] != "0" {
		t.Errorf("net change in January = %v", got)
	}
}
