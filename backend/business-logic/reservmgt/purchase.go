package reservmgt

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// PurchLineSource: a purchase line as one side of a reservation
func PurchLineSource(purchLine *gtables.PurchaseLine) Source {
	return Source{Type: TypePurchase, Subtype: int(purchLine.Document_type), ID: purchLine.Document_no, RefNo: purchLine.Line_no}
}

// purchLineCreateReservFor (PurchLineCreateReservFor): a purchase line that takes the item (a
// negative line, a credit memo) reserves a quantity from a supply
func (m *ReservationManagement) purchLineCreateReservFor(purchLine *gtables.PurchaseLine, validToDate types.Date, description types.Text, expectedReceiptDate types.Date, quantity types.Decimal, from Source, fromVariantCode, fromLocationCode, fromBinCode, fromSerialNo, fromLotNo types.Code) error {
	err := purchLine.TestField("type", gtables.PurchaseLine_Type.Item)
	if err == nil {
		err = purchLine.TestField("no")
	}
	if err == nil {
		err = purchLine.TestField("expected_receipt_date")
	}
	if err != nil {
		return err
	}
	if purchLine.Expected_receipt_date.Before(expectedReceiptDate) && from.Type != TypeItemLedgerEntry {
		return apperrors.ReservDateBeforeExpected(gtables.PurchaseLineTableName, "expected_receipt_date")
	}
	if purchLine.Outstanding_qty_base.Abs().LessThan(purchLine.Reserved_qty_base.Abs().Add(quantity)) {
		return apperrors.ReservQtyTooHigh(purchLine.Outstanding_qty_base.Abs().Sub(purchLine.Reserved_qty_base.Abs()).String())
	}
	err = purchLine.TestField("location_code", fromLocationCode)
	if err == nil {
		err = purchLine.TestField("variant_code", fromVariantCode)
	}
	if err == nil {
		err = purchLine.TestField("bin_code", fromBinCode)
	}
	if err == nil {
		err = purchLine.TestField("serial_no", fromSerialNo)
	}
	if err == nil {
		err = purchLine.TestField("lot_no", fromLotNo)
	}
	if err != nil {
		return err
	}
	if purchLine.Document_type == gtables.PurchaseLine_Document_type.CreditMemo {
		if purchLine.Quantity_base.Mul(quantity).IsNegative() || purchLine.Quantity_base.IsNegative() {
			return apperrors.ReservWrongSign(gtables.PurchaseLineTableName)
		}
	} else if purchLine.Quantity_base.Mul(quantity).IsPositive() || purchLine.Quantity_base.IsPositive() {
		return apperrors.ReservWrongSign(gtables.PurchaseLineTableName)
	}
	err = purchLine.Validate("reserved_qty_base", purchLine.Reserved_qty_base.Add(quantity))
	if err != nil {
		return err
	}
	return m.createReservEntry(PurchLineSource(purchLine), from, reservEntryValues{
		itemNo: purchLine.No, variantCode: purchLine.Variant_code, locationCode: purchLine.Location_code,
		binCode: purchLine.Bin_code, serialNo: purchLine.Serial_no, lotNo: purchLine.Lot_no,
		validToDate: validToDate, description: description,
		expectedReceiptDate: expectedReceiptDate, shipmentDate: purchLine.Expected_receipt_date, quantity: quantity})
}

// PurchLineReservQuantity (PurchLineReservQuantity): what a purchase line can reserve — a
// negative outstanding quantity (a credit memo's positive one); a line that brings the item
// reserves nothing
func PurchLineReservQuantity(purchLine *gtables.PurchaseLine, giveError bool) (types.Decimal, error) {
	qty := purchLine.Outstanding_qty_base.Neg()
	if purchLine.Document_type == gtables.PurchaseLine_Document_type.CreditMemo {
		qty = qty.Neg()
	}
	if !qty.IsNegative() {
		return qty, nil
	}
	if giveError {
		return types.Decimal{}, apperrors.ReservOnlyReduceInventory()
	}
	return types.Decimal{}, nil
}

// PurchLineSetReservFor (PurchLineSetReservFor): the purchase line is the line reserved for
func (m *ReservationManagement) PurchLineSetReservFor(currentPurchLine *gtables.PurchaseLine) error {
	m.clearAll()
	reservSetup := m.getReservSetup()
	field := map[gtables.PurchaseLineDocument_type]string{
		gtables.PurchaseLine_Document_type.Quote:        "reserve_purchase_quote",
		gtables.PurchaseLine_Document_type.Order:        "reserve_purchase_order",
		gtables.PurchaseLine_Document_type.Invoice:      "reserve_purchase_invoice",
		gtables.PurchaseLine_Document_type.CreditMemo:   "reserve_purchase_credit_memo",
		gtables.PurchaseLine_Document_type.BlanketOrder: "reserve_blanket_purchase_order",
	}[currentPurchLine.Document_type]
	err := reservSetup.TestField(field, true)
	if err == nil {
		err = currentPurchLine.TestField("type", gtables.PurchaseLine_Type.Item)
	}
	if err != nil {
		return err
	}
	m.forPurchLine = gtables.NewPurchaseLine(m.src)
	m.forPurchLine.Copy(currentPurchLine)
	m.setReservFor(TypePurchase, currentPurchLine.No, currentPurchLine.Variant_code, currentPurchLine.Location_code, currentPurchLine.Bin_code, currentPurchLine.Serial_no, currentPurchLine.Lot_no,
		currentPurchLine.Expected_receipt_date, PurchLineSource(currentPurchLine))
	return nil
}

// PurchLineUpdateValues (PurchLineUpdateValues): a purchase line as a supply (Available -
// Purchase Lines): what it can still give and what the line reserved for has from it
func (m *ReservationManagement) PurchLineUpdateValues(purchLine *gtables.PurchaseLine) (types.Decimal, types.Decimal, error) {
	var qtyToReserve types.Decimal
	if purchLine.Document_type == gtables.PurchaseLine_Document_type.CreditMemo {
		if purchLine.Quantity.IsNegative() {
			qtyToReserve = purchLine.Outstanding_qty_base.Neg().Add(purchLine.Reserved_qty_base)
		}
	} else if purchLine.Quantity.IsPositive() {
		qtyToReserve = purchLine.Outstanding_qty_base.Add(purchLine.Reserved_qty_base)
	}
	qtyReservedThisLine, err := m.QtyReservedThisLine(PurchLineSource(purchLine), true)
	return qtyToReserve, qtyReservedThisLine, err
}

// PurchLineVerifyChange (PurchLineVerifyChange): as SalesLineVerifyChange, on the expected
// receipt date
func (m *ReservationManagement) PurchLineVerifyChange(newPurchLine *gtables.PurchaseLine) error {
	return m.verifyChange(verifyLine{
		table: gtables.PurchaseLineTableName, src: PurchLineSource(newPurchLine),
		reservedQtyBase: newPurchLine.Reserved_qty_base, qtyBase: newPurchLine.Quantity_base,
		validSignNegative: newPurchLine.Document_type != gtables.PurchaseLine_Document_type.CreditMemo,
		itemNo:            newPurchLine.No, locationCode: newPurchLine.Location_code, variantCode: newPurchLine.Variant_code,
		binCode: newPurchLine.Bin_code, serialNo: newPurchLine.Serial_no, lotNo: newPurchLine.Lot_no, binField: "bin_code", serialField: "serial_no", lotField: "lot_no",
		itemField: "no", locationField: "location_code",
		date: newPurchLine.Expected_receipt_date, dateField: "expected_receipt_date"})
}
