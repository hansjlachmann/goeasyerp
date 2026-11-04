package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"io"
	"math/rand"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/company"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	_ "github.com/lib/pq"
)

// renameDB is a database with the global Company and User Member tables, for company
// rename/delete tests: SQLite always, Postgres when TEST_POSTGRES_DSN is set (in a schema
// of its own, dropped afterwards).
type renameDB struct {
	db     *sql.DB
	dbType database.DBType
}

func renameDatabases(t *testing.T) []renameDB {
	t.Helper()
	lite, err := sql.Open("sqlite3", "file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lite.Close() })
	out := []renameDB{{lite, database.DBTypeSQLite}}

	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		admin, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		schema := fmt.Sprintf("rename_test_%d", rand.New(rand.NewSource(time.Now().UnixNano())).Int63())
		if _, err := admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
			t.Fatal(err)
		}
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		pg, err := sql.Open("postgres", dsn+sep+"search_path="+schema)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = pg.Close()
			_, _ = admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
			_ = admin.Close()
		})
		out = append(out, renameDB{pg, database.DBTypePostgres})
	}

	for _, d := range out {
		for _, tbl := range []interface {
			CreateTableWithDBType(database.Executor, string, database.DBType) error
		}{&gtables.Company{}, &gtables.UserMember{}} {
			if err := tbl.CreateTableWithDBType(d.db, "", d.dbType); err != nil {
				t.Fatalf("%s: create global table: %v", d.dbType, err)
			}
		}
	}
	return out
}

// newCompany creates a company with customers C1/C2, ledger entries (SIFT totals built by
// CreateTableWithDBType) and a User Member row for it.
func newCompany(t *testing.T, d renameDB, name string) {
	t.Helper()
	var c gtables.Company
	c.InitWithDBType(d.db, "", d.dbType)
	c.Name = types.NewText(name)
	c.Display_name = types.NewText("Display " + name)
	if !c.Insert(false) {
		t.Fatalf("insert company %s", name)
	}
	if err := (&gtables.Customer{}).CreateTableWithDBType(d.db, name, d.dbType); err != nil {
		t.Fatal(err)
	}
	if err := (&gtables.CustomerLedgerEntry{}).CreateTableWithDBType(d.db, name, d.dbType); err != nil {
		t.Fatal(err)
	}
	// a company table that refers to the company (Business Unit's Company Name: renamed along)
	if err := (&gtables.BusinessUnit{}).CreateTableWithDBType(d.db, name, d.dbType); err != nil {
		t.Fatal(err)
	}
	for _, no := range []string{"C1", "C2"} {
		var cust gtables.Customer
		cust.InitWithDBType(d.db, name, d.dbType)
		cust.No = types.NewCode(no)
		cust.Name = types.NewText("Customer " + no)
		if !cust.Insert(false) {
			t.Fatalf("insert customer %s", no)
		}
	}
	for i, e := range []struct {
		cust   string
		amount float64
		open   bool
	}{{"C1", 100, true}, {"C1", 50, false}, {"C2", 70, true}} {
		addEntry(t, d, name, i+1, e.cust, e.amount, e.open)
	}
	var m gtables.UserMember
	m.InitWithDBType(d.db, "", d.dbType)
	m.User_id = types.NewCode("HANS")
	m.Role_id = types.NewCode("READER")
	m.Company = types.NewText(name)
	if !m.Insert(false) {
		t.Fatal("insert user member")
	}
}

func addEntry(t *testing.T, d renameDB, companyName string, no int, cust string, amount float64, open bool) {
	t.Helper()
	var e gtables.CustomerLedgerEntry
	e.InitWithDBType(d.db, companyName, d.dbType)
	e.Entry_no = no
	e.Customer_no = types.NewCode(cust)
	e.Posting_date = types.NewDate(2026, 1, no)
	e.Open = open
	e.Amount_lcy = types.NewDecimal(amount)
	e.Sales_lcy = types.NewDecimal(amount)
	e.Remaining_amt_lcy = types.NewDecimal(0)
	if open {
		e.Remaining_amt_lcy = types.NewDecimal(amount)
	}
	if !e.Insert(false) {
		t.Fatalf("insert entry %d", no)
	}
}

// companyApp serves the table API for the company named in the X-Company header.
func companyApp(d renameDB) *fiber.App {
	h := NewTablesHandlerWithDBType(d.db, d.dbType)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: c.Get("X-Company"), Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/tables/:table/list", h.ListRecords)
	app.Put("/api/tables/:table/modify/:id", h.ModifyRecord)
	app.Delete("/api/tables/:table/delete/:id", h.DeleteRecord)
	return app
}

func call(t *testing.T, app *fiber.App, method, target, companyName, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Company", companyName)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	return resp.StatusCode, out
}

// companyObjects counts a company's tables and indexes (exact name prefix).
func companyObjects(t *testing.T, d renameDB, companyName string) (tablesN, indexes int) {
	t.Helper()
	query := `SELECT name, type FROM sqlite_master WHERE type IN ('table', 'index') AND name NOT LIKE 'sqlite_autoindex%'`
	if d.dbType == database.DBTypePostgres {
		query = `SELECT c.relname, CASE WHEN c.relkind = 'r' THEN 'table' ELSE 'index' END
			FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
			WHERE n.nspname = current_schema() AND c.relkind IN ('r', 'i')`
	}
	rows, err := d.db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(name, companyName+"$") {
			continue
		}
		if kind == "table" {
			tablesN++
		} else {
			indexes++
		}
	}
	return tablesN, indexes
}

// balances returns the customers' Balance (LCY) from the list endpoint (SIFT totals).
func balances(t *testing.T, app *fiber.App, companyName string) map[string]float64 {
	t.Helper()
	status, out := call(t, app, "GET", "/api/tables/Customer/list", companyName, "")
	if status != 200 {
		t.Fatalf("list customers of %s: %d %v", companyName, status, out["error"])
	}
	data, _ := out["data"].(map[string]interface{})
	recs, _ := data["records"].([]interface{})
	got := map[string]float64{}
	for _, r := range recs {
		rec := r.(map[string]interface{})
		var v float64
		_, _ = fmt.Sscan(fmt.Sprint(rec["balance_lcy"]), &v)
		got[fmt.Sprint(rec["no"])] = v
	}
	return got
}

func verifySIFT(t *testing.T, d renameDB, companyName string) {
	t.Helper()
	var e gtables.CustomerLedgerEntry
	e.InitWithDBType(d.db, companyName, d.dbType)
	results, err := e.VerifySIFT(false)
	if err != nil {
		t.Fatalf("verify SIFT of %s: %v", companyName, err)
	}
	if len(results) == 0 {
		t.Fatalf("%s has no SIFT keys", companyName)
	}
	for _, r := range results {
		if r.Differences != 0 {
			t.Errorf("%s: SIFT key %s has %d differences", companyName, r.Key, r.Differences)
		}
	}
}

func memberCompany(t *testing.T, d renameDB) string {
	t.Helper()
	var name string
	if err := d.db.QueryRow(`SELECT company FROM "User_Member" WHERE user_id = 'HANS' AND company <> 'omega' AND company <> 'gamma'`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	return name
}

// Renaming a company moves its data: tables, indexes and SIFT totals follow the new name
// (and keep working), User Member is updated, nothing named after the old name is left.
func TestRenameCompanyMovesItsData(t *testing.T) {
	for _, d := range renameDatabases(t) {
		t.Run(string(d.dbType), func(t *testing.T) {
			app := companyApp(d)
			newCompany(t, d, "alpha")
			wantTables, wantIndexes := companyObjects(t, d, "alpha")
			before := balances(t, app, "alpha")
			var busUnit gtables.BusinessUnit
			busUnit.InitWithDBType(d.db, "alpha", d.dbType)
			busUnit.Code, busUnit.Company_name = "SELF", "alpha"
			if !busUnit.Insert(false) {
				t.Fatal("insert business unit")
			}

			status, out := call(t, app, "PUT", "/api/tables/Company/modify/alpha", "alpha", `{"name":" Beta "}`)
			if status != 200 {
				t.Fatalf("rename: %d %v", status, out["error"])
			}
			if data, _ := out["data"].(map[string]interface{}); data["name"] != "beta" {
				t.Errorf("renamed to %v, want beta (trimmed, lower case)", data["name"])
			}
			busUnit.InitWithDBType(d.db, "beta", d.dbType)
			if !busUnit.Get(types.NewCode("SELF")) || busUnit.Company_name != "beta" {
				t.Errorf("the business unit's company name is %q, want beta", busUnit.Company_name)
			}

			if n, i := companyObjects(t, d, "alpha"); n+i != 0 {
				t.Errorf("alpha still has %d tables and %d indexes", n, i)
			}
			if n, i := companyObjects(t, d, "beta"); n != wantTables || i != wantIndexes {
				t.Errorf("beta has %d tables, %d indexes; alpha had %d, %d", n, i, wantTables, wantIndexes)
			}
			if got := balances(t, app, "beta"); fmt.Sprint(got) != fmt.Sprint(before) {
				t.Errorf("balances after rename %v, before %v", got, before)
			}
			if got := memberCompany(t, d); got != "beta" {
				t.Errorf("User Member company = %q, want beta", got)
			}

			// The rebuilt SIFT triggers maintain the totals of the renamed tables
			addEntry(t, d, "beta", 10, "C2", 30, true)
			verifySIFT(t, d, "beta")
			if got := balances(t, app, "beta")["C2"]; got != 100 {
				t.Errorf("C2 balance after a new entry = %v, want 100", got)
			}
		})
	}
}

// A rename that cannot be done changes nothing: the transaction rolls the dropped SIFT
// objects and renamed tables back.
func TestRenameCompanyRejectedOrRolledBack(t *testing.T) {
	for _, d := range renameDatabases(t) {
		t.Run(string(d.dbType), func(t *testing.T) {
			app := companyApp(d)
			newCompany(t, d, "alpha")
			newCompany(t, d, "gamma")
			wantTables, wantIndexes := companyObjects(t, d, "alpha")
			before := balances(t, app, "alpha")
			// Leftover table named like the target company (e.g. of a deleted company)
			if _, err := d.db.Exec(`CREATE TABLE "delta$Leftover" (x INTEGER)`); err != nil {
				t.Fatal(err)
			}

			// A User Member row for "omega" (no such company): the rename's last step, carrying
			// the name over to User Member, then hits its primary key — after the SIFT objects
			// were dropped, the tables renamed and the totals rebuilt
			var m gtables.UserMember
			m.InitWithDBType(d.db, "", d.dbType)
			m.User_id, m.Role_id, m.Company = types.NewCode("HANS"), types.NewCode("READER"), types.NewText("omega")
			if !m.Insert(false) {
				t.Fatal("insert user member omega")
			}

			for _, tc := range []struct{ name, body string }{
				{"existing company", `{"name":"gamma"}`},
				{"invalid name", `{"name":"Bad Name!"}`},
				{"leftover objects", `{"name":"delta"}`},
				{"fails at the last step", `{"name":"omega"}`},
			} {
				if status, _ := call(t, app, "PUT", "/api/tables/Company/modify/alpha", "alpha", tc.body); status == 200 {
					t.Errorf("%s: rename accepted", tc.name)
				}
			}

			if n, i := companyObjects(t, d, "alpha"); n != wantTables || i != wantIndexes {
				t.Errorf("alpha has %d tables, %d indexes after failed renames; want %d, %d", n, i, wantTables, wantIndexes)
			}
			if got := memberCompany(t, d); got != "alpha" {
				t.Errorf("User Member company = %q, want alpha", got)
			}
			// SIFT objects survived the rolled-back drop: totals still follow new entries
			addEntry(t, d, "alpha", 10, "C1", 5, true)
			verifySIFT(t, d, "alpha")
			if got := balances(t, app, "alpha")["C1"]; got != before["C1"]+5 {
				t.Errorf("C1 balance = %v, want %v", got, before["C1"]+5)
			}
		})
	}
}

// Deleting a company drops exactly its objects: "_" in its name is not a wildcard, so
// company a_b does not take the tables of company axb with it.
func TestDeleteCompanyDropsOnlyItsObjects(t *testing.T) {
	for _, d := range renameDatabases(t) {
		t.Run(string(d.dbType), func(t *testing.T) {
			app := companyApp(d)
			newCompany(t, d, "a_b")
			newCompany(t, d, "axb")
			wantTables, wantIndexes := companyObjects(t, d, "axb")

			if status, out := call(t, app, "DELETE", "/api/tables/Company/delete/a_b", "axb", ""); status != 200 {
				t.Fatalf("delete: %d %v", status, out["error"])
			}
			if n, i := companyObjects(t, d, "a_b"); n+i != 0 {
				t.Errorf("a_b still has %d tables and %d indexes", n, i)
			}
			if n, i := companyObjects(t, d, "axb"); n != wantTables || i != wantIndexes {
				t.Errorf("axb has %d tables, %d indexes; want %d, %d", n, i, wantTables, wantIndexes)
			}
			var defs int
			if err := d.db.QueryRow(`SELECT COUNT(*) FROM "_sift_definition" WHERE company = 'a_b'`).Scan(&defs); err != nil || defs != 0 {
				t.Errorf("SIFT definitions of a_b left: %d (%v)", defs, err)
			}
			verifySIFT(t, d, "axb")

			// Its tables can be listed by exact name
			if names, err := company.Tables(d.db, d.dbType, "axb"); err != nil || len(names) != wantTables {
				t.Errorf("company.Tables(axb) = %v, %v", names, err)
			}
		})
	}
}
