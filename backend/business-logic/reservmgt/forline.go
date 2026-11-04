package reservmgt

import (
	"strconv"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The Reservation page (498) and the Available - … pages are opened for one line (the line
// reserved for) — its source as page parameters
const (
	ParamForType    = "for_type"
	ParamForSubtype = "for_subtype"
	ParamForID      = "for_id"
	ParamForBatch   = "for_batch"
	ParamForRef     = "for_ref"
	ParamSummary    = "summary"  // the Entry Summary line an Available - … page shows
	ParamOptional   = "optional" // the Reservation page with the optional sources
	ParamSupply     = "supply"   // Reservation Entries of a line that is a supply (its From entries)
)

// Params: a source as the pages' parameters
func Params(s Source) map[string]string {
	return map[string]string{
		ParamForType:    strconv.Itoa(s.Type),
		ParamForSubtype: strconv.Itoa(s.Subtype),
		ParamForID:      s.ID.String(),
		ParamForBatch:   s.BatchName.String(),
		ParamForRef:     strconv.Itoa(s.RefNo),
	}
}

// SourceFromParams: the line a page is opened for, from its parameters
func SourceFromParams(param func(string) string) Source {
	atoi := func(name string) int {
		n, _ := strconv.Atoi(param(name))
		return n
	}
	return Source{Type: atoi(ParamForType), Subtype: atoi(ParamForSubtype), ID: types.Code(param(ParamForID)),
		BatchName: types.Code(param(ParamForBatch)), RefNo: atoi(ParamForRef)}
}

// ForLine is the line reserved for as the Reservation page shows it (the classic page's
// ReservEntry header: item, variant, location, shipment date, valid-to date; QtyToReserve)
type ForLine struct {
	Source                            Source
	ItemNo, VariantCode, LocationCode types.Code
	BinCode, SerialNo, LotNo          types.Code
	ShipmentDate, ValidToDate         types.Date
	UnitOfMeasureCode                 types.Code
	QtyToReserve                      types.Decimal
}

// SetReservForSource (the Reservation page's Set…Line): the line of a source becomes the line
// reserved for — it must be able to reserve (Set…ReservFor, the classic page's checks: an item
// line with a date) — with its quantity to reserve and the valid-to date of new reservations
func (m *ReservationManagement) SetReservForSource(s Source) (ForLine, error) {
	f := ForLine{Source: s}
	var err error
	switch s.Type {
	case TypeSales:
		salesLine := gtables.NewSalesLine(m.src)
		err = salesLine.GetOrError(map[string]interface{}{"document_type": s.Subtype, "document_no": s.ID, "line_no": s.RefNo})
		if err == nil {
			err = salesLine.TestField("type", gtables.SalesLine_Type.Item)
		}
		if err == nil {
			err = salesLine.TestField("shipment_date")
		}
		if err == nil {
			err = m.SalesLineSetReservFor(salesLine)
		}
		if err == nil {
			f.ItemNo, f.VariantCode, f.LocationCode = salesLine.No, salesLine.Variant_code, salesLine.Location_code
			f.BinCode, f.SerialNo, f.LotNo = salesLine.Bin_code, salesLine.Serial_no, salesLine.Lot_no
			f.ShipmentDate, f.UnitOfMeasureCode = salesLine.Shipment_date, salesLine.Unit_of_measure_code
			f.QtyToReserve, err = SalesLineReservQuantity(salesLine, true)
		}
	case TypePurchase:
		purchLine := gtables.NewPurchaseLine(m.src)
		err = purchLine.GetOrError(map[string]interface{}{"document_type": s.Subtype, "document_no": s.ID, "line_no": s.RefNo})
		if err == nil {
			err = purchLine.TestField("type", gtables.PurchaseLine_Type.Item)
		}
		if err == nil {
			err = purchLine.TestField("expected_receipt_date")
		}
		if err == nil {
			err = m.PurchLineSetReservFor(purchLine)
		}
		if err == nil {
			f.ItemNo, f.VariantCode, f.LocationCode = purchLine.No, purchLine.Variant_code, purchLine.Location_code
			f.BinCode, f.SerialNo, f.LotNo = purchLine.Bin_code, purchLine.Serial_no, purchLine.Lot_no
			f.ShipmentDate, f.UnitOfMeasureCode = purchLine.Expected_receipt_date, purchLine.Unit_of_measure_code
			f.QtyToReserve, err = PurchLineReservQuantity(purchLine, true)
		}
	case TypeRequisitionLine:
		reqLine := gtables.NewRequisitionLine(m.src)
		err = reqLine.GetOrError(map[string]interface{}{"worksheet_template_name": s.ID, "journal_batch_name": s.BatchName, "line_no": s.RefNo})
		if err == nil {
			err = reqLine.TestField("expected_receipt_date")
		}
		if err == nil {
			err = m.ReqLineSetReservFor(reqLine)
		}
		if err == nil {
			f.ItemNo, f.VariantCode, f.LocationCode = reqLine.No, reqLine.Variant_code, reqLine.Location_code
			f.BinCode, f.SerialNo, f.LotNo = reqLine.Bin_code, reqLine.Serial_no, reqLine.Lot_no
			f.ShipmentDate, f.UnitOfMeasureCode = reqLine.Expected_receipt_date, reqLine.Unit_of_measure_code
			f.QtyToReserve, err = ReqLineReservQuantity(reqLine, true)
		}
	case TypeItemJournal:
		itemJnlLine := gtables.NewItemJournalLine(m.src)
		err = itemJnlLine.GetOrError(map[string]interface{}{"journal_template_name": s.ID, "journal_batch_name": s.BatchName, "line_no": s.RefNo})
		if err == nil {
			err = itemJnlLine.TestField("posting_date")
		}
		if err == nil {
			err = m.ItemJnlLineSetReservFor(itemJnlLine)
		}
		if err == nil {
			f.ItemNo, f.VariantCode, f.LocationCode = itemJnlLine.Item_no, itemJnlLine.Variant_code, itemJnlLine.Location_code
			f.BinCode, f.SerialNo, f.LotNo = itemJnlLine.Bin_code, itemJnlLine.Serial_no, itemJnlLine.Lot_no
			f.ShipmentDate, f.UnitOfMeasureCode = itemJnlLine.Posting_date, itemJnlLine.Unit_of_measure_code
			f.QtyToReserve, err = ItemJnlLineReservQuantity(itemJnlLine, true)
		}
	case TypeBOMJournal:
		bomJnlLine := gtables.NewBOMJournalLine(m.src)
		err = bomJnlLine.GetOrError(map[string]interface{}{"journal_template_name": s.ID, "journal_batch_name": s.BatchName, "line_no": s.RefNo})
		if err == nil {
			err = bomJnlLine.TestField("posting_date")
		}
		if err == nil {
			err = m.BOMJnlLineSetReservFor(bomJnlLine)
		}
		if err == nil {
			f.ItemNo, f.VariantCode, f.LocationCode = bomJnlLine.Item_no, bomJnlLine.Variant_code, bomJnlLine.Location_code
			f.BinCode, f.LotNo = bomJnlLine.Bin_code, bomJnlLine.Lot_no
			f.ShipmentDate, f.UnitOfMeasureCode = bomJnlLine.Posting_date, bomJnlLine.Unit_of_measure_code
			f.QtyToReserve, err = BOMJnlLineReservQuantity(bomJnlLine, true)
		}
	default:
		return f, apperrors.FieldRequired(gtables.ReservationEntryTableName, "for_type")
	}
	if err != nil {
		return f, err
	}
	f.ValidToDate, err = m.validToDate(f.ShipmentDate)
	return f, err
}

// ForLineEntries: the open reservation entries of the line a page is for — as a demand
// (positive reserved quantity) or as a supply
func ForLineEntries(src ftables.Source, s Source, asSupply bool) *gtables.ReservationEntry {
	reservEntry := gtables.NewReservationEntry(src)
	if asSupply {
		InitFilterAndSortingLookupFrom(reservEntry, true)
		FilterFrom(reservEntry, s)
	} else {
		InitFilterAndSortingLookupFor(reservEntry, true)
		FilterFor(reservEntry, s)
	}
	return reservEntry
}

// FilterSummaryEntries (the Reservation page's FilterReservEntry): the line's open entries from
// the source of a summary line, up to the line's date (the inventory: any date)
func FilterSummaryEntries(filterReservEntry *gtables.ReservationEntry, f ForLine, summaryEntryNo int) {
	InitFilterAndSortingLookupFor(filterReservEntry, true)
	FilterFor(filterReservEntry, f.Source)
	filterReservEntry.SetRange("item_no", f.ItemNo)
	filterReservEntry.SetRange("location_code", f.LocationCode)
	filterReservEntry.SetRange("expected_receipt_date", types.Date{}, f.ShipmentDate)
	fromType, fromSubtype, ok := SummaryFromSource(summaryEntryNo)
	if !ok {
		return
	}
	filterReservEntry.SetRange("from_type", fromType)
	filterReservEntry.SetRange("from_subtype", fromSubtype)
	if fromType == TypeItemLedgerEntry {
		filterReservEntry.SetRange("expected_receipt_date")
	}
}
