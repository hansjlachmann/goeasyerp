package pages

import (
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The G/L balance and budget views (classic forms 408–422). 408 G/L Balance by Periods and 415
// G/L Account Balance are a period matrix and a period grid (no code); the others are
// calculated matrices whose columns come from here:
//   - 409/410/411 G/L Balance by Dimension 1 / Dimension 2 / Business Units and 417/418/419 the
//     budget by the dimensions and business units: the G/L accounts against the dimension values
//     or business units, a cell the account's Net Change (Balance at Date) or Budgeted Amount
//     with the column's value in the account's FlowFilter — on the budget views a value typed
//     into a cell enters the budget;
//   - 412/420 G/L Acc. Balance/Budget by Dim. 1/2: one account, the Dimension 1 values against
//     the Dimension 2 values (420: in the card's Budget Filter, and amounts can be entered);
//   - 414 G/L Balance and 422 G/L Balance/Budget: the accounts in one period with their debit,
//     credit and net change (and the budget).
const (
	glBalanceByDim1PageID        = 409
	glBalanceByDim2PageID        = 410
	glBalanceByBusUnitsPageID    = 411
	glAccBalanceByDim12PageID    = 412
	glBalancePageID              = 414
	glBudgetByDim1PageID         = 417
	glBudgetByDim2PageID         = 418
	glBudgetByBusUnitsPageID     = 419
	glAccBudgetByDim12PageID     = 420
	glBalanceBudgetPageID        = 422
	glAccountsFlowFieldNetChange = "net_change"
	glAccountsFlowFieldBudget    = "budgeted_amount"
)

// glBudgetColumnFlowFilters: the budget entry field of a budget view's column → the account's
// FlowFilter it sets
var glBudgetColumnFlowFilters = map[string]string{
	"dimension_1_code":   "dimension_1_filter",
	"dimension_2_code":   "dimension_2_filter",
	"business_unit_code": "business_unit_filter",
}

// glBalanceColumn is one column of a matrix against codes: the code, the FlowFilter it goes into
// and the G/L entry (or budget entry) field it filters on drilldown
type glBalanceColumn struct {
	code        types.Code
	flowFilter  string
	entryFilter string
}

func init() {
	byCodes := func(pageID int, value string, columns func(src ftables.Source) []glBalanceColumn) {
		pages.OnMatrixCalculate(pageID, func(card *gtables.GLAccount, glAcc *gtables.GLAccount) ([]pages.MatrixColumn, map[string][]string, error) {
			return glAccountsByCodes(glAcc, value, columns(glAcc))
		})
	}
	byCodes(glBalanceByDim1PageID, glAccountsFlowFieldNetChange, dimension1Columns("dimension_1_code"))
	byCodes(glBalanceByDim2PageID, glAccountsFlowFieldNetChange, dimension2Columns("dimension_2_code"))
	byCodes(glBalanceByBusUnitsPageID, glAccountsFlowFieldNetChange, businessUnitColumns("business_unit_code"))
	byCodes(glBudgetByDim1PageID, glAccountsFlowFieldBudget, dimension1Columns("dimension_1_code"))
	byCodes(glBudgetByDim2PageID, glAccountsFlowFieldBudget, dimension2Columns("dimension_2_code"))
	byCodes(glBudgetByBusUnitsPageID, glAccountsFlowFieldBudget, businessUnitColumns("business_unit_code"))

	byDim12 := func(pageID int, value string) {
		pages.OnMatrixCalculate(pageID, func(card *gtables.GLAccount, dimension1 *gtables.Dimension1) ([]pages.MatrixColumn, map[string][]string, error) {
			return glAccountByDim12(card, dimension1, value)
		})
	}
	byDim12(glAccBalanceByDim12PageID, glAccountsFlowFieldNetChange)
	byDim12(glAccBudgetByDim12PageID, glAccountsFlowFieldBudget)

	// the budget views take amounts: a value typed into a cell is the account's Budgeted Amount
	// with the column's dimension value or business unit (classic: entered into the FlowField of
	// the matrix, the column's code in the account's filter)
	for _, pageID := range []int{glBudgetByDim1PageID, glBudgetByDim2PageID, glBudgetByBusUnitsPageID} {
		pages.OnMatrixColumnValidate(pageID, func(glAcc *gtables.GLAccount, column map[string]string, value types.Decimal) error {
			for entryFilter, flowFilter := range glBudgetColumnFlowFilters {
				code, ok := column[entryFilter]
				if ok {
					err := glAcc.SetFlowFilter(flowFilter, code)
					if err != nil {
						return err
					}
				}
			}
			return tables.SetGLAccBudgetedAmount(glAcc, value)
		})
	}
	pages.OnMatrixCardColumnValidate(glAccBudgetByDim12PageID, func(card *gtables.GLAccount, dimension1 *gtables.Dimension1, column map[string]string, value types.Decimal) error {
		glAcc, err := glAccountOfCard(card)
		if err != nil {
			return err
		}
		err = glAcc.SetFlowFilter("dimension_1_filter", exactCode(dimension1.Code))
		if err != nil {
			return err
		}
		err = glAcc.SetFlowFilter("dimension_2_filter", column["dimension_2_code"])
		if err != nil {
			return err
		}
		return tables.SetGLAccBudgetedAmount(glAcc, value)
	})

	pages.OnMatrixCalculate(glBalancePageID, func(card *gtables.GLAccount, glAcc *gtables.GLAccount) ([]pages.MatrixColumn, map[string][]string, error) {
		return glBalance(glAcc, false)
	})
	pages.OnMatrixCalculate(glBalanceBudgetPageID, func(card *gtables.GLAccount, glAcc *gtables.GLAccount) ([]pages.MatrixColumn, map[string][]string, error) {
		return glBalance(glAcc, true)
	})
}

// dimension1Columns: a column per Dimension 1 value (the accounts' Dimension 1 Filter)
func dimension1Columns(entryFilter string) func(src ftables.Source) []glBalanceColumn {
	return func(src ftables.Source) []glBalanceColumn {
		var columns []glBalanceColumn
		dimension1 := gtables.NewDimension1(src)
		for found := dimension1.FindSet(); found; found = dimension1.Next() {
			columns = append(columns, glBalanceColumn{dimension1.Code, "dimension_1_filter", entryFilter})
		}
		return columns
	}
}

// dimension2Columns: a column per Dimension 2 value (the accounts' Dimension 2 Filter)
func dimension2Columns(entryFilter string) func(src ftables.Source) []glBalanceColumn {
	return func(src ftables.Source) []glBalanceColumn {
		var columns []glBalanceColumn
		dimension2 := gtables.NewDimension2(src)
		for found := dimension2.FindSet(); found; found = dimension2.Next() {
			columns = append(columns, glBalanceColumn{dimension2.Code, "dimension_2_filter", entryFilter})
		}
		return columns
	}
}

// businessUnitColumns: a column per business unit (the accounts' Business Unit Filter)
func businessUnitColumns(entryFilter string) func(src ftables.Source) []glBalanceColumn {
	return func(src ftables.Source) []glBalanceColumn {
		var columns []glBalanceColumn
		businessUnit := gtables.NewBusinessUnit(src)
		for found := businessUnit.FindSet(); found; found = businessUnit.Next() {
			columns = append(columns, glBalanceColumn{businessUnit.Code, "business_unit_filter", entryFilter})
		}
		return columns
	}
}

// exactCode is a code as a filter on exactly that value (a blank code: ”)
func exactCode(code types.Code) string {
	if code == "" {
		return "''"
	}
	return string(code)
}

// glAccountsByCodes: the accounts in the matrix's filters (glAcc, with the period and the user's
// FlowFilters) against the columns — per column one calculation of the FlowField over all rows
func glAccountsByCodes(glAcc *gtables.GLAccount, value string, columns []glBalanceColumn) ([]pages.MatrixColumn, map[string][]string, error) {
	var keys []string
	var records []map[string]interface{}
	for found := glAcc.FindSet(); found; found = glAcc.Next() {
		keys = append(keys, ftables.KeyString(glAcc))
		records = append(records, glAcc.ToMap())
	}
	matrixColumns := make([]pages.MatrixColumn, len(columns))
	values := map[string][]string{}
	for _, key := range keys {
		values[key] = make([]string, len(columns))
	}
	for j, column := range columns {
		matrixColumns[j] = pages.MatrixColumn{
			Name:             column.code.String(),
			DecimalPlaces:    "2:2",
			DrilldownFilters: map[string]string{column.entryFilter: exactCode(column.code)},
		}
		saved := flowFilterExpression(glAcc, column.flowFilter)
		err := glAcc.SetFlowFilter(column.flowFilter, exactCode(column.code))
		if err != nil {
			return nil, nil, err
		}
		cells := make([]map[string]interface{}, len(records))
		for i, data := range records {
			cells[i] = map[string]interface{}{}
			for k, v := range data {
				cells[i][k] = v
			}
		}
		glAcc.CalcFieldsForRecords(cells, value)
		for i, key := range keys {
			values[key][j] = fmt.Sprint(cells[i][value])
		}
		err = glAcc.SetFlowFilter(column.flowFilter, saved)
		if err != nil {
			return nil, nil, err
		}
	}
	return matrixColumns, values, nil
}

// glAccountByDim12: the card's account (its period in its Date Filter) by the Dimension 1 values
// of the rows against the Dimension 2 values (forms 413/421: Dimension 1 Filter the row,
// Dimension 2 Filter the column)
func glAccountByDim12(card *gtables.GLAccount, dimension1 *gtables.Dimension1, value string) ([]pages.MatrixColumn, map[string][]string, error) {
	var dimension2Codes []types.Code
	dimension2 := gtables.NewDimension2(dimension1)
	for found := dimension2.FindSet(); found; found = dimension2.Next() {
		dimension2Codes = append(dimension2Codes, dimension2.Code)
	}
	columns := make([]pages.MatrixColumn, len(dimension2Codes))
	for j, code := range dimension2Codes {
		columns[j] = pages.MatrixColumn{Name: code.String(), DecimalPlaces: "2:2",
			DrilldownFilters: map[string]string{"dimension_2_code": exactCode(code)}}
	}
	glAcc, err := glAccountOfCard(card)
	if err != nil {
		return nil, nil, err
	}
	values := map[string][]string{}
	for found := dimension1.FindSet(); found; found = dimension1.Next() {
		cells := make([]string, len(dimension2Codes))
		err = glAcc.SetFlowFilter("dimension_1_filter", exactCode(dimension1.Code))
		if err != nil {
			return nil, nil, err
		}
		for j, code := range dimension2Codes {
			err = glAcc.SetFlowFilter("dimension_2_filter", exactCode(code))
			if err != nil {
				return nil, nil, err
			}
			glAcc.CalcFields(value)
			amount := glAcc.Net_change
			if value == glAccountsFlowFieldBudget {
				amount = glAcc.Budgeted_amount
			}
			cells[j] = amount.String()
		}
		values[ftables.KeyString(dimension1)] = cells
	}
	return columns, values, nil
}

// glAccountOfCard: the card's account with the card's Date Filter (the matrix's period) and
// Budget Filter
func glAccountOfCard(card *gtables.GLAccount) (*gtables.GLAccount, error) {
	glAcc := gtables.NewGLAccount(card)
	err := glAcc.GetOrError(card.No)
	if err != nil {
		return nil, err
	}
	err = glAcc.SetFlowFilter("date_filter", card.Date_filter)
	if err != nil {
		return nil, err
	}
	err = glAcc.SetFlowFilter("budget_filter", card.Budget_filter)
	if err != nil {
		return nil, err
	}
	return glAcc, nil
}

// glBalance (forms 414/422): the accounts' Debit Amount and Credit Amount (the Net Change on the
// side of its sign, OnAfterGetRecord) and Net Change in the period; with the budget also the
// Budgeted Debit/Credit Amount, Budgeted Amount and the net change as a percentage of it
func glBalance(glAcc *gtables.GLAccount, budget bool) ([]pages.MatrixColumn, map[string][]string, error) {
	language := fcodeunits.CurrentLanguage()
	caption := func(key string) string { return i18n.GetInstance().Message(key, language) }
	columns := []pages.MatrixColumn{
		{Name: caption("GL_BALANCE_DEBIT_AMOUNT"), DecimalPlaces: "2:2"},
		{Name: caption("GL_BALANCE_CREDIT_AMOUNT"), DecimalPlaces: "2:2"},
		{Name: caption("GL_BALANCE_NET_CHANGE"), DecimalPlaces: "2:2"},
	}
	if budget {
		columns = append(columns,
			pages.MatrixColumn{Name: caption("GL_BALANCE_BUDGETED_DEBIT_AMOUNT"), DecimalPlaces: "2:2"},
			pages.MatrixColumn{Name: caption("GL_BALANCE_BUDGETED_CREDIT_AMOUNT"), DecimalPlaces: "2:2"},
			pages.MatrixColumn{Name: caption("GL_BALANCE_BUDGETED_AMOUNT"), DecimalPlaces: "2:2"},
			pages.MatrixColumn{Name: caption("GL_BALANCE_BUDGET_PCT"), DecimalPlaces: "1:1", Percent: true})
	}
	var keys []string
	var records []map[string]interface{}
	for found := glAcc.FindSet(); found; found = glAcc.Next() {
		keys = append(keys, ftables.KeyString(glAcc))
		records = append(records, glAcc.ToMap())
	}
	flowFields := []string{glAccountsFlowFieldNetChange}
	if budget {
		flowFields = append(flowFields, glAccountsFlowFieldBudget)
	}
	glAcc.CalcFieldsForRecords(records, flowFields...)
	values := map[string][]string{}
	for i, key := range keys {
		netChange := decimalOf(records[i][glAccountsFlowFieldNetChange])
		cells := []string{debitText(netChange), debitText(netChange.Neg()), netChange.String()}
		if budget {
			budgetedAmount := decimalOf(records[i][glAccountsFlowFieldBudget])
			budgetPct := ""
			if !budgetedAmount.IsZero() {
				budgetPct = netChange.Div(budgetedAmount).Mul(types.NewDecimalFromInt(100)).String()
			}
			cells = append(cells, debitText(budgetedAmount), debitText(budgetedAmount.Neg()), budgetedAmount.String(), budgetPct)
		}
		values[key] = cells
	}
	return columns, values, nil
}

// debitText: an amount shown on its side only (a debit or credit column is blank otherwise)
func debitText(amount types.Decimal) string {
	if !amount.IsPositive() {
		return ""
	}
	return amount.String()
}

// decimalOf: a FlowField value of a record map as a decimal
func decimalOf(value interface{}) types.Decimal {
	switch v := value.(type) {
	case types.Decimal:
		return v
	case nil:
		return types.ZeroDecimal()
	}
	d, err := types.NewDecimalFromString(fmt.Sprint(value))
	if err != nil {
		return types.ZeroDecimal()
	}
	return d
}

// flowFilterExpression: the expression a FlowFilter of the account holds (blank: none)
func flowFilterExpression(glAcc *gtables.GLAccount, field string) string {
	for _, f := range glAcc.FlowFilterExpressions() {
		if f.Field == field {
			return f.Expression
		}
	}
	return ""
}
