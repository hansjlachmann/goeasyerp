package errors

// Report Selections errors (table 77, codeunit 229 Document-Print)
const (
	ErrReportSelectionNone ErrorCode = "ERR_REPORT_SELECTION_NONE"
)

// ReportSelectionNone: no report is selected for the usage (its option value)
func ReportSelectionNone(usage any) *AppError {
	return &AppError{Code: ErrReportSelectionNone, Table: "Report_Selections", Field: "usage", WithTableCaption: true, Value: usage}
}
