package accsched

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// testCompany: revenue 3100 and costs 4100 with entries in 2025 and 2026, a budget, monthly
// accounting periods of 2025-2026 with fiscal years from January
func testCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "c1", database.DBTypeSQLite))
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	for _, a := range []struct {
		no       string
		typ      gtables.GLAccountAccount_type
		totaling string
	}{{"3100", gtables.GLAccount_Account_type.Posting, ""}, {"4100", gtables.GLAccount_Account_type.Posting, ""},
		{"4999", gtables.GLAccount_Account_type.Total, "3000..4199"}} {
		acc := gtables.NewGLAccount(src)
		acc.No, acc.Account_type, acc.Totaling = types.Code(a.no), a.typ, types.Text(a.totaling)
		mustOK(t, acc.InsertOrError(false))
	}
	entryNo := 0
	post := func(acc string, date types.Date, amount int64) {
		entryNo++
		e := gtables.NewGLEntry(src)
		e.Entry_no, e.Gl_account_no, e.Posting_date = entryNo, types.Code(acc), date
		e.Amount = types.NewDecimalFromInt(amount)
		if amount > 0 {
			e.Debit_amount = e.Amount
		} else {
			e.Credit_amount = e.Amount.Neg()
		}
		mustOK(t, e.InsertOrError(false))
	}
	post("3100", types.NewDate(2025, 3, 10), -800)  // March last year
	post("3100", types.NewDate(2026, 1, 15), -500)  // January
	post("3100", types.NewDate(2026, 3, 10), -1000) // March
	post("4100", types.NewDate(2026, 3, 20), 400)
	post("4100", types.NewDate(2026, 3, 25), -100) // a credit on costs
	budget := gtables.NewGLBudgetName(src)
	budget.Name = "B"
	mustOK(t, budget.InsertOrError(false))
	b := gtables.NewGLBudgetEntry(src)
	b.Entry_no, b.Budget_name, b.Gl_account_no, b.Date, b.Amount = 1, "B", "3100", types.NewDate(2026, 3, 1), types.NewDecimalFromInt(-1250)
	mustOK(t, b.InsertOrError(false))
	for year := 2025; year <= 2026; year++ {
		for month := 1; month <= 12; month++ {
			p := gtables.NewAccountingPeriod(src)
			p.Starting_date = types.NewDate(year, time.Month(month), 1)
			p.New_fiscal_year = month == 1
			mustOK(t, p.InsertOrError(false))
		}
	}
	p := gtables.NewAccountingPeriod(src)
	p.Starting_date = types.NewDate(2027, 1, 1)
	p.New_fiscal_year = true
	mustOK(t, p.InsertOrError(false))
	return src
}

func addLine(t *testing.T, src ftables.Source, lineNo int, rowNo, totaling string, typ gtables.AccScheduleLineTotaling_type, reverse bool) *gtables.AccScheduleLine {
	t.Helper()
	line := gtables.NewAccScheduleLine(src)
	line.InitRecord()
	line.Schedule_name, line.Line_no, line.Row_no, line.Totaling, line.Totaling_type = "S", lineNo, types.Code(rowNo), types.Text(totaling), typ
	if reverse {
		line.Sign_control = gtables.AccScheduleLine_Sign_control.Reverse
	}
	mustOK(t, line.InsertOrError(true))
	return line
}

func addColumn(t *testing.T, src ftables.Source, lineNo int, colNo string, typ gtables.AccSchedColumnColumn_type, modify func(c *gtables.AccSchedColumn)) *gtables.AccSchedColumn {
	t.Helper()
	column := gtables.NewAccSchedColumn(src)
	column.InitRecord()
	column.Column_layout_name, column.Line_no, column.Column_no, column.Column_type = "L", lineNo, types.Code(colNo), typ
	if modify != nil {
		modify(column)
	}
	mustOK(t, column.InsertOrError(true))
	return column
}

// The cells of a schedule: account ranges and total accounts, row formulas, the sign control,
// the columns' periods (net change, balance, year to date, a year before, a period before),
// budget, debit/credit and formula columns with a percent
func TestCalcCell(t *testing.T) {
	src := testCompany(t)
	lt := gtables.AccScheduleLine_Totaling_type
	ct := gtables.AccSchedColumn_Column_type
	revenue := addLine(t, src, 10000, "R", "3100", lt.PostingAccounts, true)
	costs := addLine(t, src, 20000, "C", "4100", lt.PostingAccounts, false)
	result := addLine(t, src, 30000, "N", "R+C", lt.Formula, true)
	total := addLine(t, src, 40000, "T", "4999", lt.TotalAccounts, true)
	base := addLine(t, src, 50000, "BASE", "R", lt.SetBaseForPercent, false)
	share := addLine(t, src, 60000, "SH", "C", lt.Formula, false)
	if !gtables.NewAccScheduleName(src).Get(types.Code("S")) {
		t.Error("the first line did not create schedule S")
	}

	netChange := addColumn(t, src, 10000, "NC", ct.NetChange, nil)
	balance := addColumn(t, src, 20000, "BAL", ct.BalanceAtDate, nil)
	ytd := addColumn(t, src, 30000, "YTD", ct.YearToDate, nil)
	lastYear := addColumn(t, src, 40000, "LY", ct.NetChange, func(c *gtables.AccSchedColumn) { c.Comparison_period_formula = "-1Y" })
	lastPeriod := addColumn(t, src, 50000, "LP", ct.NetChange, func(c *gtables.AccSchedColumn) { c.Comparison_period_formula = "-2P" })
	budget := addColumn(t, src, 60000, "BUD", ct.NetChange, func(c *gtables.AccSchedColumn) {
		c.Ledger_entry_type = gtables.AccSchedColumn_Ledger_entry_type.GLBudgetEntries
	})
	variance := addColumn(t, src, 70000, "VAR", ct.Formula, func(c *gtables.AccSchedColumn) { c.Formula = "NC-BUD" })
	pct := addColumn(t, src, 80000, "PCT", ct.Formula, func(c *gtables.AccSchedColumn) { c.Formula = "NC%" })
	credit := addColumn(t, src, 90000, "CR", ct.NetChange, func(c *gtables.AccSchedColumn) {
		c.Amount_type = gtables.AccSchedColumn_Amount_type.CreditAmount
	})
	opposite := addColumn(t, src, 100000, "OPP", ct.NetChange, func(c *gtables.AccSchedColumn) { c.Show_opposite_sign = true })

	m := New(src)
	march := Filters{DateFilter: "2026-03-01..2026-03-31", BudgetFilter: "B"}
	cases := []struct {
		name   string
		line   *gtables.AccScheduleLine
		column *gtables.AccSchedColumn
		want   string
	}{
		{"revenue March (reversed)", revenue, netChange, "1000"},
		{"costs March", costs, netChange, "300"},
		{"result = R+C (raw signs, reversed)", result, netChange, "700"},
		{"total account 4999 (reversed)", total, netChange, "700"},
		{"revenue balance at end of March", revenue, balance, "2300"},
		{"revenue year to date", revenue, ytd, "1500"},
		{"revenue March last year", revenue, lastYear, "800"},
		{"revenue January (two periods before)", revenue, lastPeriod, "500"},
		{"revenue budget", revenue, budget, "1250"},
		{"revenue variance", revenue, variance, "-250"},
		{"costs as % of the base (revenue)", share, pct, "-30"},
		{"costs credit amount", costs, credit, "-100"},
		{"costs with opposite sign", costs, opposite, "-300"},
		{"base line itself", base, netChange, "-1000"},
	}
	for _, c := range cases {
		got, err := m.CalcCell(march, c.line, c.column)
		mustOK(t, err)
		want := types.MustDecimal(c.want)
		if !got.Round(2).Equal(want) {
			t.Errorf("%s: %s, want %s", c.name, got, want)
		}
	}

	// a division by zero is flagged, not an error
	zero := addColumn(t, src, 110000, "Z", ct.Formula, func(c *gtables.AccSchedColumn) { c.Formula = "NC/0" })
	_, err := m.CalcCell(march, revenue, zero)
	mustOK(t, err)
	if !m.GetDivisionError() {
		t.Error("division by zero not flagged")
	}

	// errors: a row number that does not exist, a circular reference, no date filter
	var appErr *apperrors.AppError
	bad := addLine(t, src, 70000, "BAD", "XX", lt.Formula, false)
	_, err = m.CalcCell(march, bad, netChange)
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrAccSchedIllegalRowNo {
		t.Errorf("unknown row: %v", err)
	}
	loop := addLine(t, src, 80000, "L1", "L2", lt.Formula, false)
	addLine(t, src, 90000, "L2", "L1", lt.Formula, false)
	_, err = m.CalcCell(march, loop, netChange)
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrAccSchedCircularReference {
		t.Errorf("circular reference: %v", err)
	}
	_, err = New(src).CalcCell(Filters{}, revenue, netChange)
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrAccSchedDateFilterRequired {
		t.Errorf("no date filter: %v", err)
	}

	// rounding and the overview's texts
	thousands := addColumn(t, src, 120000, "K", ct.NetChange, func(c *gtables.AccSchedColumn) {
		c.Rounding_factor = gtables.AccSchedColumn_Rounding_factor.N1000
	})
	text, err := CellText(New(src), march, revenue, thousands)
	mustOK(t, err)
	if text != "1" || DecimalPlaces(thousands) != "1:1" {
		t.Errorf("revenue in thousands: %q (%s)", text, DecimalPlaces(thousands))
	}
}

// Formulas are checked as they are entered: parentheses in pairs, no two operators in a row,
// an operand after each operator
func TestCheckAccSchedFormula(t *testing.T) {
	for formula, want := range map[string]apperrors.ErrorCode{
		"R10+R20":     "",
		"(A-B)/B%":    "",
		"R10++R20":    apperrors.ErrAccSchedTwoOperators,
		"R10+":        apperrors.ErrAccSchedOperandMissing,
		"(R10+)":      apperrors.ErrAccSchedOperandMissing,
		"(R10":        apperrors.ErrAccSchedMoreLeftParentheses,
		"R10)":        apperrors.ErrAccSchedParenthesisMisplaced,
		"R10..R90":    "",
		"-R10":        "",
		"(A)(B)":      "",
		"((A+B)*C)/D": "",
	} {
		err := tables.CheckAccSchedFormula(formula)
		var appErr *apperrors.AppError
		switch {
		case want == "" && err != nil:
			t.Errorf("%s: %v", formula, err)
		case want != "" && (!errors.As(err, &appErr) || appErr.Code != want):
			t.Errorf("%s: %v, want %s", formula, err, want)
		}
	}
}
