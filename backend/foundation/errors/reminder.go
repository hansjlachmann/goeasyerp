package errors

// Reminder and finance charge memo errors (tables 295/296, 302/303, codeunits 392–395)
const (
	ErrReminderNoSpaceForText   ErrorCode = "ERR_REMINDER_NO_SPACE_FOR_TEXT"
	ErrReminderCurrencyMismatch ErrorCode = "ERR_REMINDER_CURRENCY_MISMATCH"
	ErrReminderLineTypeMustBe   ErrorCode = "ERR_REMINDER_LINE_TYPE_MUST_BE"
	ErrReminderNothingToIssue   ErrorCode = "ERR_REMINDER_NOTHING_TO_ISSUE"
	ErrReminderAmountNegative   ErrorCode = "ERR_REMINDER_AMOUNT_NEGATIVE"
)

// ReminderNoSpaceForText: no line numbers left before the first line for the beginning texts
func ReminderNoSpaceForText() *AppError {
	return &AppError{Code: ErrReminderNoSpaceForText}
}

// ReminderCurrencyMismatch: the entry's currency differs from the document's (table: the header)
func ReminderCurrencyMismatch(table string) *AppError {
	return &AppError{Code: ErrReminderCurrencyMismatch, Table: table, Field: "currency_code", WithTableCaption: true}
}

// ReminderLineTypeMustBe: the line's Type must be one of two values (their captions)
func ReminderLineTypeMustBe(table, field string, value1, value2 any) *AppError {
	return &AppError{Code: ErrReminderLineTypeMustBe, Table: table, Field: field, Values: []any{value1, value2}}
}

// ReminderNothingToIssue: a document without interest, fee or remaining amount
func ReminderNothingToIssue() *AppError {
	return &AppError{Code: ErrReminderNothingToIssue}
}

// ReminderAmountNegative: a line's Amount must be positive or 0
func ReminderAmountNegative(table string) *AppError {
	return &AppError{Code: ErrReminderAmountNegative, Table: table, Field: "amount"}
}
