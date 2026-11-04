// Package accsched is codeunit 8 AccSchedManagement: the account schedules' names and column
// layouts (OpenSchedule, OpenColumns) and the calculation of a schedule's cells — a line (G/L
// accounts, total accounts, a formula of row numbers) in a column (a period of G/L or budget
// entries, or a formula of column numbers), with percent bases, comparison periods and the
// columns' and lines' sign and show rules. The overview (page 490) and the printout (report 25)
// calculate with it.
package accsched

import (
	"strconv"
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// maxBasePercentLines: the most lines of a schedule with Totaling Type Set Base for Percent
const maxBasePercentLines = 50

// Filters are the account schedule line's FlowFilters a schedule is calculated for (the period
// and the dimensions; classic AccountScheduleLine's filters)
type Filters struct {
	DateFilter       string
	Dimension1Filter string
	Dimension2Filter string
	BudgetFilter     string
}

// LineFilters: the FlowFilters of an account schedule line
func LineFilters(accSchedLine *gtables.AccScheduleLine) Filters {
	return Filters{DateFilter: accSchedLine.Date_filter, Dimension1Filter: accSchedLine.Dimension_1_filter,
		Dimension2Filter: accSchedLine.Dimension_2_filter, BudgetFilter: accSchedLine.Budget_filter}
}

type cellKey struct{ row, column int }

type cellValue struct {
	value types.Decimal
	err   bool
}

// AccSchedManagement calculates the cells of account schedules (one per run: it keeps the cells
// calculated, as the classic temporary Acc. Sched. Cell Value)
type AccSchedManagement struct {
	src           ftables.Source
	filters       Filters
	useAddRptCurr bool // the amounts in the additional reporting currency

	startDate, endDate, fiscalStartDate types.Date
	divisionError                       bool
	basePercentLine                     [maxBasePercentLines]int
	callLevel                           int
	callingLine                         *gtables.AccScheduleLine
	callingColumn                       *gtables.AccSchedColumn

	accSchedCellValue map[cellKey]cellValue
	oldFilters        Filters
	oldLineName       types.Code
	oldColumnName     types.Code
}

// New is codeunit 8 for a database and company
func New(src ftables.Source) *AccSchedManagement {
	return &AccSchedManagement{src: src, accSchedCellValue: map[cellKey]cellValue{}}
}

// SetParameters (SetParameters): amounts in the additional reporting currency or in LCY; a
// change forgets the cells calculated
func (m *AccSchedManagement) SetParameters(useAddRptCurr bool) {
	if useAddRptCurr != m.useAddRptCurr {
		m.accSchedCellValue = map[cellKey]cellValue{}
	}
	m.useAddRptCurr = useAddRptCurr
}

// OpenSchedule: the schedule name to open — the one given when it exists, else the first one,
// DEFAULT created when there is none (classic CheckTemplateName)
func OpenSchedule(src ftables.Source, currentSchedName types.Code) (types.Code, error) {
	accSchedName := gtables.NewAccScheduleName(src)
	if !accSchedName.Get(currentSchedName) {
		if !accSchedName.FindFirst() {
			accSchedName.Init()
			accSchedName.Name = "DEFAULT"
			accSchedName.Description = types.Text(i18n.GetInstance().Message("ACC_SCHED_DEFAULT_SCHEDULE", fcodeunits.CurrentLanguage()))
			err := accSchedName.InsertOrError(true)
			if err != nil {
				return "", err
			}
		}
		currentSchedName = accSchedName.Name
	}
	return currentSchedName, nil
}

// OpenColumns: the column layout name to open — the one given when it exists, else the first
// one, DEFAULT created when there is none (classic CheckColumnTemplateName)
func OpenColumns(src ftables.Source, currentColumnName types.Code) (types.Code, error) {
	columnName := gtables.NewAccSchedColumnLayoutName(src)
	if !columnName.Get(currentColumnName) {
		if !columnName.FindFirst() {
			columnName.Init()
			columnName.Name = "DEFAULT"
			columnName.Description = types.Text(i18n.GetInstance().Message("ACC_SCHED_DEFAULT_COLUMNS", fcodeunits.CurrentLanguage()))
			err := columnName.InsertOrError(true)
			if err != nil {
				return "", err
			}
		}
		currentColumnName = columnName.Name
	}
	return currentColumnName, nil
}

// FindFiscalYear: the start of the fiscal year of a date (the last accounting period with New
// Fiscal Year on or before it; without accounting periods the start of its calendar year)
func FindFiscalYear(src ftables.Source, balanceDate types.Date) types.Date {
	accountingPeriod := gtables.NewAccountingPeriod(src)
	accountingPeriod.SetRange("new_fiscal_year", true)
	accountingPeriod.SetRange("starting_date", types.Date{}, balanceDate)
	if accountingPeriod.FindLast() {
		return accountingPeriod.Starting_date
	}
	return types.NewDate(balanceDate.Year(), 1, 1)
}

// GetDivisionError: the last cell calculated divided by zero
func (m *AccSchedManagement) GetDivisionError() bool {
	return m.divisionError
}

// CalcCell calculates a line's value in a column for the filters (the period of the date
// filter, dimensions, budget): Sign Control reverses it, the column's Show hides a positive or
// negative value and Show Opposite Sign turns it round
func (m *AccSchedManagement) CalcCell(filters Filters, accSchedLine *gtables.AccScheduleLine, accSchedColumn *gtables.AccSchedColumn) (types.Decimal, error) {
	startDate, endDate, ok := dateRange(filters.DateFilter)
	if !ok {
		return types.Decimal{}, apperrors.AccSchedDateFilterRequired()
	}
	m.filters = filters
	m.startDate = startDate
	if m.endDate != endDate || m.fiscalStartDate.IsZero() {
		m.endDate = endDate
		m.fiscalStartDate = FindFiscalYear(m.src, m.endDate)
	}
	m.divisionError = false
	m.callLevel = 0
	m.callingLine = accSchedLine
	m.callingColumn = accSchedColumn

	if m.oldFilters != filters || m.oldLineName != accSchedLine.Schedule_name || m.oldColumnName != accSchedColumn.Column_layout_name {
		m.accSchedCellValue = map[cellKey]cellValue{}
		m.basePercentLine = [maxBasePercentLines]int{}
		m.oldFilters = filters
		m.oldLineName = accSchedLine.Schedule_name
		m.oldColumnName = accSchedColumn.Column_layout_name
	}

	result, err := m.calcCellValue(accSchedLine, accSchedColumn)
	if err != nil {
		return types.Decimal{}, err
	}
	if accSchedLine.Sign_control == gtables.AccScheduleLine_Sign_control.Reverse {
		result = result.Neg()
	}
	switch accSchedColumn.Show {
	case gtables.AccSchedColumn_Show.WhenPositive:
		if result.IsNegative() {
			result = types.Decimal{}
		}
	case gtables.AccSchedColumn_Show.WhenNegative:
		if result.IsPositive() {
			result = types.Decimal{}
		}
	}
	if accSchedColumn.Show_opposite_sign {
		result = result.Neg()
	}
	return result, nil
}

func (m *AccSchedManagement) calcCellValue(accSchedLine *gtables.AccScheduleLine, accSchedColumn *gtables.AccSchedColumn) (types.Decimal, error) {
	result := types.Decimal{}
	if accSchedLine.Totaling == "" {
		return result, nil
	}
	key := cellKey{accSchedLine.Line_no, accSchedColumn.Line_no}
	cell, found := m.accSchedCellValue[key]
	if found {
		m.divisionError = m.divisionError || cell.err
		return cell.value, nil
	}
	var err error
	lineType := gtables.AccScheduleLine_Totaling_type
	if accSchedColumn.Column_type == gtables.AccSchedColumn_Column_type.Formula {
		result, err = m.evaluateExpression(false, accSchedColumn.Formula.String(), accSchedLine, accSchedColumn)
	} else {
		if accSchedLine.Totaling_type == lineType.Formula || accSchedLine.Totaling_type == lineType.SetBaseForPercent {
			result, err = m.evaluateExpression(true, accSchedLine.Totaling.String(), accSchedLine, accSchedColumn)
		} else {
			result, err = m.calcGLAccounts(accSchedLine, accSchedColumn)
		}
	}
	if err != nil {
		return types.Decimal{}, err
	}
	m.accSchedCellValue[key] = cellValue{value: result, err: m.divisionError}
	return result, nil
}

// calcGLAccounts: the line's G/L accounts in the column — posting accounts by their filter (as a
// total account totaling it), total accounts one by one
func (m *AccSchedManagement) calcGLAccounts(accSchedLine *gtables.AccScheduleLine, accSchedColumn *gtables.AccSchedColumn) (types.Decimal, error) {
	result := types.Decimal{}
	glAcc := gtables.NewGLAccount(m.src)
	err := m.SetGLAccRowFilters(glAcc, accSchedLine)
	if err != nil {
		return result, err
	}
	zero, err := m.SetGLAccColumnFilters(glAcc, accSchedLine.Calculate, accSchedColumn)
	if err != nil || zero {
		return result, err
	}
	if accSchedLine.Totaling_type == gtables.AccScheduleLine_Totaling_type.PostingAccounts && len(accSchedLine.Totaling) <= 30 {
		glAcc.Account_type = gtables.GLAccount_Account_type.Total
		glAcc.Totaling = accSchedLine.Totaling
		return m.calcGLAcc(glAcc, accSchedLine.Calculate, accSchedColumn), nil
	}
	for found := glAcc.FindSetBuffered(); found; found = glAcc.Next() {
		result = result.Add(m.calcGLAcc(glAcc, accSchedLine.Calculate, accSchedColumn))
	}
	return result, nil
}

// calcGLAcc: the G/L account's amount in the column's ledger (G/L or budget entries) and amount
// type; a line calculating debits or credits only takes that side
func (m *AccSchedManagement) calcGLAcc(glAcc *gtables.GLAccount, rowType gtables.AccScheduleLineCalculate, accSchedColumn *gtables.AccSchedColumn) types.Decimal {
	colValue := types.Decimal{}
	if accSchedColumn.Column_type == gtables.AccSchedColumn_Column_type.Formula {
		return colValue
	}
	amountTypes := gtables.AccSchedColumn_Amount_type
	amountType := accSchedColumn.Amount_type
	switch rowType {
	case gtables.AccScheduleLine_Calculate.DebitsOnly:
		switch amountType {
		case amountTypes.NetAmount:
			amountType = amountTypes.DebitAmount
		case amountTypes.CreditAmount:
			return colValue
		}
	case gtables.AccScheduleLine_Calculate.CreditsOnly:
		switch amountType {
		case amountTypes.NetAmount:
			amountType = amountTypes.CreditAmount
		case amountTypes.DebitAmount:
			return colValue
		}
	}
	switch accSchedColumn.Ledger_entry_type {
	case gtables.AccSchedColumn_Ledger_entry_type.GLEntries:
		switch amountType {
		case amountTypes.NetAmount:
			if m.useAddRptCurr {
				glAcc.CalcFields("additional_currency_net_change")
				colValue = glAcc.Additional_currency_net_change
			} else {
				glAcc.CalcFields("net_change")
				colValue = glAcc.Net_change
			}
		case amountTypes.DebitAmount:
			if m.useAddRptCurr {
				glAcc.CalcFields("add_currency_debit_amount")
				colValue = glAcc.Add_currency_debit_amount
			} else {
				glAcc.CalcFields("debit_amount")
				colValue = glAcc.Debit_amount
			}
		case amountTypes.CreditAmount:
			// the classic leaves the additional-currency credit positive; negative as in LCY
			if m.useAddRptCurr {
				glAcc.CalcFields("add_currency_credit_amount")
				colValue = glAcc.Add_currency_credit_amount.Neg()
			} else {
				glAcc.CalcFields("credit_amount")
				colValue = glAcc.Credit_amount.Neg()
			}
		}
	case gtables.AccSchedColumn_Ledger_entry_type.GLBudgetEntries:
		glAcc.CalcFields("budgeted_amount")
		switch amountType {
		case amountTypes.NetAmount:
			colValue = glAcc.Budgeted_amount
		case amountTypes.DebitAmount:
			colValue = glAcc.Budgeted_amount
			if colValue.IsNegative() {
				colValue = types.Decimal{}
			}
		case amountTypes.CreditAmount:
			colValue = glAcc.Budgeted_amount
			if colValue.IsPositive() {
				colValue = types.Decimal{}
			}
		}
	}
	return colValue
}

// SetGLAccRowFilters: the line's dimension and budget filters and its accounts — posting
// accounts or total accounts in its Totaling
func (m *AccSchedManagement) SetGLAccRowFilters(glAcc *gtables.GLAccount, accSchedLine2 *gtables.AccScheduleLine) error {
	for field, filter := range map[string]string{"dimension_1_filter": m.filters.Dimension1Filter,
		"dimension_2_filter": m.filters.Dimension2Filter, "budget_filter": m.filters.BudgetFilter} {
		err := glAcc.SetFlowFilter(field, filter)
		if err != nil {
			return err
		}
	}
	switch accSchedLine2.Totaling_type {
	case gtables.AccScheduleLine_Totaling_type.PostingAccounts:
		glAcc.SetFilter("no", accSchedLine2.Totaling.String())
		glAcc.SetRange("account_type", gtables.GLAccount_Account_type.Posting)
	case gtables.AccScheduleLine_Totaling_type.TotalAccounts:
		glAcc.SetFilter("no", accSchedLine2.Totaling.String())
		glAcc.SetFilter("account_type", "<>"+strconv.Itoa(int(gtables.GLAccount_Account_type.Posting)))
	}
	return nil
}

// SetGLAccColumnFilters: the column's period as the account's Date Filter — the schedule's
// period moved by the Comparison Period Formula (a date formula, or a number of accounting
// periods with P), then by the column type and the line's Calculate — and the column's
// dimension 1 and dimension 2 filters. zero: the combination gives no amount (classic SETRANGE to 0D).
func (m *AccSchedManagement) SetGLAccColumnFilters(glAcc *gtables.GLAccount, rowType gtables.AccScheduleLineCalculate, colLayoutRec2 *gtables.AccSchedColumn) (zero bool, err error) {
	var fromDate, toDate, fiscalStartDate2 types.Date
	formula := colLayoutRec2.Comparison_period_formula.String()
	if strings.Contains(formula, "P") {
		fromDate, toDate, err = m.periodFormulaDates(formula)
		if err != nil {
			return false, err
		}
		fiscalStartDate2 = FindFiscalYear(m.src, toDate)
	} else {
		if formula != "0" && formula != "" {
			fromDate, err = system.CalcDate(formula, m.startDate)
			if err != nil {
				return false, m.showError(apperrors.ErrAccSchedPeriodFormulaInvalid, formula)
			}
			endOfMonth, _ := system.CalcDate("CM", m.endDate)
			if m.endDate == endOfMonth && strings.ContainsAny(formula, "MQY") {
				toDate, err = system.CalcDate(formula, m.endDate)
				if err == nil {
					toDate, err = system.CalcDate("CM", toDate)
				}
			} else {
				toDate, err = system.CalcDate(formula, m.endDate)
			}
			if err != nil {
				return false, m.showError(apperrors.ErrAccSchedPeriodFormulaInvalid, formula)
			}
			fiscalStartDate2 = FindFiscalYear(m.src, toDate)
		} else {
			fromDate = m.startDate
			toDate = m.endDate
			fiscalStartDate2 = m.fiscalStartDate
		}
	}
	fiscalEndDate, _ := system.CalcDate("+1Y-1D", fiscalStartDate2)
	calculate := gtables.AccScheduleLine_Calculate
	columnType := gtables.AccSchedColumn_Column_type
	dateFilter := ""
	switch colLayoutRec2.Column_type {
	case columnType.NetChange:
		switch rowType {
		case calculate.BeginningBalance:
			dateFilter = before(fromDate)
		case calculate.EndingBalance:
			dateFilter = upTo(toDate)
		default:
			dateFilter = between(fromDate, toDate)
		}
	case columnType.BalanceAtDate:
		if rowType == calculate.BeginningBalance {
			return true, nil
		}
		dateFilter = upTo(toDate)
	case columnType.BeginningBalance:
		if rowType == calculate.EndingBalance {
			return true, nil
		}
		dateFilter = before(fromDate)
	case columnType.YearToDate:
		switch rowType {
		case calculate.BeginningBalance:
			dateFilter = before(fiscalStartDate2)
		case calculate.EndingBalance:
			dateFilter = upTo(toDate)
		default:
			dateFilter = between(fiscalStartDate2, toDate)
		}
	case columnType.RestOfFiscalYear:
		switch rowType {
		case calculate.BeginningBalance:
			dateFilter = upTo(toDate)
		case calculate.EndingBalance:
			dateFilter = upTo(fiscalEndDate)
		default:
			dateFilter = between(toDate.AddDays(1), fiscalEndDate)
		}
	case columnType.EntireFiscalYear:
		switch rowType {
		case calculate.BeginningBalance:
			dateFilter = before(fiscalStartDate2)
		case calculate.EndingBalance:
			dateFilter = upTo(fiscalEndDate)
		default:
			dateFilter = between(fiscalStartDate2, fiscalEndDate)
		}
	}
	if dateFilter == "" {
		return true, nil
	}
	err = glAcc.SetFlowFilter("date_filter", dateFilter)
	if err != nil {
		return false, err
	}
	// the column's filters (classic: in a filter group of their own; here they take the place
	// of the line's filter of the same dimension)
	if colLayoutRec2.Dimension_1_filter != "" {
		err = glAcc.SetFlowFilter("dimension_1_filter", colLayoutRec2.Dimension_1_filter.String())
		if err != nil {
			return false, err
		}
	}
	if colLayoutRec2.Dimension_2_filter != "" {
		err = glAcc.SetFlowFilter("dimension_2_filter", colLayoutRec2.Dimension_2_filter.String())
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

// periodFormulaDates: a comparison period formula of accounting periods (-1P): the schedule's
// period, which must be whole accounting periods, moved by the number of periods
func (m *AccSchedManagement) periodFormulaDates(formula string) (fromDate, toDate types.Date, err error) {
	accPeriod := gtables.NewAccountingPeriod(m.src)
	if !accPeriod.Get(m.startDate) || !accPeriod.Get(m.endDate.AddDays(1)) {
		return fromDate, toDate, m.showError(apperrors.ErrAccSchedPeriodFormulaPeriods)
	}
	numPeriods, err := strconv.Atoi(formula[:strings.Index(formula, "P")])
	if err != nil {
		return fromDate, toDate, m.showError(apperrors.ErrAccSchedPeriodFormulaInvalid, formula)
	}
	accPeriod.Get(m.startDate)
	if !movePeriods(accPeriod, numPeriods) {
		return fromDate, toDate, m.showError(apperrors.ErrAccSchedPeriodFormulaInvalid, formula)
	}
	fromDate = accPeriod.Starting_date
	accPeriod.Get(m.endDate.AddDays(1))
	if !movePeriods(accPeriod, numPeriods) {
		return fromDate, toDate, m.showError(apperrors.ErrAccSchedPeriodFormulaInvalid, formula)
	}
	toDate = accPeriod.Starting_date.AddDays(-1)
	return fromDate, toDate, nil
}

// movePeriods: classic NEXT(steps) = steps (no steps: stays)
func movePeriods(accPeriod *gtables.AccountingPeriod, steps int) bool {
	if steps == 0 {
		return true
	}
	return accPeriod.Next(steps)
}

// evaluateExpression evaluates a formula of row numbers (a line's Totaling) or column numbers (a
// column's Formula): the operator of the lowest precedence outside parentheses splits it (+ - *
// / ^ %, from the right), parentheses are evaluated inside, a number is a number, and a row or
// column number — or a filter of them (10..50, 10|20) — is the sum of those cells. "A%" in a
// column formula is A of the line as a percent of A of the line Set Base for Percent above it.
func (m *AccSchedManagement) evaluateExpression(isAccSchedLineExpression bool, expression string, accSchedLine *gtables.AccScheduleLine, accSchedColumn *gtables.AccSchedColumn) (types.Decimal, error) {
	result := types.Decimal{}
	m.callLevel++
	defer func() { m.callLevel-- }()
	if m.callLevel > 25 {
		return result, m.showError(apperrors.ErrAccSchedCircularReference)
	}
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return result, nil
	}
	parentheses := 0
	isExpression := false
	operators := "+-*/^%"
	i := 0
	for operatorNo := 0; operatorNo < len(operators) && !isExpression; operatorNo++ {
		for i = len(expression); i > 0; i-- {
			switch expression[i-1] {
			case '(':
				parentheses++
			case ')':
				parentheses--
			}
			if parentheses == 0 && expression[i-1] == operators[operatorNo] {
				isExpression = true
				break
			}
		}
	}
	if isExpression {
		leftOperand := expression[:i-1]
		rightOperand := expression[i:]
		operator := expression[i-1]
		leftResult, err := m.evaluateExpression(isAccSchedLineExpression, leftOperand, accSchedLine, accSchedColumn)
		if err != nil {
			return result, err
		}
		var rightResult types.Decimal
		if rightOperand == "" && operator == '%' && !isAccSchedLineExpression &&
			accSchedLine.Totaling_type != gtables.AccScheduleLine_Totaling_type.SetBaseForPercent {
			baseLineNo, err := m.getSetBasePercentLine(accSchedLine)
			if err != nil {
				return result, err
			}
			accSchedLine2 := gtables.NewAccScheduleLine(m.src)
			err = accSchedLine2.GetOrError(map[string]interface{}{"schedule_name": accSchedLine.Schedule_name, "line_no": baseLineNo})
			if err != nil {
				return result, err
			}
			rightResult, err = m.evaluateExpression(isAccSchedLineExpression, leftOperand, accSchedLine2, accSchedColumn)
			if err != nil {
				return result, err
			}
		} else {
			rightResult, err = m.evaluateExpression(isAccSchedLineExpression, rightOperand, accSchedLine, accSchedColumn)
			if err != nil {
				return result, err
			}
		}
		switch operator {
		case '^':
			result = system.Power(leftResult, rightResult)
		case '%':
			if rightResult.IsZero() {
				m.divisionError = true
			} else {
				result = types.NewDecimalFromInt(100).Mul(leftResult).Div(rightResult)
			}
		case '*':
			result = leftResult.Mul(rightResult)
		case '/':
			if rightResult.IsZero() {
				m.divisionError = true
			} else {
				result = leftResult.Div(rightResult)
			}
		case '+':
			result = leftResult.Add(rightResult)
		case '-':
			result = leftResult.Sub(rightResult)
		}
		return result, nil
	}
	if strings.HasPrefix(expression, "(") && strings.HasSuffix(expression, ")") {
		return m.evaluateExpression(isAccSchedLineExpression, expression[1:len(expression)-1], accSchedLine, accSchedColumn)
	}
	isFilter := strings.ContainsAny(expression, "|<>=") || strings.Contains(expression, "..")
	if len(expression) > 10 && !isFilter {
		number, err := types.NewDecimalFromString(expression)
		if err != nil {
			return result, m.showError(apperrors.ErrAccSchedIllegalRowNo)
		}
		return number, nil
	}
	found := false
	if isAccSchedLineExpression {
		accSchedLine2 := gtables.NewAccScheduleLine(m.src)
		accSchedLine2.SetRange("schedule_name", accSchedLine.Schedule_name)
		accSchedLine2.SetFilter("row_no", expression)
		for ok := accSchedLine2.FindSetBuffered(); ok; ok = accSchedLine2.Next() {
			found = true
			if accSchedLine2.Line_no != accSchedLine.Line_no {
				line := gtables.NewAccScheduleLine(m.src)
				line.Copy(accSchedLine2)
				value, err := m.calcCellValue(line, accSchedColumn)
				if err != nil {
					return result, err
				}
				result = result.Add(value)
			}
		}
	} else {
		accSchedColumn2 := gtables.NewAccSchedColumn(m.src)
		accSchedColumn2.SetRange("column_layout_name", accSchedColumn.Column_layout_name)
		accSchedColumn2.SetFilter("column_no", expression)
		for ok := accSchedColumn2.FindSetBuffered(); ok; ok = accSchedColumn2.Next() {
			found = true
			if accSchedColumn2.Line_no != accSchedColumn.Line_no {
				column := gtables.NewAccSchedColumn(m.src)
				column.Copy(accSchedColumn2)
				value, err := m.calcCellValue(accSchedLine, column)
				if err != nil {
					return result, err
				}
				result = result.Add(value)
			}
		}
	}
	if !found {
		number, err := types.NewDecimalFromString(expression)
		if isFilter || err != nil {
			if isAccSchedLineExpression {
				return result, m.showError(apperrors.ErrAccSchedIllegalRowNo)
			}
			return result, m.showError(apperrors.ErrAccSchedIllegalColumnNo)
		}
		result = number
	}
	return result, nil
}

// initBasePercents: the lines of the schedule with Totaling Type Set Base for Percent
func (m *AccSchedManagement) initBasePercents(accSchedLine *gtables.AccScheduleLine) error {
	m.basePercentLine = [maxBasePercentLines]int{}
	baseIdx := 0
	line := gtables.NewAccScheduleLine(m.src)
	line.SetRange("schedule_name", accSchedLine.Schedule_name)
	line.SetRange("totaling_type", gtables.AccScheduleLine_Totaling_type.SetBaseForPercent)
	for found := line.FindSetBuffered(); found; found = line.Next() {
		if baseIdx >= maxBasePercentLines {
			return m.showError(apperrors.ErrAccSchedTooManyBaseLines, strconv.Itoa(maxBasePercentLines))
		}
		m.basePercentLine[baseIdx] = line.Line_no
		baseIdx++
	}
	if baseIdx == 0 {
		return m.showError(apperrors.ErrAccSchedPercentBaseRequired)
	}
	return nil
}

// getSetBasePercentLine: the line Set Base for Percent nearest above the line
func (m *AccSchedManagement) getSetBasePercentLine(accSchedLine *gtables.AccScheduleLine) (int, error) {
	if m.basePercentLine[0] == 0 {
		err := m.initBasePercents(accSchedLine)
		if err != nil {
			return 0, err
		}
	}
	for baseIdx := maxBasePercentLines - 1; baseIdx >= 0; baseIdx-- {
		if m.basePercentLine[baseIdx] != 0 && m.basePercentLine[baseIdx] < accSchedLine.Line_no {
			return m.basePercentLine[baseIdx], nil
		}
	}
	return 0, m.showError(apperrors.ErrAccSchedPercentBaseRequired)
}

// showError: a calculation error naming the cell calculated (its line and column)
func (m *AccSchedManagement) showError(code apperrors.ErrorCode, params ...string) error {
	line, column := m.callingLine, m.callingColumn
	params = append(params, line.Row_no.String(), strconv.Itoa(line.Line_no), line.Totaling.String(),
		column.Column_no.String(), strconv.Itoa(column.Line_no), column.Formula.String())
	return apperrors.AccSchedCalc(code, params...)
}

// RoundCell: a cell's value in its column's Rounding Factor (classic FormatCellAsText): whole
// numbers, or thousands / millions with one decimal
func RoundCell(accSchedColumn *gtables.AccSchedColumn, value types.Decimal) types.Decimal {
	switch accSchedColumn.Rounding_factor {
	case gtables.AccSchedColumn_Rounding_factor.N1:
		return system.Round(value, types.NewDecimalFromInt(1))
	case gtables.AccSchedColumn_Rounding_factor.N1000:
		return system.Round(value.Div(types.NewDecimalFromInt(1000)), types.MustDecimal("0.1"))
	case gtables.AccSchedColumn_Rounding_factor.N1000000:
		return system.Round(value.Div(types.NewDecimalFromInt(1000000)), types.MustDecimal("0.1"))
	}
	return value
}

// IsPercentColumn: a formula column ending in a percent
func IsPercentColumn(accSchedColumn *gtables.AccSchedColumn) bool {
	return accSchedColumn.Column_type == gtables.AccSchedColumn_Column_type.Formula && strings.Contains(accSchedColumn.Formula.String(), "%")
}

// dateRange: the first and last date of a date filter a..b (or one date)
func dateRange(filter string) (from, to types.Date, ok bool) {
	filter = strings.TrimSpace(filter)
	fromText, toText, isRange := strings.Cut(filter, "..")
	if !isRange {
		toText = fromText
	}
	from, err1 := types.NewDateFromString(strings.TrimSpace(fromText))
	to, err2 := types.NewDateFromString(strings.TrimSpace(toText))
	if err1 != nil || err2 != nil || from.IsZero() || to.IsZero() {
		return from, to, false
	}
	return from, to, true
}

func between(from, to types.Date) string { return from.String() + ".." + to.String() }
func upTo(to types.Date) string          { return ".." + to.String() }
func before(from types.Date) string      { return ".." + from.AddDays(-1).String() }
