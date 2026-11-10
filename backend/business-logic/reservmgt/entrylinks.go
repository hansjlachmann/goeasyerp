package reservmgt

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ParamHistory: Reservation Entries with the closed entries too (Show History, "1") or only the
// open ones (Hide History, "0"); without it a line's entries are its open ones, the menu's all
const ParamHistory = "history"

// The line lists the Reservation Entries' Reserved For / Reserved From open (classic form 497
// LookupReservedFor / LookupReservedFrom)
const (
	salesLinesPageID       = 516
	requisitionLinesPageID = 517
	purchaseLinesPageID    = 518
	itemJournalLinesPageID = 519
	bomJournalLinesPageID  = 520
	itemLedgerEntriesPage  = 38
)

// The Reservation Entries' Reserved For / Reserved From and Show/Hide History, the item
// journals' Reserved-for / Reserved-from Reservation Entries, the Reservation page's Insert
// Requisition Line
func init() {
	functions.Register("table", gtables.ReservationEntryTableName, "LookupReservedFor", functions.For(func(rec *gtables.ReservationEntry) (fcodeunits.Result, error) {
		lookupReservedSource(rec, forSource(rec))
		return fcodeunits.Result{Success: true}, nil
	}), functions.WithDialog())
	functions.Register("table", gtables.ReservationEntryTableName, "LookupReservedFrom", functions.For(func(rec *gtables.ReservationEntry) (fcodeunits.Result, error) {
		lookupReservedSource(rec, fromSource(rec))
		return fcodeunits.Result{Success: true}, nil
	}), functions.WithDialog())
	functions.Register("table", gtables.ReservationEntryTableName, "ShowHistory", functions.For(func(rec *gtables.ReservationEntry) (fcodeunits.Result, error) {
		return reservationEntriesPage(true), nil
	}))
	functions.Register("table", gtables.ReservationEntryTableName, "HideHistory", functions.For(func(rec *gtables.ReservationEntry) (fcodeunits.Result, error) {
		return reservationEntriesPage(false), nil
	}))
	functions.Register("table", gtables.ItemJournalLineTableName, "ShowPositiveReservEntries", functions.For(func(rec *gtables.ItemJournalLine) (fcodeunits.Result, error) {
		params := Params(ItemJnlLineSource(rec))
		params[ParamSupply] = "1"
		return openPage(ReservationEntriesPageID, params), nil
	}))
	functions.Register("table", gtables.ItemJournalLineTableName, "ShowNegativeReservEntries", functions.For(func(rec *gtables.ItemJournalLine) (fcodeunits.Result, error) {
		return openPage(ReservationEntriesPageID, Params(ItemJnlLineSource(rec))), nil
	}))
	functions.Register("table", gtables.EntrySummaryTableName, "InsertReqLine", functions.For(insertReqLine))
}

// reservationEntriesPage: the Reservation Entries again with the same line, with or without the
// closed entries (classic SETRANGE("Reservation Status") / SETRANGE(…, Open))
func reservationEntriesPage(history bool) fcodeunits.Result {
	params := map[string]string{}
	if system.PageParam(ParamForType) != "" {
		params = Params(pageSource())
		if system.PageParam(ParamSupply) == "1" {
			params[ParamSupply] = "1"
		}
	}
	params[ParamHistory] = "0"
	if history {
		params[ParamHistory] = "1"
	}
	return fcodeunits.Result{Success: true, OpenPage: &fcodeunits.PageLink{PageID: ReservationEntriesPageID, Params: params}}
}

// lookupReservedSource opens the line an entry is reserved for or from on its line list
func lookupReservedSource(rec ftables.Source, s Source) {
	switch s.Type {
	case TypeSales:
		salesLine := gtables.NewSalesLine(rec)
		salesLine.SetRange("document_type", s.Subtype)
		salesLine.SetRange("document_no", s.ID)
		salesLine.SetRange("line_no", s.RefNo)
		system.RunModal(salesLinesPageID, salesLine, false)
	case TypeRequisitionLine:
		reqLine := gtables.NewRequisitionLine(rec)
		reqLine.SetRange("worksheet_template_name", s.ID)
		reqLine.SetRange("journal_batch_name", s.BatchName)
		reqLine.SetRange("line_no", s.RefNo)
		system.RunModal(requisitionLinesPageID, reqLine, false)
	case TypePurchase:
		purchLine := gtables.NewPurchaseLine(rec)
		purchLine.SetRange("document_type", s.Subtype)
		purchLine.SetRange("document_no", s.ID)
		purchLine.SetRange("line_no", s.RefNo)
		system.RunModal(purchaseLinesPageID, purchLine, false)
	case TypeItemJournal:
		itemJnlLine := gtables.NewItemJournalLine(rec)
		itemJnlLine.SetRange("journal_template_name", s.ID)
		itemJnlLine.SetRange("journal_batch_name", s.BatchName)
		itemJnlLine.SetRange("line_no", s.RefNo)
		itemJnlLine.SetRange("entry_type", s.Subtype)
		system.RunModal(itemJournalLinesPageID, itemJnlLine, false)
	case TypeBOMJournal:
		bomJnlLine := gtables.NewBOMJournalLine(rec)
		bomJnlLine.SetRange("journal_template_name", s.ID)
		bomJnlLine.SetRange("journal_batch_name", s.BatchName)
		bomJnlLine.SetRange("line_no", s.RefNo)
		system.RunModal(bomJournalLinesPageID, bomJnlLine, false)
	case TypeItemLedgerEntry:
		itemLedgEntry := gtables.NewItemLedgerEntry(rec)
		itemLedgEntry.SetRange("entry_no", s.RefNo)
		system.RunModal(itemLedgerEntriesPage, itemLedgEntry, false)
	}
}

// insertReqLine (the Reservation page's Insert Requisition Line): what the line still has to
// reserve becomes a line of the setup's requisition worksheet, reserved for the line
func insertReqLine(rec *gtables.EntrySummary) (fcodeunits.Result, error) {
	err := inTransaction(rec, func(src ftables.Source) error {
		m := New(src)
		f, err := m.SetReservForSource(pageSource())
		if err != nil {
			return err
		}
		summary := gtables.NewEntrySummary(src)
		err = summary.InitTemporary(src.GetDB(), src.GetCompany(), src.GetDBType())
		if err != nil {
			return err
		}
		remaining, err := remainingQty(m, f, summary)
		if err != nil {
			return err
		}
		if remaining.IsZero() {
			return apperrors.ReservFullyReserved()
		}
		return m.InsertReqLine(remaining, f.UnitOfMeasureCode, f.ValidToDate, "", f.ShipmentDate)
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// InsertReqLine (InsertReqLine): a new line of the Reservation Setup's Auto Req. Wksh. Template
// and Name for the item of the line reserved for — the remaining quantity in the unit of measure,
// expected on the availability date, at the line's variant, location, bin, serial and lot no. —
// reserved for the line
func (m *ReservationManagement) InsertReqLine(remainingQtyToReserve types.Decimal, unitOfMeasureCode types.Code, validToDate types.Date, description types.Text, availabilityDate types.Date) error {
	err := m.calcReservEntry.TestField("for_type")
	if err != nil {
		return err
	}

	if !remainingQtyToReserve.IsPositive() {
		return nil
	}

	reservSetup := gtables.GetReservationSetup(m.src)
	err = reservSetup.TestField("auto_req_wksh_template")
	if err == nil {
		err = reservSetup.TestField("auto_req_wksh_name")
	}
	if err == nil {
		err = reservSetup.TestField("show_requisition_line")
	}
	if err != nil {
		return err
	}

	newReqLine := gtables.NewRequisitionLine(m.src)
	newReqLine.SetRange("worksheet_template_name", reservSetup.Auto_req_wksh_template)
	newReqLine.SetRange("journal_batch_name", reservSetup.Auto_req_wksh_name)
	if newReqLine.Find("+") {
		newReqLine.Line_no = newReqLine.Line_no + 10000
	} else {
		newReqLine.Worksheet_template_name = reservSetup.Auto_req_wksh_template
		newReqLine.Journal_batch_name = reservSetup.Auto_req_wksh_name
		newReqLine.Line_no = 10000
	}

	newReqLine.Init()
	err = newReqLine.Validate("type", gtables.RequisitionLine_Type.Item)
	if err == nil {
		err = newReqLine.Validate("no", m.calcReservEntry.Item_no)
	}
	if err == nil && unitOfMeasureCode != "" {
		err = newReqLine.Validate("unit_of_measure_code", unitOfMeasureCode)
	}
	if err == nil {
		if !newReqLine.Qty_per_unit_of_measure.IsZero() && !newReqLine.Qty_per_unit_of_measure.Equal(types.NewDecimalFromInt(1)) {
			err = newReqLine.Validate("quantity", system.Round(remainingQtyToReserve.Div(newReqLine.Qty_per_unit_of_measure), types.MustDecimal("0.00001"), "="))
		} else {
			err = newReqLine.Validate("quantity", remainingQtyToReserve)
		}
	}
	if err == nil {
		err = newReqLine.Validate("expected_receipt_date", availabilityDate)
	}
	if err == nil {
		err = newReqLine.Validate("variant_code", m.calcReservEntry.Variant_code)
	}
	if err == nil {
		err = newReqLine.Validate("location_code", m.calcReservEntry.Location_code)
	}
	if err == nil && m.calcReservEntry.Bin_code != "" {
		err = newReqLine.Validate("bin_code", m.calcReservEntry.Bin_code)
	}
	if err == nil {
		err = newReqLine.Validate("serial_no", m.calcReservEntry.Serial_no)
	}
	if err == nil {
		err = newReqLine.Validate("lot_no", m.calcReservEntry.Lot_no)
	}
	if err != nil {
		return err
	}
	err = m.CreateReservation(validToDate, description, availabilityDate, newReqLine.Quantity_base,
		ReqLineSource(newReqLine), newReqLine.Variant_code, newReqLine.Location_code,
		newReqLine.Bin_code, newReqLine.Serial_no, newReqLine.Lot_no)
	if err != nil {
		return err
	}
	newReqLine.Reserved_qty_base = newReqLine.Reserved_qty_base.Sub(newReqLine.Quantity_base)
	return newReqLine.InsertOrError(false)
}
