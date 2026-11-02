package errors

// Items (table 27 and the tables around it)
const (
	ErrItemUoMQtyNotPositive    ErrorCode = "ERR_ITEM_UOM_QTY_NOT_POSITIVE"
	ErrItemPriceInclTaxSalesTax ErrorCode = "ERR_ITEM_PRICE_INCL_TAX_SALES_TAX"
	ErrItemOpenEntriesChange    ErrorCode = "ERR_ITEM_OPEN_ENTRIES_CHANGE"
	ErrItemHasEntriesChange     ErrorCode = "ERR_ITEM_HAS_ENTRIES_CHANGE"
	ErrItemAdjustAverageCost    ErrorCode = "ERR_ITEM_ADJUST_AVERAGE_COST"
)

// ItemAdjustAverageCost: Adjust Item Costs/Prices cannot change the Average Cost of an item (no.)
// with the Costing Method (value) Average once it has ledger entries
func ItemAdjustAverageCost(no string, value any) *AppError {
	return &AppError{Code: ErrItemAdjustAverageCost, Table: "Item", Field: "average_cost", Field2: "costing_method", Params: []string{no}, Value: value}
}

// ItemUoMQtyNotPositive: Qty. per Unit of Measure must be greater than 0
func ItemUoMQtyNotPositive(table, field string) *AppError {
	return &AppError{Code: ErrItemUoMQtyNotPositive, Table: table, Field: field}
}

// ItemPriceInclTaxSalesTax: prices including tax cannot be calculated for a tax posting setup
// with Tax Calculation Type Sales Tax (value)
func ItemPriceInclTaxSalesTax(table, field string, value any) *AppError {
	return &AppError{Code: ErrItemPriceInclTaxSalesTax, Table: table, Field: field, Value: value}
}

// ItemOpenEntriesChange: a field of an item (no.) cannot change while it has open ledger entries
func ItemOpenEntriesChange(table, field, no string) *AppError {
	return &AppError{Code: ErrItemOpenEntriesChange, Table: table, Field: field, Params: []string{no}}
}

// ItemHasEntriesChange: a field of an item cannot change once it has ledger entries
func ItemHasEntriesChange(table, field string) *AppError {
	return &AppError{Code: ErrItemHasEntriesChange, Table: table, Field: field}
}

// Item journals and item posting
const (
	ErrItemCannotChangeWhen ErrorCode = "ERR_ITEM_CANNOT_CHANGE_WHEN"
	ErrItemNotOnInventory   ErrorCode = "ERR_ITEM_NOT_ON_INVENTORY"
	ErrItemMustBeReserved   ErrorCode = "ERR_ITEM_MUST_BE_RESERVED"
	ErrFieldTooLow          ErrorCode = "ERR_FIELD_TOO_LOW"
	ErrJnlClosingDate       ErrorCode = "ERR_JNL_CLOSING_DATE"
	ErrItemJnlNotNegative   ErrorCode = "ERR_ITEM_JNL_NOT_NEGATIVE"
	ErrItemJnlSameValue     ErrorCode = "ERR_ITEM_JNL_SAME_VALUE"
	ErrItemJnlPhysEntryType ErrorCode = "ERR_ITEM_JNL_PHYS_ENTRY_TYPE"
	ErrItemJnlPhysQty       ErrorCode = "ERR_ITEM_JNL_PHYS_QTY"
)

// ItemCannotChangeWhen: a field cannot be changed while another field has a value
// ("You cannot change Unit Cost when Costing Method is Standard")
func ItemCannotChangeWhen(table, field, field2 string, value any) *AppError {
	return &AppError{Code: ErrItemCannotChangeWhen, Table: table, Field: field, Field2: field2, Value: value}
}

// ItemNotOnInventory: an item (no.) has no inventory to take the line's quantity from
func ItemNotOnInventory(no string) *AppError {
	return &AppError{Code: ErrItemNotOnInventory, Params: []string{no}}
}

// ItemMustBeReserved: an item (no.) whose Reserve field is Always must be reserved before it
// leaves the inventory
func ItemMustBeReserved(table, field, no string, value any) *AppError {
	return &AppError{Code: ErrItemMustBeReserved, Table: table, Field: field, Params: []string{no}, Value: value}
}

// FieldTooLow: a field's value is too low (FieldError "is too low")
func FieldTooLow(table, field string) *AppError {
	return &AppError{Code: ErrFieldTooLow, Table: table, Field: field}
}

// JnlClosingDate: a date field cannot be a closing date
func JnlClosingDate(table, field string) *AppError {
	return &AppError{Code: ErrJnlClosingDate, Table: table, Field: field}
}

// ItemJnlNotNegative: a field must not be negative when another field has a value (Quantity of
// a transfer)
func ItemJnlNotNegative(table, field, field2 string, value any) *AppError {
	return &AppError{Code: ErrItemJnlNotNegative, Table: table, Field: field, Field2: field2, Value: value}
}

// ItemJnlSameValue: a field must have the value of another field (Invoiced Quantity of a
// transfer)
func ItemJnlSameValue(table, field, field2 string) *AppError {
	return &AppError{Code: ErrItemJnlSameValue, Table: table, Field: field, Field2: field2}
}

// ItemJnlPhysEntryType: a phys. inventory line is a positive or negative adjustment
func ItemJnlPhysEntryType(table, field, field2 string) *AppError {
	return &AppError{Code: ErrItemJnlPhysEntryType, Table: table, Field: field, Field2: field2}
}

// ItemJnlPhysQty: the quantity of a phys. inventory line is the difference between the
// counted and the calculated quantity
func ItemJnlPhysQty(table, field string) *AppError {
	return &AppError{Code: ErrItemJnlPhysQty, Table: table, Field: field}
}

// ErrItemSpecificNotYet: Costing Method Specific needs serial numbers (item tracking), which
// come later
const ErrItemSpecificNotYet ErrorCode = "ERR_ITEM_SPECIFIC_NOT_YET"

// ItemSpecificNotYet: an item with Costing Method Specific cannot be posted yet
func ItemSpecificNotYet(table, field string, value any) *AppError {
	return &AppError{Code: ErrItemSpecificNotYet, Table: table, Field: field, Value: value}
}

// Pictures (BLOB fields)
const (
	ErrPictureInvalid ErrorCode = "ERR_PICTURE_INVALID"
	ErrPictureNoField ErrorCode = "ERR_PICTURE_NO_FIELD"
)

// PictureInvalid: an uploaded file is not a PNG, JPEG or WebP image of acceptable size
func PictureInvalid() *AppError { return &AppError{Code: ErrPictureInvalid} }

// PictureNoField: a table has no picture field of that name
func PictureNoField(field string) *AppError {
	return &AppError{Code: ErrPictureNoField, Params: []string{field}}
}

// Sales documents (tables 36 and 37)
const (
	ErrSalesResetCustomerLines ErrorCode = "ERR_SALES_RESET_CUSTOMER_LINES"
)

// SalesResetCustomerLines: the customer of a document with lines cannot be cleared ("You cannot
// reset Sell-to Customer No. because the document still has one or more lines.")
func SalesResetCustomerLines(table, field string) *AppError {
	return &AppError{Code: ErrSalesResetCustomerLines, Table: table, Field: field}
}
