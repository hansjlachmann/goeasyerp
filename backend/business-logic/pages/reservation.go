package pages

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reservmgt"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The reservation pages, opened for one line (its source as page parameters, reservmgt.Params):
// 498 Reservation (a temporary Entry Summary: the sources the line can reserve from, footer the
// line), 497 Reservation Entries (the line's open entries), 499–504 Available - … (the lines of
// one summary line's source)
func init() {
	pages.OnFindRecords(reservmgt.ReservationPageID, func(rec *gtables.EntrySummary, page *pages.Open) error {
		src := realSource(rec)
		m := reservmgt.New(src)
		f, err := m.SetReservForSource(reservmgt.SourceFromParams(page.Param))
		if err != nil {
			return err
		}
		ts := i18n.GetInstance()
		lang := fcodeunits.CurrentLanguage()
		_, err = m.UpdateStatistics(rec, page.Param(reservmgt.ParamOptional) == "1", f.ShipmentDate, func(key string) string {
			return ts.Message(key, lang)
		})
		return err
	})
	pages.OnAfterGetRecord(reservmgt.ReservationPageID, func(rec *gtables.EntrySummary, page *pages.Control) {
		src := realSource(rec)
		m := reservmgt.New(src)
		s := reservmgt.SourceFromParams(system.PageParam)
		f, err := m.SetReservForSource(s)
		if err != nil {
			return
		}
		reservEntry := reservmgt.ForLineEntries(src, s, false)
		qtyReserved := types.Decimal{}
		if reservEntry.CalcSums("quantity") == nil {
			qtyReserved = reservEntry.Quantity
		}
		page.SetValue("item_no", f.ItemNo)
		page.SetValue("variant_code", f.VariantCode)
		page.SetValue("location_code", f.LocationCode)
		page.SetValue("shipment_date", f.ShipmentDate)
		page.SetValue("valid_to_date", f.ValidToDate)
		page.SetValue("qty_to_reserve", f.QtyToReserve)
		page.SetValue("qty_reserved", qtyReserved)
		page.SetValue("unreserved_quantity", f.QtyToReserve.Sub(qtyReserved))
		page.SetValue("optional_lines", system.PageParam(reservmgt.ParamOptional) == "1")
	})

	// 497 Reservation Entries: a line's open entries — as the demand (For) or, for a supply, the
	// entries reserved from it (From); opened from the menu all entries
	pages.OnOpenPage(reservmgt.ReservationEntriesPageID, func(rec *gtables.ReservationEntry, page *pages.Open) error {
		if page.Param(reservmgt.ParamForType) == "" {
			return nil
		}
		s := reservmgt.SourceFromParams(page.Param)
		prefix := "for_"
		if page.Param(reservmgt.ParamSupply) == "1" {
			prefix = "from_"
		}
		page.SetFilter("reservation_status", int(gtables.ReservationEntry_Reservation_status.Open))
		page.SetFilter(prefix+"type", s.Type)
		page.SetFilter(prefix+"subtype", s.Subtype)
		page.SetFilter(prefix+"id", s.ID)
		page.SetFilter(prefix+"batch_name", s.BatchName)
		page.SetFilter(prefix+"prod_order_line", s.ProdOrderLine)
		page.SetFilter(prefix+"ref_no", s.RefNo)
		return nil
	})
	pages.OnAfterGetRecord(reservmgt.ReservationEntriesPageID, func(rec *gtables.ReservationEntry, page *pages.Control) {
		lang := fcodeunits.CurrentLanguage()
		page.SetValue("reserved_for", reservmgt.CreateForText(rec, lang))
		page.SetValue("reserved_from", reservmgt.CreateFromText(rec, lang))
	})

	// 499 Available - Sales Lines: the item lines of the summary line's document type that give
	// the item (a credit memo; negative order lines), up to the line's date
	pages.OnOpenPage(reservmgt.AvailableSalesLinesPageID, func(rec *gtables.SalesLine, page *pages.Open) error {
		f, fromSubtype, err := availableFor(rec, page)
		if err != nil {
			return err
		}
		page.SetFilter("document_type", fromSubtype)
		page.SetFilter("type", int(gtables.SalesLine_Type.Item))
		page.SetFilter("no", f.ItemNo)
		page.SetFilter("variant_code", f.VariantCode)
		page.SetFilter("location_code", f.LocationCode)
		page.SetFilter("bin_code", f.BinCode)
		page.SetFilter("serial_no", f.SerialNo)
		page.SetFilter("lot_no", f.LotNo)
		page.SetFilterExpression("shipment_date", ".."+f.ShipmentDate.String())
		if fromSubtype == int(gtables.SalesLine_Document_type.CreditMemo) {
			page.SetFilterExpression("quantity_base", ">0")
		} else {
			page.SetFilterExpression("quantity_base", "<0")
		}
		return nil
	})
	pages.OnAfterGetRecord(reservmgt.AvailableSalesLinesPageID, func(rec *gtables.SalesLine, page *pages.Control) {
		availableValues(rec, page, func(m *reservmgt.ReservationManagement) (types.Decimal, types.Decimal, error) {
			return m.SalesLineUpdateValues(rec)
		})
	})

	// 500 Available - Requisition Lines: the item lines up to the line's date
	pages.OnOpenPage(reservmgt.AvailableReqLinesPageID, func(rec *gtables.RequisitionLine, page *pages.Open) error {
		f, _, err := availableFor(rec, page)
		if err != nil {
			return err
		}
		page.SetFilter("type", int(gtables.RequisitionLine_Type.Item))
		page.SetFilter("no", f.ItemNo)
		page.SetFilter("variant_code", f.VariantCode)
		page.SetFilter("location_code", f.LocationCode)
		page.SetFilter("bin_code", f.BinCode)
		page.SetFilter("serial_no", f.SerialNo)
		page.SetFilter("lot_no", f.LotNo)
		page.SetFilterExpression("expected_receipt_date", ".."+f.ShipmentDate.String())
		page.SetFilterExpression("quantity_base", ">0")
		return nil
	})
	pages.OnAfterGetRecord(reservmgt.AvailableReqLinesPageID, func(rec *gtables.RequisitionLine, page *pages.Control) {
		availableValues(rec, page, func(m *reservmgt.ReservationManagement) (types.Decimal, types.Decimal, error) {
			return m.ReqLineUpdateValues(rec)
		})
	})

	// 501 Available - Purchase Lines: the item lines of the summary line's document type that
	// bring the item, received up to the line's date
	pages.OnOpenPage(reservmgt.AvailablePurchLinesPageID, func(rec *gtables.PurchaseLine, page *pages.Open) error {
		f, fromSubtype, err := availableFor(rec, page)
		if err != nil {
			return err
		}
		page.SetFilter("document_type", fromSubtype)
		page.SetFilter("type", int(gtables.PurchaseLine_Type.Item))
		page.SetFilter("no", f.ItemNo)
		page.SetFilter("variant_code", f.VariantCode)
		page.SetFilter("location_code", f.LocationCode)
		page.SetFilter("bin_code", f.BinCode)
		page.SetFilter("serial_no", f.SerialNo)
		page.SetFilter("lot_no", f.LotNo)
		page.SetFilterExpression("expected_receipt_date", ".."+f.ShipmentDate.String())
		if fromSubtype == int(gtables.PurchaseLine_Document_type.CreditMemo) {
			page.SetFilterExpression("quantity_base", "<0")
		} else {
			page.SetFilterExpression("quantity_base", ">0")
		}
		return nil
	})
	pages.OnAfterGetRecord(reservmgt.AvailablePurchLinesPageID, func(rec *gtables.PurchaseLine, page *pages.Control) {
		availableValues(rec, page, func(m *reservmgt.ReservationManagement) (types.Decimal, types.Decimal, error) {
			return m.PurchLineUpdateValues(rec)
		})
	})

	// 502 Available - Item Jnl. Lines: the lines of the summary line's entry type that bring
	// the item to the location (a transfer: to its new location), posted up to the line's date
	pages.OnOpenPage(reservmgt.AvailableItemJnlLinesPageID, func(rec *gtables.ItemJournalLine, page *pages.Open) error {
		f, fromSubtype, err := availableFor(rec, page)
		if err != nil {
			return err
		}
		page.SetFilter("entry_type", fromSubtype)
		page.SetFilter("item_no", f.ItemNo)
		page.SetFilter("variant_code", f.VariantCode)
		if fromSubtype == int(gtables.ItemJournalLine_Entry_type.Transfer) {
			page.SetFilter("new_location_code", f.LocationCode)
			page.SetFilter("new_bin_code", f.BinCode)
			page.SetFilter("new_serial_no", f.SerialNo)
			page.SetFilter("new_lot_no", f.LotNo)
		} else {
			page.SetFilter("location_code", f.LocationCode)
			page.SetFilter("bin_code", f.BinCode)
			page.SetFilter("serial_no", f.SerialNo)
			page.SetFilter("lot_no", f.LotNo)
		}
		page.SetFilterExpression("posting_date", ".."+f.ShipmentDate.String())
		switch fromSubtype {
		case int(gtables.ItemJournalLine_Entry_type.Sale), int(gtables.ItemJournalLine_Entry_type.NegativeAdjmt):
			page.SetFilterExpression("quantity_base", "<0")
		default:
			page.SetFilterExpression("quantity_base", ">0")
		}
		return nil
	})
	pages.OnAfterGetRecord(reservmgt.AvailableItemJnlLinesPageID, func(rec *gtables.ItemJournalLine, page *pages.Control) {
		availableValues(rec, page, func(m *reservmgt.ReservationManagement) (types.Decimal, types.Decimal, error) {
			return m.ItemJnlLineUpdateValues(rec)
		})
	})

	// 503 Available - BOM Jnl. Lines: the assembled items posted up to the line's date
	pages.OnOpenPage(reservmgt.AvailableBOMJnlLinesPageID, func(rec *gtables.BOMJournalLine, page *pages.Open) error {
		f, _, err := availableFor(rec, page)
		if err != nil {
			return err
		}
		page.SetFilter("item_no", f.ItemNo)
		page.SetFilter("variant_code", f.VariantCode)
		page.SetFilter("location_code", f.LocationCode)
		page.SetFilter("bin_code", f.BinCode)
		page.SetFilter("lot_no", f.LotNo)
		page.SetFilterExpression("posting_date", ".."+f.ShipmentDate.String())
		page.SetFilterExpression("quantity_base", ">0")
		return nil
	})
	pages.OnAfterGetRecord(reservmgt.AvailableBOMJnlLinesPageID, func(rec *gtables.BOMJournalLine, page *pages.Control) {
		availableValues(rec, page, func(m *reservmgt.ReservationManagement) (types.Decimal, types.Decimal, error) {
			return m.BOMJnlLineUpdateValues(rec)
		})
	})

	// 504 Available - Item Ledg. Entries: the open inbound entries of the item at the location
	pages.OnOpenPage(reservmgt.AvailableItemLedgEntriesPageID, func(rec *gtables.ItemLedgerEntry, page *pages.Open) error {
		f, _, err := availableFor(rec, page)
		if err != nil {
			return err
		}
		page.SetFilter("item_no", f.ItemNo)
		page.SetFilter("variant_code", f.VariantCode)
		page.SetFilter("location_code", f.LocationCode)
		page.SetFilter("bin_code", f.BinCode)
		page.SetFilter("serial_no", f.SerialNo)
		page.SetFilter("lot_no", f.LotNo)
		page.SetFilter("drop_shipment", false)
		page.SetFilter("open", true)
		page.SetFilter("positive", true)
		page.SetFilterExpression("remaining_quantity", ">0")
		return nil
	})
	pages.OnAfterGetRecord(reservmgt.AvailableItemLedgEntriesPageID, func(rec *gtables.ItemLedgerEntry, page *pages.Control) {
		availableValues(rec, page, func(m *reservmgt.ReservationManagement) (types.Decimal, types.Decimal, error) {
			return m.ItemLedgEntryUpdateValues(rec)
		})
	})
}

// realSource: the database of a page's record (a temporary record's triggers use the real one)
func realSource(rec ftables.Source) ftables.Source {
	return ftables.DBSource(rec.GetDB(), rec.GetCompany(), rec.GetDBType())
}

// availableFor (the Available - … pages' OnOpenForm): the line the page reserves for and the
// document or entry type of its summary line
func availableFor(rec ftables.Source, page *pages.Open) (reservmgt.ForLine, int, error) {
	f, err := reservmgt.New(realSource(rec)).SetReservForSource(reservmgt.SourceFromParams(page.Param))
	if err != nil {
		return f, 0, err
	}
	summaryEntryNo, _ := strconv.Atoi(page.Param(reservmgt.ParamSummary))
	_, fromSubtype, _ := reservmgt.SummaryFromSource(summaryEntryNo)
	return f, fromSubtype, nil
}

// availableValues (the Available - … pages' OnAfterGetRecord, …UpdateValues): the supply line's
// Available Quantity and what the line the page is for has reserved from it
func availableValues(rec ftables.Source, page *pages.Control, updateValues func(m *reservmgt.ReservationManagement) (types.Decimal, types.Decimal, error)) {
	m := reservmgt.New(realSource(rec))
	_, err := m.SetReservForSource(reservmgt.SourceFromParams(system.PageParam))
	if err != nil {
		return
	}
	qtyToReserve, qtyReservedThisLine, err := updateValues(m)
	if err != nil {
		return
	}
	page.SetValue("available_quantity", qtyToReserve)
	page.SetValue("current_reserved_quantity", qtyReservedThisLine)
}
