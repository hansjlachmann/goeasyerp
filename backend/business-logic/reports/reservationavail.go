package reports

import (
	"strconv"

	"github.com/hansjlachmann/openerp/backend/business-logic/reservmgt"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Reports 209 Sales Reservation Avail., 409 Purchase Reservation Avail. and 723 Item Jnl.
// Reservation Avail.: per document (journal batch) the item lines with what they reserved —
// their status (Shipped / Full / Partial / No Shipment; the journal: Qty. = 0 and … Posting), the
// latest expected receipt date of what they reserved from documents and the quantity reserved
// from the inventory; the reservation entries below each line (Show Reservation Entries) and the
// document's status after its lines. 209/409 can set the order lines' Qty. to Ship / Receive to
// the quantity on hand. The classic group footer per document is a data item after the lines'
// entries that runs on a document's last line (the engine has no group sections).
const (
	SalesReservationAvailID    = 209
	PurchaseReservationAvailID = 409
	ItemJnlReservationAvailID  = 723
)

func init() {
	Register(SalesReservationAvailID, "sales-reservation-avail", func() Report { return &salesReservationAvail{} })
	Register(PurchaseReservationAvailID, "purchase-reservation-avail", func() Report { return &purchaseReservationAvail{} })
	Register(ItemJnlReservationAvailID, "item-jnl-reservation-avail", func() Report { return &itemJnlReservationAvail{} })
}

// The line and document status (classic ' ,Shipped,Full Shipment,Partial Shipment,No Shipment';
// the journal: ' ,Qty. = 0,Full Posting,Partial Posting,No Posting'); texts STATUS_1 … STATUS_4
const (
	statusBlank = iota
	statusShipped
	statusFull
	statusPartial
	statusNo
)

// reservAvail is what the three reports share: the options, the line's and the document's
// status, dates and quantities
type reservAvail struct {
	showLines              bool
	showReservationEntries bool
	modifyQtyToShip        bool
	clearDocumentStatus    bool
	lastLine               bool // the line is the document's last (its footer follows)
	lineStatus             int
	documentStatus         int
	lineReceiptDate        types.Date
	documentReceiptDate    types.Date
	lineQuantityOnHand     types.Decimal
}

func reservAvailOptions(showLines string) []Option {
	return []Option{
		{Name: showLines, Type: "bool", Default: "true"},
		{Name: "show_reservation_entries", Type: "bool", Default: "false"},
	}
}

// read takes the request page's choices (no reservation entries without the lines)
func (r *reservAvail) read(run *Run, showLines string) {
	r.showLines = run.OptionBool(showLines)
	r.showReservationEntries = run.OptionBool("show_reservation_entries") && r.showLines
	run.SetValue("show_lines", r.showLines)
	run.SetValue("show_reservation_entries", r.showReservationEntries)
}

// calcLine (the lines' OnAfterGetRecord): the line's status, receipt date and quantity on hand.
// done: nothing outstanding (Shipped; the journal: Qty. = 0); reservQty: what it can reserve;
// reserved: its Reserved Qty. (Base) compared with qty
func (r *reservAvail) calcLine(run *Run, reserveNever, done bool, reservQty types.Decimal, s reservmgt.Source, reserved, qty types.Decimal) {
	if !reserveNever {
		r.lineReceiptDate = types.Date{}
		r.lineQuantityOnHand = types.Decimal{}
		if done {
			r.lineStatus = statusShipped
		} else {
			if reservQty.IsPositive() {
				reservEntry := gtables.NewReservationEntry(run)
				reservmgt.InitFilterAndSortingLookupFor(reservEntry, true)
				reservmgt.FilterFor(reservEntry, s)
				reservEntry.SetFilter("from_type", "<>"+strconv.Itoa(reservmgt.TypeItemLedgerEntry))
				if reservEntry.FindLast() {
					r.lineReceiptDate = reservEntry.Expected_receipt_date
					reservEntry.SetRange("from_type", reservmgt.TypeItemLedgerEntry)
					if reservEntry.FindSet() {
						for found := true; found; found = reservEntry.Next() {
							r.lineQuantityOnHand = r.lineQuantityOnHand.Add(reservEntry.Quantity)
						}
						r.lineStatus = statusPartial
					} else {
						r.lineStatus = statusNo
					}
				} else {
					r.lineQuantityOnHand = reserved
					if qty.Abs().Equal(reserved.Abs()) {
						r.lineStatus = statusFull
					} else {
						if reserved.IsZero() {
							r.lineStatus = statusNo
						} else {
							r.lineStatus = statusPartial
						}
					}
				}
			} else {
				r.lineStatus = statusFull
			}
		}
	} else {
		r.lineReceiptDate = types.Date{}
		r.lineQuantityOnHand = reservQty
		if done {
			r.lineStatus = statusShipped
		} else {
			r.lineStatus = statusFull
		}
	}
}

// addToDocument: the document's status and receipt date with the line's (the first line of a
// document starts them afresh); returns whether the line is the document's first
func (r *reservAvail) addToDocument() bool {
	firstLine := r.clearDocumentStatus
	if r.clearDocumentStatus {
		r.documentReceiptDate = types.Date{}
		r.documentStatus = statusBlank
		r.clearDocumentStatus = false
	}
	if r.lineReceiptDate.After(r.documentReceiptDate) {
		r.documentReceiptDate = r.lineReceiptDate
	}
	switch r.documentStatus {
	case statusBlank:
		r.documentStatus = r.lineStatus
	case statusShipped:
		switch r.lineStatus {
		case statusShipped:
			r.documentStatus = statusShipped
		case statusFull, statusPartial:
			r.documentStatus = statusPartial
		case statusNo:
			r.documentStatus = statusNo
		}
	case statusFull:
		switch r.lineStatus {
		case statusShipped, statusFull:
			r.documentStatus = statusFull
		case statusPartial, statusNo:
			r.documentStatus = statusPartial
		}
	case statusPartial:
		r.documentStatus = statusPartial
	case statusNo:
		switch r.lineStatus {
		case statusShipped, statusNo:
			r.documentStatus = statusNo
		case statusFull, statusPartial:
			r.documentStatus = statusPartial
		}
	}
	return firstLine
}

func statusText(run *Run, status int) string {
	if status == statusBlank {
		return ""
	}
	return run.Text("STATUS_" + strconv.Itoa(status))
}

// setLineValues: what the line's body prints
func (r *reservAvail) setLineValues(run *Run, firstLine bool, documentText string) {
	run.SetValue("first_line", firstLine && r.showLines)
	run.SetValue("document_text", documentText)
	run.SetValue("line_status", statusText(run, r.lineStatus))
	run.SetValue("line_receipt_date", r.lineReceiptDate)
	run.SetValue("line_quantity_on_hand", r.lineQuantityOnHand)
}

// entryOnAfterGetRecord (the Reservation Entry data item): the entry's supply, its date (none
// from the inventory) and quantity on hand; a supply that is a document moves the line's and
// the document's receipt date
func (r *reservAvail) entryOnAfterGetRecord(reservEntry *gtables.ReservationEntry, run *Run) error {
	showReservDate := reservEntry.Expected_receipt_date
	if int(reservEntry.From_type) == reservmgt.TypeItemLedgerEntry {
		showReservDate = types.Date{}
	}
	run.SetValue("reserv_text", reservmgt.CreateFromText(reservEntry, fcodeunits.CurrentLanguage()))
	run.SetValue("show_reserv_date", showReservDate)
	entryQuantityOnHand := reservEntry.Quantity
	if int(reservEntry.From_type) != reservmgt.TypeItemLedgerEntry {
		if reservEntry.Expected_receipt_date.After(r.lineReceiptDate) {
			r.lineReceiptDate = reservEntry.Expected_receipt_date
		}
		if reservEntry.Expected_receipt_date.After(r.documentReceiptDate) {
			r.documentReceiptDate = reservEntry.Expected_receipt_date
		}
		entryQuantityOnHand = types.Decimal{}
	}
	run.SetValue("entry_quantity_on_hand", entryQuantityOnHand)
	return nil
}

// footerDataItem: the document's footer, after the entries of its last line
func (r *reservAvail) footerDataItem(setValues func(run *Run)) *DataItem {
	return &DataItem{
		Name:  "DocumentFooter",
		Table: IntegerTable,
		OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
			if r.lastLine {
				rec.SetRange("number", 1, 1)
			} else {
				rec.SetRange("number", 1, 0)
			}
			return nil
		}),
		OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
			run.SetValue("document_status", statusText(run, r.documentStatus))
			run.SetValue("document_receipt_date", r.documentReceiptDate)
			setValues(run)
			r.clearDocumentStatus = true
			return nil
		}),
	}
}

// entryDataItem: the line's open reservation entries as the demand
func (r *reservAvail) entryDataItem(forType int, link map[string]string, subtype func() int) *DataItem {
	return &DataItem{
		Name:  "Reservation_Entry",
		Table: gtables.ReservationEntryTableName,
		Key: []string{"reservation_status", "for_type", "for_subtype", "for_id", "for_batch_name",
			"for_prod_order_line", "for_ref_no", "expected_receipt_date"},
		TableView: []ftables.FieldFilter{
			{Field: "reservation_status", Expression: strconv.Itoa(int(gtables.ReservationEntry_Reservation_status.Open))},
			{Field: "for_type", Expression: strconv.Itoa(forType)},
			{Field: "for_prod_order_line", Expression: "0"},
		},
		Link: link,
		OnPreDataItem: Trigger(func(reservEntry *gtables.ReservationEntry, run *Run) error {
			reservEntry.SetRange("for_subtype", subtype())
			return nil
		}),
		OnAfterGetRecord: Trigger(r.entryOnAfterGetRecord),
	}
}

func optionCaption(table, field, value, lang string) string {
	return i18n.GetInstance().OptionCaption(table, field, i18n.Key(value), lang)
}

// Report 209 Sales Reservation Avail.
type salesReservationAvail struct {
	reservAvail
	salesLine   *gtables.SalesLine
	salesHeader *gtables.SalesHeader
}

func (r *salesReservationAvail) Definition() *Definition {
	return &Definition{
		Options:     append(reservAvailOptions("show_sales_lines"), Option{Name: "modify_qty_to_ship", Type: "bool", Default: "false"}),
		Transaction: true,
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			r.read(run, "show_sales_lines")
			r.modifyQtyToShip = run.OptionBool("modify_qty_to_ship")
			return nil
		},
		DataItems: []*DataItem{{
			Name:                "Sales_Line",
			Table:               gtables.SalesLineTableName,
			Key:                 []string{"document_type", "document_no", "line_no"},
			TableView:           []ftables.FieldFilter{{Field: "type", Expression: strconv.Itoa(int(gtables.SalesLine_Type.Item))}},
			RequestFilterFields: []string{"document_type", "document_no", "no", "location_code"},
			OnPreDataItem: Trigger(func(salesLine *gtables.SalesLine, run *Run) error {
				setFilterText(run, salesLine)
				r.clearDocumentStatus = true
				return nil
			}),
			OnAfterGetRecord: Trigger(r.salesLineOnAfterGetRecord),
			DataItems: []*DataItem{
				r.entryDataItem(reservmgt.TypeSales, map[string]string{"for_id": "document_no", "for_ref_no": "line_no"},
					func() int { return int(r.salesLine.Document_type) }),
				r.footerDataItem(func(run *Run) {
					lang := fcodeunits.CurrentLanguage()
					run.SetValue("header_shipment_date", r.salesHeader.Shipment_date)
					run.SetValue("header_reserve", optionCaption(gtables.SalesHeaderTableName, "reserve", r.salesHeader.Reserve.String(), lang))
				}),
			},
		}},
	}
}

func (r *salesReservationAvail) salesLineOnAfterGetRecord(salesLine *gtables.SalesLine, run *Run) error {
	r.salesLine = salesLine
	reservQty, err := reservmgt.SalesLineReservQuantity(salesLine, false)
	if err != nil {
		return err
	}
	r.calcLine(run, salesLine.Reserve == gtables.SalesLine_Reserve.Never, salesLine.Outstanding_qty_base.IsZero(), reservQty,
		reservmgt.SalesLineSource(salesLine), salesLine.Reserved_qty_base, salesLine.Outstanding_qty_base)

	if r.modifyQtyToShip && salesLine.Document_type == gtables.SalesLine_Document_type.Order &&
		!salesLine.Qty_to_ship_base.Equal(r.lineQuantityOnHand) {
		err = salesLine.Validate("qty_to_ship_base", r.lineQuantityOnHand)
		if err != nil {
			return err
		}
		err = salesLine.ModifyOrError(false)
		if err != nil {
			return err
		}
	}

	if r.clearDocumentStatus {
		r.salesHeader = gtables.NewSalesHeader(run)
		err = r.salesHeader.GetOrError(map[string]interface{}{"document_type": int(salesLine.Document_type), "no": salesLine.Document_no})
		if err != nil {
			return err
		}
	}
	firstLine := r.addToDocument()

	next := gtables.NewSalesLine(run)
	next.CopyFilters(salesLine)
	next.SetRange("document_type", salesLine.Document_type)
	next.SetRange("document_no", salesLine.Document_no)
	next.SetFilter("line_no", ">"+strconv.Itoa(salesLine.Line_no))
	r.lastLine = next.IsEmpty()

	lang := fcodeunits.CurrentLanguage()
	r.setLineValues(run, firstLine, system.StrSubstNo("%1 %2",
		optionCaption(gtables.SalesLineTableName, "document_type", salesLine.Document_type.String(), lang), salesLine.Document_no))
	return nil
}

// Report 409 Purchase Reservation Avail.
type purchaseReservationAvail struct {
	reservAvail
	purchLine   *gtables.PurchaseLine
	purchHeader *gtables.PurchaseHeader
}

func (r *purchaseReservationAvail) Definition() *Definition {
	return &Definition{
		Options:     append(reservAvailOptions("show_purchase_lines"), Option{Name: "modify_qty_to_receive", Type: "bool", Default: "false"}),
		Transaction: true,
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			r.read(run, "show_purchase_lines")
			r.modifyQtyToShip = run.OptionBool("modify_qty_to_receive")
			return nil
		},
		DataItems: []*DataItem{{
			Name:                "Purchase_Line",
			Table:               gtables.PurchaseLineTableName,
			Key:                 []string{"document_type", "document_no", "line_no"},
			TableView:           []ftables.FieldFilter{{Field: "type", Expression: strconv.Itoa(int(gtables.PurchaseLine_Type.Item))}},
			RequestFilterFields: []string{"document_type", "document_no", "line_no", "no", "location_code"},
			OnPreDataItem: Trigger(func(purchLine *gtables.PurchaseLine, run *Run) error {
				setFilterText(run, purchLine)
				r.clearDocumentStatus = true
				return nil
			}),
			OnAfterGetRecord: Trigger(r.purchLineOnAfterGetRecord),
			DataItems: []*DataItem{
				r.entryDataItem(reservmgt.TypePurchase, map[string]string{"for_id": "document_no", "for_ref_no": "line_no"},
					func() int { return int(r.purchLine.Document_type) }),
				r.footerDataItem(func(run *Run) {
					lang := fcodeunits.CurrentLanguage()
					run.SetValue("header_expected_receipt_date", r.purchHeader.Expected_receipt_date)
					run.SetValue("header_reserve", optionCaption(gtables.PurchaseHeaderTableName, "reserve", r.purchHeader.Reserve.String(), lang))
				}),
			},
		}},
	}
}

func (r *purchaseReservationAvail) purchLineOnAfterGetRecord(purchLine *gtables.PurchaseLine, run *Run) error {
	r.purchLine = purchLine
	reservQty, err := reservmgt.PurchLineReservQuantity(purchLine, false)
	if err != nil {
		return err
	}
	r.calcLine(run, purchLine.Reserve == gtables.PurchaseLine_Reserve.Never, purchLine.Outstanding_qty_base.IsZero(), reservQty,
		reservmgt.PurchLineSource(purchLine), purchLine.Reserved_qty_base, purchLine.Outstanding_qty_base)

	if r.modifyQtyToShip && purchLine.Document_type == gtables.PurchaseLine_Document_type.Order &&
		!purchLine.Qty_to_receive_base.Equal(r.lineQuantityOnHand) {
		err = purchLine.Validate("qty_to_receive_base", r.lineQuantityOnHand)
		if err != nil {
			return err
		}
		err = purchLine.ModifyOrError(false)
		if err != nil {
			return err
		}
	}

	if r.clearDocumentStatus {
		r.purchHeader = gtables.NewPurchaseHeader(run)
		err = r.purchHeader.GetOrError(map[string]interface{}{"document_type": int(purchLine.Document_type), "no": purchLine.Document_no})
		if err != nil {
			return err
		}
	}
	firstLine := r.addToDocument()

	next := gtables.NewPurchaseLine(run)
	next.CopyFilters(purchLine)
	next.SetRange("document_type", purchLine.Document_type)
	next.SetRange("document_no", purchLine.Document_no)
	next.SetFilter("line_no", ">"+strconv.Itoa(purchLine.Line_no))
	r.lastLine = next.IsEmpty()

	lang := fcodeunits.CurrentLanguage()
	r.setLineValues(run, firstLine, system.StrSubstNo("%1 %2",
		optionCaption(gtables.PurchaseLineTableName, "document_type", purchLine.Document_type.String(), lang), purchLine.Document_no))
	return nil
}

// Report 723 Item Jnl. Reservation Avail.: per journal batch
type itemJnlReservationAvail struct {
	reservAvail
	itemJnlLine *gtables.ItemJournalLine
}

func (r *itemJnlReservationAvail) Definition() *Definition {
	return &Definition{
		Options: reservAvailOptions("show_item_jnl_lines"),
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			r.read(run, "show_item_jnl_lines")
			return nil
		},
		DataItems: []*DataItem{{
			Name:                "Item_Journal_Line",
			Table:               gtables.ItemJournalLineTableName,
			Key:                 []string{"journal_template_name", "journal_batch_name", "line_no"},
			RequestFilterFields: []string{"journal_template_name", "journal_batch_name"},
			OnPreDataItem: Trigger(func(itemJnlLine *gtables.ItemJournalLine, run *Run) error {
				setFilterText(run, itemJnlLine)
				r.clearDocumentStatus = true
				return nil
			}),
			OnAfterGetRecord: Trigger(r.itemJnlLineOnAfterGetRecord),
			DataItems: []*DataItem{
				r.entryDataItem(reservmgt.TypeItemJournal,
					map[string]string{"for_id": "journal_template_name", "for_batch_name": "journal_batch_name", "for_ref_no": "line_no"},
					func() int { return int(r.itemJnlLine.Entry_type) }),
				r.footerDataItem(func(run *Run) {}),
			},
		}},
	}
}

func (r *itemJnlReservationAvail) itemJnlLineOnAfterGetRecord(itemJnlLine *gtables.ItemJournalLine, run *Run) error {
	r.itemJnlLine = itemJnlLine
	reservQty, err := reservmgt.ItemJnlLineReservQuantity(itemJnlLine, false)
	if err != nil {
		return err
	}
	r.calcLine(run, itemJnlLine.Reserve == gtables.ItemJournalLine_Reserve.Never, itemJnlLine.Quantity_base.IsZero(), reservQty,
		reservmgt.ItemJnlLineSource(itemJnlLine), itemJnlLine.Negative_reserved_qty_base, itemJnlLine.Quantity_base)
	firstLine := r.addToDocument()

	next := gtables.NewItemJournalLine(run)
	next.CopyFilters(itemJnlLine)
	next.SetRange("journal_template_name", itemJnlLine.Journal_template_name)
	next.SetRange("journal_batch_name", itemJnlLine.Journal_batch_name)
	next.SetFilter("line_no", ">"+strconv.Itoa(itemJnlLine.Line_no))
	r.lastLine = next.IsEmpty()

	r.setLineValues(run, firstLine, run.Text("BATCH_TEXT", itemJnlLine.Journal_template_name, itemJnlLine.Journal_batch_name))
	return nil
}
