package reservmgt

import (
	"database/sql"
	"strconv"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The pages of the reservations
const (
	ReservationEntriesPageID = 497
	ReservationPageID        = 498
)

// The page functions of the reservations: Reserve and Reservation Entries on the lines (classic
// ShowReservation / ShowReservationEntries), the Reservation page's Auto Reserve, Reserve from
// Current Line, Cancel Reservation from Current Line, Available to Reserve, Show/Hide Optional
// Lines, the Available - … pages' Reserve and Cancel Reservation, the Reservation Entries'
// Cancel Reservation
func init() {
	for _, f := range []struct {
		table  string
		source func(rec any) (Source, bool) // the line and whether it is a supply
	}{
		{gtables.SalesLineTableName, func(rec any) (Source, bool) {
			l := rec.(*gtables.SalesLine)
			signed := l.Quantity_base.Neg()
			if l.Document_type == gtables.SalesLine_Document_type.CreditMemo {
				signed = l.Quantity_base
			}
			return SalesLineSource(l), !signed.IsNegative()
		}},
		{gtables.PurchaseLineTableName, func(rec any) (Source, bool) {
			l := rec.(*gtables.PurchaseLine)
			signed := l.Quantity_base
			if l.Document_type == gtables.PurchaseLine_Document_type.CreditMemo {
				signed = l.Quantity_base.Neg()
			}
			return PurchLineSource(l), !signed.IsNegative()
		}},
		{gtables.ItemJournalLineTableName, func(rec any) (Source, bool) {
			l := rec.(*gtables.ItemJournalLine)
			signed := l.Quantity_base
			switch l.Entry_type {
			case gtables.ItemJournalLine_Entry_type.Sale, gtables.ItemJournalLine_Entry_type.NegativeAdjmt, gtables.ItemJournalLine_Entry_type.Transfer:
				signed = l.Quantity_base.Neg()
			}
			return ItemJnlLineSource(l), !signed.IsNegative()
		}},
		{gtables.BOMJournalLineTableName, func(rec any) (Source, bool) {
			l := rec.(*gtables.BOMJournalLine)
			return BOMJnlLineSource(l), !l.Quantity_base.IsNegative()
		}},
		{gtables.RequisitionLineTableName, func(rec any) (Source, bool) {
			l := rec.(*gtables.RequisitionLine)
			return ReqLineSource(l), !l.Quantity_base.IsNegative()
		}},
	} {
		f := f
		functions.Register("table", f.table, "ShowReservation", lineFunc(f.table, func(rec lineRecord) (fcodeunits.Result, error) {
			s, _ := f.source(rec)
			// TESTFIELD(Type,Type::Item); TESTFIELD("No."); TESTFIELD(Reserve)
			tester := rec.(interface {
				TestField(fieldName string, value ...any) error
			})
			var err error
			switch f.table {
			case gtables.SalesLineTableName:
				err = tester.TestField("type", gtables.SalesLine_Type.Item)
			case gtables.PurchaseLineTableName:
				err = tester.TestField("type", gtables.PurchaseLine_Type.Item)
			case gtables.RequisitionLineTableName:
				err = tester.TestField("type", gtables.RequisitionLine_Type.Item)
			}
			if err == nil {
				switch f.table {
				case gtables.ItemJournalLineTableName, gtables.BOMJournalLineTableName:
					err = tester.TestField("item_no")
				default:
					err = tester.TestField("no")
				}
			}
			if err == nil {
				err = tester.TestField("reserve")
			}
			if err != nil {
				return fcodeunits.Result{}, err
			}
			_, err = New(rec).SetReservForSource(s)
			if err != nil {
				return fcodeunits.Result{}, err
			}
			return openPage(ReservationPageID, Params(s)), nil
		}))
		functions.Register("table", f.table, "ShowReservationEntries", lineFunc(f.table, func(rec lineRecord) (fcodeunits.Result, error) {
			s, supply := f.source(rec)
			params := Params(s)
			if supply {
				params[ParamSupply] = "1"
			}
			return openPage(ReservationEntriesPageID, params), nil
		}))
	}

	functions.Register("table", gtables.EntrySummaryTableName, "AutoReserve", functions.For(autoReserve), functions.WithDialog())
	functions.Register("table", gtables.EntrySummaryTableName, "ReserveFromCurrentLine", functions.For(reserveFromCurrentLine))
	functions.Register("table", gtables.EntrySummaryTableName, "CancelReservation", functions.For(cancelSummaryReservation), functions.WithDialog())
	functions.Register("table", gtables.EntrySummaryTableName, "ShowAvailable", functions.For(showAvailable))
	functions.Register("table", gtables.EntrySummaryTableName, "ShowReservationEntries", functions.For(func(rec *gtables.EntrySummary) (fcodeunits.Result, error) {
		return openPage(ReservationEntriesPageID, Params(pageSource())), nil
	}))
	functions.Register("table", gtables.EntrySummaryTableName, "ShowOptionalLines", functions.For(func(rec *gtables.EntrySummary) (fcodeunits.Result, error) {
		return reservationPage(true), nil
	}))
	functions.Register("table", gtables.EntrySummaryTableName, "HideOptionalLines", functions.For(func(rec *gtables.EntrySummary) (fcodeunits.Result, error) {
		return reservationPage(false), nil
	}))

	functions.Register("table", gtables.SalesLineTableName, "ReserveAvailable", functions.For(func(rec *gtables.SalesLine) (fcodeunits.Result, error) {
		return reserveAvailable(rec, func(m *ReservationManagement) (types.Decimal, Source, types.Date, error) {
			qtyToReserve, _, err := m.SalesLineUpdateValues(rec)
			return qtyToReserve, SalesLineSource(rec), rec.Shipment_date, err
		}, rec.Variant_code, rec.Location_code, rec.Bin_code, rec.Serial_no, rec.Lot_no)
	}))
	functions.Register("table", gtables.PurchaseLineTableName, "ReserveAvailable", functions.For(func(rec *gtables.PurchaseLine) (fcodeunits.Result, error) {
		return reserveAvailable(rec, func(m *ReservationManagement) (types.Decimal, Source, types.Date, error) {
			qtyToReserve, _, err := m.PurchLineUpdateValues(rec)
			return qtyToReserve, PurchLineSource(rec), rec.Expected_receipt_date, err
		}, rec.Variant_code, rec.Location_code, rec.Bin_code, rec.Serial_no, rec.Lot_no)
	}))
	functions.Register("table", gtables.ItemJournalLineTableName, "ReserveAvailable", functions.For(func(rec *gtables.ItemJournalLine) (fcodeunits.Result, error) {
		locationCode, binCode, serialNo, lotNo := rec.Location_code, rec.Bin_code, rec.Serial_no, rec.Lot_no
		if rec.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer {
			locationCode, binCode, serialNo, lotNo = rec.New_location_code, rec.New_bin_code, rec.New_serial_no, rec.New_lot_no
		}
		return reserveAvailable(rec, func(m *ReservationManagement) (types.Decimal, Source, types.Date, error) {
			qtyToReserve, _, err := m.ItemJnlLineUpdateValues(rec)
			return qtyToReserve, ItemJnlLineSource(rec), rec.Posting_date, err
		}, rec.Variant_code, locationCode, binCode, serialNo, lotNo)
	}))
	functions.Register("table", gtables.BOMJournalLineTableName, "ReserveAvailable", functions.For(func(rec *gtables.BOMJournalLine) (fcodeunits.Result, error) {
		return reserveAvailable(rec, func(m *ReservationManagement) (types.Decimal, Source, types.Date, error) {
			qtyToReserve, _, err := m.BOMJnlLineUpdateValues(rec)
			return qtyToReserve, BOMJnlLineSource(rec), rec.Posting_date, err
		}, rec.Variant_code, rec.Location_code, rec.Bin_code, "", rec.Lot_no)
	}))
	functions.Register("table", gtables.RequisitionLineTableName, "ReserveAvailable", functions.For(func(rec *gtables.RequisitionLine) (fcodeunits.Result, error) {
		return reserveAvailable(rec, func(m *ReservationManagement) (types.Decimal, Source, types.Date, error) {
			qtyToReserve, _, err := m.ReqLineUpdateValues(rec)
			return qtyToReserve, ReqLineSource(rec), rec.Expected_receipt_date, err
		}, rec.Variant_code, rec.Location_code, rec.Bin_code, rec.Serial_no, rec.Lot_no)
	}))
	functions.Register("table", gtables.ItemLedgerEntryTableName, "ReserveAvailable", functions.For(func(rec *gtables.ItemLedgerEntry) (fcodeunits.Result, error) {
		return reserveAvailable(rec, func(m *ReservationManagement) (types.Decimal, Source, types.Date, error) {
			qtyToReserve, _, err := m.ItemLedgEntryUpdateValues(rec)
			return qtyToReserve, ItemLedgEntrySource(rec), types.Date{}, err
		}, rec.Variant_code, rec.Location_code, rec.Bin_code, rec.Serial_no, rec.Lot_no)
	}))
	for table, source := range map[string]func(rec any) Source{
		gtables.SalesLineTableName:       func(rec any) Source { return SalesLineSource(rec.(*gtables.SalesLine)) },
		gtables.PurchaseLineTableName:    func(rec any) Source { return PurchLineSource(rec.(*gtables.PurchaseLine)) },
		gtables.ItemJournalLineTableName: func(rec any) Source { return ItemJnlLineSource(rec.(*gtables.ItemJournalLine)) },
		gtables.BOMJournalLineTableName:  func(rec any) Source { return BOMJnlLineSource(rec.(*gtables.BOMJournalLine)) },
		gtables.RequisitionLineTableName: func(rec any) Source { return ReqLineSource(rec.(*gtables.RequisitionLine)) },
		gtables.ItemLedgerEntryTableName: func(rec any) Source { return ItemLedgEntrySource(rec.(*gtables.ItemLedgerEntry)) },
	} {
		source := source
		functions.Register("table", table, "CancelAvailableReservation", lineFunc(table, func(rec lineRecord) (fcodeunits.Result, error) {
			return cancelAvailableReservation(rec, source(rec))
		}), functions.WithDialog())
	}
	functions.Register("table", gtables.ReservationEntryTableName, "CancelReservation", functions.For(cancelReservationEntries), functions.WithDialog())
}

// lineRecord is a line record (its table and its database)
type lineRecord interface {
	ftables.Table
	ftables.Source
}

// lineFunc registers a function of any line table through its record
func lineFunc(table string, fn func(rec lineRecord) (fcodeunits.Result, error)) functions.Func {
	switch table {
	case gtables.SalesLineTableName:
		return functions.For(func(rec *gtables.SalesLine) (fcodeunits.Result, error) { return fn(rec) })
	case gtables.PurchaseLineTableName:
		return functions.For(func(rec *gtables.PurchaseLine) (fcodeunits.Result, error) { return fn(rec) })
	case gtables.ItemJournalLineTableName:
		return functions.For(func(rec *gtables.ItemJournalLine) (fcodeunits.Result, error) { return fn(rec) })
	case gtables.BOMJournalLineTableName:
		return functions.For(func(rec *gtables.BOMJournalLine) (fcodeunits.Result, error) { return fn(rec) })
	case gtables.RequisitionLineTableName:
		return functions.For(func(rec *gtables.RequisitionLine) (fcodeunits.Result, error) { return fn(rec) })
	}
	return functions.For(func(rec *gtables.ItemLedgerEntry) (fcodeunits.Result, error) { return fn(rec) })
}

// openPage: a result that opens a page with parameters, Esc going back
func openPage(pageID int, params map[string]string) fcodeunits.Result {
	return fcodeunits.Result{Success: true, OpenPage: &fcodeunits.PageLink{PageID: pageID, Params: params, Return: true}}
}

// pageSource: the line the running page is for (its parameters)
func pageSource() Source {
	return SourceFromParams(system.PageParam)
}

// reservationPage: the Reservation page again, with or without the optional sources
func reservationPage(optional bool) fcodeunits.Result {
	params := Params(pageSource())
	if optional {
		params[ParamOptional] = "1"
	}
	return fcodeunits.Result{Success: true, OpenPage: &fcodeunits.PageLink{PageID: ReservationPageID, Params: params}}
}

// inTransaction runs fn on a transaction of the record's database (committed when it succeeds);
// a record already in a transaction runs fn in it
func inTransaction(rec ftables.Source, fn func(src ftables.Source) error) error {
	_, inTx := rec.GetDB().(*sql.Tx)
	if inTx {
		return fn(rec)
	}
	tx, err := ftables.BeginTransaction(rec.GetDB())
	if err != nil {
		return err
	}
	err = fn(ftables.DBSource(tx, rec.GetCompany(), rec.GetDBType()))
	if err != nil {
		_ = ftables.Rollback(tx)
		return err
	}
	return ftables.Commit(tx)
}

// reservedQty: the line's reserved quantity and what it still has to reserve
func remainingQty(m *ReservationManagement, f ForLine, summary *gtables.EntrySummary) (types.Decimal, error) {
	qtyReserved, err := m.UpdateStatistics(summary, true, f.ShipmentDate, func(string) string { return "" })
	if err != nil {
		return types.Decimal{}, err
	}
	return f.QtyToReserve.Sub(qtyReserved), nil
}

// autoReserve (the Reservation page's Auto Reserve): the line reserves by the setup's
// priorities; a message when not all of it could be reserved
func autoReserve(rec *gtables.EntrySummary) (fcodeunits.Result, error) {
	var full bool
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
		full, err = m.AutoReserve(f.ValidToDate, "", f.ShipmentDate)
		return err
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !full {
		return withRefresh(system.Message(i18n.GetInstance().Message("RESERV_FULL_AUTO_NOT_POSSIBLE", fcodeunits.CurrentLanguage()))), nil
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// withRefresh: a result that also reloads the page
func withRefresh(result fcodeunits.Result) fcodeunits.Result {
	result.Refresh = true
	return result
}

// reserveFromCurrentLine (Reserve from Current Line): the line reserves what it still needs
// from the summary line's source
func reserveFromCurrentLine(rec *gtables.EntrySummary) (fcodeunits.Result, error) {
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
		if !remaining.IsPositive() {
			return apperrors.ReservFullyReserved()
		}
		left, err := m.AutoReserveOneLine(rec.Entry_no, remaining, f.ValidToDate, "", f.ShipmentDate)
		if err != nil {
			return err
		}
		if left.Equal(remaining) {
			return apperrors.ReservNothingAvailable()
		}
		return nil
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// cancelSummaryReservation (Cancel Reservation from Current Line): after a question all the
// line's reservations from the summary line's source are canceled
func cancelSummaryReservation(rec *gtables.EntrySummary) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if system.GuiAllowed() && !system.Confirm(ts.Message("RESERV_CANCEL_ALL_CONFIRM", lang), rec.Summary_type) {
		return fcodeunits.Result{Success: true}, nil
	}
	err := inTransaction(rec, func(src ftables.Source) error {
		m := New(src)
		f, err := m.SetReservForSource(pageSource())
		if err != nil {
			return err
		}
		reservEntry2 := gtables.NewReservationEntry(src)
		FilterSummaryEntries(reservEntry2, f, rec.Entry_no)
		var entryNos []int
		for found := reservEntry2.FindSet(); found; found = reservEntry2.Next() {
			if reservEntry2.Quantity.IsNegative() {
				return apperrors.ReservNegativeManually()
			}
			entryNos = append(entryNos, reservEntry2.Entry_no)
		}
		if len(entryNos) == 0 {
			return apperrors.ReservNothingToCancel()
		}
		for _, entryNo := range entryNos {
			err = m.CloseReservEntry(entryNo, gtables.ReservationEntry_Reservation_status.CanceledOrChanged, true, true)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// showAvailable (Available to Reserve): the summary line's source's lines (Available - …)
func showAvailable(rec *gtables.EntrySummary) (fcodeunits.Result, error) {
	if rec.Available_page_id == 0 {
		return fcodeunits.Result{Success: true}, nil
	}
	params := Params(pageSource())
	params[ParamSummary] = strconv.Itoa(rec.Entry_no)
	return openPage(rec.Available_page_id, params), nil
}

// reserveAvailable (the Available - … pages' Reserve): the line the page is for reserves from
// the supply line what both still have (the supply line's available quantity, the line's
// remaining one)
func reserveAvailable(rec ftables.Source, values func(m *ReservationManagement) (types.Decimal, Source, types.Date, error), variantCode, locationCode, binCode, serialNo, lotNo types.Code) (fcodeunits.Result, error) {
	err := inTransaction(rec, func(src ftables.Source) error {
		gtables.NewReservationEntry(src).LockTable()
		m := New(src)
		f, err := m.SetReservForSource(pageSource())
		if err != nil {
			return err
		}
		qtyToReserve, from, expectedDate, err := values(m)
		if err != nil {
			return err
		}
		newQtyReservedThisLine, err := m.CalculateRemainingQty()
		if err != nil {
			return err
		}
		if newQtyReservedThisLine.IsZero() {
			return apperrors.ReservFullyReserved()
		}
		reserveQuantity := newQtyReservedThisLine
		if newQtyReservedThisLine.GreaterThan(qtyToReserve) {
			reserveQuantity = qtyToReserve
		}
		if !reserveQuantity.IsPositive() {
			return apperrors.ReservAvailableQty(qtyToReserve.String())
		}
		if variantCode != f.VariantCode {
			return apperrors.FieldMustBe(gtables.ReservationEntryTableName, "variant_code", f.VariantCode.String())
		}
		if locationCode != f.LocationCode {
			return apperrors.FieldMustBe(gtables.ReservationEntryTableName, "location_code", f.LocationCode.String())
		}
		if binCode != f.BinCode {
			return apperrors.FieldMustBe(gtables.ReservationEntryTableName, "bin_code", f.BinCode.String())
		}
		if serialNo != f.SerialNo {
			return apperrors.FieldMustBe(gtables.ReservationEntryTableName, "serial_no", f.SerialNo.String())
		}
		if lotNo != f.LotNo {
			return apperrors.FieldMustBe(gtables.ReservationEntryTableName, "lot_no", f.LotNo.String())
		}
		err = m.CreateReservation(f.ValidToDate, "", expectedDate, reserveQuantity, from, variantCode, locationCode, binCode, serialNo, lotNo)
		if err != nil {
			return err
		}
		if from.Type == TypeItemLedgerEntry {
			// the entry's new Reserved Quantity through Item Entry-Edit (codeunit 405)
			itemLedgEntry := gtables.NewItemLedgerEntry(src)
			err = itemLedgEntry.GetOrError(from.RefNo)
			if err != nil {
				return err
			}
			reservEntry := gtables.NewReservationEntry(src)
			reservEntry.Item_no, reservEntry.Variant_code, reservEntry.Location_code = f.ItemNo, variantCode, locationCode
			reservEntry.Bin_code, reservEntry.Serial_no, reservEntry.Lot_no = binCode, serialNo, lotNo
			itemLedgEntry.Reserved_quantity = itemLedgEntry.Reserved_quantity.Sub(reserveQuantity)
			return m.ModifyReservedQuantity(itemLedgEntry, reservEntry)
		}
		return m.changeReservedQty(from, reserveQuantity.Neg(), false)
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// cancelAvailableReservation (the Available - … pages' Cancel Reservation): after a question
// the reservations of the line the page is for from the supply line are canceled
func cancelAvailableReservation(rec ftables.Source, from Source) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if system.GuiAllowed() && !system.Confirm(ts.Message("RESERV_CANCEL_CONFIRM_SHORT", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}
	err := inTransaction(rec, func(src ftables.Source) error {
		m := New(src)
		reservEntry2 := ForLineEntries(src, pageSource(), false)
		FilterFrom(reservEntry2, from)
		var entryNos []int
		for found := reservEntry2.FindSet(); found; found = reservEntry2.Next() {
			if reservEntry2.Quantity.IsNegative() {
				return apperrors.ReservNegativeManually()
			}
			entryNos = append(entryNos, reservEntry2.Entry_no)
		}
		for _, entryNo := range entryNos {
			err := m.CloseReservEntry(entryNo, gtables.ReservationEntry_Reservation_status.CanceledOrChanged, true, true)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// cancelReservationEntries (Reservation Entries' Cancel Reservation): each selected open entry
// is canceled after its question (the quantity, item, demand and supply)
func cancelReservationEntries(rec *gtables.ReservationEntry) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	rec.SetSelectionFilter()
	var entries []*gtables.ReservationEntry
	for found := rec.FindSet(); found; found = rec.Next() {
		err := rec.TestField("reservation_status", gtables.ReservationEntry_Reservation_status.Open)
		if err != nil {
			return fcodeunits.Result{}, err
		}
		e := gtables.NewReservationEntry(rec)
		e.Copy(rec)
		entries = append(entries, e)
	}
	var canceled []int
	for _, e := range entries {
		if system.GuiAllowed() && !system.Confirm(ts.Message("RESERV_CANCEL_ENTRY_CONFIRM", lang),
			e.Quantity, e.Item_no, CreateForText(e, lang), CreateFromText(e, lang)) {
			continue
		}
		canceled = append(canceled, e.Entry_no)
	}
	err := inTransaction(rec, func(src ftables.Source) error {
		m := New(src)
		for _, entryNo := range canceled {
			err := m.CloseReservEntry(entryNo, gtables.ReservationEntry_Reservation_status.CanceledOrChanged, true, true)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// CreateForText / CreateFromText (CreateForText, CreateFromText): an entry's demand and supply
// as text — the type, the document or entry type and the document or journal
func CreateForText(e *gtables.ReservationEntry, lang string) string {
	return sourceText(e, "for_type", int(e.For_type), e.For_subtype, e.For_id, e.For_batch_name, e.For_ref_no, lang)
}

func CreateFromText(e *gtables.ReservationEntry, lang string) string {
	return sourceText(e, "from_type", int(e.From_type), e.From_subtype, e.From_id, e.From_batch_name, e.From_ref_no, lang)
}

func sourceText(e *gtables.ReservationEntry, typeField string, sourceType, subtype int, id, batchName types.Code, refNo int, lang string) string {
	ts := i18n.GetInstance()
	typeText := ts.OptionCaption(gtables.ReservationEntryTableName, typeField, i18n.Key(gtables.ReservationEntryFor_type(sourceType).String()), lang)
	switch sourceType {
	case TypeSales:
		docType := ts.OptionCaption(gtables.SalesLineTableName, "document_type", i18n.Key(gtables.SalesLineDocument_type(subtype).String()), lang)
		return system.StrSubstNo("%1 %2 %3", typeText, docType, id)
	case TypePurchase:
		docType := ts.OptionCaption(gtables.PurchaseLineTableName, "document_type", i18n.Key(gtables.PurchaseLineDocument_type(subtype).String()), lang)
		return system.StrSubstNo("%1 %2 %3", typeText, docType, id)
	case TypeRequisitionLine, TypeBOMJournal:
		return system.StrSubstNo("%1 %2 %3", typeText, id, batchName)
	case TypeItemJournal:
		entryType := ts.OptionCaption(gtables.ItemJournalLineTableName, "entry_type", i18n.Key(gtables.ItemJournalLineEntry_type(subtype).String()), lang)
		return system.StrSubstNo("%1 %2 %3 %4", typeText, entryType, id, batchName)
	case TypeItemLedgerEntry:
		return system.StrSubstNo("%1 %2", typeText, refNo)
	}
	return ""
}
