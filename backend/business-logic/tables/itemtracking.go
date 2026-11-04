package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Item tracking on the lines: the lookup of Serial No. and Lot No. (classic OnLookup →
// SelectItemEntry) shows the open item ledger entries of the line's item — those of the sign
// the line takes — and the picked entry gives the line its location, serial and lot number (purchase and item
// journal lines: its bin too).
// The pages run them as the fields' on_lookup.
const itemLedgerEntriesPageID = 38

func init() {
	functions.Register("table", gtables.SalesLineTableName, "SelectItemEntry", functions.ForAssistEdit(SalesLineSelectItemEntry), functions.WithAssistEdit())
	functions.Register("table", gtables.PurchaseLineTableName, "SelectItemEntry", functions.ForAssistEdit(PurchLineSelectItemEntry), functions.WithAssistEdit())
	functions.Register("table", gtables.ItemJournalLineTableName, "SelectItemEntry", functions.ForAssistEdit(func(rec *gtables.ItemJournalLine) (bool, error) {
		return ItemJnlLineSelectItemEntry(rec, false)
	}), functions.WithAssistEdit())
	functions.Register("table", gtables.ItemJournalLineTableName, "SelectNewItemEntry", functions.ForAssistEdit(func(rec *gtables.ItemJournalLine) (bool, error) {
		return ItemJnlLineSelectItemEntry(rec, true)
	}), functions.WithAssistEdit())
}

// SalesLineSelectItemEntry (Sales Line SelectItemEntry): the item's open entries — an order its
// inventory, a credit memo the entries it brings back — the picked one's location, serial and
// lot number and unit cost
func SalesLineSelectItemEntry(rec *gtables.SalesLine) (bool, error) {
	err := rec.TestField("type", gtables.SalesLine_Type.Item)
	if err != nil {
		return false, err
	}
	itemLedgEntry := gtables.NewItemLedgerEntry(rec)
	itemLedgEntry.SetCurrentKey("item_no", "variant_code", "open")
	itemLedgEntry.SetRange("item_no", rec.No)
	itemLedgEntry.SetRange("open", true)
	if !rec.Quantity.IsZero() {
		if rec.Document_type == gtables.SalesLine_Document_type.CreditMemo {
			itemLedgEntry.SetRange("positive", rec.Quantity.IsNegative())
		} else {
			itemLedgEntry.SetRange("positive", rec.Quantity.IsPositive())
		}
	}
	if system.RunModal(itemLedgerEntriesPageID, itemLedgEntry, true) == system.ActionLookupOK {
		rec.Location_code = itemLedgEntry.Location_code
		rec.Serial_no = itemLedgEntry.Serial_no
		rec.Lot_no = itemLedgEntry.Lot_no
		rec.Unit_cost_lcy = itemLedgEntry.Unit_cost
		return true, nil
	}
	return false, nil
}

// PurchLineSelectItemEntry (Purchase Line SelectItemEntry): the item's open entries — a return
// the inventory it sends back — the picked one's location, serial and lot number
func PurchLineSelectItemEntry(rec *gtables.PurchaseLine) (bool, error) {
	err := rec.TestField("type", gtables.PurchaseLine_Type.Item)
	if err != nil {
		return false, err
	}
	itemLedgEntry := gtables.NewItemLedgerEntry(rec)
	itemLedgEntry.SetCurrentKey("item_no", "variant_code", "open")
	itemLedgEntry.SetRange("item_no", rec.No)
	itemLedgEntry.SetRange("open", true)
	if !rec.Quantity.IsZero() {
		if rec.Document_type == gtables.PurchaseLine_Document_type.CreditMemo {
			itemLedgEntry.SetRange("positive", rec.Quantity.IsPositive())
		} else {
			itemLedgEntry.SetRange("positive", rec.Quantity.IsNegative())
		}
	}
	if system.RunModal(itemLedgerEntriesPageID, itemLedgEntry, true) == system.ActionLookupOK {
		rec.Location_code = itemLedgEntry.Location_code
		rec.Serial_no = itemLedgEntry.Serial_no
		rec.Bin_code = itemLedgEntry.Bin_code
		rec.Lot_no = itemLedgEntry.Lot_no
		return true, nil
	}
	return false, nil
}

// ItemJnlLineSelectItemEntry (Item Journal Line SelectItemEntry): the item's open entries of the
// sign the line takes (newFields: New Serial No./New Lot No. of a transfer — the other sign);
// the picked one's location, serial and lot number (or the new ones)
func ItemJnlLineSelectItemEntry(rec *gtables.ItemJournalLine, newFields bool) (bool, error) {
	itemLedgEntry := gtables.NewItemLedgerEntry(rec)
	itemLedgEntry.SetCurrentKey("item_no", "variant_code", "open")
	itemLedgEntry.SetRange("item_no", rec.Item_no)
	itemLedgEntry.SetRange("open", true)
	if !rec.Quantity.IsZero() {
		if newFields {
			itemLedgEntry.SetRange("positive", rec.Quantity.IsNegative())
		} else {
			switch rec.Entry_type {
			case gtables.ItemJournalLine_Entry_type.Sale, gtables.ItemJournalLine_Entry_type.NegativeAdjmt, gtables.ItemJournalLine_Entry_type.Transfer:
				itemLedgEntry.SetRange("positive", rec.Quantity.IsPositive())
			default:
				itemLedgEntry.SetRange("positive", rec.Quantity.IsNegative())
			}
		}
	}
	if system.RunModal(itemLedgerEntriesPageID, itemLedgEntry, true) == system.ActionLookupOK {
		if newFields {
			rec.New_location_code = itemLedgEntry.Location_code
			rec.New_serial_no = itemLedgEntry.Serial_no
			rec.New_bin_code = itemLedgEntry.Bin_code
			rec.New_lot_no = itemLedgEntry.Lot_no
		} else {
			rec.Location_code = itemLedgEntry.Location_code
			rec.Serial_no = itemLedgEntry.Serial_no
			rec.Bin_code = itemLedgEntry.Bin_code
			rec.Lot_no = itemLedgEntry.Lot_no
		}
		return true, nil
	}
	return false, nil
}
