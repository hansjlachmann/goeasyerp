package errors

// Bank account reconciliation errors (tables 273/274, codeunits 370–376)
const (
	ErrBankRecStatementExists = ErrorCode("ERR_BANK_REC_STATEMENT_EXISTS")
	ErrBankRecUpdateCanceled  = ErrorCode("ERR_BANK_REC_UPDATE_CANCELED")
	ErrBankRecEndingBalance   = ErrorCode("ERR_BANK_REC_ENDING_BALANCE")
	ErrBankRecApplication     = ErrorCode("ERR_BANK_REC_APPLICATION")
	ErrBankRecTotalDifference = ErrorCode("ERR_BANK_REC_TOTAL_DIFFERENCE")
)

// BankRecStatementExists: a reconciliation with the number of a posted statement
func BankRecStatementExists(statementNo string) *AppError {
	return &AppError{Code: ErrBankRecStatementExists, Params: []string{statementNo}}
}

// BankRecUpdateCanceled: the user kept the application of a line whose type changes
func BankRecUpdateCanceled() *AppError {
	return &AppError{Code: ErrBankRecUpdateCanceled}
}

// BankRecEndingBalance: the lines do not add up to Statement Ending Balance less Balance Last
// Statement (table: the reconciliation)
func BankRecEndingBalance(table string) *AppError {
	return &AppError{Code: ErrBankRecEndingBalance, Table: table, Field: "statement_ending_balance"}
}

// BankRecApplication: the lines' amounts are not the applied amounts and differences
func BankRecApplication(applied, total string) *AppError {
	return &AppError{Code: ErrBankRecApplication, Params: []string{applied, total}}
}

// BankRecTotalDifference: the lines' Difference does not add up to the difference lines
func BankRecTotalDifference(difference, total string) *AppError {
	return &AppError{Code: ErrBankRecTotalDifference, Params: []string{difference, total}}
}
