package errors

// Tax statement errors (report 12)
const ErrTaxStmtRowLoop = ErrorCode("ERR_TAX_STMT_ROW_LOOP")

// TaxStmtRowLoop: a row total that goes in a circle or nests deeper than six rows (the rows)
func TaxStmtRowLoop(table, rowNo, rows string) *AppError {
	return &AppError{Code: ErrTaxStmtRowLoop, Table: table, Field: "row_no", Params: []string{rowNo, rows}}
}
