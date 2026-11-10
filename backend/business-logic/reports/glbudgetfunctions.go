package reports

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The G/L budget functions of G/L Budget by Periods 113 (Functions menu; the classic local reports
// 10030/10031 and dataport 10000, renumbered): 81 Budget Amount by Period (an amount spread over
// periods, growing by a percentage), 82 Budget from History (each period's net change of a past
// period, changed by a percentage) and 83 Import/Export G/L Budget (the budget's entries as a CSV
// file and back). Run from a budget, it is the budget written to; the G/L accounts are chosen in
// the request filter (posting accounts), Dimension 1 and 2 Filter of one value give the entries'
// dimension codes.
const (
	BudgetAmountByPeriodID = 81
	BudgetFromHistoryID    = 82
	ImportExportGLBudgetID = 83
)

func init() {
	Register(BudgetAmountByPeriodID, "budget-amount-by-period", func() Report { return &budgetAmountByPeriod{} })
	Register(BudgetFromHistoryID, "budget-from-history", func() Report { return &budgetFromHistory{} })
	Register(ImportExportGLBudgetID, "import-export-gl-budget", func() Report { return &importExportGLBudget{} })
}

// The period lengths and rounding of the request pages (classic PeriodLength and Rounding)
const (
	budgetPeriodDay = iota
	budgetPeriodWeek
	budgetPeriodMonth
	budgetPeriodQuarter
	budgetPeriodYear
	budgetPeriodAccountingPeriod
)

var budgetPeriodOptions = []string{"DAY", "WEEK", "MONTH", "QUARTER", "YEAR", "PERIOD"}
var budgetRoundingOptions = []string{"ROUND_0_05", "ROUND_1", "ROUND_100", "ROUND_1000", "ROUND_1000000"}
var budgetRoundingPrecision = []string{"0.05", "1", "100", "1000", "1000000"}

// budgetOptionsInit: run from a budget, the budget's name
func budgetOptionsInit(run *Run) error {
	glBudget, ok := run.Context().(*gtables.GLBudgetName)
	if ok && glBudget.Name != "" {
		run.SetOption("budget_name", glBudget.Name.String())
	}
	return nil
}

// budgetGLAccountItem: the G/L accounts the budget is made for (posting accounts, a No. filter)
func budgetGLAccountItem(onPreDataItem, onAfterGetRecord func(*gtables.GLAccount, *Run) error) *DataItem {
	return &DataItem{
		Name:                "GLAccount",
		Table:               gtables.GLAccountTableName,
		Key:                 []string{"no"},
		TableView:           []ftables.FieldFilter{{Field: "account_type", Expression: "0"}},
		RequestFilterFields: []string{"no", "dimension_1_filter", "dimension_2_filter"},
		OnPreDataItem:       Trigger(onPreDataItem),
		OnAfterGetRecord:    Trigger(onAfterGetRecord),
	}
}

// budgetPrecision: the rounding chosen (classic Pennies 0.05 … Millions)
func budgetPrecision(run *Run) types.Decimal {
	i := run.OptionInt("round_to")
	if i < 0 || i >= len(budgetRoundingPrecision) {
		i = 1
	}
	return types.MustDecimal(budgetRoundingPrecision[i])
}

// nextBudgetDate (NextDate): the start of the next period of the length chosen; an accounting
// period the next one's Starting Date (there must be one)
func nextBudgetDate(run *Run, currentDate types.Date, periodLength int) (types.Date, error) {
	switch periodLength {
	case budgetPeriodDay:
		return system.CalcDate("+1D", currentDate)
	case budgetPeriodWeek:
		return system.CalcDate("+1W", currentDate)
	case budgetPeriodMonth:
		return system.CalcDate("+1M", currentDate)
	case budgetPeriodQuarter:
		return system.CalcDate("+1Q", currentDate)
	case budgetPeriodYear:
		return system.CalcDate("+1Y", currentDate)
	}
	acctPeriod := gtables.NewAccountingPeriod(run)
	acctPeriod.SetFilter("starting_date", ">"+currentDate.String())
	if !acctPeriod.FindFirst() {
		return types.Date{}, system.Error(run.Text("NOT_ENOUGH_PERIODS"))
	}
	return acctPeriod.Starting_date, nil
}

// singleFilterValue: a filter of one value is the dimension code of the entries (classic
// GETFILTER); a range or several values give none
func singleFilterValue(filter string) types.Code {
	if strings.ContainsAny(filter, ".|*<>&=") {
		return ""
	}
	return types.Code(system.UpperCase(strings.TrimSpace(filter)))
}

// glBudgetEntryWriter numbers the new budget entries after the last one
type glBudgetEntryWriter struct {
	nextEntryNo int
	inserted    int
}

func (w *glBudgetEntryWriter) init(run *Run) {
	glBudgetEntry := gtables.NewGLBudgetEntry(run)
	glBudgetEntry.LockTable()
	w.nextEntryNo = 1
	if glBudgetEntry.FindLast() {
		w.nextEntryNo = glBudgetEntry.Entry_no + 1
	}
}

func (w *glBudgetEntryWriter) insert(run *Run, budgetName, glAccNo types.Code, date types.Date, dimension1Code, dimension2Code types.Code, amount types.Decimal, description types.Text) error {
	glBudgetEntry := gtables.NewGLBudgetEntry(run)
	glBudgetEntry.Entry_no = w.nextEntryNo
	glBudgetEntry.Budget_name = budgetName
	glBudgetEntry.Gl_account_no = glAccNo
	glBudgetEntry.Date = date
	glBudgetEntry.Dimension_1_code = dimension1Code
	glBudgetEntry.Dimension_2_code = dimension2Code
	glBudgetEntry.Amount = amount
	glBudgetEntry.Description = description
	err := glBudgetEntry.InsertOrError(true)
	if err != nil {
		return err
	}
	w.nextEntryNo++
	w.inserted++
	return nil
}

// budgetName: the budget written to (it must exist)
func budgetName(run *Run) (types.Code, error) {
	glBudget := gtables.NewGLBudgetName(run)
	err := glBudget.GetOrError(types.Code(system.UpperCase(run.Option("budget_name"))))
	if err != nil {
		return "", err
	}
	return glBudget.Name, nil
}

// Report 81 Budget Amount by Period (classic local 10030)
type budgetAmountByPeriod struct {
	glBudgetEntryWriter
	budgetName                types.Code
	distributeAmt, percentChg types.Decimal
	baseAmount, precision     types.Decimal
	budgetDate                types.Date
	periodLength, noPeriods   int
	totalAmount               bool
}

func (r *budgetAmountByPeriod) Definition() *Definition {
	return &Definition{
		Options: []Option{
			{Name: "budget_name", Type: "text", Required: true, Relation: &OptionRelation{Table: gtables.GLBudgetNameTableName, Columns: []string{"name", "description"}}},
			{Name: "budget_beginning_date", Type: "date", Required: true},
			{Name: "period_length", Type: "option", Default: "2", Options: budgetPeriodOptions},
			{Name: "no_of_periods", Type: "int", Default: "12"},
			{Name: "budget_amount", Type: "number"},
			{Name: "amount_type", Type: "option", Default: "0", Options: []string{"TOTAL_AMOUNT", "BEGINNING_AMOUNT"}},
			{Name: "period_percent_change", Type: "number"},
			{Name: "round_to", Type: "option", Default: "1", Options: budgetRoundingOptions},
		},
		Transaction:  true,
		ContextTable: gtables.GLBudgetNameTableName,
		OnInitReport: budgetOptionsInit,
		OnPreReport:  r.onPreReport,
		DataItems:    []*DataItem{budgetGLAccountItem(r.onPreDataItem, r.onAfterGetRecord)},
	}
}

func (r *budgetAmountByPeriod) onPreReport(run *Run) error {
	var err error
	r.budgetName, err = budgetName(run)
	if err != nil {
		return err
	}
	r.distributeAmt = run.OptionDecimal("budget_amount")
	r.budgetDate = run.OptionDate("budget_beginning_date")
	r.periodLength = run.OptionInt("period_length")
	r.noPeriods = run.OptionInt("no_of_periods")
	r.totalAmount = run.OptionInt("amount_type") == 0
	r.percentChg = run.OptionDecimal("period_percent_change")
	r.precision = budgetPrecision(run)
	return nil
}

// onPreDataItem: an amount, a beginning date and a number of periods; the growth per period and
// the first period's amount (a total spread so that the periods grow by the percentage)
func (r *budgetAmountByPeriod) onPreDataItem(glAcc *gtables.GLAccount, run *Run) error {
	if r.distributeAmt.IsZero() {
		return system.Error(run.Text("ENTER_AMOUNT"))
	}
	if r.budgetDate.IsZero() {
		return system.Error(run.Text("ENTER_BEGINNING_DATE"))
	}
	if r.noPeriods == 0 {
		return system.Error(run.Text("ENTER_NO_OF_PERIODS"))
	}
	if glAcc.GetFilter("no") == "" {
		return system.Error(run.Text("ENTER_GL_ACCOUNT"))
	}
	hundred := types.NewDecimalFromInt(100)
	one := types.NewDecimalFromInt(1)
	r.percentChg = r.percentChg.Div(hundred)
	if !r.percentChg.GreaterThan(one) {
		r.percentChg = one.Add(r.percentChg)
	}
	if r.totalAmount {
		if !r.percentChg.Equal(one) {
			// the geometric series: base × (1 + p + … + p^(n−1)) = the total
			r.baseAmount = r.distributeAmt.Div(r.percentChg.Sub(system.Power(r.percentChg, types.NewDecimalFromInt(int64(r.noPeriods)))).Div(one.Sub(r.percentChg)).Add(one))
		} else {
			r.baseAmount = r.distributeAmt.Div(types.NewDecimalFromInt(int64(r.noPeriods)))
		}
	} else {
		r.baseAmount = r.distributeAmt
	}
	r.init(run)
	return nil
}

// onAfterGetRecord: an entry per period on the period's first date, each the amount before it
// grown by the percentage
func (r *budgetAmountByPeriod) onAfterGetRecord(glAcc *gtables.GLAccount, run *Run) error {
	baseAmount := r.baseAmount
	budgetDate := r.budgetDate
	for currentPeriod := 1; currentPeriod <= r.noPeriods; currentPeriod++ {
		if currentPeriod > 1 {
			baseAmount = baseAmount.Mul(r.percentChg)
			var err error
			budgetDate, err = nextBudgetDate(run, budgetDate, r.periodLength)
			if err != nil {
				return err
			}
		}
		err := r.insert(run, r.budgetName, glAcc.No, budgetDate, singleFilterValue(glAcc.Dimension_1_filter), singleFilterValue(glAcc.Dimension_2_filter),
			system.Round(baseAmount, r.precision), "")
		if err != nil {
			return err
		}
	}
	run.Message(run.Text("ENTRIES_CREATED", r.inserted))
	run.Refresh()
	return nil
}

// Report 82 Budget from History (classic local 10031)
type budgetFromHistory struct {
	glBudgetEntryWriter
	budgetName              types.Code
	percentChg, precision   types.Decimal
	historyDate, budgetDate types.Date
	periodLength, noPeriods int
}

func (r *budgetFromHistory) Definition() *Definition {
	return &Definition{
		Options: []Option{
			{Name: "budget_name", Type: "text", Required: true, Relation: &OptionRelation{Table: gtables.GLBudgetNameTableName, Columns: []string{"name", "description"}}},
			{Name: "history_beginning_date", Type: "date", Required: true},
			{Name: "budget_beginning_date", Type: "date", Required: true},
			{Name: "period_length", Type: "option", Default: "2", Options: budgetPeriodOptions},
			{Name: "no_of_periods", Type: "int", Default: "12"},
			{Name: "percent_change", Type: "number"},
			{Name: "round_to", Type: "option", Default: "1", Options: budgetRoundingOptions},
		},
		Transaction:  true,
		ContextTable: gtables.GLBudgetNameTableName,
		OnInitReport: budgetOptionsInit,
		OnPreReport:  r.onPreReport,
		DataItems:    []*DataItem{budgetGLAccountItem(r.onPreDataItem, r.onAfterGetRecord)},
	}
}

func (r *budgetFromHistory) onPreReport(run *Run) error {
	var err error
	r.budgetName, err = budgetName(run)
	if err != nil {
		return err
	}
	r.historyDate = run.OptionDate("history_beginning_date")
	r.budgetDate = run.OptionDate("budget_beginning_date")
	r.periodLength = run.OptionInt("period_length")
	r.noPeriods = run.OptionInt("no_of_periods")
	r.percentChg = run.OptionDecimal("percent_change")
	r.precision = budgetPrecision(run)
	return nil
}

func (r *budgetFromHistory) onPreDataItem(glAcc *gtables.GLAccount, run *Run) error {
	if r.historyDate.IsZero() {
		return system.Error(run.Text("ENTER_HISTORY_DATE"))
	}
	if r.budgetDate.IsZero() {
		return system.Error(run.Text("ENTER_BEGINNING_DATE"))
	}
	if r.noPeriods == 0 {
		return system.Error(run.Text("ENTER_NO_OF_PERIODS"))
	}
	one := types.NewDecimalFromInt(1)
	r.percentChg = r.percentChg.Div(types.NewDecimalFromInt(100))
	if r.percentChg.LessThan(one) {
		r.percentChg = one.Add(r.percentChg)
	}
	r.init(run)
	return nil
}

// onAfterGetRecord: per period the account's net change in the history period, changed by the
// percentage, as an entry on the budget period's first date
func (r *budgetFromHistory) onAfterGetRecord(glAcc *gtables.GLAccount, run *Run) error {
	runHistoryDate := r.historyDate
	runBudgetDate := r.budgetDate
	for currentPeriod := 1; currentPeriod <= r.noPeriods; currentPeriod++ {
		nextHistoryDate, err := nextBudgetDate(run, runHistoryDate, r.periodLength)
		if err != nil {
			return err
		}
		err = glAcc.SetFlowFilter("date_filter", runHistoryDate.String()+".."+nextHistoryDate.AddDays(-1).String())
		if err != nil {
			return err
		}
		glAcc.CalcFields("net_change")
		runHistoryDate = nextHistoryDate
		err = r.insert(run, r.budgetName, glAcc.No, runBudgetDate, singleFilterValue(glAcc.Dimension_1_filter), singleFilterValue(glAcc.Dimension_2_filter),
			system.Round(glAcc.Net_change.Mul(r.percentChg), r.precision), "")
		if err != nil {
			return err
		}
		runBudgetDate, err = nextBudgetDate(run, runBudgetDate, r.periodLength)
		if err != nil {
			return err
		}
	}
	run.Message(run.Text("ENTRIES_CREATED", r.inserted))
	run.Refresh()
	return nil
}

// Report 83 Import/Export G/L Budget (classic dataport 10000): Export writes the budget's entries
// in the request filters as a CSV file (G/L account, date ISO, dimension 1, dimension 2, amount
// with a point, description; a heading line); Import adds the lines of such a file to the budget
// as new entries — an account or dimension value that does not exist refuses the file, naming
// the line.
type importExportGLBudget struct {
	glBudgetEntryWriter
	budgetName types.Code
	importing  bool
	file       []byte
	out        bytes.Buffer
	csvWriter  *csv.Writer
	exported   int
}

var glBudgetFileColumns = []string{"gl_account_no", "date", "dimension_1_code", "dimension_2_code", "amount", "description"}

func (r *importExportGLBudget) Definition() *Definition {
	return &Definition{
		Options: []Option{
			{Name: "budget_name", Type: "text", Required: true, Relation: &OptionRelation{Table: gtables.GLBudgetNameTableName, Columns: []string{"name", "description"}}},
			{Name: "direction", Type: "option", Default: "1", Options: []string{"IMPORT", "EXPORT"}},
			{Name: "file", Type: "file"},
		},
		Transaction:  true,
		ContextTable: gtables.GLBudgetNameTableName,
		OnInitReport: budgetOptionsInit,
		OnPreReport:  r.onPreReport,
		OnPostReport: r.onPostReport,
		DataItems: []*DataItem{{
			Name:                "GLBudgetEntry",
			Table:               gtables.GLBudgetEntryTableName,
			Key:                 []string{"entry_no"},
			RequestFilterFields: []string{"gl_account_no", "date", "dimension_1_code", "dimension_2_code"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
		}},
	}
}

func (r *importExportGLBudget) onPreReport(run *Run) error {
	var err error
	r.budgetName, err = budgetName(run)
	if err != nil {
		return err
	}
	r.importing = run.OptionInt("direction") == 0
	if r.importing {
		r.file = run.OptionFile("file")
		if len(r.file) == 0 {
			return system.Error(run.Text("ENTER_FILE"))
		}
		return r.importFile(run)
	}
	r.csvWriter = csv.NewWriter(&r.out)
	return r.csvWriter.Write(glBudgetFileColumns)
}

// onPreDataItem: an import reads no entries; an export the budget's
func (r *importExportGLBudget) onPreDataItem(glBudgetEntry *gtables.GLBudgetEntry, run *Run) error {
	if r.importing {
		return Break()
	}
	glBudgetEntry.SetRange("budget_name", r.budgetName)
	return nil
}

func (r *importExportGLBudget) onAfterGetRecord(glBudgetEntry *gtables.GLBudgetEntry, run *Run) error {
	r.exported++
	return r.csvWriter.Write([]string{glBudgetEntry.Gl_account_no.String(), glBudgetEntry.Date.String(),
		glBudgetEntry.Dimension_1_code.String(), glBudgetEntry.Dimension_2_code.String(),
		glBudgetEntry.Amount.String(), glBudgetEntry.Description.String()})
}

func (r *importExportGLBudget) onPostReport(run *Run) error {
	if r.importing {
		run.Message(run.Text("ENTRIES_IMPORTED", r.inserted))
		run.Refresh()
		return nil
	}
	r.csvWriter.Flush()
	err := r.csvWriter.Error()
	if err != nil {
		return err
	}
	run.WriteFile("budget-"+strings.ToLower(r.budgetName.String())+".csv", r.out.Bytes())
	run.Message(run.Text("ENTRIES_EXPORTED", r.exported))
	return nil
}

// importFile: every line after the heading becomes an entry of the budget; the accounts and
// dimension values are checked by validating them
func (r *importExportGLBudget) importFile(run *Run) error {
	reader := csv.NewReader(bytes.NewReader(r.file))
	reader.FieldsPerRecord = -1
	lines, err := reader.ReadAll()
	if err != nil {
		return system.Error(run.Text("FILE_NOT_READ", err.Error()))
	}
	r.init(run)
	for i, line := range lines {
		if i == 0 && len(line) > 0 && strings.EqualFold(strings.TrimSpace(line[0]), glBudgetFileColumns[0]) {
			continue // the heading
		}
		lineNo := strconv.Itoa(i + 1)
		for len(line) < len(glBudgetFileColumns) {
			line = append(line, "")
		}
		var date types.Date
		ok := system.Evaluate(&date, strings.TrimSpace(line[1]))
		if !ok || date.IsZero() {
			return system.Error(run.Text("FILE_LINE_INVALID", lineNo, line[1]))
		}
		amount, err := types.NewDecimalFromString(strings.TrimSpace(line[4]))
		if err != nil {
			return system.Error(run.Text("FILE_LINE_INVALID", lineNo, line[4]))
		}
		glBudgetEntry := gtables.NewGLBudgetEntry(run)
		err = glBudgetEntry.Validate("gl_account_no", types.Code(system.UpperCase(strings.TrimSpace(line[0]))))
		if err == nil {
			err = glBudgetEntry.Validate("dimension_1_code", types.Code(system.UpperCase(strings.TrimSpace(line[2]))))
		}
		if err == nil {
			err = glBudgetEntry.Validate("dimension_2_code", types.Code(system.UpperCase(strings.TrimSpace(line[3]))))
		}
		if err != nil {
			return system.Error(run.Text("FILE_LINE_ERROR", lineNo, err.Error()))
		}
		err = r.insert(run, r.budgetName, glBudgetEntry.Gl_account_no, date, glBudgetEntry.Dimension_1_code, glBudgetEntry.Dimension_2_code,
			amount, types.Text(strings.TrimSpace(line[5])))
		if err != nil {
			return err
		}
	}
	return nil
}
