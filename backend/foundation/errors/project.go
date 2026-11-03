package errors

// Project errors (tables 167 Project, 168 Project Budget Line, 211 Project Budget Entry)
const (
	ErrProjectDeleteStatus      ErrorCode = "ERR_PROJECT_DELETE_STATUS"
	ErrProjectPreviousStatus    ErrorCode = "ERR_PROJECT_PREVIOUS_STATUS"
	ErrProjectBudgetLineRename  ErrorCode = "ERR_PROJECT_BUDGET_LINE_RENAME"
	ErrProjectBudgetEntryNoDate ErrorCode = "ERR_PROJECT_BUDGET_ENTRY_NO_DATE"
	ErrProjectApplyToEntry      ErrorCode = "ERR_PROJECT_APPLY_TO_ENTRY_MISSING"
	ErrProjectApplyOppositeSign ErrorCode = "ERR_PROJECT_APPLY_OPPOSITE_SIGN"
	ErrProjectNotRecognized     ErrorCode = "ERR_PROJECT_ENTRIES_NOT_RECOGNIZED"
	ErrProjectOutstandingOrders ErrorCode = "ERR_PROJECT_OUTSTANDING_PURCH_ORDERS"
)

// ProjectDeleteStatus: a project with status Order cannot be deleted
func ProjectDeleteStatus(table string, status any) *AppError {
	return &AppError{Code: ErrProjectDeleteStatus, Table: table, Field: "status", Value: status}
}

// ProjectPreviousStatus: a project's status only moves on (Planning, Quote, Order, Completed)
func ProjectPreviousStatus(table string) *AppError {
	return &AppError{Code: ErrProjectPreviousStatus, Table: table, Field: "status"}
}

// ProjectBudgetLineRename: a budget line's key is not changed — the line is deleted and a new
// one entered
func ProjectBudgetLineRename(table string) *AppError {
	return &AppError{Code: ErrProjectBudgetLineRename, Table: table, WithTableCaption: true}
}

// ProjectBudgetEntryNoDate: a budget quantity needs a date — the entry's, else its budget
// line's Starting Date
func ProjectBudgetEntryNoDate(table string) *AppError {
	return &AppError{Code: ErrProjectBudgetEntryNoDate, Table: table, Field: "date"}
}

// ProjectApplyToEntryMissing: the Applies-to Entry of a project journal line does not exist
func ProjectApplyToEntryMissing(table string) *AppError {
	return &AppError{Code: ErrProjectApplyToEntry, Table: table, Field: "applies_to_entry"}
}

// ProjectApplyOppositeSign: a project entry applies only to entries of the other sign
func ProjectApplyOppositeSign(table string) *AppError {
	return &AppError{Code: ErrProjectApplyOppositeSign, Table: table}
}

// ProjectEntriesNotRecognized: a project whose entries are not all recognized is not deleted
func ProjectEntriesNotRecognized(table, no string) *AppError {
	return &AppError{Code: ErrProjectNotRecognized, Table: table, WithTableCaption: true, Params: []string{no}}
}

// ProjectOutstandingPurchOrders: a project with purchase order lines is not deleted
func ProjectOutstandingPurchOrders(table, no string) *AppError {
	return &AppError{Code: ErrProjectOutstandingOrders, Table: table, WithTableCaption: true, Params: []string{no}}
}
