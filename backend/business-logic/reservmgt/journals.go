package reservmgt

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ReqLineSource / ItemJnlLineSource / BOMJnlLineSource: a requisition, item journal or BOM
// journal line as one side of a reservation (an item journal line by its entry type)
func ReqLineSource(reqLine *gtables.RequisitionLine) Source {
	return Source{Type: TypeRequisitionLine, ID: reqLine.Worksheet_template_name, BatchName: reqLine.Journal_batch_name, RefNo: reqLine.Line_no}
}

func ItemJnlLineSource(itemJnlLine *gtables.ItemJournalLine) Source {
	return Source{Type: TypeItemJournal, Subtype: int(itemJnlLine.Entry_type), ID: itemJnlLine.Journal_template_name, BatchName: itemJnlLine.Journal_batch_name, RefNo: itemJnlLine.Line_no}
}

func BOMJnlLineSource(bomJnlLine *gtables.BOMJournalLine) Source {
	return Source{Type: TypeBOMJournal, ID: bomJnlLine.Journal_template_name, BatchName: bomJnlLine.Journal_batch_name, RefNo: bomJnlLine.Line_no}
}

// ItemLedgEntrySource: an item ledger entry as the supply of a reservation
func ItemLedgEntrySource(itemLedgEntry *gtables.ItemLedgerEntry) Source {
	return Source{Type: TypeItemLedgerEntry, RefNo: itemLedgEntry.Entry_no}
}

// checkCreateReservFor: the checks every …CreateReservFor makes — the line's date not before
// the supply's (but any inventory), no more than its quantity, the supply's location, variant,
// serial and lot number
func checkCreateReservFor(table, dateField string, date, expectedReceiptDate types.Date, from Source, qtyBase, reservedQtyBase, quantity types.Decimal,
	locationCode, fromLocationCode, variantCode, fromVariantCode, binCode, fromBinCode, serialNo, fromSerialNo, lotNo, fromLotNo types.Code) error {
	if date.Before(expectedReceiptDate) && from.Type != TypeItemLedgerEntry {
		return apperrors.ReservDateBeforeExpected(table, dateField)
	}
	if qtyBase.Abs().LessThan(reservedQtyBase.Abs().Add(quantity)) {
		return apperrors.ReservQtyTooHigh(qtyBase.Abs().Sub(reservedQtyBase.Abs()).String())
	}
	if locationCode != fromLocationCode {
		return apperrors.FieldMustBe(table, "location_code", fromLocationCode.String())
	}
	if variantCode != fromVariantCode {
		return apperrors.FieldMustBe(table, "variant_code", fromVariantCode.String())
	}
	if binCode != fromBinCode {
		return apperrors.FieldMustBe(table, "bin_code", fromBinCode.String())
	}
	if serialNo != fromSerialNo {
		return apperrors.FieldMustBe(table, "serial_no", fromSerialNo.String())
	}
	if lotNo != fromLotNo {
		return apperrors.FieldMustBe(table, "lot_no", fromLotNo.String())
	}
	return nil
}

// reqLineCreateReservFor (ReqLineCreateReservFor): a negative requisition line reserves from a
// supply
func (m *ReservationManagement) reqLineCreateReservFor(reqLine *gtables.RequisitionLine, validToDate types.Date, description types.Text, expectedReceiptDate types.Date, quantity types.Decimal, from Source, fromVariantCode, fromLocationCode, fromBinCode, fromSerialNo, fromLotNo types.Code) error {
	err := reqLine.TestField("type", gtables.RequisitionLine_Type.Item)
	if err == nil {
		err = reqLine.TestField("no")
	}
	if err == nil {
		err = reqLine.TestField("expected_receipt_date")
	}
	if err == nil {
		err = checkCreateReservFor(gtables.RequisitionLineTableName, "expected_receipt_date", reqLine.Expected_receipt_date, expectedReceiptDate, from,
			reqLine.Quantity_base, reqLine.Reserved_qty_base, quantity, reqLine.Location_code, fromLocationCode, reqLine.Variant_code, fromVariantCode, reqLine.Bin_code, fromBinCode, reqLine.Serial_no, fromSerialNo, reqLine.Lot_no, fromLotNo)
	}
	if err != nil {
		return err
	}
	if reqLine.Quantity_base.Mul(quantity).IsPositive() || reqLine.Quantity_base.IsPositive() {
		return apperrors.ReservWrongSign(gtables.RequisitionLineTableName)
	}
	err = reqLine.Validate("reserved_qty_base", reqLine.Reserved_qty_base.Add(quantity))
	if err != nil {
		return err
	}
	return m.createReservEntry(ReqLineSource(reqLine), from, reservEntryValues{
		itemNo: reqLine.No, variantCode: reqLine.Variant_code, locationCode: reqLine.Location_code,
		binCode: reqLine.Bin_code, serialNo: reqLine.Serial_no, lotNo: reqLine.Lot_no,
		validToDate: validToDate, description: description,
		expectedReceiptDate: expectedReceiptDate, shipmentDate: reqLine.Expected_receipt_date, quantity: quantity})
}

// ReqLineReservQuantity (ReqLineReservQuantity): a negative requisition line reserves its
// quantity
func ReqLineReservQuantity(reqLine *gtables.RequisitionLine, giveError bool) (types.Decimal, error) {
	if !reqLine.Quantity_base.IsPositive() {
		return reqLine.Quantity_base.Neg(), nil
	}
	if giveError {
		return types.Decimal{}, apperrors.ReservOnlyReduceInventory()
	}
	return types.Decimal{}, nil
}

// ReqLineSetReservFor (ReqLineSetReservFor): the requisition line is the line reserved for
func (m *ReservationManagement) ReqLineSetReservFor(currentReqLine *gtables.RequisitionLine) error {
	m.clearAll()
	err := m.getReservSetup().TestField("reserve_requisition_line", true)
	if err == nil {
		err = currentReqLine.TestField("type", gtables.RequisitionLine_Type.Item)
	}
	if err != nil {
		return err
	}
	m.forReqLine = gtables.NewRequisitionLine(m.src)
	m.forReqLine.Copy(currentReqLine)
	m.setReservFor(TypeRequisitionLine, currentReqLine.No, currentReqLine.Variant_code, currentReqLine.Location_code, currentReqLine.Bin_code, currentReqLine.Serial_no, currentReqLine.Lot_no,
		currentReqLine.Expected_receipt_date, ReqLineSource(currentReqLine))
	return nil
}

// ReqLineUpdateValues (ReqLineUpdateValues): a requisition line as a supply
func (m *ReservationManagement) ReqLineUpdateValues(reqLine *gtables.RequisitionLine) (types.Decimal, types.Decimal, error) {
	var qtyToReserve types.Decimal
	if reqLine.Quantity.IsPositive() {
		qtyToReserve = reqLine.Quantity.Add(reqLine.Reserved_qty_base)
	}
	qtyReservedThisLine, err := m.QtyReservedThisLine(ReqLineSource(reqLine), true)
	return qtyToReserve, qtyReservedThisLine, err
}

// ReqLineVerifyChange (ReqLineVerifyChange)
func (m *ReservationManagement) ReqLineVerifyChange(newReqLine *gtables.RequisitionLine) error {
	return m.verifyChange(verifyLine{
		table: gtables.RequisitionLineTableName, src: ReqLineSource(newReqLine),
		reservedQtyBase: newReqLine.Reserved_qty_base, qtyBase: newReqLine.Quantity_base, validSignNegative: true,
		itemNo: newReqLine.No, locationCode: newReqLine.Location_code, variantCode: newReqLine.Variant_code,
		binCode: newReqLine.Bin_code, serialNo: newReqLine.Serial_no, lotNo: newReqLine.Lot_no, binField: "bin_code", serialField: "serial_no", lotField: "lot_no",
		itemField: "no", locationField: "location_code",
		date: newReqLine.Expected_receipt_date, dateField: "expected_receipt_date"})
}

// itemJnlLineTakes: the item journal line takes the item (sale, negative adjustment, transfer
// out — a positive quantity) rather than bringing it
func itemJnlLineTakes(entryType gtables.ItemJournalLineEntry_type) bool {
	switch entryType {
	case gtables.ItemJournalLine_Entry_type.Purchase, gtables.ItemJournalLine_Entry_type.PositiveAdjmt:
		return false
	}
	return true
}

// itemJnlLineCreateReservFor (ItemJnlLineCreateReservFor): an item journal line that takes the
// item reserves from a supply
func (m *ReservationManagement) itemJnlLineCreateReservFor(itemJnlLine *gtables.ItemJournalLine, validToDate types.Date, description types.Text, expectedReceiptDate types.Date, quantity types.Decimal, from Source, fromVariantCode, fromLocationCode, fromBinCode, fromSerialNo, fromLotNo types.Code) error {
	err := itemJnlLine.TestField("item_no")
	if err == nil {
		err = itemJnlLine.TestField("posting_date")
	}
	if err == nil {
		err = itemJnlLine.TestField("drop_shipment", false)
	}
	if err == nil {
		err = checkCreateReservFor(gtables.ItemJournalLineTableName, "posting_date", itemJnlLine.Posting_date, expectedReceiptDate, from,
			itemJnlLine.Quantity_base, itemJnlLine.Positive_reserved_qty_base, quantity, itemJnlLine.Location_code, fromLocationCode, itemJnlLine.Variant_code, fromVariantCode, itemJnlLine.Bin_code, fromBinCode, itemJnlLine.Serial_no, fromSerialNo, itemJnlLine.Lot_no, fromLotNo)
	}
	if err != nil {
		return err
	}
	if itemJnlLineTakes(itemJnlLine.Entry_type) {
		if itemJnlLine.Quantity_base.Mul(quantity).IsNegative() || itemJnlLine.Quantity_base.IsNegative() {
			return apperrors.ReservWrongSign(gtables.ItemJournalLineTableName)
		}
	} else if itemJnlLine.Quantity_base.Mul(quantity).IsPositive() || itemJnlLine.Quantity_base.IsPositive() {
		return apperrors.ReservWrongSign(gtables.ItemJournalLineTableName)
	}
	err = itemJnlLine.Validate("positive_reserved_qty_base", itemJnlLine.Positive_reserved_qty_base.Add(quantity))
	if err != nil {
		return err
	}
	return m.createReservEntry(ItemJnlLineSource(itemJnlLine), from, reservEntryValues{
		itemNo: itemJnlLine.Item_no, variantCode: itemJnlLine.Variant_code, locationCode: itemJnlLine.Location_code,
		binCode: itemJnlLine.Bin_code, serialNo: itemJnlLine.Serial_no, lotNo: itemJnlLine.Lot_no,
		validToDate: validToDate, description: description,
		expectedReceiptDate: expectedReceiptDate, shipmentDate: itemJnlLine.Posting_date, quantity: quantity})
}

// ItemJnlLineReservQuantity (ItemJnlLineReservQuantity): what an item journal line that takes
// the item reserves (its quantity; a purchase or positive adjustment only when negative)
func ItemJnlLineReservQuantity(itemJnlLine *gtables.ItemJournalLine, giveError bool) (types.Decimal, error) {
	if itemJnlLineTakes(itemJnlLine.Entry_type) {
		if itemJnlLine.Quantity_base.IsPositive() {
			return itemJnlLine.Quantity_base, nil
		}
	} else if !itemJnlLine.Quantity_base.IsPositive() {
		return itemJnlLine.Quantity_base.Neg(), nil
	}
	if giveError {
		return types.Decimal{}, apperrors.ReservOnlyReduceInventory()
	}
	return types.Decimal{}, nil
}

// ItemJnlLineSetReservFor (ItemJnlLineSetReservFor): the item journal line is the line reserved
// for — its entry type must reserve (Reservation Setup)
func (m *ReservationManagement) ItemJnlLineSetReservFor(currentItemJnlLine *gtables.ItemJournalLine) error {
	m.clearAll()
	field := map[gtables.ItemJournalLineEntry_type]string{
		gtables.ItemJournalLine_Entry_type.Purchase:      "reserve_item_jnl_line_purch",
		gtables.ItemJournalLine_Entry_type.Sale:          "reserve_item_jnl_line_sales",
		gtables.ItemJournalLine_Entry_type.PositiveAdjmt: "reserve_item_jnl_line_pos",
		gtables.ItemJournalLine_Entry_type.NegativeAdjmt: "reserve_item_jnl_line_neg",
		gtables.ItemJournalLine_Entry_type.Transfer:      "reserve_item_jnl_line_trans",
	}[currentItemJnlLine.Entry_type]
	err := m.getReservSetup().TestField(field, true)
	if err == nil {
		err = currentItemJnlLine.TestField("drop_shipment", false)
	}
	if err != nil {
		return err
	}
	m.forItemJnlLine = gtables.NewItemJournalLine(m.src)
	m.forItemJnlLine.Copy(currentItemJnlLine)
	m.setReservFor(TypeItemJournal, currentItemJnlLine.Item_no, currentItemJnlLine.Variant_code, currentItemJnlLine.Location_code, currentItemJnlLine.Bin_code, currentItemJnlLine.Serial_no, currentItemJnlLine.Lot_no,
		currentItemJnlLine.Posting_date, ItemJnlLineSource(currentItemJnlLine))
	return nil
}

// ItemJnlLineUpdateValues (ItemJnlLineUpdateValues): an item journal line as a supply
func (m *ReservationManagement) ItemJnlLineUpdateValues(itemJnlLine *gtables.ItemJournalLine) (types.Decimal, types.Decimal, error) {
	var qtyToReserve types.Decimal
	switch itemJnlLine.Entry_type {
	case gtables.ItemJournalLine_Entry_type.Purchase, gtables.ItemJournalLine_Entry_type.PositiveAdjmt, gtables.ItemJournalLine_Entry_type.Transfer:
		if itemJnlLine.Quantity.IsPositive() {
			qtyToReserve = itemJnlLine.Quantity.Add(itemJnlLine.Negative_reserved_qty_base)
		}
	default:
		if itemJnlLine.Quantity.IsNegative() {
			qtyToReserve = itemJnlLine.Quantity.Neg().Add(itemJnlLine.Negative_reserved_qty_base)
		}
	}
	qtyReservedThisLine, err := m.QtyReservedThisLine(ItemJnlLineSource(itemJnlLine), true)
	return qtyToReserve, qtyReservedThisLine, err
}

// ItemJnlLineVerifyChange (ItemJnlLineVerifyChange): the line's reservations as a demand
// (Positive) and as a supply (Negative) — a transfer's other side at its new location
func (m *ReservationManagement) ItemJnlLineVerifyChange(newItemJnlLine *gtables.ItemJournalLine) error {
	if newItemJnlLine.Positive_reserved_qty_base.IsZero() && newItemJnlLine.Negative_reserved_qty_base.IsZero() {
		return nil
	}
	if newItemJnlLine.Posting_date.IsZero() {
		return apperrors.ReservFieldMustBeFilled(gtables.ItemJournalLineTableName, "posting_date")
	}
	if newItemJnlLine.Drop_shipment {
		return apperrors.ReservFieldMustNotBeFilled(gtables.ItemJournalLineTableName, "drop_shipment")
	}
	transfer := newItemJnlLine.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer
	l := verifyLine{
		table: gtables.ItemJournalLineTableName, src: ItemJnlLineSource(newItemJnlLine), qtyBase: newItemJnlLine.Quantity_base,
		itemNo: newItemJnlLine.Item_no, variantCode: newItemJnlLine.Variant_code, itemField: "item_no",
		locationCode: newItemJnlLine.Location_code, locationField: "location_code",
		binCode: newItemJnlLine.Bin_code, serialNo: newItemJnlLine.Serial_no, lotNo: newItemJnlLine.Lot_no, binField: "bin_code", serialField: "serial_no", lotField: "lot_no",
		date: newItemJnlLine.Posting_date, dateField: "posting_date"}
	if !newItemJnlLine.Positive_reserved_qty_base.IsZero() {
		lp := l
		lp.reservedQtyBase = newItemJnlLine.Positive_reserved_qty_base
		lp.validSignNegative = !itemJnlLineTakes(newItemJnlLine.Entry_type)
		if transfer && newItemJnlLine.Quantity_base.IsNegative() {
			lp.locationCode, lp.locationField = newItemJnlLine.New_location_code, "new_location_code"
			lp.binCode, lp.binField = newItemJnlLine.New_bin_code, "new_bin_code"
			lp.serialNo, lp.serialField = newItemJnlLine.New_serial_no, "new_serial_no"
			lp.lotNo, lp.lotField = newItemJnlLine.New_lot_no, "new_lot_no"
		}
		err := m.verifyChange(lp)
		if err != nil {
			return err
		}
	}
	if !newItemJnlLine.Negative_reserved_qty_base.IsZero() {
		ln := l
		ln.reservedQtyBase = newItemJnlLine.Negative_reserved_qty_base
		switch newItemJnlLine.Entry_type {
		case gtables.ItemJournalLine_Entry_type.Purchase, gtables.ItemJournalLine_Entry_type.PositiveAdjmt, gtables.ItemJournalLine_Entry_type.Transfer:
			ln.validSignNegative = true
		}
		if transfer && newItemJnlLine.Quantity_base.IsPositive() {
			ln.locationCode, ln.locationField = newItemJnlLine.New_location_code, "new_location_code"
			ln.binCode, ln.binField = newItemJnlLine.New_bin_code, "new_bin_code"
			ln.serialNo, ln.serialField = newItemJnlLine.New_serial_no, "new_serial_no"
			ln.lotNo, ln.lotField = newItemJnlLine.New_lot_no, "new_lot_no"
		}
		return m.verifyChange(ln)
	}
	return nil
}

// bomJnlLineCreateReservFor (BOMJnlLineCreateReservFor): a negative BOM journal line reserves
// from a supply
func (m *ReservationManagement) bomJnlLineCreateReservFor(bomJnlLine *gtables.BOMJournalLine, validToDate types.Date, description types.Text, expectedReceiptDate types.Date, quantity types.Decimal, from Source, fromVariantCode, fromLocationCode, fromBinCode, fromSerialNo, fromLotNo types.Code) error {
	err := bomJnlLine.TestField("item_no")
	if err == nil {
		err = bomJnlLine.TestField("posting_date")
	}
	if err == nil {
		err = checkCreateReservFor(gtables.BOMJournalLineTableName, "posting_date", bomJnlLine.Posting_date, expectedReceiptDate, from,
			bomJnlLine.Quantity_base, bomJnlLine.Reserved_qty_base, quantity, bomJnlLine.Location_code, fromLocationCode, bomJnlLine.Variant_code, fromVariantCode, bomJnlLine.Bin_code, fromBinCode, "", fromSerialNo, bomJnlLine.Lot_no, fromLotNo)
	}
	if err != nil {
		return err
	}
	if bomJnlLine.Quantity_base.Mul(quantity).IsPositive() || bomJnlLine.Quantity_base.IsPositive() {
		return apperrors.ReservWrongSign(gtables.BOMJournalLineTableName)
	}
	err = bomJnlLine.Validate("reserved_qty_base", bomJnlLine.Reserved_qty_base.Add(quantity))
	if err != nil {
		return err
	}
	return m.createReservEntry(BOMJnlLineSource(bomJnlLine), from, reservEntryValues{
		itemNo: bomJnlLine.Item_no, variantCode: bomJnlLine.Variant_code, locationCode: bomJnlLine.Location_code,
		lotNo:       bomJnlLine.Lot_no,
		validToDate: validToDate, description: description,
		expectedReceiptDate: expectedReceiptDate, shipmentDate: bomJnlLine.Posting_date, quantity: quantity})
}

// BOMJnlLineReservQuantity (BOMJnlLineReservQuantity): a negative BOM journal line reserves its
// quantity
func BOMJnlLineReservQuantity(bomJnlLine *gtables.BOMJournalLine, giveError bool) (types.Decimal, error) {
	if !bomJnlLine.Quantity_base.IsPositive() {
		return bomJnlLine.Quantity_base.Neg(), nil
	}
	if giveError {
		return types.Decimal{}, apperrors.ReservOnlyReduceInventory()
	}
	return types.Decimal{}, nil
}

// BOMJnlLineSetReservFor (BOMJnlLineSetReservFor): the BOM journal line is the line reserved for
func (m *ReservationManagement) BOMJnlLineSetReservFor(currentBOMJnlLine *gtables.BOMJournalLine) error {
	m.clearAll()
	err := m.getReservSetup().TestField("reserve_bom_journal_line", true)
	if err != nil {
		return err
	}
	m.forBOMJnlLine = gtables.NewBOMJournalLine(m.src)
	m.forBOMJnlLine.Copy(currentBOMJnlLine)
	m.setReservFor(TypeBOMJournal, currentBOMJnlLine.Item_no, currentBOMJnlLine.Variant_code, currentBOMJnlLine.Location_code, currentBOMJnlLine.Bin_code, "", currentBOMJnlLine.Lot_no,
		currentBOMJnlLine.Posting_date, BOMJnlLineSource(currentBOMJnlLine))
	return nil
}

// BOMJnlLineUpdateValues (BOMJnlLineUpdateValues): a BOM journal line (an assembly) as a supply
func (m *ReservationManagement) BOMJnlLineUpdateValues(bomJnlLine *gtables.BOMJournalLine) (types.Decimal, types.Decimal, error) {
	var qtyToReserve types.Decimal
	if bomJnlLine.Quantity.IsPositive() {
		qtyToReserve = bomJnlLine.Quantity.Add(bomJnlLine.Reserved_qty_base)
	}
	qtyReservedThisLine, err := m.QtyReservedThisLine(BOMJnlLineSource(bomJnlLine), true)
	return qtyToReserve, qtyReservedThisLine, err
}

// BOMJnlLineVerifyChange (BOMJnlLineVerifyChange)
func (m *ReservationManagement) BOMJnlLineVerifyChange(newBOMJnlLine *gtables.BOMJournalLine) error {
	return m.verifyChange(verifyLine{
		table: gtables.BOMJournalLineTableName, src: BOMJnlLineSource(newBOMJnlLine),
		reservedQtyBase: newBOMJnlLine.Reserved_qty_base, qtyBase: newBOMJnlLine.Quantity_base, validSignNegative: true,
		itemNo: newBOMJnlLine.Item_no, locationCode: newBOMJnlLine.Location_code, variantCode: newBOMJnlLine.Variant_code,
		binCode: newBOMJnlLine.Bin_code, lotNo: newBOMJnlLine.Lot_no, binField: "bin_code", serialField: "serial_no", lotField: "lot_no",
		itemField: "item_no", locationField: "location_code",
		date: newBOMJnlLine.Posting_date, dateField: "posting_date"})
}

// ItemLedgEntryUpdateValues (ItemLedgEntryUpdateValues): an inventory entry as a supply (Available
// - Item Ledg. Entries): its remaining quantity not reserved yet and what the line reserved for
// has from it
func (m *ReservationManagement) ItemLedgEntryUpdateValues(itemLedgEntry *gtables.ItemLedgerEntry) (types.Decimal, types.Decimal, error) {
	var qtyToReserve types.Decimal
	if itemLedgEntry.Quantity.IsPositive() {
		qtyToReserve = itemLedgEntry.Remaining_quantity.Add(itemLedgEntry.Reserved_quantity)
	}
	qtyReservedThisLine, err := m.QtyReservedThisLine(ItemLedgEntrySource(itemLedgEntry), false)
	return qtyToReserve, qtyReservedThisLine, err
}
