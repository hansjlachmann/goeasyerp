package errors

// G/L budget errors (tables 15/95/96, the budget matrix 113)
const (
	ErrGLAccHasLedgerEntries ErrorCode = "ERR_GL_ACC_HAS_LEDGER_ENTRIES"
	ErrGLAccHasBudgetEntries ErrorCode = "ERR_GL_ACC_HAS_BUDGET_ENTRIES"
	ErrBudgetNotOneValue     ErrorCode = "ERR_BUDGET_NOT_ONE_VALUE"
)

// GLAccHasLedgerEntries: Account Type changed away from Posting while the account has G/L entries
func GLAccHasLedgerEntries(table string) *AppError {
	return &AppError{Code: ErrGLAccHasLedgerEntries, Table: table, Field: "account_type"}
}

// GLAccHasBudgetEntries: Account Type changed away from Posting while the account has budget
// entries
func GLAccHasBudgetEntries(table string) *AppError {
	return &AppError{Code: ErrGLAccHasBudgetEntries, Table: table, Field: "account_type"}
}

// BudgetNotOneValue: a budgeted amount entered while a filter of the account (Budget Filter, Date
// Filter …) is not one value, so the budget entry cannot get it
func BudgetNotOneValue(table, field string) *AppError {
	return &AppError{Code: ErrBudgetNotOneValue, Table: table, Field: field}
}
