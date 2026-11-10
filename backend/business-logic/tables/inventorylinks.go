package tables

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The inventory pages' links: Item Ledger Entries › Application › Invoicing Entries (page 63)
// and Applied Entries (page 523), and the Available - … pages' Show Entire Worksheet / Show
// Entire Batch (the worksheet or journal page of the line's template on its batch)
const (
	AppliedInvoicingItemEntriesPageID = 63
	ClosingItemApplnEntriesPageID     = 523
)

func init() {
	functions.Register("table", gtables.ItemLedgerEntryTableName, "ShowInvoicingEntries", functions.For(ItemLedgEntryShowInvoicingEntries), functions.WithDialog())
	functions.Register("table", gtables.ItemLedgerEntryTableName, "ShowAppliedEntries", functions.For(ItemLedgEntryShowAppliedEntries), functions.WithDialog())
	functions.Register("table", gtables.RequisitionLineTableName, "ShowEntireWorksheet", functions.For(ReqLineShowEntireWorksheet))
	functions.Register("table", gtables.ItemJournalLineTableName, "ShowEntireBatch", functions.For(ItemJnlLineShowEntireBatch))
	functions.Register("table", gtables.BOMJournalLineTableName, "ShowEntireBatch", functions.For(BOMJnlLineShowEntireBatch))
}

// ItemLedgEntryShowInvoicingEntries (form 63 Applied Invoicing Item Entries): the entries the
// entry closed — Closed by Entry No. is the entry, the entry itself left out
func ItemLedgEntryShowInvoicingEntries(shipItemLedgEntry *gtables.ItemLedgerEntry) (fcodeunits.Result, error) {
	itemLedgEntry := gtables.NewItemLedgerEntry(shipItemLedgEntry)
	itemLedgEntry.SetCurrentKey("closed_by_entry_no")
	itemLedgEntry.SetRange("closed_by_entry_no", shipItemLedgEntry.Entry_no)
	itemLedgEntry.SetFilter("entry_no", system.StrSubstNo("<>%1", shipItemLedgEntry.Entry_no))
	system.RunModal(AppliedInvoicingItemEntriesPageID, itemLedgEntry, false)
	return fcodeunits.Result{Success: true}, nil
}

// ItemLedgEntryShowAppliedEntries (form 523 Closing Item Appln. Entries): the application entries
// of the entry — for an inbound entry those applied to it from an outbound one, for an outbound
// entry its own applications
func ItemLedgEntryShowAppliedEntries(itemLedgEntry *gtables.ItemLedgerEntry) (fcodeunits.Result, error) {
	itemApplnEntry := gtables.NewItemApplicationEntry(itemLedgEntry)
	if itemLedgEntry.Positive {
		itemApplnEntry.SetCurrentKey("item_ledger_entry_no", "applied_to_entry_no", "applied_from_entry_no")
		itemApplnEntry.SetRange("applied_to_entry_no", itemLedgEntry.Entry_no)
		itemApplnEntry.SetFilter("applied_from_entry_no", "<>0")
	} else {
		itemApplnEntry.SetCurrentKey("item_ledger_entry_no", "applied_to_entry_no", "applied_from_entry_no")
		itemApplnEntry.SetRange("item_ledger_entry_no", itemLedgEntry.Entry_no)
		itemApplnEntry.SetRange("applied_from_entry_no", itemLedgEntry.Entry_no)
	}
	system.RunModal(ClosingItemApplnEntriesPageID, itemApplnEntry, false)
	return fcodeunits.Result{Success: true}, nil
}

// showJournalBatch opens a journal or worksheet page on a template and batch, Esc going back
func showJournalBatch(pageID int, template, batch string) fcodeunits.Result {
	return fcodeunits.Result{Success: true, OpenPage: &fcodeunits.PageLink{PageID: pageID,
		Params: map[string]string{"template": template, "batch": batch}, Return: true}}
}

// ReqLineShowEntireWorksheet (Show Entire Worksheet): the line's worksheet on its template's page
func ReqLineShowEntireWorksheet(reqLine *gtables.RequisitionLine) (fcodeunits.Result, error) {
	reqWkshTemplate := gtables.NewReqWkshTemplate(reqLine)
	err := reqWkshTemplate.GetOrError(reqLine.Worksheet_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return showJournalBatch(reqWkshTemplate.Page_id, reqLine.Worksheet_template_name.String(), reqLine.Journal_batch_name.String()), nil
}

// ItemJnlLineShowEntireBatch (Show Entire Batch): the line's batch on its template's page
func ItemJnlLineShowEntireBatch(itemJnlLine *gtables.ItemJournalLine) (fcodeunits.Result, error) {
	itemJnlTemplate := gtables.NewItemJournalTemplate(itemJnlLine)
	err := itemJnlTemplate.GetOrError(itemJnlLine.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return showJournalBatch(itemJnlTemplate.Page_id, itemJnlLine.Journal_template_name.String(), itemJnlLine.Journal_batch_name.String()), nil
}

// BOMJnlLineShowEntireBatch (Show Entire Batch): the line's batch on its template's page
func BOMJnlLineShowEntireBatch(bomJnlLine *gtables.BOMJournalLine) (fcodeunits.Result, error) {
	bomJnlTemplate := gtables.NewBOMJournalTemplate(bomJnlLine)
	err := bomJnlTemplate.GetOrError(bomJnlLine.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return showJournalBatch(bomJnlTemplate.Page_id, bomJnlLine.Journal_template_name.String(), bomJnlLine.Journal_batch_name.String()), nil
}
