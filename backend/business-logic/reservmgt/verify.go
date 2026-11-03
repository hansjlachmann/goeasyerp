package reservmgt

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// verifyLine is a line with a reserved quantity as its …VerifyChange sees it: its source, its
// reserved and own quantity (base), the sign their product with an entry's quantity must have
// (the line's quantity sign as a demand), its item, location and variant and its date — with the
// fields that hold them
type verifyLine struct {
	table                             string
	src                               Source
	reservedQtyBase, qtyBase          types.Decimal
	validSignNegative                 bool
	itemNo, locationCode, variantCode types.Code
	binCode, serialNo, lotNo          types.Code
	itemField, locationField          string
	binField, serialField, lotField   string // "" when the line has no such field (blank: none)
	date                              types.Date
	dateField                         string
}

// verifyChange is the classic …VerifyChange shared by the lines: the line's first open entry —
// as a demand the last one of its supplies (the inventory last), as a supply the first — must
// still match the line: date filled in, a reserved quantity of a valid sign, the same item,
// location, variant, serial and lot number; a demand's date may not come before its supply's expected receipt (a
// supply's not after its demand's shipment) and a new date goes to the entries
func (m *ReservationManagement) verifyChange(l verifyLine) error {
	if l.reservedQtyBase.IsZero() {
		return nil
	}
	tempReservEntry := gtables.NewReservationEntry(m.src)
	if l.reservedQtyBase.IsPositive() {
		InitFilterAndSortingLookupFor(tempReservEntry, true)
		FilterFor(tempReservEntry, l.src)
		tempReservEntry.SetFilter("from_type", "<>6")
		if !tempReservEntry.FindLast() {
			tempReservEntry.SetRange("from_type")
			if !tempReservEntry.FindLast() {
				return nil
			}
		} else {
			tempReservEntry.SetRange("from_type")
		}
	} else {
		InitFilterAndSortingLookupFrom(tempReservEntry, true)
		FilterFrom(tempReservEntry, l.src)
		if !tempReservEntry.FindFirst() {
			return nil
		}
	}
	if l.date.IsZero() {
		return apperrors.ReservFieldMustBeFilled(l.table, l.dateField)
	}
	product := l.qtyBase.Mul(l.reservedQtyBase).Mul(tempReservEntry.Quantity)
	if l.validSignNegative && product.IsPositive() || !l.validSignNegative && product.IsNegative() {
		return apperrors.FieldMustBeValue(l.table, l.reservedField(), types.Decimal{})
	}
	if l.itemNo != tempReservEntry.Item_no {
		return apperrors.ReservFieldMustNotChange(l.table, l.itemField)
	}
	if l.locationCode != tempReservEntry.Location_code {
		return apperrors.ReservFieldMustNotChange(l.table, l.locationField)
	}
	if l.variantCode != tempReservEntry.Variant_code {
		return apperrors.ReservFieldMustNotChange(l.table, "variant_code")
	}
	if l.binCode != tempReservEntry.Bin_code {
		return apperrors.ReservFieldMustNotChange(l.table, l.binField)
	}
	if l.serialNo != tempReservEntry.Serial_no {
		return apperrors.ReservFieldMustNotChange(l.table, l.serialField)
	}
	if l.lotNo != tempReservEntry.Lot_no {
		return apperrors.ReservFieldMustNotChange(l.table, l.lotField)
	}
	if l.reservedQtyBase.IsPositive() {
		if l.date.Before(tempReservEntry.Expected_receipt_date) && int(tempReservEntry.From_type) != TypeItemLedgerEntry {
			return apperrors.ReservDateBeforeReceipt(l.table, l.dateField)
		}
		if !l.date.Equal(tempReservEntry.Shipment_date) {
			return m.modifyShipmentDate(tempReservEntry, l.date)
		}
		return nil
	}
	if l.date.After(tempReservEntry.Shipment_date) {
		return apperrors.ReservDateAfterShipment(l.table, l.dateField)
	}
	if !l.date.Equal(tempReservEntry.Expected_receipt_date) {
		return m.modifyExpectedReceiptDate(tempReservEntry, l.date)
	}
	return nil
}

// reservedField: the line's reserved quantity field (an item journal line has two)
func (l verifyLine) reservedField() string {
	if l.table == gtables.ItemJournalLineTableName {
		if l.reservedQtyBase.IsPositive() {
			return "positive_reserved_qty_base"
		}
		return "negative_reserved_qty_base"
	}
	return "reserved_qty_base"
}
