package errors

// Date compression errors (codeunit 356, the date compression reports)
const (
	ErrDateComprNoPeriod   ErrorCode = "ERR_DATE_COMPR_NO_PERIOD"
	ErrDateComprEndingDate ErrorCode = "ERR_DATE_COMPR_ENDING_DATE"
)

// DateComprNoPeriod: no accounting period (fiscal year) covers the date — the date compression
// works on the fiscal years of the accounting periods
func DateComprNoPeriod(date any) *AppError {
	return &AppError{Code: ErrDateComprNoPeriod, Table: "Accounting_Period", WithTableCaption: true, Value: date}
}

// DateComprEndingDate: a date compression needs its Ending Date
func DateComprEndingDate() *AppError {
	return &AppError{Code: ErrDateComprEndingDate, Table: "Date_Compr_Register", Field: "ending_date"}
}
