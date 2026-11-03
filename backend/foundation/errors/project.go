package errors

// Project errors (tables 167 Project, 168 Project Budget Line, 211 Project Budget Entry)
const (
	ErrProjectDeleteStatus      ErrorCode = "ERR_PROJECT_DELETE_STATUS"
	ErrProjectPreviousStatus    ErrorCode = "ERR_PROJECT_PREVIOUS_STATUS"
	ErrProjectBudgetLineRename  ErrorCode = "ERR_PROJECT_BUDGET_LINE_RENAME"
	ErrProjectBudgetEntryNoDate ErrorCode = "ERR_PROJECT_BUDGET_ENTRY_NO_DATE"
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
