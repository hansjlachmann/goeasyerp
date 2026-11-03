package reservmgt

import (
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The Entry Summary lines of the Reservation page (classic UpdateStatistics' entry numbers):
// inventory, purchase documents, requisition lines, sales documents, item journal lines by entry
// type, BOM journal lines
const (
	SummaryItemLedgerEntry    = 1
	SummaryPurchaseQuote      = 11
	SummaryPurchaseOrder      = 12
	SummaryPurchaseInvoice    = 13
	SummaryPurchaseCreditMemo = 14
	SummaryBlanketPurchOrder  = 15
	SummaryRequisitionLine    = 21
	SummarySalesQuote         = 31
	SummarySalesOrder         = 32
	SummarySalesInvoice       = 33
	SummarySalesCreditMemo    = 34
	SummaryBlanketSalesOrder  = 35
	SummaryItemJnlPurchase    = 41
	SummaryItemJnlSale        = 42
	SummaryItemJnlPositiveAdj = 43
	SummaryItemJnlNegativeAdj = 44
	SummaryItemJnlTransfer    = 45
	SummaryBOMJournalLine     = 51
)

// The pages of the sources' available lines (Available - … Lines), opened from the summary
const (
	AvailableSalesLinesPageID      = 499
	AvailableReqLinesPageID        = 500
	AvailablePurchLinesPageID      = 501
	AvailableItemJnlLinesPageID    = 502
	AvailableBOMJnlLinesPageID     = 503
	AvailableItemLedgEntriesPageID = 504
)

// purchaseDocumentType / salesDocumentType: the document type of a summary line (classic
// "Entry No." − 11 / − 31); itemJnlEntryType: − 41
func purchaseDocumentType(summaryEntryNo int) int { return summaryEntryNo - SummaryPurchaseQuote }
func salesDocumentType(summaryEntryNo int) int    { return summaryEntryNo - SummarySalesQuote }
func itemJnlEntryType(summaryEntryNo int) int     { return summaryEntryNo - SummaryItemJnlPurchase }

// summaryLine: a summary line's caption key (Summary Type), table and Show… setting
type summaryLine struct {
	entryNo  int
	tableID  int
	caption  string // the summary type: the table's caption and the document / entry type
	show     int    // Never / Optional / Always
	priority int
	pageID   int
}

// summaryLines: the summary lines the setup has, in their order
func (m *ReservationManagement) summaryLines() []summaryLine {
	s := m.getReservSetup()
	return []summaryLine{
		{SummaryItemLedgerEntry, gtables.ItemLedgerEntryTableID, "SUMMARY_ITEM_LEDGER_ENTRY", int(s.Show_item_ledger_entry), s.Item_ledger_entry_priority, AvailableItemLedgEntriesPageID},
		{SummaryPurchaseQuote, gtables.PurchaseLineTableID, "SUMMARY_PURCHASE_QUOTE", int(s.Show_purchase_quote), s.Purchase_quote_priority, AvailablePurchLinesPageID},
		{SummaryPurchaseOrder, gtables.PurchaseLineTableID, "SUMMARY_PURCHASE_ORDER", int(s.Show_purchase_order), s.Purchase_order_priority, AvailablePurchLinesPageID},
		{SummaryPurchaseInvoice, gtables.PurchaseLineTableID, "SUMMARY_PURCHASE_INVOICE", int(s.Show_purchase_invoice), s.Purchase_invoice_priority, AvailablePurchLinesPageID},
		{SummaryPurchaseCreditMemo, gtables.PurchaseLineTableID, "SUMMARY_PURCHASE_CREDIT_MEMO", int(s.Show_purchase_credit_memo), s.Purchase_credit_memo_priority, AvailablePurchLinesPageID},
		{SummaryBlanketPurchOrder, gtables.PurchaseLineTableID, "SUMMARY_BLANKET_PURCHASE_ORDER", int(s.Show_blanket_purchase_order), s.Blanket_purch_order_priority, AvailablePurchLinesPageID},
		{SummaryRequisitionLine, gtables.RequisitionLineTableID, "SUMMARY_REQUISITION_LINE", int(s.Show_requisition_line), s.Requisition_line_priority, AvailableReqLinesPageID},
		{SummarySalesQuote, gtables.SalesLineTableID, "SUMMARY_SALES_QUOTE", int(s.Show_sales_quote), s.Sales_quote_priority, AvailableSalesLinesPageID},
		{SummarySalesOrder, gtables.SalesLineTableID, "SUMMARY_SALES_ORDER", int(s.Show_sales_order), s.Sales_order_priority, AvailableSalesLinesPageID},
		{SummarySalesInvoice, gtables.SalesLineTableID, "SUMMARY_SALES_INVOICE", int(s.Show_sales_invoice), s.Sales_invoice_priority, AvailableSalesLinesPageID},
		{SummarySalesCreditMemo, gtables.SalesLineTableID, "SUMMARY_SALES_CREDIT_MEMO", int(s.Show_sales_credit_memo), s.Sales_credit_memo_priority, AvailableSalesLinesPageID},
		// the classic shows the blanket sales orders by Show Sales Order
		{SummaryBlanketSalesOrder, gtables.SalesLineTableID, "SUMMARY_BLANKET_SALES_ORDER", int(s.Show_sales_order), s.Blanket_sales_order_priority, AvailableSalesLinesPageID},
		{SummaryItemJnlPurchase, gtables.ItemJournalLineTableID, "SUMMARY_ITEM_JNL_PURCHASE", int(s.Show_item_jnl_line_purchases), s.Item_jnl_line_purch_priority, AvailableItemJnlLinesPageID},
		{SummaryItemJnlSale, gtables.ItemJournalLineTableID, "SUMMARY_ITEM_JNL_SALE", int(s.Show_item_jnl_line_sales), s.Item_jnl_line_sales_priority, AvailableItemJnlLinesPageID},
		{SummaryItemJnlPositiveAdj, gtables.ItemJournalLineTableID, "SUMMARY_ITEM_JNL_POSITIVE_ADJMT", int(s.Show_item_jnl_line_pos), s.Item_jnl_line_pos_priority, AvailableItemJnlLinesPageID},
		{SummaryItemJnlNegativeAdj, gtables.ItemJournalLineTableID, "SUMMARY_ITEM_JNL_NEGATIVE_ADJMT", int(s.Show_item_jnl_line_neg), s.Item_jnl_line_neg_priority, AvailableItemJnlLinesPageID},
		{SummaryItemJnlTransfer, gtables.ItemJournalLineTableID, "SUMMARY_ITEM_JNL_TRANSFER", int(s.Show_item_jnl_line_transfer), s.Item_jnl_line_trans_priority, AvailableItemJnlLinesPageID},
		{SummaryBOMJournalLine, gtables.BOMJournalLineTableID, "SUMMARY_BOM_JOURNAL_LINE", int(s.Show_bom_journal_line), s.Bom_journal_line_priority, AvailableBOMJnlLinesPageID},
	}
}

// showLine (ShowLine): a source shown Always, or Optional with the optional lines shown
func showLine(selection int, showAll bool) bool {
	const optional, always = 1, 2
	return selection == always || selection == optional && showAll
}

// SummaryFromSource: the From Type and Subtype of a summary line's reservations
func SummaryFromSource(entryNo int) (int, int, bool) {
	switch {
	case entryNo == SummaryItemLedgerEntry:
		return TypeItemLedgerEntry, 0, true
	case entryNo >= SummaryPurchaseQuote && entryNo <= SummaryBlanketPurchOrder:
		return TypePurchase, purchaseSubtype(entryNo), true
	case entryNo == SummaryRequisitionLine:
		return TypeRequisitionLine, 0, true
	case entryNo >= SummarySalesQuote && entryNo <= SummaryBlanketSalesOrder:
		return TypeSales, salesSubtype(entryNo), true
	case entryNo >= SummaryItemJnlPurchase && entryNo <= SummaryItemJnlTransfer:
		return TypeItemJournal, itemJnlEntryType(entryNo), true
	case entryNo == SummaryBOMJournalLine:
		return TypeBOMJournal, 0, true
	}
	return 0, 0, false
}

// purchaseSubtype / salesSubtype: the document type of a summary line (Quote 0, Order 1,
// Invoice 2, Credit Memo 3, Blanket Order 4)
func purchaseSubtype(entryNo int) int { return purchaseDocumentType(entryNo) }
func salesSubtype(entryNo int) int    { return salesDocumentType(entryNo) }

// SummaryCaption: a summary line's text key (reservmgt's message SUMMARY_…)
func (m *ReservationManagement) SummaryCaption(entryNo int) string {
	for _, l := range m.summaryLines() {
		if l.entryNo == entryNo {
			return l.caption
		}
	}
	return ""
}

// UpdateStatistics (UpdateStatistics): the summary lines of the line reserved for — per source
// of supply the setup shows (optional ones with extendedReservation) the item's total quantity
// up to the availability date, what is reserved from it, what is left and what the line
// reserves from it — and the quantity the line has reserved (from the inventory any time, from
// the others up to the availability date). captions gives each line's Summary Type.
func (m *ReservationManagement) UpdateStatistics(reservSummEntry *gtables.EntrySummary, extendedReservation bool, availabilityDate types.Date, captions func(key string) string) (types.Decimal, error) {
	err := m.calcReservEntry.TestField("for_type")
	if err != nil {
		return types.Decimal{}, err
	}
	reservSummEntry.DeleteAll()
	item := m.calcReservEntry

	calcReservEntry3 := gtables.NewReservationEntry(m.src)
	calcReservEntry3.SetCurrentKey("reservation_status", "item_no", "variant_code", "location_code", "from_type", "from_subtype", "expected_receipt_date")
	calcReservEntry3.SetRange("reservation_status", gtables.ReservationEntry_Reservation_status.Open)
	calcReservEntry3.SetRange("item_no", item.Item_no)
	calcReservEntry3.SetRange("variant_code", item.Variant_code)
	calcReservEntry3.SetRange("location_code", item.Location_code)

	for _, l := range m.summaryLines() {
		if !showLine(l.show, extendedReservation) {
			continue
		}
		totalQuantity, err := m.summaryTotalQuantity(l.entryNo, availabilityDate)
		if err != nil {
			return types.Decimal{}, err
		}
		fromType, fromSubtype, _ := SummaryFromSource(l.entryNo)
		calcReservEntry3.SetRange("from_type", fromType)
		if fromType == TypeItemLedgerEntry {
			calcReservEntry3.SetRange("from_subtype")
			calcReservEntry3.SetRange("expected_receipt_date")
			m.calcReservEntry2.SetRange("expected_receipt_date")
		} else {
			calcReservEntry3.SetRange("from_subtype", fromSubtype)
			calcReservEntry3.SetRange("expected_receipt_date", types.Date{}, availabilityDate)
			m.calcReservEntry2.SetRange("expected_receipt_date", types.Date{}, availabilityDate)
		}
		err = calcReservEntry3.CalcSums("quantity")
		if err != nil {
			return types.Decimal{}, err
		}
		m.calcReservEntry2.SetRange("from_type", fromType)
		if fromType == TypeItemLedgerEntry {
			m.calcReservEntry2.SetRange("from_subtype")
		} else {
			m.calcReservEntry2.SetRange("from_subtype", fromSubtype)
		}
		err = m.calcReservEntry2.CalcSums("quantity")
		if err != nil {
			return types.Decimal{}, err
		}
		reservSummEntry.Init()
		reservSummEntry.Entry_no = l.entryNo
		reservSummEntry.Table_id = l.tableID
		reservSummEntry.Summary_type = types.Text(captions(l.caption))
		reservSummEntry.Total_quantity = totalQuantity
		reservSummEntry.Total_reserved_quantity = calcReservEntry3.Quantity
		reservSummEntry.Total_available_quantity = totalQuantity.Sub(calcReservEntry3.Quantity)
		reservSummEntry.Current_reserved_quantity = m.calcReservEntry2.Quantity
		reservSummEntry.Available_page_id = l.pageID
		err = reservSummEntry.InsertOrError(false)
		if err != nil {
			return types.Decimal{}, err
		}
	}

	m.calcReservEntry2.SetRange("from_type", TypeItemLedgerEntry)
	m.calcReservEntry2.SetRange("from_subtype")
	m.calcReservEntry2.SetRange("expected_receipt_date")
	err = m.calcReservEntry2.CalcSums("quantity")
	if err != nil {
		return types.Decimal{}, err
	}
	qtyReserved := m.calcReservEntry2.Quantity
	m.calcReservEntry2.SetFilter("from_type", "<>6")
	m.calcReservEntry2.SetRange("expected_receipt_date", types.Date{}, availabilityDate)
	err = m.calcReservEntry2.CalcSums("quantity")
	if err != nil {
		return types.Decimal{}, err
	}
	qtyReserved = qtyReserved.Add(m.calcReservEntry2.Quantity)
	m.calcReservEntry2.SetRange("from_type")
	m.calcReservEntry2.SetRange("expected_receipt_date")
	return qtyReserved, nil
}

// summaryTotalQuantity: a source's quantity of the item (inventory: the open entries'
// remaining quantity; documents: the outstanding quantity of the lines up to the availability
// date, sales — which take the item — negative, credit memos the other way)
func (m *ReservationManagement) summaryTotalQuantity(entryNo int, availabilityDate types.Date) (types.Decimal, error) {
	item := m.calcReservEntry
	switch {
	case entryNo == SummaryItemLedgerEntry:
		calcItemLedgEntry := gtables.NewItemLedgerEntry(m.src)
		calcItemLedgEntry.SetRange("item_no", item.Item_no)
		calcItemLedgEntry.SetRange("drop_shipment", false)
		calcItemLedgEntry.SetRange("variant_code", item.Variant_code)
		calcItemLedgEntry.SetRange("location_code", item.Location_code)
		err := calcItemLedgEntry.CalcSums("remaining_quantity")
		return calcItemLedgEntry.Remaining_quantity, err
	case entryNo >= SummaryPurchaseQuote && entryNo <= SummaryBlanketPurchOrder:
		calcPurchLine := m.purchLineFilter(purchaseSubtype(entryNo), availabilityDate)
		err := calcPurchLine.CalcSums("outstanding_qty_base")
		if entryNo == SummaryPurchaseCreditMemo {
			return calcPurchLine.Outstanding_qty_base.Neg(), err
		}
		return calcPurchLine.Outstanding_qty_base, err
	case entryNo == SummaryRequisitionLine:
		calcReqLine := m.reqLineFilter(availabilityDate)
		err := calcReqLine.CalcSums("quantity_base")
		return calcReqLine.Quantity_base, err
	case entryNo >= SummarySalesQuote && entryNo <= SummaryBlanketSalesOrder:
		calcSalesLine := m.salesLineFilter(salesSubtype(entryNo), availabilityDate)
		err := calcSalesLine.CalcSums("outstanding_qty_base")
		if entryNo == SummarySalesCreditMemo {
			return calcSalesLine.Outstanding_qty_base, err
		}
		return calcSalesLine.Outstanding_qty_base.Neg(), err
	case entryNo >= SummaryItemJnlPurchase && entryNo <= SummaryItemJnlTransfer:
		calcItemJnlLine := m.itemJnlLineFilter(itemJnlEntryType(entryNo), availabilityDate)
		err := calcItemJnlLine.CalcSums("quantity_base")
		switch entryNo {
		case SummaryItemJnlSale, SummaryItemJnlNegativeAdj:
			return calcItemJnlLine.Quantity_base.Neg(), err
		}
		return calcItemJnlLine.Quantity_base, err
	case entryNo == SummaryBOMJournalLine:
		calcBOMJnlLine := m.bomJnlLineFilter(availabilityDate)
		err := calcBOMJnlLine.CalcSums("quantity_base")
		return calcBOMJnlLine.Quantity_base, err
	}
	return types.Decimal{}, nil
}

// purchLineFilter: the item lines of a document type for the item, variant and location up to
// the availability date
func (m *ReservationManagement) purchLineFilter(documentType int, availabilityDate types.Date) *gtables.PurchaseLine {
	item := m.calcReservEntry
	calcPurchLine := gtables.NewPurchaseLine(m.src)
	calcPurchLine.SetCurrentKey("document_type", "type", "no", "variant_code", "location_code", "expected_receipt_date", "document_no", "line_no")
	calcPurchLine.SetRange("document_type", documentType)
	calcPurchLine.SetRange("type", gtables.PurchaseLine_Type.Item)
	calcPurchLine.SetRange("no", item.Item_no)
	calcPurchLine.SetRange("variant_code", item.Variant_code)
	calcPurchLine.SetRange("location_code", item.Location_code)
	calcPurchLine.SetRange("expected_receipt_date", types.Date{}, availabilityDate)
	return calcPurchLine
}

func (m *ReservationManagement) salesLineFilter(documentType int, availabilityDate types.Date) *gtables.SalesLine {
	item := m.calcReservEntry
	calcSalesLine := gtables.NewSalesLine(m.src)
	calcSalesLine.SetCurrentKey("document_type", "type", "no", "variant_code", "location_code", "shipment_date", "document_no", "line_no")
	calcSalesLine.SetRange("document_type", documentType)
	calcSalesLine.SetRange("type", gtables.SalesLine_Type.Item)
	calcSalesLine.SetRange("no", item.Item_no)
	calcSalesLine.SetRange("variant_code", item.Variant_code)
	calcSalesLine.SetRange("location_code", item.Location_code)
	calcSalesLine.SetRange("shipment_date", types.Date{}, availabilityDate)
	return calcSalesLine
}

func (m *ReservationManagement) reqLineFilter(availabilityDate types.Date) *gtables.RequisitionLine {
	item := m.calcReservEntry
	calcReqLine := gtables.NewRequisitionLine(m.src)
	calcReqLine.SetCurrentKey("type", "no", "variant_code", "location_code", "expected_receipt_date", "worksheet_template_name", "journal_batch_name", "line_no")
	calcReqLine.SetRange("type", gtables.RequisitionLine_Type.Item)
	calcReqLine.SetRange("no", item.Item_no)
	calcReqLine.SetRange("variant_code", item.Variant_code)
	calcReqLine.SetRange("location_code", item.Location_code)
	calcReqLine.SetRange("expected_receipt_date", types.Date{}, availabilityDate)
	return calcReqLine
}

// itemJnlLineFilter: the item journal lines of an entry type — a transfer by its new location
func (m *ReservationManagement) itemJnlLineFilter(entryType int, availabilityDate types.Date) *gtables.ItemJournalLine {
	item := m.calcReservEntry
	calcItemJnlLine := gtables.NewItemJournalLine(m.src)
	calcItemJnlLine.SetRange("entry_type", entryType)
	calcItemJnlLine.SetRange("item_no", item.Item_no)
	calcItemJnlLine.SetRange("variant_code", item.Variant_code)
	if entryType == int(gtables.ItemJournalLine_Entry_type.Transfer) {
		calcItemJnlLine.SetCurrentKey("entry_type", "item_no", "variant_code", "new_location_code", "posting_date", "journal_template_name", "journal_batch_name", "line_no")
		calcItemJnlLine.SetRange("new_location_code", item.Location_code)
	} else {
		calcItemJnlLine.SetCurrentKey("entry_type", "item_no", "variant_code", "location_code", "posting_date", "journal_template_name", "journal_batch_name", "line_no")
		calcItemJnlLine.SetRange("location_code", item.Location_code)
	}
	calcItemJnlLine.SetRange("posting_date", types.Date{}, availabilityDate)
	return calcItemJnlLine
}

func (m *ReservationManagement) bomJnlLineFilter(availabilityDate types.Date) *gtables.BOMJournalLine {
	item := m.calcReservEntry
	calcBOMJnlLine := gtables.NewBOMJournalLine(m.src)
	calcBOMJnlLine.SetCurrentKey("item_no", "variant_code", "location_code", "posting_date", "journal_template_name", "journal_batch_name", "line_no")
	calcBOMJnlLine.SetRange("item_no", item.Item_no)
	calcBOMJnlLine.SetRange("variant_code", item.Variant_code)
	calcBOMJnlLine.SetRange("location_code", item.Location_code)
	calcBOMJnlLine.SetRange("posting_date", types.Date{}, availabilityDate)
	return calcBOMJnlLine
}

// AutoReserve (AutoReserve): the line reserves what it still needs from the sources by the
// setup's priorities (1 first, 0 never); full when nothing is left
func (m *ReservationManagement) AutoReserve(validToDate types.Date, description types.Text, availabilityDate types.Date) (bool, error) {
	err := m.calcReservEntry.TestField("for_type")
	if err != nil {
		return false, err
	}
	remainingQtyToReserve, err := m.CalculateRemainingQty()
	if err != nil {
		return false, err
	}
	lines := m.summaryLines()
	for i := 1; i <= 10; i++ {
		if remainingQtyToReserve.IsZero() {
			return true, nil
		}
		for _, l := range lines {
			if l.priority != i {
				continue
			}
			remainingQtyToReserve, err = m.AutoReserveOneLine(l.entryNo, remainingQtyToReserve, validToDate, description, availabilityDate)
			if err != nil {
				return false, err
			}
		}
	}
	return remainingQtyToReserve.IsZero(), nil
}

// AutoReserveOneLine (AutoReserveOneLine): the line reserves up to remainingQtyToReserve from
// one source — the open inventory entries in posting order, the other sources' lines from the
// last (latest date) back — what each has not reserved yet; the quantity still to reserve
func (m *ReservationManagement) AutoReserveOneLine(reservSummEntryNo int, remainingQtyToReserve types.Decimal, validToDate types.Date, description types.Text, availabilityDate types.Date) (types.Decimal, error) {
	err := m.calcReservEntry.TestField("for_type")
	if err != nil {
		return remainingQtyToReserve, err
	}
	if remainingQtyToReserve.IsZero() {
		return remainingQtyToReserve, nil
	}
	item := m.calcReservEntry
	// take: the quantity to reserve from a line with this much free
	take := func(free types.Decimal) types.Decimal {
		if free.GreaterThan(remainingQtyToReserve) {
			return remainingQtyToReserve
		}
		return free
	}
	switch {
	case reservSummEntryNo == SummaryItemLedgerEntry:
		calcItemLedgEntry := gtables.NewItemLedgerEntry(m.src)
		calcItemLedgEntry.SetCurrentKey("item_no", "variant_code", "open", "positive", "location_code", "posting_date", "entry_no")
		calcItemLedgEntry.SetRange("item_no", item.Item_no)
		calcItemLedgEntry.SetRange("open", true)
		calcItemLedgEntry.SetRange("positive", true)
		calcItemLedgEntry.SetRange("variant_code", item.Variant_code)
		calcItemLedgEntry.SetRange("location_code", item.Location_code)
		calcItemLedgEntry.SetRange("drop_shipment", false)
		for found := calcItemLedgEntry.FindSet(); found && !remainingQtyToReserve.IsZero(); found = calcItemLedgEntry.Next() {
			free := calcItemLedgEntry.Remaining_quantity.Add(calcItemLedgEntry.Reserved_quantity)
			if !free.IsPositive() {
				continue
			}
			qtyThisLine := take(free)
			err = m.CreateReservation(validToDate, description, types.Date{}, qtyThisLine,
				Source{Type: TypeItemLedgerEntry, RefNo: calcItemLedgEntry.Entry_no},
				calcItemLedgEntry.Variant_code, calcItemLedgEntry.Location_code)
			if err != nil {
				return remainingQtyToReserve, err
			}
			stored := gtables.NewItemLedgerEntry(m.src)
			err = stored.GetOrError(calcItemLedgEntry.Entry_no)
			if err != nil {
				return remainingQtyToReserve, err
			}
			stored.Reserved_quantity = stored.Reserved_quantity.Sub(qtyThisLine)
			err = stored.ModifyOrError(false)
			if err != nil {
				return remainingQtyToReserve, err
			}
			remainingQtyToReserve = remainingQtyToReserve.Sub(qtyThisLine)
		}
	case reservSummEntryNo >= SummaryPurchaseQuote && reservSummEntryNo <= SummaryBlanketPurchOrder:
		calcPurchLine := m.purchLineFilter(purchaseSubtype(reservSummEntryNo), availabilityDate)
		for found := calcPurchLine.Find("+"); found && !remainingQtyToReserve.IsZero(); found = calcPurchLine.Next(-1) {
			qtyThisLine := calcPurchLine.Outstanding_qty_base
			if calcPurchLine.Document_type == gtables.PurchaseLine_Document_type.CreditMemo {
				qtyThisLine = qtyThisLine.Neg()
			}
			if !qtyThisLine.IsPositive() || !qtyThisLine.Add(calcPurchLine.Reserved_qty_base).IsPositive() {
				continue
			}
			qtyThisLine = take(qtyThisLine.Add(calcPurchLine.Reserved_qty_base))
			err = m.CreateReservation(validToDate, description, calcPurchLine.Expected_receipt_date, qtyThisLine,
				PurchLineSource(calcPurchLine), calcPurchLine.Variant_code, calcPurchLine.Location_code)
			if err != nil {
				return remainingQtyToReserve, err
			}
			err = m.changeReservedQty(PurchLineSource(calcPurchLine), qtyThisLine.Neg(), false)
			if err != nil {
				return remainingQtyToReserve, err
			}
			remainingQtyToReserve = remainingQtyToReserve.Sub(qtyThisLine)
		}
	case reservSummEntryNo == SummaryRequisitionLine:
		calcReqLine := m.reqLineFilter(availabilityDate)
		for found := calcReqLine.Find("+"); found && !remainingQtyToReserve.IsZero(); found = calcReqLine.Next(-1) {
			free := calcReqLine.Quantity_base.Add(calcReqLine.Reserved_qty_base)
			if !free.IsPositive() {
				continue
			}
			qtyThisLine := take(free)
			err = m.CreateReservation(validToDate, description, calcReqLine.Expected_receipt_date, qtyThisLine,
				ReqLineSource(calcReqLine), calcReqLine.Variant_code, calcReqLine.Location_code)
			if err != nil {
				return remainingQtyToReserve, err
			}
			err = m.changeReservedQty(ReqLineSource(calcReqLine), qtyThisLine.Neg(), false)
			if err != nil {
				return remainingQtyToReserve, err
			}
			remainingQtyToReserve = remainingQtyToReserve.Sub(qtyThisLine)
		}
	case reservSummEntryNo >= SummarySalesQuote && reservSummEntryNo <= SummaryBlanketSalesOrder:
		calcSalesLine := m.salesLineFilter(salesSubtype(reservSummEntryNo), availabilityDate)
		for found := calcSalesLine.Find("+"); found && !remainingQtyToReserve.IsZero(); found = calcSalesLine.Next(-1) {
			qtyThisLine := calcSalesLine.Outstanding_qty_base.Neg()
			if calcSalesLine.Document_type == gtables.SalesLine_Document_type.CreditMemo {
				qtyThisLine = qtyThisLine.Neg()
			}
			if !qtyThisLine.IsPositive() || !qtyThisLine.Add(calcSalesLine.Reserved_qty_base).IsPositive() {
				continue
			}
			qtyThisLine = take(qtyThisLine.Add(calcSalesLine.Reserved_qty_base))
			err = m.CreateReservation(validToDate, description, calcSalesLine.Shipment_date, qtyThisLine,
				SalesLineSource(calcSalesLine), calcSalesLine.Variant_code, calcSalesLine.Location_code)
			if err != nil {
				return remainingQtyToReserve, err
			}
			err = m.changeReservedQty(SalesLineSource(calcSalesLine), qtyThisLine.Neg(), false)
			if err != nil {
				return remainingQtyToReserve, err
			}
			remainingQtyToReserve = remainingQtyToReserve.Sub(qtyThisLine)
		}
	case reservSummEntryNo >= SummaryItemJnlPurchase && reservSummEntryNo <= SummaryItemJnlTransfer:
		calcItemJnlLine := m.itemJnlLineFilter(itemJnlEntryType(reservSummEntryNo), availabilityDate)
		for found := calcItemJnlLine.Find("+"); found && !remainingQtyToReserve.IsZero(); found = calcItemJnlLine.Next(-1) {
			var qtyThisLine types.Decimal
			switch calcItemJnlLine.Entry_type {
			case gtables.ItemJournalLine_Entry_type.Purchase, gtables.ItemJournalLine_Entry_type.PositiveAdjmt, gtables.ItemJournalLine_Entry_type.Transfer:
				if calcItemJnlLine.Quantity_base.IsPositive() {
					qtyThisLine = calcItemJnlLine.Quantity_base
				}
			default:
				if calcItemJnlLine.Quantity_base.IsNegative() {
					qtyThisLine = calcItemJnlLine.Quantity_base.Neg()
				}
			}
			if !qtyThisLine.IsPositive() || !qtyThisLine.Add(calcItemJnlLine.Negative_reserved_qty_base).IsPositive() {
				continue
			}
			qtyThisLine = take(qtyThisLine.Add(calcItemJnlLine.Negative_reserved_qty_base))
			locationCode := calcItemJnlLine.Location_code
			if calcItemJnlLine.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer {
				locationCode = calcItemJnlLine.New_location_code
			}
			err = m.CreateReservation(validToDate, description, calcItemJnlLine.Posting_date, qtyThisLine,
				ItemJnlLineSource(calcItemJnlLine), calcItemJnlLine.Variant_code, locationCode)
			if err != nil {
				return remainingQtyToReserve, err
			}
			err = m.changeReservedQty(ItemJnlLineSource(calcItemJnlLine), qtyThisLine.Neg(), false)
			if err != nil {
				return remainingQtyToReserve, err
			}
			remainingQtyToReserve = remainingQtyToReserve.Sub(qtyThisLine)
		}
	case reservSummEntryNo == SummaryBOMJournalLine:
		calcBOMJnlLine := m.bomJnlLineFilter(availabilityDate)
		for found := calcBOMJnlLine.Find("+"); found && !remainingQtyToReserve.IsZero(); found = calcBOMJnlLine.Next(-1) {
			free := calcBOMJnlLine.Quantity_base.Add(calcBOMJnlLine.Reserved_qty_base)
			if !free.IsPositive() {
				continue
			}
			qtyThisLine := take(free)
			err = m.CreateReservation(validToDate, description, calcBOMJnlLine.Posting_date, qtyThisLine,
				BOMJnlLineSource(calcBOMJnlLine), calcBOMJnlLine.Variant_code, calcBOMJnlLine.Location_code)
			if err != nil {
				return remainingQtyToReserve, err
			}
			err = m.changeReservedQty(BOMJnlLineSource(calcBOMJnlLine), qtyThisLine.Neg(), false)
			if err != nil {
				return remainingQtyToReserve, err
			}
			remainingQtyToReserve = remainingQtyToReserve.Sub(qtyThisLine)
		}
	}
	return remainingQtyToReserve, nil
}

// CreateReservation (CreateReservation): the line reserved for (Set…ReservFor) reserves a
// quantity from a supply line; the line is saved with its new reserved quantity (the supply
// line's is changed by the caller)
func (m *ReservationManagement) CreateReservation(validToDate types.Date, description types.Text, expectedDate types.Date, quantity types.Decimal, from Source, fromVariantCode, fromLocationCode types.Code) error {
	err := m.calcReservEntry.TestField("for_type")
	if err != nil {
		return err
	}
	switch int(m.calcReservEntry.For_type) {
	case TypeSales:
		err = m.salesLineCreateReservFor(m.forSalesLine, validToDate, description, expectedDate, quantity, from, fromVariantCode, fromLocationCode)
		if err == nil {
			err = m.forSalesLine.ModifyOrError(false)
		}
	case TypeRequisitionLine:
		err = m.reqLineCreateReservFor(m.forReqLine, validToDate, description, expectedDate, quantity, from, fromVariantCode, fromLocationCode)
		if err == nil {
			err = m.forReqLine.ModifyOrError(false)
		}
	case TypePurchase:
		err = m.purchLineCreateReservFor(m.forPurchLine, validToDate, description, expectedDate, quantity, from, fromVariantCode, fromLocationCode)
		if err == nil {
			err = m.forPurchLine.ModifyOrError(false)
		}
	case TypeItemJournal:
		err = m.itemJnlLineCreateReservFor(m.forItemJnlLine, validToDate, description, expectedDate, quantity, from, fromVariantCode, fromLocationCode)
		if err == nil {
			err = m.forItemJnlLine.ModifyOrError(false)
		}
	case TypeBOMJournal:
		err = m.bomJnlLineCreateReservFor(m.forBOMJnlLine, validToDate, description, expectedDate, quantity, from, fromVariantCode, fromLocationCode)
		if err == nil {
			err = m.forBOMJnlLine.ModifyOrError(false)
		}
	}
	return err
}

// CalculateRemainingQty (CalculateRemainingQty): what the line reserved for still has to reserve
func (m *ReservationManagement) CalculateRemainingQty() (types.Decimal, error) {
	err := m.calcReservEntry.TestField("for_type")
	if err != nil {
		return types.Decimal{}, err
	}
	switch int(m.calcReservEntry.For_type) {
	case TypeSales:
		qty, err := SalesLineReservQuantity(m.forSalesLine, false)
		return qty.Sub(m.forSalesLine.Reserved_qty_base), err
	case TypeRequisitionLine:
		qty, err := ReqLineReservQuantity(m.forReqLine, false)
		return qty.Sub(m.forReqLine.Reserved_qty_base), err
	case TypePurchase:
		qty, err := PurchLineReservQuantity(m.forPurchLine, false)
		return qty.Sub(m.forPurchLine.Reserved_qty_base), err
	case TypeItemJournal:
		qty, err := ItemJnlLineReservQuantity(m.forItemJnlLine, false)
		return qty.Sub(m.forItemJnlLine.Positive_reserved_qty_base), err
	case TypeBOMJournal:
		qty, err := BOMJnlLineReservQuantity(m.forBOMJnlLine, false)
		return qty.Sub(m.forBOMJnlLine.Reserved_qty_base), err
	}
	return types.Decimal{}, nil
}

// setReservFor fills CalcReservEntry with the line reserved for (item, variant, location, date)
// and CalcReservEntry2 with its open entries
func (m *ReservationManagement) setReservFor(forType int, itemNo, variantCode, locationCode types.Code, date types.Date, forSrc Source) {
	m.calcReservEntry.Item_no = itemNo
	m.calcReservEntry.Variant_code = variantCode
	m.calcReservEntry.Location_code = locationCode
	m.calcReservEntry.Expected_receipt_date = date
	m.calcReservEntry.For_type = gtables.ReservationEntryFor_type(forType)
	m.calcReservEntry2 = gtables.NewReservationEntry(m.src)
	m.calcReservEntry2.Copy(m.calcReservEntry)
	InitFilterAndSortingFor(m.calcReservEntry2, true)
	FilterFor(m.calcReservEntry2, forSrc)
}

// QtyReservedThisLine (the …UpdateValues' QtyReservedThisLine): what the line reserved for has
// reserved from a supply line
func (m *ReservationManagement) QtyReservedThisLine(from Source, keepDate bool) (types.Decimal, error) {
	calcReservEntry3 := gtables.NewReservationEntry(m.src)
	calcReservEntry3.CopyFilters(m.calcReservEntry2)
	if !keepDate {
		calcReservEntry3.SetRange("expected_receipt_date")
	}
	FilterFrom(calcReservEntry3, from)
	calcReservEntry3.SetRange("reservation_status", gtables.ReservationEntry_Reservation_status.Open)
	err := calcReservEntry3.CalcSums("quantity")
	return calcReservEntry3.Quantity, err
}
