package reports

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// customers C01..C03 with ledger entries: C01 has 1, 2; C02 has 3; C03 has 4, 5, 6
func newLedger(t *testing.T) *sql.DB {
	t.Helper()
	db := newTestDB(t)
	for _, r := range []ftables.Table{&gtables.Customer{}, &gtables.CustomerLedgerEntry{}} {
		if err := r.(interface {
			CreateTableWithDBType(database.Executor, string, database.DBType) error
		}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	for i, no := range []string{"C01", "C02", "C03"} {
		c := &gtables.Customer{}
		c.InitWithDBType(db, "c1", database.DBTypeSQLite)
		c.No, c.Name, c.City = types.NewCode(no), types.NewText("Customer "+no), types.NewText([]string{"Oslo", "Bergen", "Oslo"}[i])
		if !c.Insert(false) {
			t.Fatal("customer")
		}
	}
	for no, customer := range map[int]string{1: "C01", 2: "C01", 3: "C02", 4: "C03", 5: "C03", 6: "C03"} {
		e := &gtables.CustomerLedgerEntry{}
		e.InitWithDBType(db, "c1", database.DBTypeSQLite)
		e.Entry_no, e.Customer_no, e.Amount_lcy = no, types.NewCode(customer), types.NewDecimalFromInt(int64(no*100))
		if !e.Insert(false) {
			t.Fatal("entry")
		}
	}
	return db
}

// trace records what a test report's triggers saw
type traceReport struct {
	log  []string
	skip map[string]bool // customer → Skip
	stop int             // entry no. → Break the entries
	quit string          // customer → Quit
}

func (r *traceReport) Definition() *Definition {
	return &Definition{
		Options: []Option{{Name: "factor", Type: "number", Default: "2"}},
		DataItems: []*DataItem{{
			Name: "Customer", Table: "Customer", Key: []string{"no"}, RequestFilterFields: []string{"no", "city"},
			OnAfterGetRecord: Trigger(func(c *gtables.Customer, run *Run) error {
				if string(c.No) == r.quit {
					return Quit()
				}
				r.log = append(r.log, string(c.No))
				if r.skip[string(c.No)] {
					return Skip()
				}
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "Entry", Table: "Customer_ledger_entry", Key: []string{"entry_no"}, Descending: true,
				Link: map[string]string{"customer_no": "no"},
				OnAfterGetRecord: Trigger(func(e *gtables.CustomerLedgerEntry, run *Run) error {
					if e.Entry_no == r.stop {
						return Break()
					}
					r.log = append(r.log, fmt.Sprintf("%d:%d", e.Entry_no, e.Entry_no*run.OptionInt("factor")))
					return nil
				}),
				OnPostDataItem: Trigger(func(e *gtables.CustomerLedgerEntry, run *Run) error {
					r.log = append(r.log, "/")
					return nil
				}),
			}},
		}},
		OnPostReport: func(run *Run) error {
			run.Message("done %1", len(r.log))
			return nil
		},
	}
}

func runTrace(t *testing.T, db *sql.DB, r *traceReport, opts RunOptions) (string, Result) {
	t.Helper()
	Register(99901, "trace-test", func() Report { return r })
	res, err := Execute(99901, db, "c1", database.DBTypeSQLite, opts)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(r.log, " "), res
}

// Data items nest and link, sort by their key, and run their triggers; Skip/Break/Quit
func TestDataItems(t *testing.T) {
	db := newLedger(t)
	got, res := runTrace(t, db, &traceReport{}, RunOptions{})
	if got != "C01 2:4 1:2 / C02 3:6 / C03 6:12 5:10 4:8 /" || res.Message != "done 12" {
		t.Errorf("all: %q, %q", got, res.Message)
	}
	got, _ = runTrace(t, db, &traceReport{skip: map[string]bool{"C02": true}, stop: 5}, RunOptions{Options: map[string]string{"factor": "1"}})
	if got != "C01 2:2 1:1 / C02 C03 6:6 /" {
		t.Errorf("Skip C02, Break at entry 5: %q", got)
	}
	got, res = runTrace(t, db, &traceReport{quit: "C02"}, RunOptions{})
	if got != "C01 2:4 1:2 /" || res.Message != "" {
		t.Errorf("Quit at C02: %q, %q", got, res.Message)
	}
	// A record's filters are the request filters of the data items of its table
	var filter gtables.Customer
	filter.SetFilter("city", "Oslo")
	got, _ = runTrace(t, db, &traceReport{}, RunOptions{Record: &filter})
	if got != "C01 2:4 1:2 / C03 6:12 5:10 4:8 /" {
		t.Errorf("filtered on Oslo: %q", got)
	}
}

// The request page asks for the options and, per data item, filters on any field
func TestRequestPage(t *testing.T) {
	db := newLedger(t)
	d := fcodeunits.OpenDialogWithID("", "report-request-test")
	fcodeunits.SetCurrentDialog(d)
	defer fcodeunits.ClearCurrentDialog()

	respond := func(values map[string]string) {
		go func() {
			ev := <-d.Events()
			fields, _ := ev.Data["fields"].([]fcodeunits.InputField)
			items, _ := ev.Data["data_items"].([]fcodeunits.RequestDataItem)
			if ev.EventType != "request_input" || len(fields) != 1 || fields[0].Name != "option:factor" || len(items) != 2 ||
				ev.Data["options_caption"] != "" {
				t.Errorf("request page %+v", ev.Data)
			} else {
				customer, entry := items[0], items[1]
				var fieldNames []string
				for _, f := range customer.Fields {
					fieldNames = append(fieldNames, f.Name)
				}
				all := strings.Join(fieldNames, ",")
				if customer.Name != "Customer" || customer.Indent != 0 || entry.Name != "Entry" || entry.Indent != 1 ||
					len(customer.Filters) != 2 || customer.Filters[0].Field != "no" || customer.Filters[1].Field != "city" ||
					!strings.Contains(all, "name") || !strings.Contains(all, "balance_lcy") || !strings.Contains(all, "date_filter") {
					t.Errorf("data items %+v / %+v", customer, entry)
				}
			}
			for !d.RespondToInput(ev.ConfirmID, values) {
				time.Sleep(time.Millisecond)
			}
		}()
	}
	// Any field can be filtered, also on the nested data item, with the list's filter syntax
	respond(map[string]string{
		"option:factor":    "10",
		"filters:Customer": `[{"field":"no","expression":"C02..C03"},{"field":"city","expression":""},{"field":"name","expression":"*3"}]`,
		"filters:Entry":    `[{"field":"entry_no","expression":"5|6"}]`,
	})
	got, _ := runTrace(t, db, &traceReport{}, RunOptions{RequestPage: true})
	if got != "C03 6:60 5:50 /" {
		t.Errorf("request page: %q", got)
	}
	respond(map[string]string{}) // Cancel
	got, res := runTrace(t, db, &traceReport{}, RunOptions{RequestPage: true})
	if got != "" || !res.Cancelled {
		t.Errorf("cancelled: %q", got)
	}
	// A filter on a field that cannot be filtered is an error, as on a list
	respond(map[string]string{"filters:Customer": `[{"field":"bogus","expression":"x"}]`})
	Register(99901, "trace-test", func() Report { return &traceReport{} })
	if _, err := Execute(99901, db, "c1", database.DBTypeSQLite, RunOptions{RequestPage: true}); err == nil {
		t.Error("filter on an unknown field accepted")
	}
}

// Request page options: a "bool" option reads Yes/No; an option with a table relation sends
// the table's rows as its dropdown, each code once (Rounding Method has a row per minimum
// amount)
func TestOptionBoolAndLookup(t *testing.T) {
	db := newTestDB(t)
	if err := (&gtables.RoundingMethod{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	for _, m := range []struct{ code, min string }{{"A", "0"}, {"A", "100"}, {"B", "0"}} {
		rec := gtables.NewRoundingMethod(src)
		rec.Code, rec.Minimum_amount = types.Code(m.code), types.MustDecimal(m.min)
		if err := rec.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	run := &Run{DB: db, Company: "c1", DBType: database.DBTypeSQLite, options: map[string]string{"a": "true", "b": "false", "c": "Yes"}}
	if !run.OptionBool("a") || run.OptionBool("b") || !run.OptionBool("c") || run.OptionBool("missing") {
		t.Errorf("OptionBool: a %v b %v c %v", run.OptionBool("a"), run.OptionBool("b"), run.OptionBool("c"))
	}
	lookup := optionLookup(run, &OptionRelation{Table: "Rounding_Method", Columns: []string{"code"}}, "en-US")
	if lookup == nil || len(lookup.Rows) != 2 || lookup.Rows[0]["_key"] != "A" || lookup.Rows[1]["_key"] != "B" || lookup.Columns[0].Source != "code" {
		t.Errorf("lookup = %+v", lookup)
	}
	if optionLookup(run, nil, "en-US") != nil {
		t.Error("an option without relation has a lookup")
	}
}

// txReport writes a rounding method and fails when told to
type txReport struct{ fail bool }

func (r *txReport) Definition() *Definition {
	return &Definition{
		Transaction: true,
		OnPreReport: func(run *Run) error {
			rec := gtables.NewRoundingMethod(run)
			rec.Code = "TX"
			if err := rec.InsertOrError(false); err != nil {
				return err
			}
			if r.fail {
				return fmt.Errorf("failed")
			}
			return nil
		},
	}
}

// A Transaction report's writes are kept when it succeeds and rolled back when it fails
func TestReportTransaction(t *testing.T) {
	db := newTestDB(t)
	if err := (&gtables.RoundingMethod{}).CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	Register(99902, "tx-test", func() Report { return &txReport{fail: true} })
	if _, err := Execute(99902, db, "c1", database.DBTypeSQLite, RunOptions{}); err == nil {
		t.Fatal("failing report: no error")
	}
	if n := gtables.NewRoundingMethod(src).Count(); n != 0 {
		t.Errorf("%d records after a failed run, want 0", n)
	}
	Register(99902, "tx-test", func() Report { return &txReport{} })
	if _, err := Execute(99902, db, "c1", database.DBTypeSQLite, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if n := gtables.NewRoundingMethod(src).Count(); n != 1 {
		t.Errorf("%d records after a run, want 1", n)
	}
}
