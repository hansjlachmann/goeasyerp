package errors

import "strconv"

// Drop shipments: a sales order line delivered by the vendor straight to the customer, linked to
// the purchase order line that buys it
const (
	ErrSalesLineAssocPurchOrder  ErrorCode = "ERR_SALES_LINE_ASSOC_PURCH_ORDER"
	ErrSalesLineDeleteAssoc      ErrorCode = "ERR_SALES_LINE_DELETE_ASSOC"
	ErrPurchLineAssocSalesOrder  ErrorCode = "ERR_PURCH_LINE_ASSOC_SALES_ORDER"
	ErrPurchHeaderAssocSales     ErrorCode = "ERR_PURCH_HEADER_ASSOC_SALES_ORDERS"
	ErrDropShptNoLinesToRetrieve ErrorCode = "ERR_DROP_SHPT_NO_LINES_TO_RETRIEVE"
	ErrDropShptNoSalesLines      ErrorCode = "ERR_DROP_SHPT_NO_SALES_LINES"
	ErrDropShptNotAssociated     ErrorCode = "ERR_DROP_SHPT_NOT_ASSOCIATED"
	ErrDropShptReceiveAndInvoice ErrorCode = "ERR_DROP_SHPT_RECEIVE_AND_INVOICE"
	ErrDropShptInvoiceSalesFirst ErrorCode = "ERR_DROP_SHPT_INVOICE_SALES_FIRST"
	ErrDropShptAlreadyAssociated ErrorCode = "ERR_DROP_SHPT_ALREADY_ASSOCIATED"
	ErrDropShptReserved          ErrorCode = "ERR_DROP_SHPT_RESERVED"
)

// SalesLineAssocPurchOrder: a field of a sales order line linked to a purchase order line
// cannot change ("You cannot change %1 because the order line is associated with purchase order
// %2.")
func SalesLineAssocPurchOrder(table, field, purchOrderNo string) *AppError {
	return &AppError{Code: ErrSalesLineAssocPurchOrder, Table: table, Field: field, Params: []string{purchOrderNo}}
}

// SalesLineDeleteAssoc: a sales order line linked to a purchase order line is not deleted
func SalesLineDeleteAssoc(purchOrderNo string) *AppError {
	return &AppError{Code: ErrSalesLineDeleteAssoc, Params: []string{purchOrderNo}}
}

// PurchLineAssocSalesOrder: a field of a drop shipment purchase line cannot change
func PurchLineAssocSalesOrder(table, field, salesOrderNo string) *AppError {
	return &AppError{Code: ErrPurchLineAssocSalesOrder, Table: table, Field: field, Params: []string{salesOrderNo}}
}

// PurchHeaderAssocSales: the customer and ship-to address of a purchase order with drop
// shipment lines cannot change
func PurchHeaderAssocSales(table, field string) *AppError {
	return &AppError{Code: ErrPurchHeaderAssocSales, Table: table, Field: field}
}

// DropShptNoLinesToRetrieve: Get Sales Order found no drop shipment lines on the sales order
func DropShptNoLinesToRetrieve(salesOrderNo string) *AppError {
	return &AppError{Code: ErrDropShptNoLinesToRetrieve, Params: []string{salesOrderNo}}
}

// DropShptNoSalesLines: Get Sales Orders found no drop shipment lines
func DropShptNoSalesLines() *AppError {
	return &AppError{Code: ErrDropShptNoSalesLines}
}

// DropShptNotAssociated: a drop shipment line without purchase order cannot ship
func DropShptNotAssociated(lineNo int) *AppError {
	return &AppError{Code: ErrDropShptNotAssociated, Params: []string{strconv.Itoa(lineNo)}}
}

// DropShptReceiveAndInvoice: a drop shipment purchase line is received and invoiced in two
// postings
func DropShptReceiveAndInvoice() *AppError {
	return &AppError{Code: ErrDropShptReceiveAndInvoice}
}

// DropShptInvoiceSalesFirst: the purchase of a drop shipment is invoiced after its sale
func DropShptInvoiceSalesFirst(salesOrderNo string) *AppError {
	return &AppError{Code: ErrDropShptInvoiceSalesFirst, Params: []string{salesOrderNo}}
}

// DropShptAlreadyAssociated: Make Orders finds the sales order line linked already
func DropShptAlreadyAssociated(itemNo, salesOrderNo, purchOrderNo string) *AppError {
	return &AppError{Code: ErrDropShptAlreadyAssociated, Params: []string{itemNo, salesOrderNo, purchOrderNo}}
}

// DropShptReserved: a line with a reservation cannot be a drop shipment (field: Drop Shipment,
// Sales Order No. …)
func DropShptReserved(table, field string) *AppError {
	return &AppError{Code: ErrDropShptReserved, Table: table, Field: field}
}
