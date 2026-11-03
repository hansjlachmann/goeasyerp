// Package reservmgt is codeunit 400 Reservation Management: a reservation entry (table 337)
// ties a quantity of an item from a supply — the inventory (an item ledger entry), a purchase,
// sales, requisition, item or BOM journal line that brings the item — to a demand that takes it
// (For: a sales, purchase, requisition, item or BOM journal line). The lines keep the
// quantity reserved in Reserved Qty. (Base) — positive on the demand (For) side, negative on the
// supply (From) side; an item ledger entry's Reserved Quantity is negative while items are
// reserved from it. Reservations are made from the Reservation page (498: per source of supply
// what is there, what is reserved and what the line reserves from it; Auto Reserve by the
// setup's priorities) and the Available - … pages, followed when a line changes
// (…VerifyChange), carried over when a document or journal line is posted (Transfer…) and
// applied when the demand is posted from the inventory (codeunit 22).
//
// Not migrated: production orders, item tracking (Serial/Lot No.) and bins (the fields of the
// classic entry are left out), drop shipments and jobs (the lines have none).
package reservmgt

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The For/From Type of a reservation entry (classic option ' ,Sales,Requisition Line,Purchase,
// Item Journal,BOM Journal,Item Ledger Entry')
const (
	TypeSales           = 1
	TypeRequisitionLine = 2
	TypePurchase        = 3
	TypeItemJournal     = 4
	TypeBOMJournal      = 5
	TypeItemLedgerEntry = 6
)

// Source is the line one side of a reservation entry points at: its type, subtype (document or
// entry type), ID (document no. or journal template), batch, prod. order line and reference
// (line no. or entry no.)
type Source struct {
	Type          int
	Subtype       int
	ID            types.Code
	BatchName     types.Code
	ProdOrderLine int
	RefNo         int
}

// forSource / fromSource: an entry's For and From side
func forSource(e *gtables.ReservationEntry) Source {
	return Source{int(e.For_type), e.For_subtype, e.For_id, e.For_batch_name, e.For_prod_order_line, e.For_ref_no}
}

func fromSource(e *gtables.ReservationEntry) Source {
	return Source{int(e.From_type), e.From_subtype, e.From_id, e.From_batch_name, e.From_prod_order_line, e.From_ref_no}
}

// ReservationManagement is one use of codeunit 400: the line reserved for (Set…ReservFor) and
// its filters (CalcReservEntry: the line's item, variant, location and date; CalcReservEntry2:
// the line's open entries)
type ReservationManagement struct {
	src ftables.Source

	calcReservEntry  *gtables.ReservationEntry
	calcReservEntry2 *gtables.ReservationEntry
	calcReservEntry3 *gtables.ReservationEntry

	forSalesLine   *gtables.SalesLine
	forPurchLine   *gtables.PurchaseLine
	forItemJnlLine *gtables.ItemJournalLine
	forBOMJnlLine  *gtables.BOMJournalLine
	forReqLine     *gtables.RequisitionLine

	reservSetup          *gtables.ReservationSetup
	reservSetupRetrieved bool
}

// New is codeunit 400 for a database and company
func New(src ftables.Source) *ReservationManagement {
	m := &ReservationManagement{src: src}
	m.clearAll()
	return m
}

// clearAll (CLEARALL): no line reserved for yet
func (m *ReservationManagement) clearAll() {
	m.calcReservEntry = gtables.NewReservationEntry(m.src)
	m.calcReservEntry2 = gtables.NewReservationEntry(m.src)
	m.calcReservEntry3 = gtables.NewReservationEntry(m.src)
	m.forSalesLine, m.forPurchLine, m.forItemJnlLine, m.forBOMJnlLine, m.forReqLine = nil, nil, nil, nil, nil
	m.reservSetupRetrieved = false
}

// getReservSetup (GetReservSetup)
func (m *ReservationManagement) getReservSetup() *gtables.ReservationSetup {
	if !m.reservSetupRetrieved {
		m.reservSetupRetrieved = true
		m.reservSetup = gtables.GetReservationSetup(m.src)
	}
	return m.reservSetup
}

// reservEntryValues are the values of a new reservation entry (CreateReservEntry's parameters)
type reservEntryValues struct {
	itemNo, variantCode, locationCode types.Code
	serialNo, lotNo                   types.Code
	validToDate                       types.Date
	description                       types.Text
	expectedReceiptDate, shipmentDate types.Date
	quantity                          types.Decimal
	transferredFromEntryNo            int
}

// createReservEntry (CreateReservEntry with CreateReservEntryFor/From): a new open entry of a
// quantity reserved for one line from another
func (m *ReservationManagement) createReservEntry(forSrc, fromSrc Source, v reservEntryValues) error {
	if v.quantity.IsNegative() {
		return apperrors.ReservQtyNotPositive()
	}
	if v.validToDate.IsZero() {
		return apperrors.ReservValidToDateMissing()
	}
	reservEntry := gtables.NewReservationEntry(m.src)
	reservEntry.LockTable()
	nextEntryNo := 1
	if reservEntry.FindLast() {
		nextEntryNo = reservEntry.Entry_no + 1
	}
	if forSrc.Type == 0 {
		return apperrors.FieldRequired(gtables.ReservationEntryTableName, "for_type")
	}
	if fromSrc.Type == 0 {
		return apperrors.FieldRequired(gtables.ReservationEntryTableName, "from_type")
	}
	reservEntry.Init()
	reservEntry.Entry_no = nextEntryNo
	reservEntry.For_type = gtables.ReservationEntryFor_type(forSrc.Type)
	reservEntry.For_subtype = forSrc.Subtype
	reservEntry.For_id = forSrc.ID
	reservEntry.For_batch_name = forSrc.BatchName
	reservEntry.For_prod_order_line = forSrc.ProdOrderLine
	reservEntry.For_ref_no = forSrc.RefNo
	reservEntry.From_type = gtables.ReservationEntryFrom_type(fromSrc.Type)
	reservEntry.From_subtype = fromSrc.Subtype
	reservEntry.From_id = fromSrc.ID
	reservEntry.From_batch_name = fromSrc.BatchName
	reservEntry.From_prod_order_line = fromSrc.ProdOrderLine
	reservEntry.From_ref_no = fromSrc.RefNo
	reservEntry.Item_no = v.itemNo
	reservEntry.Variant_code = v.variantCode
	reservEntry.Location_code = v.locationCode
	reservEntry.Serial_no = v.serialNo
	reservEntry.Lot_no = v.lotNo
	reservEntry.Quantity = v.quantity
	reservEntry.Valid_to_date = v.validToDate
	reservEntry.Description = v.description
	reservEntry.Creation_date = system.WorkDate()
	reservEntry.Created_by = types.Code(system.UserId())
	reservEntry.Expected_receipt_date = v.expectedReceiptDate
	reservEntry.Shipment_date = v.shipmentDate
	reservEntry.Reservation_status = gtables.ReservationEntry_Reservation_status.Open
	reservEntry.Transferred_from_entry_no = v.transferredFromEntryNo
	return reservEntry.InsertOrError(false)
}

// entryValues: an entry's values for a copy of it
func entryValues(e *gtables.ReservationEntry) reservEntryValues {
	return reservEntryValues{itemNo: e.Item_no, variantCode: e.Variant_code, locationCode: e.Location_code,
		serialNo: e.Serial_no, lotNo: e.Lot_no,
		validToDate: e.Valid_to_date, description: e.Description,
		expectedReceiptDate: e.Expected_receipt_date, shipmentDate: e.Shipment_date,
		quantity: e.Quantity, transferredFromEntryNo: e.Entry_no}
}

// createRemainingReservEntry (CreateRemainingReservEntry): the rest of an entry partly
// transferred, as a new entry between the same lines
func (m *ReservationManagement) createRemainingReservEntry(oldReservEntry *gtables.ReservationEntry, remainingQuantity types.Decimal) error {
	v := entryValues(oldReservEntry)
	v.quantity = remainingQuantity
	return m.createReservEntry(forSource(oldReservEntry), fromSource(oldReservEntry), v)
}

// CloseReservEntry (CloseReservEntry): an open entry ends with a new status — its lines' reserved
// quantities follow when asked; with Track Changes the entry stays with the status (who and
// when), else it is deleted
func (m *ReservationManagement) CloseReservEntry(entryNo int, newState gtables.ReservationEntryReservation_status, modifyReservedFor, modifyReservedFrom bool) error {
	reservSetup := m.getReservSetup()
	reservEntry := gtables.NewReservationEntry(m.src)
	err := reservEntry.GetOrError(entryNo)
	if err != nil {
		return err
	}
	err = reservEntry.TestField("reservation_status", gtables.ReservationEntry_Reservation_status.Open)
	if err != nil {
		return err
	}
	if newState != gtables.ReservationEntry_Reservation_status.Open && modifyReservedFor {
		err = m.modifyFor(reservEntry, reservEntry.Quantity)
		if err != nil {
			return err
		}
	}
	if newState != gtables.ReservationEntry_Reservation_status.Open && modifyReservedFrom {
		err = m.modifyFrom(reservEntry, reservEntry.Quantity)
		if err != nil {
			return err
		}
	}
	if reservSetup.Track_changes {
		reservEntry.Cancelation_date = system.WorkDate()
		reservEntry.Canceled_or_changed_by = types.Code(system.UserId())
		reservEntry.Reservation_status = newState
		return reservEntry.ModifyOrError(false)
	}
	return reservEntry.DeleteOrError(false)
}

// ModifyReservEntry (ModifyReservEntry): an open entry's quantity reduced (not to 0: cancel it),
// its valid-to date and description changed — with Track Changes as a new entry, the old one
// Canceled or Changed
func (m *ReservationManagement) ModifyReservEntry(entryNo int, newQuantity types.Decimal, newValidToDate types.Date, newDescription types.Text, modifyReservedFor, modifyReservedFrom bool) error {
	reservSetup := m.getReservSetup()
	reservEntry := gtables.NewReservationEntry(m.src)
	err := reservEntry.GetOrError(entryNo)
	if err != nil {
		return err
	}
	err = reservEntry.TestField("reservation_status", gtables.ReservationEntry_Reservation_status.Open)
	if err != nil {
		return err
	}
	if newQuantity.IsNegative() {
		return apperrors.ReservQtyGreaterZero(gtables.ReservationEntryTableName, "quantity")
	}
	if newQuantity.IsZero() {
		return apperrors.ReservUseCancel()
	}
	if newQuantity.GreaterThan(reservEntry.Quantity) {
		return apperrors.ReservQtyOnlyReduced(gtables.ReservationEntryTableName, "quantity")
	}
	if modifyReservedFor {
		err = m.modifyFor(reservEntry, reservEntry.Quantity.Sub(newQuantity))
		if err != nil {
			return err
		}
	}
	if modifyReservedFrom {
		err = m.modifyFrom(reservEntry, reservEntry.Quantity.Sub(newQuantity))
		if err != nil {
			return err
		}
	}
	if reservSetup.Track_changes {
		v := entryValues(reservEntry)
		v.validToDate, v.description, v.quantity = newValidToDate, newDescription, newQuantity
		err = m.createReservEntry(forSource(reservEntry), fromSource(reservEntry), v)
		if err != nil {
			return err
		}
		reservEntry.Cancelation_date = system.WorkDate()
		reservEntry.Canceled_or_changed_by = types.Code(system.UserId())
		reservEntry.Reservation_status = gtables.ReservationEntry_Reservation_status.CanceledOrChanged
		return reservEntry.ModifyOrError(false)
	}
	reservEntry.Quantity = newQuantity
	reservEntry.Valid_to_date = newValidToDate
	reservEntry.Description = newDescription
	reservEntry.Canceled_or_changed_by = types.Code(system.UserId())
	return reservEntry.ModifyOrError(false)
}

// modifyFor (ModifyFor): the demand line reserves changeQuantity less
func (m *ReservationManagement) modifyFor(reservEntry *gtables.ReservationEntry, changeQuantity types.Decimal) error {
	if changeQuantity.IsZero() {
		return nil
	}
	return m.changeReservedQty(forSource(reservEntry), changeQuantity.Neg(), true)
}

// modifyFrom (ModifyFrom): the supply line has changeQuantity less reserved from it (its
// negative reserved quantity rises)
func (m *ReservationManagement) modifyFrom(reservEntry *gtables.ReservationEntry, changeQuantity types.Decimal) error {
	if changeQuantity.IsZero() {
		return nil
	}
	return m.changeReservedQty(fromSource(reservEntry), changeQuantity, false)
}

// changeReservedQty adds change to the Reserved Qty. (Base) of the line of a source (an item
// journal line: Positive for the For side, Negative for the From side; an item ledger entry:
// its Reserved Quantity)
func (m *ReservationManagement) changeReservedQty(s Source, change types.Decimal, forSide bool) error {
	switch s.Type {
	case TypeSales:
		salesLine := gtables.NewSalesLine(m.src)
		err := salesLine.GetOrError(map[string]interface{}{"document_type": s.Subtype, "document_no": s.ID, "line_no": s.RefNo})
		if err != nil {
			return err
		}
		err = salesLine.Validate("reserved_qty_base", salesLine.Reserved_qty_base.Add(change))
		if err != nil {
			return err
		}
		return salesLine.ModifyOrError(false)
	case TypeRequisitionLine:
		reqLine := gtables.NewRequisitionLine(m.src)
		err := reqLine.GetOrError(map[string]interface{}{"worksheet_template_name": s.ID, "journal_batch_name": s.BatchName, "line_no": s.RefNo})
		if err != nil {
			return err
		}
		err = reqLine.Validate("reserved_qty_base", reqLine.Reserved_qty_base.Add(change))
		if err != nil {
			return err
		}
		return reqLine.ModifyOrError(false)
	case TypePurchase:
		purchLine := gtables.NewPurchaseLine(m.src)
		err := purchLine.GetOrError(map[string]interface{}{"document_type": s.Subtype, "document_no": s.ID, "line_no": s.RefNo})
		if err != nil {
			return err
		}
		err = purchLine.Validate("reserved_qty_base", purchLine.Reserved_qty_base.Add(change))
		if err != nil {
			return err
		}
		return purchLine.ModifyOrError(false)
	case TypeItemJournal:
		itemJnlLine := gtables.NewItemJournalLine(m.src)
		err := itemJnlLine.GetOrError(map[string]interface{}{"journal_template_name": s.ID, "journal_batch_name": s.BatchName, "line_no": s.RefNo})
		if err != nil {
			return err
		}
		if forSide {
			err = itemJnlLine.Validate("positive_reserved_qty_base", itemJnlLine.Positive_reserved_qty_base.Add(change))
		} else {
			err = itemJnlLine.Validate("negative_reserved_qty_base", itemJnlLine.Negative_reserved_qty_base.Add(change))
		}
		if err != nil {
			return err
		}
		return itemJnlLine.ModifyOrError(false)
	case TypeBOMJournal:
		bomJnlLine := gtables.NewBOMJournalLine(m.src)
		err := bomJnlLine.GetOrError(map[string]interface{}{"journal_template_name": s.ID, "journal_batch_name": s.BatchName, "line_no": s.RefNo})
		if err != nil {
			return err
		}
		err = bomJnlLine.Validate("reserved_qty_base", bomJnlLine.Reserved_qty_base.Add(change))
		if err != nil {
			return err
		}
		return bomJnlLine.ModifyOrError(false)
	case TypeItemLedgerEntry:
		if forSide {
			return nil
		}
		itemLedgEntry := gtables.NewItemLedgerEntry(m.src)
		err := itemLedgEntry.GetOrError(s.RefNo)
		if err != nil {
			return err
		}
		itemLedgEntry.Reserved_quantity = itemLedgEntry.Reserved_quantity.Add(change)
		return itemLedgEntry.ModifyOrError(false)
	}
	return nil
}

// modifyShipmentDate (ModifyShipmentDate): the demand's entries (the given one and those before
// it) get a new shipment date — with Track Changes as new entries
func (m *ReservationManagement) modifyShipmentDate(reservEntry *gtables.ReservationEntry, newShipmentDate types.Date) error {
	reservSetup := m.getReservSetup()
	for {
		reservEntry2 := gtables.NewReservationEntry(m.src)
		reservEntry2.Copy(reservEntry)
		if reservSetup.Track_changes {
			v := entryValues(reservEntry)
			v.shipmentDate = newShipmentDate
			err := m.createReservEntry(forSource(reservEntry), fromSource(reservEntry), v)
			if err != nil {
				return err
			}
			reservEntry2.Cancelation_date = system.WorkDate()
			reservEntry2.Canceled_or_changed_by = types.Code(system.UserId())
			reservEntry2.Reservation_status = gtables.ReservationEntry_Reservation_status.CanceledOrChanged
		} else {
			reservEntry2.Shipment_date = newShipmentDate
			reservEntry2.Canceled_or_changed_by = types.Code(system.UserId())
		}
		err := reservEntry2.ModifyOrError(false)
		if err != nil {
			return err
		}
		if !reservEntry.Next(-1) {
			return nil
		}
	}
}

// modifyExpectedReceiptDate (ModifyExpectedReceiptDate): the supply's entries (the given one
// and those after it) get a new expected receipt date — with Track Changes as new entries
// (only up to the last entry before them)
func (m *ReservationManagement) modifyExpectedReceiptDate(reservEntry *gtables.ReservationEntry, newExpectedReceiptDate types.Date) error {
	reservSetup := m.getReservSetup()
	if reservSetup.Track_changes {
		reservEntry2 := gtables.NewReservationEntry(m.src)
		reservEntry2.CopyFilters(reservEntry)
		if reservEntry2.FindLast() {
			reservEntry.SetRange("entry_no", 0, reservEntry2.Entry_no)
		}
	}
	for {
		reservEntry2 := gtables.NewReservationEntry(m.src)
		reservEntry2.Copy(reservEntry)
		if reservSetup.Track_changes {
			v := entryValues(reservEntry)
			v.expectedReceiptDate = newExpectedReceiptDate
			err := m.createReservEntry(forSource(reservEntry), fromSource(reservEntry), v)
			if err != nil {
				return err
			}
			reservEntry2.Cancelation_date = system.WorkDate()
			reservEntry2.Canceled_or_changed_by = types.Code(system.UserId())
			reservEntry2.Reservation_status = gtables.ReservationEntry_Reservation_status.CanceledOrChanged
		} else {
			reservEntry2.Expected_receipt_date = newExpectedReceiptDate
			reservEntry2.Canceled_or_changed_by = types.Code(system.UserId())
		}
		err := reservEntry2.ModifyOrError(false)
		if err != nil {
			return err
		}
		if !reservEntry.Next() {
			return nil
		}
	}
}

// InitFilterAndSortingFor (InitFilterAndSortingFor): the open entries of an item, variant and
// location (the record's own values) in the order of their demands
func InitFilterAndSortingFor(filterReservEntry *gtables.ReservationEntry, setFilters bool) {
	itemNo, variantCode, locationCode := filterReservEntry.Item_no, filterReservEntry.Variant_code, filterReservEntry.Location_code
	filterReservEntry.Reset()
	filterReservEntry.SetCurrentKey("reservation_status", "item_no", "variant_code", "location_code",
		"expected_receipt_date", "for_type", "for_subtype", "for_id", "for_batch_name", "for_prod_order_line", "for_ref_no",
		"from_type", "from_subtype")
	if setFilters {
		filterReservEntry.SetRange("reservation_status", gtables.ReservationEntry_Reservation_status.Open)
		filterReservEntry.SetRange("item_no", itemNo)
		filterReservEntry.SetRange("variant_code", variantCode)
		filterReservEntry.SetRange("location_code", locationCode)
	}
}

// InitFilterAndSortingLookupFor (InitFilterAndSortingLookupFor): the open entries in the order
// of their demands
func InitFilterAndSortingLookupFor(filterReservEntry *gtables.ReservationEntry, setFilters bool) {
	filterReservEntry.Reset()
	filterReservEntry.SetCurrentKey("reservation_status", "for_type", "for_subtype", "for_id", "for_batch_name",
		"for_prod_order_line", "for_ref_no", "expected_receipt_date")
	if setFilters {
		filterReservEntry.SetRange("reservation_status", gtables.ReservationEntry_Reservation_status.Open)
	}
}

// InitFilterAndSortingFrom (InitFilterAndSortingFrom): the open entries of an item, variant and
// location in the order of their supplies
func InitFilterAndSortingFrom(filterReservEntry *gtables.ReservationEntry, setFilters bool) {
	itemNo, variantCode, locationCode := filterReservEntry.Item_no, filterReservEntry.Variant_code, filterReservEntry.Location_code
	filterReservEntry.Reset()
	filterReservEntry.SetCurrentKey("reservation_status", "item_no", "variant_code", "location_code",
		"from_type", "from_subtype", "expected_receipt_date")
	if setFilters {
		filterReservEntry.SetRange("reservation_status", gtables.ReservationEntry_Reservation_status.Open)
		filterReservEntry.SetRange("item_no", itemNo)
		filterReservEntry.SetRange("variant_code", variantCode)
		filterReservEntry.SetRange("location_code", locationCode)
	}
}

// InitFilterAndSortingLookupFrom (InitFilterAndSortingLookupFrom): the open entries in the
// order of their supplies
func InitFilterAndSortingLookupFrom(filterReservEntry *gtables.ReservationEntry, setFilters bool) {
	filterReservEntry.Reset()
	filterReservEntry.SetCurrentKey("reservation_status", "from_type", "from_subtype", "from_id", "from_batch_name",
		"from_prod_order_line", "from_ref_no", "shipment_date")
	if setFilters {
		filterReservEntry.SetRange("reservation_status", gtables.ReservationEntry_Reservation_status.Open)
	}
}

// FilterFor / FilterFrom: the entries reserved for / from a source's line
func FilterFor(filterReservEntry *gtables.ReservationEntry, s Source) {
	filterReservEntry.SetRange("for_type", s.Type)
	filterReservEntry.SetRange("for_subtype", s.Subtype)
	filterReservEntry.SetRange("for_id", s.ID)
	filterReservEntry.SetRange("for_batch_name", s.BatchName)
	filterReservEntry.SetRange("for_prod_order_line", s.ProdOrderLine)
	filterReservEntry.SetRange("for_ref_no", s.RefNo)
}

func FilterFrom(filterReservEntry *gtables.ReservationEntry, s Source) {
	filterReservEntry.SetRange("from_type", s.Type)
	filterReservEntry.SetRange("from_subtype", s.Subtype)
	filterReservEntry.SetRange("from_id", s.ID)
	filterReservEntry.SetRange("from_batch_name", s.BatchName)
	filterReservEntry.SetRange("from_prod_order_line", s.ProdOrderLine)
	filterReservEntry.SetRange("from_ref_no", s.RefNo)
}

// validToDate: the setup's Valid-to Formula from a date (the date itself without a formula)
func (m *ReservationManagement) validToDate(date types.Date) (types.Date, error) {
	formula := m.getReservSetup().Valid_to_formula
	if formula == "" {
		return date, nil
	}
	return system.CalcDate(string(formula), date)
}
