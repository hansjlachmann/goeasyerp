package migrations

import (
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration039GLBudgets{})
}

// Migration039GLBudgets goes with the G/L budgets: G/L Budget Name 95 and G/L Budget Entry 96.
// The READER role may read them, as all business tables; the tables come from the table sync.
type Migration039GLBudgets struct{}

func (m *Migration039GLBudgets) Version() int {
	return 39
}

func (m *Migration039GLBudgets) Name() string {
	return "gl-budgets"
}

func (m *Migration039GLBudgets) Description() string {
	return "READER reads the G/L budget names and entries"
}

func (m *Migration039GLBudgets) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"G_L_BUDGET_NAME", "G_L_BUDGET_ENTRY"})
}
