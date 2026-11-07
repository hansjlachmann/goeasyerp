package tables

import (
	"database/sql"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// addPeriods inserts monthly accounting periods from a year's January to the next year's
// January (a new fiscal year each January); closed and date locked through closedUntil
func addPeriods(t *testing.T, db *sql.DB, year, years int, closedUntil types.Date) {
	t.Helper()
	for i := 0; i <= years*12; i++ {
		p := gtables.NewAccountingPeriod(ftables.DBSource(db, "c1", database.DBTypeSQLite))
		start := types.NewDate(year+i/12, time.Month(1+i%12), 1)
		if err := p.Validate("starting_date", start); err != nil {
			t.Fatal(err)
		}
		p.New_fiscal_year = i%12 == 0
		p.Closed = !start.After(closedUntil)
		p.Date_locked = p.Closed
		if err := p.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
}

// Accounting Period: the name is the month; a date locked period cannot be deleted, renamed or
// changed to a new fiscal year, and no period may be inserted before a date locked one
func TestAccountingPeriodTriggers(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.AccountingPeriod{})
	addPeriods(t, db, 2025, 1, types.NewDate(2025, 12, 1))
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)

	p := gtables.NewAccountingPeriod(src)
	if err := p.GetOrError(types.NewDate(2025, 3, 1)); err != nil || p.Name != "March" {
		t.Fatalf("March period: %q, %v", p.Name, err)
	}
	if p.Delete(true) {
		t.Error("a date locked period was deleted")
	}
	if err := p.Validate("new_fiscal_year", true); err == nil {
		t.Error("New Fiscal Year changed in a date locked period")
	}
	before := gtables.NewAccountingPeriod(src)
	before.Starting_date = types.NewDate(2024, 12, 1)
	if before.Insert(true) {
		t.Error("a period was inserted before a date locked one")
	}
	open := gtables.NewAccountingPeriod(src)
	if !open.Get(types.NewDate(2026, 1, 1)) || !open.Delete(true) {
		t.Errorf("the open period: %v", open.TriggerError())
	}
}

// DateFilter-Calc: the filter and name of the accounting period and fiscal year of a date,
// one before, and the open end after the last period
func TestDateFilterCalc(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.AccountingPeriod{})
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	if filter, name := CreateAccountingPeriodFilter(src, types.NewDate(2026, 5, 17), 0); filter != "" || name != "" {
		t.Errorf("without periods: %q %q", filter, name)
	}
	addPeriods(t, db, 2025, 2, types.Date{})
	for _, c := range []struct {
		fiscalYear bool
		date       types.Date
		step       int
		filter     string
		name       string
	}{
		{false, types.NewDate(2026, 5, 17), 0, "2026-05-01..2026-05-31", "May"},
		{false, types.NewDate(2026, 5, 1), -1, "2026-04-01..2026-04-30", "April"},
		{true, types.NewDate(2026, 5, 17), 0, "2026-01-01..2026-12-31", "Fiscal Year 2026"},
		{true, types.NewDate(2026, 5, 17), -1, "2025-01-01..2025-12-31", "Fiscal Year 2025"},
		{true, types.NewDate(2026, 5, 17), -2, "..2024-12-31", "..."},
		{true, types.NewDate(2027, 3, 1), 0, "2027-01-01..9999-12-31", "Fiscal Year 2027..."},
		{true, types.NewDate(2024, 6, 1), 0, "..2024-12-31", "..."}, // before the first year
	} {
		var filter, name string
		if c.fiscalYear {
			filter, name = CreateFiscalYearFilter(src, c.date, c.step, "Fiscal Year %1")
		} else {
			filter, name = CreateAccountingPeriodFilter(src, c.date, c.step)
		}
		if filter != c.filter || name != c.name {
			t.Errorf("%v %s %d: %q %q, want %q %q", c.fiscalYear, c.date, c.step, filter, name, c.filter, c.name)
		}
	}
}

// Customer OnDelete: only entries of a fiscal year that is not closed refuse the delete; the
// customer's comments, bank accounts and ship-to addresses go with it. A G/L account's
// comments go with it. Ship-to addresses get the customer's name; Comment is an Exist FlowField
func TestCustomerDeleteWithPeriodsAndRelatedRecords(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.Customer{}, &gtables.CustomerLedgerEntry{}, &gtables.AccountingPeriod{},
		&gtables.CommentLine{}, &gtables.ShipToAddress{}, &gtables.CustomerBankAccount{}, &gtables.GLAccount{}, &gtables.GLEntry{})
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	addPeriods(t, db, 2025, 2, types.NewDate(2025, 12, 1))

	cust := gtables.NewCustomer(src)
	cust.No, cust.Name = "C1", "Fjordkraft AS"
	if err := cust.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	entry := gtables.NewCustomerLedgerEntry(src)
	entry.Entry_no, entry.Customer_no, entry.Posting_date = 1, "C1", types.NewDate(2026, 2, 1)
	if err := entry.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	shipTo := gtables.NewShipToAddress(src)
	shipTo.Customer_no, shipTo.Code = "C1", "MAIN"
	if err := shipTo.InsertOrError(true); err != nil || shipTo.Name != "Fjordkraft AS" {
		t.Fatalf("ship-to name %q, %v", shipTo.Name, err)
	}
	bankAcc := gtables.NewCustomerBankAccount(src)
	bankAcc.Customer_no, bankAcc.Code = "C1", "DNB"
	if err := bankAcc.InsertOrError(true); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		tableName gtables.CommentLineTable_name
		no        types.Code
	}{{gtables.CommentLine_Table_name.Customer, "C1"}, {gtables.CommentLine_Table_name.GLAccount, "C1"}} {
		comment := gtables.NewCommentLine(src)
		comment.Table_name, comment.No, comment.Line_no = c.tableName, c.no, 10000
		SetUpNewLine(comment)
		if !comment.Date.Equal(system.WorkDate()) {
			t.Errorf("first comment date %v", comment.Date)
		}
		comment.Comment = "Note"
		if err := comment.InsertOrError(true); err != nil {
			t.Fatal(err)
		}
	}
	second := gtables.NewCommentLine(src)
	second.Table_name, second.No, second.Line_no = gtables.CommentLine_Table_name.Customer, "C1", 20000
	SetUpNewLine(second)
	if !second.Date.IsZero() {
		t.Errorf("second comment date %v", second.Date)
	}
	cust.CalcFields("comment")
	if !cust.Comment {
		t.Error("Comment is false with a comment line")
	}

	// The entry is in the open fiscal year 2026: refused
	if cust.Delete(true) || errCode(t, cust.TriggerError()) != apperrors.ErrDeleteHasEntries {
		t.Fatalf("delete with an entry in an open year: %v", cust.TriggerError())
	}
	// In the closed year 2025 it does not count (it is not open either)
	entry.Posting_date = types.NewDate(2025, 6, 1)
	if err := entry.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := cust.DeleteOrError(true); err != nil {
		t.Fatalf("delete with an entry in a closed year: %v", err)
	}
	comments := gtables.NewCommentLine(src)
	comments.SetRange("table_name", gtables.CommentLine_Table_name.Customer)
	if comments.Count() != 0 || shipTo.Get(types.Code("C1")) || gtables.NewCustomerBankAccount(src).Count() != 0 || gtables.NewShipToAddress(src).Count() != 0 {
		t.Error("the customer's comments, bank accounts or ship-to addresses are left")
	}
	if !entry.Get(1) || entry.Customer_no != "" {
		t.Errorf("entry customer no %q", entry.Customer_no)
	}

	glAcc := gtables.NewGLAccount(src)
	glAcc.No = "C1"
	if err := glAcc.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := glAcc.DeleteOrError(true); err != nil {
		t.Fatal(err)
	}
	if gtables.NewCommentLine(src).Count() != 0 {
		t.Error("the G/L account's comment is left")
	}
}

// Next after Get / FindFirst / FindLast moves from the record in the current key within the
// filters (classic NEXT after FIND); a step past the end leaves the last record reached
func TestNextAfterFind(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.AccountingPeriod{})
	addPeriods(t, db, 2025, 1, types.Date{})
	p := gtables.NewAccountingPeriod(ftables.DBSource(db, "c1", database.DBTypeSQLite))
	if !p.Get(types.NewDate(2025, 3, 1)) || !p.Next() || p.Name != "April" || !p.Next(-2) || p.Name != "February" {
		t.Fatalf("Next from March: %s", p.Name)
	}
	p.SetRange("new_fiscal_year", true)
	if !p.FindFirst() || !p.Next() || !p.Starting_date.Equal(types.NewDate(2026, 1, 1)) {
		t.Fatalf("next fiscal year: %s", p.Starting_date)
	}
	if p.Next() || !p.Starting_date.Equal(types.NewDate(2026, 1, 1)) {
		t.Errorf("past the last one: %s", p.Starting_date)
	}
	if p.Next(-3) || !p.Starting_date.Equal(types.NewDate(2025, 1, 1)) {
		t.Errorf("three back from 2026 (two years): %s", p.Starting_date)
	}
}
