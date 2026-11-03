package reservmgt

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// lineValues are what a line's reservation entries must still show when they are transferred:
// its item, variant, location and date (the supply's expected receipt date, the demand's
// shipment date)
type lineValues struct {
	itemNo, variantCode, locationCode types.Code
	date                              types.Date
}

// transferEntries moves a line's open reservations — as a supply (asSupply) or as a demand — up
// to limit (all with a negative limit) to another line (to): the other side stays, the rest of a
// partly moved entry stays with the old line, the old entry is closed Transferred (Track
// Changes) or deleted. It returns the quantity moved. The callers change the lines' reserved
// quantities (the classic Transfer… procedures).
func (m *ReservationManagement) transferEntries(line Source, asSupply bool, values lineValues, to Source, limit types.Decimal) (types.Decimal, error) {
	reservSetup := m.getReservSetup()
	oldReservEntry := gtables.NewReservationEntry(m.src)
	if asSupply {
		InitFilterAndSortingLookupFrom(oldReservEntry, true)
		FilterFrom(oldReservEntry, line)
	} else {
		InitFilterAndSortingLookupFor(oldReservEntry, true)
		FilterFor(oldReservEntry, line)
	}
	var entries []*gtables.ReservationEntry
	for found := oldReservEntry.FindSet(); found; found = oldReservEntry.Next() {
		e := gtables.NewReservationEntry(m.src)
		e.Copy(oldReservEntry)
		entries = append(entries, e)
	}
	all := limit.IsNegative()
	var transferred types.Decimal
	for _, e := range entries {
		if !all && limit.IsZero() {
			break
		}
		err := e.TestField("item_no", values.itemNo)
		if err == nil {
			err = e.TestField("variant_code", values.variantCode)
		}
		if err == nil {
			err = e.TestField("location_code", values.locationCode)
		}
		if err == nil && asSupply {
			err = e.TestField("expected_receipt_date", values.date)
		}
		if err == nil && !asSupply {
			err = e.TestField("shipment_date", values.date)
		}
		if err != nil {
			return transferred, err
		}
		transferQty := e.Quantity
		if !all && limit.LessThan(e.Quantity) {
			transferQty = limit
		}
		forSrc, fromSrc := forSource(e), fromSource(e)
		if asSupply {
			fromSrc = to
		} else {
			forSrc = to
		}
		v := entryValues(e)
		v.quantity = transferQty
		err = m.createReservEntry(forSrc, fromSrc, v)
		if err != nil {
			return transferred, err
		}
		if e.Quantity.GreaterThan(transferQty) {
			err = m.createRemainingReservEntry(e, e.Quantity.Sub(transferQty))
			if err != nil {
				return transferred, err
			}
		}
		if reservSetup.Track_changes {
			err = m.CloseReservEntry(e.Entry_no, gtables.ReservationEntry_Reservation_status.Transferred, false, false)
		} else {
			err = e.DeleteOrError(false)
		}
		if err != nil {
			return transferred, err
		}
		transferred = transferred.Add(transferQty)
		if !all {
			limit = limit.Sub(transferQty)
		}
	}
	return transferred, nil
}

// TransferSalesLineToItemJnlLine (TransferSalesLineToItemJnlLine): when a sales line is posted
// its reservations (up to the quantity posted) move to the item journal line posting it — as a
// demand (Positive) or, for a return, as a supply (Negative)
func (m *ReservationManagement) TransferSalesLineToItemJnlLine(salesLine *gtables.SalesLine, itemJnlLine *gtables.ItemJournalLine, quantity types.Decimal) error {
	if salesLine.Reserved_qty_base.IsZero() {
		return nil
	}
	err := checkSameLine(gtables.ItemJournalLineTableName, "item_no", itemJnlLine.Item_no, salesLine.No,
		itemJnlLine.Variant_code, salesLine.Variant_code, itemJnlLine.Location_code, salesLine.Location_code,
		itemJnlLine.Posting_date, salesLine.Shipment_date)
	if err != nil {
		return err
	}
	values := lineValues{salesLine.No, salesLine.Variant_code, salesLine.Location_code, salesLine.Shipment_date}
	asSupply := salesLine.Reserved_qty_base.IsNegative()
	transferred, err := m.transferEntries(SalesLineSource(salesLine), asSupply, values, ItemJnlLineSource(itemJnlLine), quantity.Abs())
	if err != nil {
		return err
	}
	return moveReserved(asSupply, transferred, itemJnlLine, func(change types.Decimal) error {
		return salesLine.Validate("reserved_qty_base", salesLine.Reserved_qty_base.Add(change))
	})
}

// TransferPurchLineToItemJnlLine (TransferPurchLineToItemJnlLine): when a purchase line is posted
// its reservations move to the item journal line — as a supply (Negative) the reservations from
// the receipt, as a demand (Positive) those of a return
func (m *ReservationManagement) TransferPurchLineToItemJnlLine(purchLine *gtables.PurchaseLine, itemJnlLine *gtables.ItemJournalLine, quantity types.Decimal) error {
	if purchLine.Reserved_qty_base.IsZero() {
		return nil
	}
	err := checkSameLine(gtables.ItemJournalLineTableName, "item_no", itemJnlLine.Item_no, purchLine.No,
		itemJnlLine.Variant_code, purchLine.Variant_code, itemJnlLine.Location_code, purchLine.Location_code,
		itemJnlLine.Posting_date, purchLine.Expected_receipt_date)
	if err != nil {
		return err
	}
	values := lineValues{purchLine.No, purchLine.Variant_code, purchLine.Location_code, purchLine.Expected_receipt_date}
	asSupply := purchLine.Reserved_qty_base.IsNegative()
	transferred, err := m.transferEntries(PurchLineSource(purchLine), asSupply, values, ItemJnlLineSource(itemJnlLine), quantity.Abs())
	if err != nil {
		return err
	}
	return moveReserved(asSupply, transferred, itemJnlLine, func(change types.Decimal) error {
		return purchLine.Validate("reserved_qty_base", purchLine.Reserved_qty_base.Add(change))
	})
}

// moveReserved: the reserved quantities after a transfer to an item journal line — the journal
// line reserves as the old line did, the old line the transferred quantity less
func moveReserved(asSupply bool, transferred types.Decimal, itemJnlLine *gtables.ItemJournalLine, changeOld func(types.Decimal) error) error {
	if transferred.IsZero() {
		return nil
	}
	var err error
	if asSupply {
		err = itemJnlLine.Validate("negative_reserved_qty_base", itemJnlLine.Negative_reserved_qty_base.Sub(transferred))
		if err == nil {
			err = changeOld(transferred)
		}
	} else {
		err = itemJnlLine.Validate("positive_reserved_qty_base", itemJnlLine.Positive_reserved_qty_base.Add(transferred))
		if err == nil {
			err = changeOld(transferred.Neg())
		}
	}
	return err
}

// checkSameLine: the line a reservation moves to has the old line's item, variant, location
// and date
func checkSameLine(table, itemField string, itemNo, oldItemNo, variantCode, oldVariantCode, locationCode, oldLocationCode types.Code, date, oldDate types.Date) error {
	if itemNo != oldItemNo {
		return apperrors.FieldMustBe(table, itemField, oldItemNo.String())
	}
	if variantCode != oldVariantCode {
		return apperrors.FieldMustBe(table, "variant_code", oldVariantCode.String())
	}
	if locationCode != oldLocationCode {
		return apperrors.FieldMustBe(table, "location_code", oldLocationCode.String())
	}
	if !date.Equal(oldDate) {
		return apperrors.FieldMustBeValue(table, "posting_date", oldDate)
	}
	return nil
}

// TransferItemJnlToItemLedgEntry (TransferItemJnlToItemLedgEntry): the reservations from an
// item journal line (its Negative reserved quantity: the demands it was to supply) move to the
// item ledger entry it posted — the inventory now supplies them
func (m *ReservationManagement) TransferItemJnlToItemLedgEntry(itemJnlLine *gtables.ItemJournalLine, itemLedgEntry *gtables.ItemLedgerEntry) error {
	if itemJnlLine.Negative_reserved_qty_base.IsZero() {
		return nil
	}
	locationCode, binCode, serialNo, lotNo := itemJnlLine.Location_code, itemJnlLine.Bin_code, itemJnlLine.Serial_no, itemJnlLine.Lot_no
	if itemJnlLine.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer {
		locationCode, binCode, serialNo, lotNo = itemJnlLine.New_location_code, itemJnlLine.New_bin_code, itemJnlLine.New_serial_no, itemJnlLine.New_lot_no
	}
	err := itemLedgEntry.TestField("item_no", itemJnlLine.Item_no)
	if err == nil {
		err = itemLedgEntry.TestField("variant_code", itemJnlLine.Variant_code)
	}
	if err == nil {
		err = itemLedgEntry.TestField("location_code", locationCode)
	}
	if err == nil {
		err = itemLedgEntry.TestField("bin_code", binCode)
	}
	if err == nil {
		err = itemLedgEntry.TestField("serial_no", serialNo)
	}
	if err == nil {
		err = itemLedgEntry.TestField("lot_no", lotNo)
	}
	if err == nil {
		err = itemLedgEntry.TestField("posting_date", itemJnlLine.Posting_date)
	}
	if err != nil {
		return err
	}
	if itemJnlLine.Negative_reserved_qty_base.IsPositive() {
		return apperrors.ReservOnlyPositive()
	}
	reservSetup := m.getReservSetup()
	oldReservEntry := gtables.NewReservationEntry(m.src)
	InitFilterAndSortingLookupFrom(oldReservEntry, true)
	FilterFrom(oldReservEntry, ItemJnlLineSource(itemJnlLine))
	var entries []*gtables.ReservationEntry
	for found := oldReservEntry.FindSet(); found; found = oldReservEntry.Next() {
		e := gtables.NewReservationEntry(m.src)
		e.Copy(oldReservEntry)
		entries = append(entries, e)
	}
	for _, e := range entries {
		err = e.TestField("item_no", itemJnlLine.Item_no)
		if err == nil {
			err = e.TestField("variant_code", itemJnlLine.Variant_code)
		}
		if err == nil {
			err = e.TestField("location_code", locationCode)
		}
		if err == nil {
			err = e.TestField("bin_code", binCode)
		}
		if err == nil {
			err = e.TestField("serial_no", serialNo)
		}
		if err == nil {
			err = e.TestField("lot_no", lotNo)
		}
		if err == nil {
			err = e.TestField("expected_receipt_date", itemJnlLine.Posting_date)
		}
		if err != nil {
			return err
		}
		v := entryValues(e)
		v.expectedReceiptDate = types.Date{}
		err = m.createReservEntry(forSource(e), ItemLedgEntrySource(itemLedgEntry), v)
		if err != nil {
			return err
		}
		itemLedgEntry.Reserved_quantity = itemLedgEntry.Reserved_quantity.Sub(e.Quantity)
		if reservSetup.Track_changes {
			err = m.CloseReservEntry(e.Entry_no, gtables.ReservationEntry_Reservation_status.Transferred, false, false)
		} else {
			err = e.DeleteOrError(false)
		}
		if err != nil {
			return err
		}
		err = itemJnlLine.Validate("negative_reserved_qty_base", itemJnlLine.Negative_reserved_qty_base.Add(e.Quantity))
		if err != nil {
			return err
		}
	}
	return itemJnlLine.TestField("negative_reserved_qty_base", types.Decimal{})
}

// TransferSaleLineToSalesLine (TransferSaleLineToSalesLine): a sales line's reservations (up to
// the quantity) move to another sales line — a quote becoming an order, a blanket order line an
// order line
func (m *ReservationManagement) TransferSaleLineToSalesLine(oldSalesLine, newSalesLine *gtables.SalesLine, quantity types.Decimal) error {
	if oldSalesLine.Reserved_qty_base.IsZero() {
		return nil
	}
	err := checkSameLine(gtables.SalesLineTableName, "no", newSalesLine.No, oldSalesLine.No,
		newSalesLine.Variant_code, oldSalesLine.Variant_code, newSalesLine.Location_code, oldSalesLine.Location_code,
		types.Date{}, types.Date{})
	if err == nil && !newSalesLine.Shipment_date.Equal(oldSalesLine.Shipment_date) {
		err = apperrors.FieldMustBeValue(gtables.SalesLineTableName, "shipment_date", oldSalesLine.Shipment_date)
	}
	if err != nil {
		return err
	}
	values := lineValues{oldSalesLine.No, oldSalesLine.Variant_code, oldSalesLine.Location_code, oldSalesLine.Shipment_date}
	asSupply := oldSalesLine.Reserved_qty_base.IsNegative()
	transferred, err := m.transferEntries(SalesLineSource(oldSalesLine), asSupply, values, SalesLineSource(newSalesLine), quantity.Abs())
	if err != nil || transferred.IsZero() {
		return err
	}
	if asSupply {
		err = newSalesLine.Validate("reserved_qty_base", newSalesLine.Reserved_qty_base.Sub(transferred))
		if err == nil {
			err = oldSalesLine.Validate("reserved_qty_base", oldSalesLine.Reserved_qty_base.Add(transferred))
		}
		return err
	}
	err = newSalesLine.Validate("reserved_qty_base", newSalesLine.Reserved_qty_base.Add(transferred))
	if err == nil {
		err = oldSalesLine.Validate("reserved_qty_base", oldSalesLine.Reserved_qty_base.Sub(transferred))
	}
	return err
}

// TransferPurchLineToPurchLine (TransferPurchLineToPurchLine): a purchase line's reservations
// (up to the quantity) move to another purchase line — a quote or blanket order becoming an
// order
func (m *ReservationManagement) TransferPurchLineToPurchLine(oldPurchLine, newPurchLine *gtables.PurchaseLine, quantity types.Decimal) error {
	if oldPurchLine.Reserved_qty_base.IsZero() {
		return nil
	}
	err := checkSameLine(gtables.PurchaseLineTableName, "no", newPurchLine.No, oldPurchLine.No,
		newPurchLine.Variant_code, oldPurchLine.Variant_code, newPurchLine.Location_code, oldPurchLine.Location_code,
		types.Date{}, types.Date{})
	if err == nil && !newPurchLine.Expected_receipt_date.Equal(oldPurchLine.Expected_receipt_date) {
		err = apperrors.FieldMustBeValue(gtables.PurchaseLineTableName, "expected_receipt_date", oldPurchLine.Expected_receipt_date)
	}
	if err != nil {
		return err
	}
	values := lineValues{oldPurchLine.No, oldPurchLine.Variant_code, oldPurchLine.Location_code, oldPurchLine.Expected_receipt_date}
	asSupply := oldPurchLine.Reserved_qty_base.IsNegative()
	transferred, err := m.transferEntries(PurchLineSource(oldPurchLine), asSupply, values, PurchLineSource(newPurchLine), quantity.Abs())
	if err != nil || transferred.IsZero() {
		return err
	}
	if asSupply {
		err = newPurchLine.Validate("reserved_qty_base", newPurchLine.Reserved_qty_base.Sub(transferred))
		if err == nil {
			err = oldPurchLine.Validate("reserved_qty_base", oldPurchLine.Reserved_qty_base.Add(transferred))
		}
		return err
	}
	err = newPurchLine.Validate("reserved_qty_base", newPurchLine.Reserved_qty_base.Add(transferred))
	if err == nil {
		err = oldPurchLine.Validate("reserved_qty_base", oldPurchLine.Reserved_qty_base.Sub(transferred))
	}
	return err
}

// TransferReqLineToPurchLine (TransferReqLineToPurchLine): Carry Out Action Message / Make
// Orders: the reservations from a requisition line move to the purchase order line made of it
func (m *ReservationManagement) TransferReqLineToPurchLine(reqLine *gtables.RequisitionLine, purchLine *gtables.PurchaseLine) error {
	if reqLine.Reserved_qty_base.IsZero() {
		return nil
	}
	err := checkSameLine(gtables.PurchaseLineTableName, "no", purchLine.No, reqLine.No,
		purchLine.Variant_code, reqLine.Variant_code, purchLine.Location_code, reqLine.Location_code,
		types.Date{}, types.Date{})
	if err == nil && !purchLine.Expected_receipt_date.Equal(reqLine.Expected_receipt_date) {
		err = apperrors.FieldMustBeValue(gtables.PurchaseLineTableName, "expected_receipt_date", reqLine.Expected_receipt_date)
	}
	if err != nil {
		return err
	}
	values := lineValues{reqLine.No, reqLine.Variant_code, reqLine.Location_code, reqLine.Expected_receipt_date}
	asSupply := reqLine.Reserved_qty_base.IsNegative()
	transferred, err := m.transferEntries(ReqLineSource(reqLine), asSupply, values, PurchLineSource(purchLine), types.NewDecimalFromInt(-1))
	if err != nil || transferred.IsZero() {
		return err
	}
	if asSupply {
		err = purchLine.Validate("reserved_qty_base", purchLine.Reserved_qty_base.Sub(transferred))
		if err == nil {
			err = reqLine.Validate("reserved_qty_base", reqLine.Reserved_qty_base.Add(transferred))
		}
		return err
	}
	err = purchLine.Validate("reserved_qty_base", purchLine.Reserved_qty_base.Add(transferred))
	if err == nil {
		err = reqLine.Validate("reserved_qty_base", reqLine.Reserved_qty_base.Sub(transferred))
	}
	return err
}

// TransfBOMJnlLineToItemJnlLine (TransfBOMJnlLineToItemJnlLine): when a BOM journal line is
// posted, the reservations of the assembled item move to the item journal line posting it
func (m *ReservationManagement) TransfBOMJnlLineToItemJnlLine(bomJnlLine *gtables.BOMJournalLine, itemJnlLine *gtables.ItemJournalLine) error {
	if bomJnlLine.Reserved_qty_base.IsZero() {
		return nil
	}
	err := checkSameLine(gtables.ItemJournalLineTableName, "item_no", itemJnlLine.Item_no, bomJnlLine.Item_no,
		itemJnlLine.Variant_code, bomJnlLine.Variant_code, itemJnlLine.Location_code, bomJnlLine.Location_code,
		itemJnlLine.Posting_date, bomJnlLine.Posting_date)
	if err != nil {
		return err
	}
	values := lineValues{bomJnlLine.Item_no, bomJnlLine.Variant_code, bomJnlLine.Location_code, bomJnlLine.Posting_date}
	asSupply := bomJnlLine.Reserved_qty_base.IsNegative()
	transferred, err := m.transferEntries(BOMJnlLineSource(bomJnlLine), asSupply, values, ItemJnlLineSource(itemJnlLine), types.NewDecimalFromInt(-1))
	if err != nil {
		return err
	}
	return moveReserved(asSupply, transferred, itemJnlLine, func(change types.Decimal) error {
		return bomJnlLine.Validate("reserved_qty_base", bomJnlLine.Reserved_qty_base.Add(change))
	})
}
