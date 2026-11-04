package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/accsched"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The demo account schedules on the demo ledger: the income statement's Net Income is the
// income statement accounts' net change (reversed), the balance sheet balances, the overview and
// the printout show them
func TestAccountSchedules(t *testing.T) {
	src := listCompany(t)
	workDate := system.WorkDate()
	period := accsched.FindFiscalYear(src, workDate).String() + ".." + workDate.String()

	// the expected net income: -(net change of 3000..6999)
	glAcc := gtables.NewGLAccount(src)
	glAcc.Account_type = gtables.GLAccount_Account_type.Total
	glAcc.Totaling = "3000..6999"
	mustOK(t, glAcc.SetFlowFilter("date_filter", period))
	glAcc.CalcFields("net_change")
	netIncome := glAcc.Net_change.Neg()

	line := func(schedule, rowNo string) *gtables.AccScheduleLine {
		l := gtables.NewAccScheduleLine(src)
		l.SetRange("schedule_name", types.Code(schedule))
		l.SetRange("row_no", types.Code(rowNo))
		if !l.FindFirst() {
			t.Fatalf("%s %s: no line", schedule, rowNo)
		}
		return l
	}
	column := func(layout, colNo string) *gtables.AccSchedColumn {
		c := gtables.NewAccSchedColumn(src)
		c.SetRange("column_layout_name", types.Code(layout))
		c.SetRange("column_no", types.Code(colNo))
		if !c.FindFirst() {
			t.Fatalf("%s %s: no column", layout, colNo)
		}
		return c
	}
	m := accsched.New(src)
	filters := accsched.Filters{DateFilter: period}
	got, err := m.CalcCell(filters, line("INCOME", "R120"), column("ACTUAL", "A"))
	mustOK(t, err)
	if !got.Round(2).Equal(netIncome.Round(2)) || netIncome.IsZero() {
		t.Errorf("net income %s, want %s", got, netIncome)
	}
	assets, err := m.CalcCell(filters, line("BALANCE", "B50"), column("BALANCE", "A"))
	mustOK(t, err)
	liabilities, err := m.CalcCell(filters, line("BALANCE", "B90"), column("BALANCE", "A"))
	mustOK(t, err)
	if assets.IsZero() || !assets.Round(2).Equal(liabilities.Round(2)) {
		t.Errorf("balance sheet: assets %s, liabilities and equity %s", assets, liabilities)
	}

	// the overview: the layout's five columns, the net income in the first
	name := gtables.NewAccScheduleName(src)
	name.Get(types.Code("INCOME"))
	lines := gtables.NewAccScheduleLine(src)
	lines.SetRange("schedule_name", types.Code("INCOME"))
	mustOK(t, lines.SetFlowFilter("date_filter", period))
	columns, values, err := accsched.Overview(name, lines)
	mustOK(t, err)
	if len(columns) != 5 || columns[3].Percent != true {
		t.Fatalf("overview columns: %+v", columns)
	}
	netIncomeLine := line("INCOME", "R120")
	cell := values["INCOME,"+itoa(netIncomeLine.Line_no)][0]
	if v, _ := types.NewDecimalFromString(cell); !v.Round(2).Equal(netIncome.Round(2)) {
		t.Errorf("overview net income %q, want %s", cell, netIncome)
	}

	// the printout from the schedule: its lines and the net income
	result, err := reports.Execute(reports.AccountScheduleID, src.GetDB(), src.GetCompany(), src.GetDBType(),
		reports.RunOptions{Output: reports.OutputPDF, Context: name})
	mustOK(t, err)
	texts := printedTexts(result, 0)
	for _, want := range []string{"Income Statement", "Actual against Budget", "Gross Profit", "Net Income", "% of Budget", amountText(netIncome)} {
		if !strings.Contains(texts, want) {
			t.Errorf("account schedule does not print %q", want)
		}
	}
}

// Insert Accounts puts the G/L accounts of the filter before the current line; the lines from
// there on move down
func TestAccSchedInsertAccounts(t *testing.T) {
	src := listCompany(t)
	current := gtables.NewAccScheduleLine(src)
	current.SetRange("schedule_name", types.Code("INCOME"))
	current.SetRange("row_no", types.Code("R40"))
	if !current.FindFirst() {
		t.Fatal("no line R40")
	}
	before := gtables.NewAccScheduleLine(src)
	before.SetRange("schedule_name", types.Code("INCOME"))
	count := before.Count()
	accounts := gtables.NewGLAccount(src)
	accounts.SetFilter("no", "5110..5130")
	_, err := reports.Execute(reports.AccSchedInsertAccID, src.GetDB(), src.GetCompany(), src.GetDBType(),
		reports.RunOptions{Context: current, Record: accounts})
	mustOK(t, err)
	if n := before.Count(); n != count+3 {
		t.Fatalf("%d lines, want %d", n, count+3)
	}
	inserted := gtables.NewAccScheduleLine(src)
	mustOK(t, inserted.GetOrError(map[string]interface{}{"schedule_name": "INCOME", "line_no": current.Line_no}))
	if inserted.Row_no != "5110" || inserted.Totaling != "5110" || inserted.Totaling_type != gtables.AccScheduleLine_Totaling_type.PostingAccounts {
		t.Errorf("inserted line: %s %s %v", inserted.Row_no, inserted.Totaling, inserted.Totaling_type)
	}
	moved := gtables.NewAccScheduleLine(src)
	moved.SetRange("schedule_name", types.Code("INCOME"))
	moved.SetRange("row_no", types.Code("R40"))
	if !moved.FindFirst() || moved.Line_no != current.Line_no+30000 {
		t.Errorf("line R40 at %d, want %d", moved.Line_no, current.Line_no+30000)
	}
}

func itoa(i int) string { return types.NewDecimalFromInt(int64(i)).String() }
