package errors

// Checks: a posted check voided financially (Check Ledger Entries › Void Check)
const (
	ErrCheckVoidDateBeforeCheckDate ErrorCode = "ERR_CHECK_VOID_DATE_BEFORE_CHECK_DATE"
	ErrCheckReverseNotAllowed       ErrorCode = "ERR_CHECK_REVERSE_NOT_ALLOWED"
)

// CheckVoidDateBeforeCheckDate: the void date of a financial void is before the check's date
// (checkDate formatted in the user's language)
func CheckVoidDateBeforeCheckDate(checkDate string) *AppError {
	return &AppError{Code: ErrCheckVoidDateBeforeCheckDate, Params: []string{checkDate}}
}

// CheckReverseNotAllowed: Reverse Original Entry is not possible when an entry of the check was
// financially voided or is on a bank statement
func CheckReverseNotAllowed(checkNo string) *AppError {
	return &AppError{Code: ErrCheckReverseNotAllowed, Params: []string{checkNo}}
}
