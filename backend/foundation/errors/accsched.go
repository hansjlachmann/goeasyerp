package errors

import "strconv"

// Account schedule errors (tables 85/334, the calculation of codeunit 8)
const (
	ErrAccSchedParenthesisMisplaced  ErrorCode = "ERR_ACC_SCHED_PARENTHESIS_MISPLACED"
	ErrAccSchedTwoOperators          ErrorCode = "ERR_ACC_SCHED_TWO_OPERATORS"
	ErrAccSchedOperandMissing        ErrorCode = "ERR_ACC_SCHED_OPERAND_MISSING"
	ErrAccSchedMoreLeftParentheses   ErrorCode = "ERR_ACC_SCHED_MORE_LEFT_PARENTHESES"
	ErrAccSchedMoreRightParentheses  ErrorCode = "ERR_ACC_SCHED_MORE_RIGHT_PARENTHESES"
	ErrAccSchedTooManyBaseLines      ErrorCode = "ERR_ACC_SCHED_TOO_MANY_BASE_LINES"
	ErrAccSchedPercentBaseRequired   ErrorCode = "ERR_ACC_SCHED_PERCENT_BASE_REQUIRED"
	ErrAccSchedPeriodFormulaPeriods  ErrorCode = "ERR_ACC_SCHED_PERIOD_FORMULA_PERIODS"
	ErrAccSchedPeriodFormulaInvalid  ErrorCode = "ERR_ACC_SCHED_PERIOD_FORMULA_INVALID"
	ErrAccSchedCircularReference     ErrorCode = "ERR_ACC_SCHED_CIRCULAR_REFERENCE"
	ErrAccSchedIllegalRowNo          ErrorCode = "ERR_ACC_SCHED_ILLEGAL_ROW_NO"
	ErrAccSchedIllegalColumnNo       ErrorCode = "ERR_ACC_SCHED_ILLEGAL_COLUMN_NO"
	ErrAccSchedDateFilterRequired    ErrorCode = "ERR_ACC_SCHED_DATE_FILTER_REQUIRED"
	ErrAccSchedColumnLayoutNotExists ErrorCode = "ERR_ACC_SCHED_COLUMN_LAYOUT_NOT_EXISTS"
)

// AccSchedFormula: a formula that does not parse (code: one of the formula errors; position: the
// 1-based position of the error, 0 for none)
func AccSchedFormula(code ErrorCode, position int) *AppError {
	if position == 0 {
		return &AppError{Code: code}
	}
	return &AppError{Code: code, Params: []string{strconv.Itoa(position)}}
}

// AccSchedCalc: a cell that cannot be calculated; params are the error's own values followed by
// the cell: the line's Row No., Line No. and Totaling, the column's Column No., Line No. and
// Formula (the messages end with them)
func AccSchedCalc(code ErrorCode, params ...string) *AppError {
	return &AppError{Code: code, Params: params}
}

// AccSchedDateFilterRequired: an account schedule calculated without a date filter
func AccSchedDateFilterRequired() *AppError {
	return &AppError{Code: ErrAccSchedDateFilterRequired}
}

// AccSchedColumnLayoutNotExists: the column layout name does not exist
func AccSchedColumnLayoutNotExists(name string) *AppError {
	return &AppError{Code: ErrAccSchedColumnLayoutNotExists, Params: []string{name}}
}
