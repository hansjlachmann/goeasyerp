package reservmgt

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// SalesLineSource: a sales line as one side of a reservation
func SalesLineSource(salesLine *gtables.SalesLine) Source {
	return Source{Type: TypeSales, Subtype: int(salesLine.Document_type), ID: salesLine.Document_no, RefNo: salesLine.Line_no}
}

// salesLineCreateReservFor (SalesLineCreateReservFor): the sales line reserves a quantity from
// a supply of its item, variant and location dated no later than its shipment date
func (m *ReservationManagement) salesLineCreateReservFor(salesLine *gtables.SalesLine, validToDate types.Date, description types.Text, expectedReceiptDate types.Date, quantity types.Decimal, from Source, fromVariantCode, fromLocationCode, fromBinCode, fromSerialNo, fromLotNo types.Code) error {
	err := salesLine.TestField("type", gtables.SalesLine_Type.Item)
	if err == nil {
		err = salesLine.TestField("no")
	}
	if err == nil {
		err = salesLine.TestField("shipment_date")
	}
	if err != nil {
		return err
	}
	if salesLine.Shipment_date.Before(expectedReceiptDate) && from.Type != TypeItemLedgerEntry {
		return apperrors.ReservDateBeforeExpected(gtables.SalesLineTableName, "shipment_date")
	}
	if salesLine.Outstanding_qty_base.Abs().LessThan(salesLine.Reserved_qty_base.Abs().Add(quantity)) {
		return apperrors.ReservQtyTooHigh(salesLine.Outstanding_qty_base.Abs().Sub(salesLine.Reserved_qty_base.Abs()).String())
	}
	err = salesLine.TestField("location_code", fromLocationCode)
	if err == nil {
		err = salesLine.TestField("variant_code", fromVariantCode)
	}
	if err == nil {
		err = salesLine.TestField("bin_code", fromBinCode)
	}
	if err == nil {
		err = salesLine.TestField("serial_no", fromSerialNo)
	}
	if err == nil {
		err = salesLine.TestField("lot_no", fromLotNo)
	}
	if err != nil {
		return err
	}
	switch salesLine.Document_type {
	case gtables.SalesLine_Document_type.CreditMemo:
		if salesLine.Quantity_base.Mul(quantity).IsPositive() || salesLine.Quantity_base.IsPositive() {
			return apperrors.ReservWrongSign(gtables.SalesLineTableName)
		}
	default:
		if salesLine.Quantity_base.Mul(quantity).IsNegative() || salesLine.Quantity_base.IsNegative() {
			return apperrors.ReservWrongSign(gtables.SalesLineTableName)
		}
	}
	err = salesLine.Validate("reserved_qty_base", salesLine.Reserved_qty_base.Add(quantity))
	if err != nil {
		return err
	}
	return m.createReservEntry(SalesLineSource(salesLine), from, reservEntryValues{
		itemNo: salesLine.No, variantCode: salesLine.Variant_code, locationCode: salesLine.Location_code,
		binCode: salesLine.Bin_code, serialNo: salesLine.Serial_no, lotNo: salesLine.Lot_no,
		validToDate: validToDate, description: description,
		expectedReceiptDate: expectedReceiptDate, shipmentDate: salesLine.Shipment_date, quantity: quantity})
}

// SalesLineReservQuantity (SalesLineReservQuantity): what a sales line can reserve — the
// outstanding quantity of an order (a credit memo's negative one); a line that brings the item
// back reserves nothing (an error when asked)
func SalesLineReservQuantity(salesLine *gtables.SalesLine, giveError bool) (types.Decimal, error) {
	qty := salesLine.Outstanding_qty_base
	if salesLine.Document_type == gtables.SalesLine_Document_type.CreditMemo {
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

// SalesLineSetReservFor (SalesLineSetReservFor): the sales line is the line reserved for —
// its document type must reserve (Reservation Setup) and it must be an item line
func (m *ReservationManagement) SalesLineSetReservFor(currentSalesLine *gtables.SalesLine) error {
	m.clearAll()
	reservSetup := m.getReservSetup()
	field := map[gtables.SalesLineDocument_type]string{
		gtables.SalesLine_Document_type.Quote:        "reserve_sales_quote",
		gtables.SalesLine_Document_type.Order:        "reserve_sales_order",
		gtables.SalesLine_Document_type.Invoice:      "reserve_sales_invoice",
		gtables.SalesLine_Document_type.CreditMemo:   "reserve_sales_credit_memo",
		gtables.SalesLine_Document_type.BlanketOrder: "reserve_blanket_sales_order",
	}[currentSalesLine.Document_type]
	err := reservSetup.TestField(field, true)
	if err == nil {
		err = currentSalesLine.TestField("type", gtables.SalesLine_Type.Item)
	}
	if err != nil {
		return err
	}
	m.forSalesLine = gtables.NewSalesLine(m.src)
	m.forSalesLine.Copy(currentSalesLine)
	m.setReservFor(TypeSales, currentSalesLine.No, currentSalesLine.Variant_code, currentSalesLine.Location_code, currentSalesLine.Bin_code, currentSalesLine.Serial_no, currentSalesLine.Lot_no,
		currentSalesLine.Shipment_date, SalesLineSource(currentSalesLine))
	return nil
}

// SalesLineUpdateValues (SalesLineUpdateValues): a sales line as a supply (Available - Sales
// Lines): what it can still give (a return or a negative order line) and what the line reserved
// for has reserved from it
func (m *ReservationManagement) SalesLineUpdateValues(salesLine *gtables.SalesLine) (types.Decimal, types.Decimal, error) {
	var qtyToReserve types.Decimal
	if salesLine.Document_type == gtables.SalesLine_Document_type.CreditMemo {
		if salesLine.Quantity.IsPositive() {
			qtyToReserve = salesLine.Outstanding_qty_base.Add(salesLine.Reserved_qty_base)
		}
	} else if salesLine.Quantity.IsNegative() {
		qtyToReserve = salesLine.Outstanding_qty_base.Neg().Add(salesLine.Reserved_qty_base)
	}
	qtyReservedThisLine, err := m.QtyReservedThisLine(SalesLineSource(salesLine), true)
	return qtyToReserve, qtyReservedThisLine, err
}

// SalesLineVerifyChange (SalesLineVerifyChange): a sales line with a reserved quantity keeps its
// item, location and variant and a quantity of the same sign; its shipment date may not come
// before the supply it reserves from (or after the demand reserving from it) — a new date goes
// to its entries
func (m *ReservationManagement) SalesLineVerifyChange(newSalesLine *gtables.SalesLine) error {
	return m.verifyChange(verifyLine{
		table: gtables.SalesLineTableName, src: SalesLineSource(newSalesLine),
		reservedQtyBase: newSalesLine.Reserved_qty_base, qtyBase: newSalesLine.Quantity_base,
		validSignNegative: newSalesLine.Document_type == gtables.SalesLine_Document_type.CreditMemo,
		itemNo:            newSalesLine.No, locationCode: newSalesLine.Location_code, variantCode: newSalesLine.Variant_code,
		binCode: newSalesLine.Bin_code, serialNo: newSalesLine.Serial_no, lotNo: newSalesLine.Lot_no, binField: "bin_code", serialField: "serial_no", lotField: "lot_no",
		itemField: "no", locationField: "location_code",
		date: newSalesLine.Shipment_date, dateField: "shipment_date"})
}
