package reports_test

import (
	"testing"

	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The budget views take amounts: a value typed into a cell of 417 G/L Budget by Dimension 1 is the
// account's Budgeted Amount with the column's Dimension 1 value (a budget entry with the
// difference); on 420 G/L Acc. Budget by Dim. 1/2 the card's account and Budget Filter, the row's
// Dimension 1 value and the column's Dimension 2 value (blank: ”)
func TestBudgetViewsTakeAmounts(t *testing.T) {
	src := listCompany(t)
	budgetName := gtables.NewGLBudgetName(src)
	if !budgetName.FindFirst() {
		t.Fatal("no demo budget")
	}
	dimension1 := gtables.NewDimension1(src)
	if !dimension1.FindFirst() {
		t.Fatal("no Dimension 1 value")
	}
	budgeted := func(dim1, dim2, dateFilter string) types.Decimal {
		glAcc := gtables.NewGLAccount(src)
		mustOK(t, glAcc.GetOrError(types.Code("6110")))
		mustOK(t, glAcc.SetFlowFilter("budget_filter", string(budgetName.Name)))
		mustOK(t, glAcc.SetFlowFilter("dimension_1_filter", dim1))
		if dim2 != "" {
			mustOK(t, glAcc.SetFlowFilter("dimension_2_filter", dim2))
		}
		mustOK(t, glAcc.SetFlowFilter("date_filter", dateFilter))
		glAcc.CalcFields("budgeted_amount")
		return glAcc.Budgeted_amount
	}

	// 417: the row's account in the matrix's filters, the column's Dimension 1 value
	glAcc := gtables.NewGLAccount(src)
	mustOK(t, glAcc.GetOrError(types.Code("6110")))
	mustOK(t, glAcc.SetFlowFilter("budget_filter", string(budgetName.Name)))
	mustOK(t, glAcc.SetFlowFilter("date_filter", "2030-10-01..2030-10-31"))
	column := map[string]string{"dimension_1_code": string(dimension1.Code)}
	mustOK(t, pages.RunMatrixValidate(417, nil, glAcc, column, types.NewDecimalFromInt(1000)))
	if got := budgeted(string(dimension1.Code), "", "2030-10-01..2030-10-31"); !got.Equal(types.NewDecimalFromInt(1000)) {
		t.Errorf("417: budgeted %s, want 1000", got)
	}

	// 420: the card's account, period and Budget Filter
	card := gtables.NewGLAccount(src)
	mustOK(t, card.GetOrError(types.Code("6110")))
	mustOK(t, card.SetFlowFilter("budget_filter", string(budgetName.Name)))
	mustOK(t, card.SetFlowFilter("date_filter", "2030-11-01..2030-11-30"))
	mustOK(t, pages.RunMatrixValidate(420, card, dimension1, map[string]string{"dimension_2_code": "''"}, types.NewDecimalFromInt(500)))
	if got := budgeted(string(dimension1.Code), "''", "2030-11-01..2030-11-30"); !got.Equal(types.NewDecimalFromInt(500)) {
		t.Errorf("420: budgeted %s, want 500", got)
	}
}
