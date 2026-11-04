package pages

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 151 sums the customer's sales and profit for this period, this year, last year and
// to date (the work date's accounting periods); without periods only To Date has amounts
func TestCustomerStatistics(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.Customer{}, &gtables.CustomerLedgerEntry{}, &gtables.AccountingPeriod{}} {
		if err := tbl.CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	cust := gtables.NewCustomer(src)
	cust.No = "C1"
	if err := cust.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	workDate := system.WorkDate()
	thisMonth := types.NewDate(workDate.Year(), workDate.Month(), 1)
	// Sales of 100 (profit 25) this month, 200 earlier this year or last month, 400 last year
	for i, e := range []struct {
		date  types.Date
		sales int64
	}{{thisMonth, 100}, {thisMonth.AddMonths(-1), 200}, {types.NewDate(workDate.Year()-1, 6, 1), 400}} {
		entry := gtables.NewCustomerLedgerEntry(src)
		entry.Entry_no, entry.Customer_no, entry.Posting_date = i+1, "C1", e.date
		entry.Sales_lcy, entry.Profit_lcy = types.NewDecimalFromInt(e.sales), types.NewDecimalFromInt(e.sales/4)
		if err := entry.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	values := func() map[string]interface{} {
		t.Helper()
		control, err := pages.RunAfterGetRecordControl(151, cust)
		if err != nil {
			t.Fatal(err)
		}
		return control.Values()
	}
	sales := func(v map[string]interface{}, column string) string {
		return fmt.Sprint(v[column+"_sales_lcy"])
	}

	v := values()
	if sales(v, "this_period") != "0" || sales(v, "this_year") != "0" || sales(v, "to_date") != "700" || fmt.Sprint(v["to_date_profit_pct"]) != "25" {
		t.Errorf("without periods: %v", v)
	}
	// Monthly periods from last year's January to next year's January
	for i := 0; i <= 24; i++ {
		p := gtables.NewAccountingPeriod(src)
		if err := p.Validate("starting_date", types.NewDate(workDate.Year()-1+i/12, time.Month(1+i%12), 1)); err != nil {
			t.Fatal(err)
		}
		p.New_fiscal_year = i%12 == 0
		if err := p.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	v = values()
	wantThisYear := "300"
	if workDate.Month() == time.January {
		wantThisYear = "100" // last month was last year
	}
	if sales(v, "this_period") != "100" || sales(v, "this_year") != wantThisYear || sales(v, "to_date") != "700" ||
		v["this_period_name"] != workDate.Month().String() {
		t.Errorf("with periods: %v", v)
	}
	if v["this_year_name"] == "" || v["last_year_name"] == "" {
		t.Errorf("fiscal year names %q %q", v["this_year_name"], v["last_year_name"])
	}
}
