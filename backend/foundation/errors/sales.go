package errors

import "strconv"

// Sales documents
const (
	ErrSalesQtyLessThan         ErrorCode = "ERR_SALES_QTY_LESS_THAN"
	ErrSalesCannotShipMore      ErrorCode = "ERR_SALES_CANNOT_SHIP_MORE"
	ErrSalesCannotShipMoreBase  ErrorCode = "ERR_SALES_CANNOT_SHIP_MORE_BASE"
	ErrSalesCannotInvoiceMore   ErrorCode = "ERR_SALES_CANNOT_INVOICE_MORE"
	ErrSalesCannotInvoiceMoreBs ErrorCode = "ERR_SALES_CANNOT_INVOICE_MORE_BASE"
	ErrSalesShipOrInvoice       ErrorCode = "ERR_SALES_SHIP_OR_INVOICE"
	ErrSalesNothingToPost       ErrorCode = "ERR_SALES_NOTHING_TO_POST"
	ErrSalesCannotPostType      ErrorCode = "ERR_SALES_CANNOT_POST_TYPE"
)

// SalesShipOrInvoice: an order is posted with neither Ship nor Invoice ("Please enter Yes in
// Ship and/or Invoice.")
func SalesShipOrInvoice(table, field, field2 string) *AppError {
	return &AppError{Code: ErrSalesShipOrInvoice, Table: table, Field: field, Field2: field2}
}

// SalesNothingToPost: a document has no line with something to ship or invoice
func SalesNothingToPost() *AppError {
	return &AppError{Code: ErrSalesNothingToPost}
}

// SalesCannotPostType: a document of a type that is not posted (a quote, a blanket order) —
// "Document Type Quote cannot be posted."
func SalesCannotPostType(table, field string, value any) *AppError {
	return &AppError{Code: ErrSalesCannotPostType, Table: table, Field: field, Value: value}
}

// SalesQtyLessThan: a line's quantity is below what was shipped or invoiced ("Quantity must not
// be less than Quantity Shipped.")
func SalesQtyLessThan(table, field, field2 string) *AppError {
	return &AppError{Code: ErrSalesQtyLessThan, Table: table, Field: field, Field2: field2}
}

// SalesCannotShipMore: Qty. to Ship above the outstanding quantity ("You cannot ship more than
// %1 units."; base: in base units)
func SalesCannotShipMore(qty string, base bool) *AppError {
	if base {
		return &AppError{Code: ErrSalesCannotShipMoreBase, Params: []string{qty}}
	}
	return &AppError{Code: ErrSalesCannotShipMore, Params: []string{qty}}
}

// SalesCannotInvoiceMore: Qty. to Invoice above what can be invoiced ("You cannot invoice more
// than %1 units."; base: in base units)
func SalesCannotInvoiceMore(qty string, base bool) *AppError {
	if base {
		return &AppError{Code: ErrSalesCannotInvoiceMoreBs, Params: []string{qty}}
	}
	return &AppError{Code: ErrSalesCannotInvoiceMore, Params: []string{qty}}
}

// ErrBlanketOrderQtyTooHigh: a blanket order line would ship / receive or invoice more than its
// quantity (Field: the shipped/received quantity, Field2: Quantity)
const ErrBlanketOrderQtyTooHigh ErrorCode = "ERR_BLANKET_ORDER_QTY_TOO_HIGH"

// BlanketOrderQtyTooHigh: the field of the blanket order line would be greater than field2
func BlanketOrderQtyTooHigh(table, field, field2 string) *AppError {
	return &AppError{Code: ErrBlanketOrderQtyTooHigh, Table: table, Field: field, Field2: field2}
}

// ErrBlanketOrderQtyToMake: a blanket order line's quantity to ship / receive is more than is
// left of it after the orders made from it (params: the line no., what is outstanding, on
// orders, possible)
const ErrBlanketOrderQtyToMake ErrorCode = "ERR_BLANKET_ORDER_QTY_TO_MAKE"

// BlanketOrderQtyToMake: the field (Qty. to Ship/Receive (Base)) of a blanket line is too high
func BlanketOrderQtyToMake(table, field string, lineNo int, outstanding, onOrders, possible string) *AppError {
	return &AppError{Code: ErrBlanketOrderQtyToMake, Table: table, Field: field,
		Params: []string{strconv.Itoa(lineNo), outstanding, onOrders, possible}}
}

// ErrBlanketOrderNoLines: Make Order on a blanket order with nothing to make
const ErrBlanketOrderNoLines ErrorCode = "ERR_BLANKET_ORDER_NO_LINES"

// BlanketOrderNoLines: there are no lines to create
func BlanketOrderNoLines() *AppError {
	return &AppError{Code: ErrBlanketOrderNoLines}
}

// ErrIntrastatStatisticsPeriod: an Intrastat batch's Statistics Period is not YYMM (Field)
const ErrIntrastatStatisticsPeriod ErrorCode = "ERR_INTRASTAT_STATISTICS_PERIOD"

// ErrIntrastatMonth: the month of a Statistics Period is not 1-12
const ErrIntrastatMonth ErrorCode = "ERR_INTRASTAT_MONTH"

// IntrastatStatisticsPeriod: the batch's Statistics Period must be 4 characters
func IntrastatStatisticsPeriod(table string) *AppError {
	return &AppError{Code: ErrIntrastatStatisticsPeriod, Table: table, Field: "statistics_period"}
}

// IntrastatMonth: the Statistics Period's month is not a month
func IntrastatMonth() *AppError {
	return &AppError{Code: ErrIntrastatMonth}
}
