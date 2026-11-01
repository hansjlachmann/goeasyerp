package errors

// Recurring journal and allocation errors (tables 81/221, codeunits 11/13/231)
const (
	ErrJnlAllocGenPostingType      ErrorCode = "ERR_JNL_ALLOC_GEN_POSTING_TYPE"
	ErrJnlAllocRecurringOnly       ErrorCode = "ERR_JNL_ALLOC_RECURRING_ONLY"
	ErrJnlRecurringNoBalAccount    ErrorCode = "ERR_JNL_RECURRING_NO_BAL_ACCOUNT"
	ErrJnlRecurringMethodCurrency  ErrorCode = "ERR_JNL_RECURRING_METHOD_CURRENCY"
	ErrJnlRecurringBalanceGLOnly   ErrorCode = "ERR_JNL_RECURRING_BALANCE_GL_ONLY"
	ErrJnlReversingDocOutOfBalance ErrorCode = "ERR_JNL_REVERSING_DOC_OUT_OF_BALANCE"
	ErrJnlReversingOutOfBalance    ErrorCode = "ERR_JNL_REVERSING_OUT_OF_BALANCE"
	ErrJnlRecurringDateFiltered    ErrorCode = "ERR_JNL_RECURRING_DATE_FILTERED"
)

// JnlAllocGenPostingType: an allocation has a general posting type while its journal line has
// one (table: the journal line)
func JnlAllocGenPostingType(table, field string) *AppError {
	return &AppError{Code: ErrJnlAllocGenPostingType, Table: table, Field: field}
}

// JnlAllocRecurringOnly: allocations on a line of a journal that is not recurring
func JnlAllocRecurringOnly() *AppError {
	return &AppError{Code: ErrJnlAllocRecurringOnly}
}

// JnlRecurringNoBalAccount: a recurring line with a balancing account
func JnlRecurringNoBalAccount(table string) *AppError {
	return &AppError{Code: ErrJnlRecurringNoBalAccount, Table: table, Field: "bal_account_no"}
}

// JnlRecurringMethodCurrency: a currency on a line with the (Reversing) Balance method (method:
// the line's Recurring Method)
func JnlRecurringMethodCurrency(table string, method any) *AppError {
	return &AppError{Code: ErrJnlRecurringMethodCurrency, Table: table, Field: "currency_code", Field2: "recurring_method", Value: method}
}

// JnlRecurringBalanceGLOnly: the (Reversing) Balance method on an account that is not a G/L
// account
func JnlRecurringBalanceGLOnly() *AppError {
	return &AppError{Code: ErrJnlRecurringBalanceGLOnly}
}

// JnlReversingDocOutOfBalance: a document's reversing lines do not balance
func JnlReversingDocOutOfBalance(table, docNo, amount string) *AppError {
	return &AppError{Code: ErrJnlReversingDocOutOfBalance, Table: table, Field: "recurring_method", Field2: "document_no", Params: []string{docNo, amount}}
}

// JnlReversingOutOfBalance: a posting date's reversing lines do not balance
func JnlReversingOutOfBalance(table, date, amount string) *AppError {
	return &AppError{Code: ErrJnlReversingOutOfBalance, Table: table, Field: "recurring_method", Field2: "posting_date", Params: []string{date, amount}}
}

// JnlRecurringDateFiltered: a recurring journal posted with a filter on Posting Date
func JnlRecurringDateFiltered(table string) *AppError {
	return &AppError{Code: ErrJnlRecurringDateFiltered, Table: table, Field: "posting_date"}
}
