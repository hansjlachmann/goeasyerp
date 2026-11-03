package errors

// Reservations (codeunit 400 Reservation Management and the reserving lines)
const (
	ErrReservQtyNotPositive       ErrorCode = "ERR_RESERV_QTY_NOT_POSITIVE"
	ErrReservValidToDateMissing   ErrorCode = "ERR_RESERV_VALID_TO_DATE_MISSING"
	ErrReservQtyGreaterZero       ErrorCode = "ERR_RESERV_QTY_GREATER_ZERO"
	ErrReservUseCancel            ErrorCode = "ERR_RESERV_USE_CANCEL"
	ErrReservQtyOnlyReduced       ErrorCode = "ERR_RESERV_QTY_ONLY_REDUCED"
	ErrReservDateBeforeExpected   ErrorCode = "ERR_RESERV_DATE_BEFORE_EXPECTED"
	ErrReservQtyTooHigh           ErrorCode = "ERR_RESERV_QTY_TOO_HIGH"
	ErrReservWrongSign            ErrorCode = "ERR_RESERV_WRONG_SIGN"
	ErrReservOnlyReduceInventory  ErrorCode = "ERR_RESERV_ONLY_REDUCE_INVENTORY"
	ErrReservFieldMustBeFilled    ErrorCode = "ERR_RESERV_FIELD_MUST_BE_FILLED"
	ErrReservFieldMustNotBeFilled ErrorCode = "ERR_RESERV_FIELD_MUST_NOT_BE_FILLED"
	ErrReservFieldMustNotChange   ErrorCode = "ERR_RESERV_FIELD_MUST_NOT_CHANGE"
	ErrReservDateBeforeReceipt    ErrorCode = "ERR_RESERV_DATE_BEFORE_RECEIPT"
	ErrReservDateAfterShipment    ErrorCode = "ERR_RESERV_DATE_AFTER_SHIPMENT"
	ErrReservFullyReserved        ErrorCode = "ERR_RESERV_FULLY_RESERVED"
	ErrReservNothingAvailable     ErrorCode = "ERR_RESERV_NOTHING_AVAILABLE"
	ErrReservNegativeManually     ErrorCode = "ERR_RESERV_NEGATIVE_MANUALLY"
	ErrReservNothingToCancel      ErrorCode = "ERR_RESERV_NOTHING_TO_CANCEL"
	ErrReservAvailableQty         ErrorCode = "ERR_RESERV_AVAILABLE_QTY"
	ErrReservNotOnInventory       ErrorCode = "ERR_RESERV_NOT_ON_INVENTORY"
	ErrReservError                ErrorCode = "ERR_RESERV_ERROR"
	ErrReservOnlyPositive         ErrorCode = "ERR_RESERV_ONLY_POSITIVE"
	ErrReservMustBeReduced        ErrorCode = "ERR_RESERV_MUST_BE_REDUCED"
	ErrReservDateNotPostingDate   ErrorCode = "ERR_RESERV_DATE_NOT_POSTING_DATE"
)

// ReservQtyNotPositive: a reservation's quantity must be greater than 0
func ReservQtyNotPositive() *AppError { return &AppError{Code: ErrReservQtyNotPositive} }

// ReservValidToDateMissing: a reservation needs a valid-to date
func ReservValidToDateMissing() *AppError { return &AppError{Code: ErrReservValidToDateMissing} }

// ReservQtyGreaterZero: a field (the entry's Quantity) must be greater than 0
func ReservQtyGreaterZero(table, field string) *AppError {
	return &AppError{Code: ErrReservQtyGreaterZero, Table: table, Field: field}
}

// ReservUseCancel: a reservation changed to 0 is canceled instead
func ReservUseCancel() *AppError { return &AppError{Code: ErrReservUseCancel} }

// ReservQtyOnlyReduced: a reservation's quantity (field) can only be reduced
func ReservQtyOnlyReduced(table, field string) *AppError {
	return &AppError{Code: ErrReservQtyOnlyReduced, Table: table, Field: field}
}

// ReservDateBeforeExpected: the line's date (field) is before the supply's expected date
func ReservDateBeforeExpected(table, field string) *AppError {
	return &AppError{Code: ErrReservDateBeforeExpected, Table: table, Field: field}
}

// ReservQtyTooHigh: a line cannot reserve more than its quantity (the rest: max)
func ReservQtyTooHigh(max string) *AppError {
	return &AppError{Code: ErrReservQtyTooHigh, Params: []string{max}}
}

// ReservWrongSign: the line (table) has a quantity of the wrong sign for the reservation
func ReservWrongSign(table string) *AppError {
	return &AppError{Code: ErrReservWrongSign, Table: table, WithTableCaption: true}
}

// ReservOnlyReduceInventory: a line that adds to the inventory reserves nothing
func ReservOnlyReduceInventory() *AppError { return &AppError{Code: ErrReservOnlyReduceInventory} }

// ReservFieldMustBeFilled: a field must have a value while the line has a reserved quantity
func ReservFieldMustBeFilled(table, field string) *AppError {
	return &AppError{Code: ErrReservFieldMustBeFilled, Table: table, Field: field}
}

// ReservFieldMustNotBeFilled: a field must be blank while the line has a reserved quantity
func ReservFieldMustNotBeFilled(table, field string) *AppError {
	return &AppError{Code: ErrReservFieldMustNotBeFilled, Table: table, Field: field}
}

// ReservFieldMustNotChange: a field cannot change while the line has a reserved quantity
func ReservFieldMustNotChange(table, field string) *AppError {
	return &AppError{Code: ErrReservFieldMustNotChange, Table: table, Field: field}
}

// ReservDateBeforeReceipt: the line's date (field) would come before the reserved supply's
// expected receipt
func ReservDateBeforeReceipt(table, field string) *AppError {
	return &AppError{Code: ErrReservDateBeforeReceipt, Table: table, Field: field}
}

// ReservDateAfterShipment: the supply's date (field) would come after the shipment it is
// reserved for
func ReservDateAfterShipment(table, field string) *AppError {
	return &AppError{Code: ErrReservDateAfterShipment, Table: table, Field: field}
}

// ReservFullyReserved: the line is reserved in full
func ReservFullyReserved() *AppError { return &AppError{Code: ErrReservFullyReserved} }

// ReservNothingAvailable: nothing could be reserved from the chosen source
func ReservNothingAvailable() *AppError { return &AppError{Code: ErrReservNothingAvailable} }

// ReservNegativeManually: a negative reservation is not canceled by hand
func ReservNegativeManually() *AppError { return &AppError{Code: ErrReservNegativeManually} }

// ReservNothingToCancel: the source has no reservations of the line
func ReservNothingToCancel() *AppError { return &AppError{Code: ErrReservNothingToCancel} }

// ReservAvailableQty: a supply line has only qty available
func ReservAvailableQty(qty string) *AppError {
	return &AppError{Code: ErrReservAvailableQty, Params: []string{qty}}
}

// ReservNotOnInventory: the item (no.) reserved for the posted line is not on inventory yet
func ReservNotOnInventory(no string) *AppError {
	return &AppError{Code: ErrReservNotOnInventory, Params: []string{no}}
}

// ReservError: the reservation entries of a posted line are not there
func ReservError() *AppError { return &AppError{Code: ErrReservError} }

// ReservOnlyPositive: only reservations from the line are transferred to an item ledger entry
func ReservOnlyPositive() *AppError { return &AppError{Code: ErrReservOnlyPositive} }

// ReservMustBeReduced: the reserved quantity (field) is more than the line's outstanding
// quantity — reduce it first
func ReservMustBeReduced(table, field string) *AppError {
	return &AppError{Code: ErrReservMustBeReduced, Table: table, Field: field}
}

// ReservShipmentDateNotPostingDate: a line that posts a reserved quantity must have its shipment
// (or expected receipt) date on the posting date
func ReservShipmentDateNotPostingDate(table, field string) *AppError {
	return &AppError{Code: ErrReservDateNotPostingDate, Table: table, Field: field}
}
