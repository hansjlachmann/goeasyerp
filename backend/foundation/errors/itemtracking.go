package errors

// Item tracking (serial and lot numbers) errors
const (
	ErrItemTrackingNotOnInventory  ErrorCode = "ERR_ITEM_TRACKING_NOT_ON_INVENTORY"
	ErrSerialAlreadyOnInventory    ErrorCode = "ERR_SERIAL_ALREADY_ON_INVENTORY"
	ErrSerialShippedNotReceived    ErrorCode = "ERR_SERIAL_SHIPPED_NOT_RECEIVED"
	ErrSerialQtyMustBeOne          ErrorCode = "ERR_SERIAL_QTY_MUST_BE_ONE"
)

// ItemTrackingNotOnInventory: an outbound entry of an item with a lot/serial number that is
// not on inventory (tracking: the item's lot and serial number as text, ItemTrackingText)
func ItemTrackingNotOnInventory(itemNo, tracking string) *AppError {
	return &AppError{Code: ErrItemTrackingNotOnInventory, Params: []string{itemNo, tracking}}
}

// SerialAlreadyOnInventory: an inbound entry of a serial number that is on inventory already
func SerialAlreadyOnInventory(serialNo string) *AppError {
	return &AppError{Code: ErrSerialAlreadyOnInventory, Params: []string{serialNo}}
}

// SerialShippedNotReceived: an outbound entry of a serial number shipped already and not
// received again
func SerialShippedNotReceived(serialNo string) *AppError {
	return &AppError{Code: ErrSerialShippedNotReceived, Params: []string{serialNo}}
}

// SerialQtyMustBeOne: a line with a serial number posts one piece (the field: Quantity or
// Invoiced Quantity)
func SerialQtyMustBeOne(table, field string) *AppError {
	return &AppError{Code: ErrSerialQtyMustBeOne, Table: table, Field: field}
}
