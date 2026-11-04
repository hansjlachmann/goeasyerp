package demodata

import (
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// demoColumn is a column of a demo column layout
type demoColumn struct {
	no, header      string
	columnType      gtables.AccSchedColumnColumn_type
	ledgerEntryType gtables.AccSchedColumnLedger_entry_type
	formula         string
	comparison      string
	oppositeSign    bool // the lines reverse their sign: a ratio of two amounts turns back
}

// demoLine is a line of a demo account schedule
type demoLine struct {
	rowNo, description, totaling string
	totalingType                 gtables.AccScheduleLineTotaling_type
	reverse                      bool
	show                         gtables.AccScheduleLineShow
}

// insertDemoAccSchedules: two account schedules on the demo chart of accounts — INCOME (the
// income statement against the budget: actual, budget, variance, % of budget, year to date) and
// BALANCE (the balance sheet this year and a year before) — with their column layouts. Formula
// lines add the rows' amounts as posted, so the results reverse their sign like the revenue and
// cost lines.
func insertDemoAccSchedules(src ftables.Source, counts *Counts) error {
	colType := gtables.AccSchedColumn_Column_type
	glEntries := gtables.AccSchedColumn_Ledger_entry_type.GLEntries
	budgetEntries := gtables.AccSchedColumn_Ledger_entry_type.GLBudgetEntries
	err := insertDemoColumnLayout(src, "ACTUAL", "Actual against Budget", []demoColumn{
		{no: "A", header: "Actual", columnType: colType.NetChange, ledgerEntryType: glEntries},
		{no: "B", header: "Budget", columnType: colType.NetChange, ledgerEntryType: budgetEntries},
		{no: "C", header: "Variance", columnType: colType.Formula, formula: "A-B"},
		{no: "D", header: "% of Budget", columnType: colType.Formula, formula: "A%B", oppositeSign: true},
		{no: "E", header: "Year to Date", columnType: colType.YearToDate, ledgerEntryType: glEntries},
	}, counts)
	if err != nil {
		return err
	}
	err = insertDemoColumnLayout(src, "BALANCE", "Balance this Year and Last Year", []demoColumn{
		{no: "A", header: "Balance", columnType: colType.BalanceAtDate, ledgerEntryType: glEntries},
		{no: "B", header: "Last Year", columnType: colType.BalanceAtDate, ledgerEntryType: glEntries, comparison: "-1Y"},
		{no: "C", header: "Change", columnType: colType.Formula, formula: "A-B"},
	}, counts)
	if err != nil {
		return err
	}

	posting := gtables.AccScheduleLine_Totaling_type.PostingAccounts
	formula := gtables.AccScheduleLine_Totaling_type.Formula
	underline := gtables.AccScheduleLine_Totaling_type.Underline
	doubleUnderline := gtables.AccScheduleLine_Totaling_type.DoubleUnderline
	yes := gtables.AccScheduleLine_Show.Yes
	bold := gtables.AccScheduleLine_Show.Bold
	err = insertDemoAccSchedule(src, "INCOME", "Income Statement", "ACTUAL", []demoLine{
		{"R10", "Revenue", "3100..3199", posting, true, yes},
		{"R20", "Cost of Goods Sold", "4000..4199", posting, true, yes},
		{"", "", "", underline, false, yes},
		{"R30", "Gross Profit", "R10+R20", formula, true, bold},
		{"R40", "Personnel", "5100..5199", posting, true, yes},
		{"R50", "Premises", "5200..5299", posting, true, yes},
		{"R60", "Vehicles", "5300..5399", posting, true, yes},
		{"R70", "Administration", "5400..5499", posting, true, yes},
		{"R80", "Marketing", "5500..5599", posting, true, yes},
		{"R90", "Depreciation", "5600..5699", posting, true, yes},
		{"", "", "", underline, false, yes},
		{"R95", "Total Operating Expenses", "R40..R90", formula, true, bold},
		{"R100", "Operating Result", "R30+R95", formula, true, bold},
		{"R110", "Financial Items", "6000..6199", posting, true, yes},
		{"", "", "", doubleUnderline, false, yes},
		{"R120", "Net Income", "R100+R110", formula, true, bold},
	}, counts)
	if err != nil {
		return err
	}
	return insertDemoAccSchedule(src, "BALANCE", "Balance Sheet", "BALANCE", []demoLine{
		{"B10", "Fixed Assets", "1200..1299", posting, false, yes},
		{"B20", "Inventory", "1300..1399", posting, false, yes},
		{"B30", "Receivables", "1400..1499", posting, false, yes},
		{"B40", "Liquid Assets", "1500..1599", posting, false, yes},
		{"", "", "", underline, false, yes},
		{"B50", "Total Assets", "B10..B40", formula, false, bold},
		{"B60", "Equity", "2100..2199", posting, true, yes},
		{"B65", "Net Income", "3000..6999", posting, true, yes},
		{"B70", "Long-term Liabilities", "2200..2299", posting, true, yes},
		{"B80", "Short-term Liabilities", "2300..2399", posting, true, yes},
		{"", "", "", underline, false, yes},
		{"B90", "Total Liabilities and Equity", "B60..B80", formula, true, bold},
	}, counts)
}

func insertDemoColumnLayout(src ftables.Source, name, description string, columns []demoColumn, counts *Counts) error {
	layout := gtables.NewAccSchedColumnLayoutName(src)
	layout.Name = types.Code(name)
	layout.Description = types.Text(description)
	err := layout.InsertOrError(true)
	if err != nil {
		return err
	}
	for i, c := range columns {
		column := gtables.NewAccSchedColumn(src)
		column.InitRecord()
		column.Column_layout_name = layout.Name
		column.Line_no = (i + 1) * 10000
		column.Column_no = types.Code(c.no)
		column.Column_header = types.Text(c.header)
		column.Column_type = c.columnType
		column.Ledger_entry_type = c.ledgerEntryType
		column.Formula = types.Code(c.formula)
		column.Comparison_period_formula = types.Code(c.comparison)
		column.Show_opposite_sign = c.oppositeSign
		err = column.InsertOrError(true)
		if err != nil {
			return err
		}
		counts.AccSchedColumns++
	}
	return nil
}

func insertDemoAccSchedule(src ftables.Source, name, description, columnLayout string, lines []demoLine, counts *Counts) error {
	accSchedName := gtables.NewAccScheduleName(src)
	accSchedName.Name = types.Code(name)
	accSchedName.Description = types.Text(description)
	accSchedName.Default_column_layout = types.Code(columnLayout)
	err := accSchedName.InsertOrError(true)
	if err != nil {
		return err
	}
	for i, l := range lines {
		line := gtables.NewAccScheduleLine(src)
		line.InitRecord()
		line.Schedule_name = accSchedName.Name
		line.Line_no = (i + 1) * 10000
		line.Row_no = types.Code(l.rowNo)
		line.Description = types.Text(l.description)
		line.Totaling_type = l.totalingType
		line.Totaling = types.Text(l.totaling)
		line.Show = l.show
		if l.reverse {
			line.Sign_control = gtables.AccScheduleLine_Sign_control.Reverse
		}
		err = line.InsertOrError(true)
		if err != nil {
			return err
		}
		counts.AccScheduleLines++
	}
	return nil
}
