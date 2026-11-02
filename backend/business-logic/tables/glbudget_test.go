package tables

import (
	"errors"
	"testing"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

func newBudgetCompany(t *testing.T) ftables.Source {
	t.Helper()
	db := newTestDB(t)
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	at := gtables.GLAccount_Account_type
	addAccount(t, db, "6100", at.Posting, gtables.GLAccount_Income_balance.IncomeStatement)
	addAccount(t, db, "6200", at.Posting, gtables.GLAccount_Income_balance.IncomeStatement)
	total := gtables.NewGLAccount(src)
	total.No = "6999"
	total.Account_type = at.Total
	total.Totaling = "6100..6200"
	if !total.Insert(false) {
		t.Fatal("insert total account")
	}
	return src
}

// budgetAcc: an account with the matrix cell's filters (budget, period)
func budgetAcc(t *testing.T, src ftables.Source, no, budget, dateFilter string) *gtables.GLAccount {
	t.Helper()
	acc := gtables.NewGLAccount(src)
	if !acc.Get(types.Code(no)) {
		t.Fatalf("account %s", no)
	}
	mustNoErr(t, acc.SetFlowFilter("budget_filter", budget))
	mustNoErr(t, acc.SetFlowFilter("date_filter", dateFilter))
	return acc
}

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// A budgeted amount typed into the matrix enters a budget entry with the difference; totals sum
// their accounts; the filters must be one value
func TestSetGLAccBudgetedAmount(t *testing.T) {
	src := newBudgetCompany(t)

	// GLBudget-Open: no budget yet → DEFAULT, put into the Budget Filter
	acc := gtables.NewGLAccount(src)
	mustNoErr(t, GLBudgetOpen(acc))
	if acc.Budget_filter != "DEFAULT" || !gtables.NewGLBudgetName(src).Get(types.Code("DEFAULT")) {
		t.Fatalf("GLBudget-Open: budget filter %q", acc.Budget_filter)
	}

	jan := "2026-01-01..2026-01-31"
	mustNoErr(t, SetGLAccBudgetedAmount(budgetAcc(t, src, "6100", "DEFAULT", jan), types.NewDecimalFromInt(1000)))
	// again with another amount: an entry with the difference
	mustNoErr(t, SetGLAccBudgetedAmount(budgetAcc(t, src, "6100", "DEFAULT", jan), types.NewDecimalFromInt(1500)))
	// the same amount: no entry
	mustNoErr(t, SetGLAccBudgetedAmount(budgetAcc(t, src, "6100", "DEFAULT", jan), types.NewDecimalFromInt(1500)))
	mustNoErr(t, SetGLAccBudgetedAmount(budgetAcc(t, src, "6200", "DEFAULT", "2026-02-01..2026-02-28"), types.NewDecimalFromInt(200)))

	entry := gtables.NewGLBudgetEntry(src)
	if n := entry.Count(); n != 3 {
		t.Fatalf("%d budget entries, want 3", n)
	}
	entry.SetRange("gl_account_no", types.Code("6100"))
	for found := entry.FindSet(); found; found = entry.Next() {
		if entry.Date != types.NewDate(2026, 1, 1) || entry.Budget_name != "DEFAULT" {
			t.Errorf("entry %d: date %s budget %s", entry.Entry_no, entry.Date, entry.Budget_name)
		}
	}
	mustNoErr(t, entry.CalcSums("amount"))
	if !entry.Amount.Equal(types.NewDecimalFromInt(1500)) {
		t.Errorf("6100 budgeted %s, want 1500", entry.Amount)
	}

	// a total account sums its accounts (Budgeted Amount, and Budget at Date up to the date)
	total := budgetAcc(t, src, "6999", "DEFAULT", "2026-01-01..2026-12-31")
	total.CalcFields("budgeted_amount", "budget_at_date")
	if !total.Budgeted_amount.Equal(types.NewDecimalFromInt(1700)) {
		t.Errorf("total budgeted %s, want 1700", total.Budgeted_amount)
	}
	total = budgetAcc(t, src, "6999", "DEFAULT", "2026-01-01..2026-01-31")
	total.CalcFields("budget_at_date")
	if !total.Budget_at_date.Equal(types.NewDecimalFromInt(1500)) {
		t.Errorf("total budget at date %s, want 1500", total.Budget_at_date)
	}
	// ... but takes no amount itself
	if err := SetGLAccBudgetedAmount(total, types.NewDecimalFromInt(5)); err == nil {
		t.Error("a total account took a budgeted amount")
	}

	// the filters must be one value: no budget, a range of budgets, no period start
	var appErr *apperrors.AppError
	for _, c := range []struct{ budget, date, field string }{
		{"A|B", jan, "budget_filter"},
		{"DEFAULT", "..2026-01-31", "date_filter"},
	} {
		err := SetGLAccBudgetedAmount(budgetAcc(t, src, "6100", c.budget, c.date), types.NewDecimalFromInt(1))
		if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrBudgetNotOneValue || appErr.Field != c.field {
			t.Errorf("budget %q date %q: %v, want %s not one value", c.budget, c.date, err, c.field)
		}
	}
	if err := SetGLAccBudgetedAmount(budgetAcc(t, src, "6100", "", jan), types.NewDecimalFromInt(1)); err == nil {
		t.Error("an amount was budgeted without a budget")
	}

	// a posting account in a budget stays a posting account
	acc = gtables.NewGLAccount(src)
	acc.Get(types.Code("6100"))
	err := acc.Validate("account_type", gtables.GLAccount_Account_type.Heading)
	if !errors.As(err, &appErr) || appErr.Code != apperrors.ErrGLAccHasBudgetEntries {
		t.Errorf("account type of a budgeted account: %v", err)
	}

	// deleting the budget deletes its entries; deleting an account its entries
	budget2 := gtables.NewGLBudgetName(src)
	budget2.Name = "B2"
	mustNoErr(t, budget2.InsertOrError(true))
	mustNoErr(t, SetGLAccBudgetedAmount(budgetAcc(t, src, "6200", "B2", jan), types.NewDecimalFromInt(10)))
	acc.Get(types.Code("6200"))
	mustNoErr(t, acc.DeleteOrError(true))
	entry = gtables.NewGLBudgetEntry(src)
	entry.SetRange("gl_account_no", types.Code("6200"))
	if !entry.IsEmpty() {
		t.Error("the deleted account's budget entries remain")
	}
	budget := gtables.NewGLBudgetName(src)
	budget.Get(types.Code("DEFAULT"))
	mustNoErr(t, budget.DeleteOrError(true))
	if n := gtables.NewGLBudgetEntry(src).Count(); n != 0 {
		t.Errorf("%d budget entries after deleting the budgets' accounts and budget", n)
	}
}

// A capacity typed into Resource Capacity / Res. Group Capacity enters the difference as a
// capacity entry on the period's first date (a resource's with its group)
func TestSetResCapacity(t *testing.T) {
	db := newTestDB(t)
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	group := gtables.NewResourceGroup(src)
	group.No = "G1"
	mustNoErr(t, group.InsertOrError(false))
	res := gtables.NewResource(src)
	res.No, res.Resource_group_no = "R1", "G1"
	mustNoErr(t, res.InsertOrError(false))

	cell := func() *gtables.Resource {
		r := gtables.NewResource(src)
		r.Get(types.Code("R1"))
		mustNoErr(t, r.SetFlowFilter("date_filter", "2026-03-02"))
		return r
	}
	mustNoErr(t, SetResCapacity(cell(), types.NewDecimalFromInt(8)))
	mustNoErr(t, SetResCapacity(cell(), types.NewDecimalFromInt(6)))
	r := cell()
	r.CalcFields("capacity")
	if !r.Capacity.Equal(types.NewDecimalFromInt(6)) {
		t.Errorf("capacity %s, want 6", r.Capacity)
	}
	entry := gtables.NewResCapacityEntry(src)
	if n := entry.Count(); n != 2 {
		t.Errorf("%d capacity entries, want 2", n)
	}
	entry.FindFirst()
	if entry.Resource_group_no != "G1" || entry.Date != types.NewDate(2026, 3, 2) {
		t.Errorf("entry: group %s date %s", entry.Resource_group_no, entry.Date)
	}

	g := gtables.NewResourceGroup(src)
	g.Get(types.Code("G1"))
	mustNoErr(t, g.SetFlowFilter("date_filter", "2026-03-02"))
	mustNoErr(t, SetResGroupCapacity(g, types.NewDecimalFromInt(10)))
	g.CalcFields("capacity")
	if !g.Capacity.Equal(types.NewDecimalFromInt(10)) {
		t.Errorf("group capacity %s, want 10", g.Capacity)
	}
	// no single date: refused
	r = cell()
	mustNoErr(t, r.SetFlowFilter("date_filter", "..2026-03-02"))
	if SetResCapacity(r, types.NewDecimalFromInt(1)) == nil {
		t.Error("a capacity without a date was entered")
	}
}

// An Intrastat batch's Statistics Period is YYMM; a line's statistical value is the amount and
// the indirect cost (or the cost regulation % of the amount), its total weight the net weight of
// the quantity
func TestIntrastatJnl(t *testing.T) {
	db := newTestDB(t)
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	batch := gtables.NewIntrastatJnlBatch(src)
	for period, ok := range map[string]bool{"2610": true, "261": false, "2613": false, "26AB": false} {
		err := batch.Validate("statistics_period", types.Code(period))
		if (err == nil) != ok {
			t.Errorf("statistics period %s: %v", period, err)
		}
	}
	line := gtables.NewIntrastatJnlLine(src)
	mustNoErr(t, line.Validate("net_weight", types.MustDecimal("2.5")))
	mustNoErr(t, line.Validate("quantity", types.NewDecimalFromInt(4)))
	mustNoErr(t, line.Validate("amount", types.NewDecimalFromInt(1000)))
	mustNoErr(t, line.Validate("cost_regulation_percent", types.NewDecimalFromInt(5)))
	if !line.Total_weight.Equal(types.NewDecimalFromInt(10)) || !line.Indirect_cost.Equal(types.NewDecimalFromInt(50)) ||
		!line.Statistical_value.Equal(types.NewDecimalFromInt(1050)) {
		t.Errorf("weight %s, indirect cost %s, statistical value %s", line.Total_weight, line.Indirect_cost, line.Statistical_value)
	}
	mustNoErr(t, line.Validate("indirect_cost", types.NewDecimalFromInt(20)))
	if !line.Cost_regulation_percent.IsZero() || !line.Statistical_value.Equal(types.NewDecimalFromInt(1020)) {
		t.Errorf("own indirect cost: %% %s, value %s", line.Cost_regulation_percent, line.Statistical_value)
	}
}
