package reports

import (
	"strconv"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 798 Date Compress Item Ledger: the closed item ledger entries of closed fiscal years
// whose cost is adjusted (with those they were applied to) become one entry per item, inventory
// and general posting groups, entry type, period and sign of the quantity — and per the
// retained fields; their item application entries are deleted, each new entry gets one of its
// own; an item register of the run.
const DateCompressItemLedgerID = 798

func init() {
	Register(DateCompressItemLedgerID, "date-compress-item-ledger", func() Report { return &dateCompressItemLedger{} })
}

type dateCompressItemLedger struct {
	dateCompression
	ownRegister
	sourceCodeSetup  *gtables.SourceCodeSetup
	newItemLedgEntry *gtables.ItemLedgerEntry
	itemApplnEntry   *gtables.ItemApplicationEntry
	lastEntryNo      int
	lastApplnEntryNo int
	window           *system.Window
}

func (r *dateCompressItemLedger) Definition() *Definition {
	r.tableID = gtables.ItemLedgerEntryTableID
	r.fields = []retainField{{name: "document_no"}, {name: "location_code"}, {name: "serial_no"}, {name: "source_type"}, {name: "source_no"},
		{name: "source_posting_group"}, {name: "salespers_purch_code"}, {name: "transaction_type"}, {name: "transport_method"},
		{name: "country_code"}, {name: "department_code"}, {name: "project_code"}, {name: "variant_code"}, {name: "lot_no"}}
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        dateComprOptions(true, r.fields),
		Transaction:    true,
		OnInitReport:   r.onInitReport,
		OnPreReport: func(run *Run) error {
			r.read(run)
			r.filter = requestFilter(run, "Item_Ledger_Entry")
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Item_Ledger_Entry", Table: gtables.ItemLedgerEntryTableName, Key: []string{"cost_is_adjusted", "item_no", "adjust_invoice_cost"},
			TableView:           []ftables.FieldFilter{{Field: "cost_is_adjusted", Expression: "true"}, {Field: "open", Expression: "false"}},
			RequestFilterFields: []string{"item_no", "inventory_posting_group"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *dateCompressItemLedger) onPreDataItem(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))

	r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	err = r.sourceCodeSetup.TestField("compress_item_ledger")
	if err != nil {
		return err
	}

	r.newItemLedgEntry = gtables.NewItemLedgerEntry(run)
	r.newItemLedgEntry.LockTable()
	r.itemApplnEntry = gtables.NewItemApplicationEntry(run)
	r.itemApplnEntry.LockTable()
	r.reg = gtables.NewItemRegister(run)
	r.reg.LockTable()
	r.dateComprReg.LockTable()

	itemLedgEntry2 := gtables.NewItemLedgerEntry(run)
	r.lastEntryNo = 0
	if itemLedgEntry2.FindLast() {
		r.lastEntryNo = itemLedgEntry2.Entry_no
	}
	itemLedgEntry.SetRange("entry_no", 0, r.lastEntryNo)
	itemLedgEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)
	r.lastApplnEntryNo = 0
	if r.itemApplnEntry.FindLast() {
		r.lastApplnEntryNo = r.itemApplnEntry.Entry_no
	}

	r.initRegisters(run)
	return nil
}

func (r *dateCompressItemLedger) onAfterGetRecord(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) error {
	// compressed with an entry before it
	stillThere := gtables.NewItemLedgerEntry(run)
	if !stillThere.Get(itemLedgEntry.Entry_no) {
		return Skip()
	}
	if !r.itemEntryAdjusted(itemLedgEntry, run) {
		return Skip()
	}

	itemLedgEntry2 := gtables.NewItemLedgerEntry(run)
	itemLedgEntry2.Copy(itemLedgEntry)
	itemLedgEntry2.SetCurrentKey("cost_is_adjusted", "item_no", "adjust_invoice_cost")
	itemLedgEntry2.CopyFilters(itemLedgEntry)
	itemLedgEntry2.SetRange("item_no", itemLedgEntry2.Item_no)
	dateFilter, periodStart, err := r.dateFilter(itemLedgEntry2.Posting_date, true)
	if err != nil {
		return err
	}
	itemLedgEntry2.SetFilter("posting_date", dateFilter)
	itemLedgEntry2.SetRange("inventory_posting_group", itemLedgEntry2.Inventory_posting_group)
	itemLedgEntry2.SetRange("gen_bus_posting_group", itemLedgEntry2.Gen_bus_posting_group)
	itemLedgEntry2.SetRange("gen_prod_posting_group", itemLedgEntry2.Gen_prod_posting_group)
	itemLedgEntry2.SetRange("entry_type", itemLedgEntry2.Entry_type)

	r.lastEntryNo++

	newItemLedgEntry := r.newItemLedgEntry
	newItemLedgEntry.Init()
	newItemLedgEntry.Entry_no = r.lastEntryNo
	newItemLedgEntry.Item_no = itemLedgEntry2.Item_no
	newItemLedgEntry.Posting_date = periodStart
	newItemLedgEntry.Description = r.description
	newItemLedgEntry.Inventory_posting_group = itemLedgEntry2.Inventory_posting_group
	newItemLedgEntry.Gen_bus_posting_group = itemLedgEntry2.Gen_bus_posting_group
	newItemLedgEntry.Gen_prod_posting_group = itemLedgEntry2.Gen_prod_posting_group
	newItemLedgEntry.Entry_type = itemLedgEntry2.Entry_type
	newItemLedgEntry.Cost_is_adjusted = true
	newItemLedgEntry.Source_code = r.sourceCodeSetup.Compress_item_ledger
	newItemLedgEntry.User_id = types.Code(fcodeunits.CurrentUserID())
	r.window.Update(1, newItemLedgEntry.Item_no)
	r.window.Update(2, newItemLedgEntry.Posting_date)
	r.newRecord()
	r.window.Update(3, r.dateComprReg.No_of_new_records)

	if r.retainNo("document_no") {
		itemLedgEntry2.SetRange("document_no", itemLedgEntry2.Document_no)
		newItemLedgEntry.Document_no = itemLedgEntry2.Document_no
	}
	if r.retainNo("variant_code") {
		itemLedgEntry2.SetRange("variant_code", itemLedgEntry2.Variant_code)
		newItemLedgEntry.Variant_code = itemLedgEntry2.Variant_code
	}
	if r.retainNo("location_code") {
		itemLedgEntry2.SetRange("location_code", itemLedgEntry2.Location_code)
		newItemLedgEntry.Location_code = itemLedgEntry2.Location_code
	}
	if r.retainNo("serial_no") {
		itemLedgEntry2.SetRange("serial_no", itemLedgEntry2.Serial_no)
		newItemLedgEntry.Serial_no = itemLedgEntry2.Serial_no
	}
	if r.retainNo("lot_no") {
		itemLedgEntry2.SetRange("lot_no", itemLedgEntry2.Lot_no)
		newItemLedgEntry.Lot_no = itemLedgEntry2.Lot_no
	}
	if r.retainNo("source_type") {
		itemLedgEntry2.SetRange("source_type", itemLedgEntry2.Source_type)
		newItemLedgEntry.Source_type = itemLedgEntry2.Source_type
	}
	if r.retainNo("source_no") {
		itemLedgEntry2.SetRange("source_no", itemLedgEntry2.Source_no)
		newItemLedgEntry.Source_no = itemLedgEntry2.Source_no
	}
	if r.retainNo("source_posting_group") {
		itemLedgEntry2.SetRange("source_posting_group", itemLedgEntry2.Source_posting_group)
		newItemLedgEntry.Source_posting_group = itemLedgEntry2.Source_posting_group
	}
	if r.retainNo("salespers_purch_code") {
		itemLedgEntry2.SetRange("salespers_purch_code", itemLedgEntry2.Salespers_purch_code)
		newItemLedgEntry.Salespers_purch_code = itemLedgEntry2.Salespers_purch_code
	}
	if r.retainNo("transaction_type") {
		itemLedgEntry2.SetRange("transaction_type", itemLedgEntry2.Transaction_type)
		newItemLedgEntry.Transaction_type = itemLedgEntry2.Transaction_type
	}
	if r.retainNo("transport_method") {
		itemLedgEntry2.SetRange("transport_method", itemLedgEntry2.Transport_method)
		newItemLedgEntry.Transport_method = itemLedgEntry2.Transport_method
	}
	if r.retainNo("country_code") {
		itemLedgEntry2.SetRange("country_code", itemLedgEntry2.Country_code)
		newItemLedgEntry.Country_code = itemLedgEntry2.Country_code
	}
	if r.retainNo("department_code") {
		itemLedgEntry2.SetRange("department_code", itemLedgEntry2.Department_code)
		newItemLedgEntry.Department_code = itemLedgEntry2.Department_code
	}
	if r.retainNo("project_code") {
		itemLedgEntry2.SetRange("project_code", itemLedgEntry2.Project_code)
		newItemLedgEntry.Project_code = itemLedgEntry2.Project_code
	}
	if !itemLedgEntry2.Quantity.IsNegative() {
		itemLedgEntry2.SetFilter("quantity", ">=0")
	} else {
		itemLedgEntry2.SetFilter("quantity", "<0")
	}
	err = r.comprItemEntry(itemLedgEntry2)
	if err != nil {
		return err
	}
	// the entries of the group whose cost is adjusted with the entries they were applied to
	var group []*gtables.ItemLedgerEntry
	for found := itemLedgEntry2.FindSetBuffered(); found; found = itemLedgEntry2.Next() {
		entry := gtables.NewItemLedgerEntry(run)
		entry.Copy(itemLedgEntry2)
		group = append(group, entry)
	}
	for _, entry := range group {
		if r.itemEntryAdjusted(entry, run) {
			err = r.comprItemEntry(entry)
			if err != nil {
				return err
			}
		}
	}
	newItemLedgEntry.Positive = newItemLedgEntry.Quantity.IsPositive()
	err = newItemLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	itemApplnEntry := r.itemApplnEntry
	itemApplnEntry.Init()
	r.lastApplnEntryNo++
	itemApplnEntry.Entry_no = r.lastApplnEntryNo
	itemApplnEntry.Item_no = newItemLedgEntry.Item_no
	itemApplnEntry.Item_ledger_entry_no = newItemLedgEntry.Entry_no
	if newItemLedgEntry.Quantity.IsPositive() {
		itemApplnEntry.Applied_to_entry_no = newItemLedgEntry.Entry_no
	} else {
		itemApplnEntry.Applied_from_entry_no = newItemLedgEntry.Entry_no
		itemApplnEntry.Fixed_application = true
	}
	itemApplnEntry.Quantity = newItemLedgEntry.Quantity
	itemApplnEntry.Entry_type = gtables.ItemApplicationEntryEntry_type(newItemLedgEntry.Entry_type)
	itemApplnEntry.Posting_date = newItemLedgEntry.Posting_date
	itemApplnEntry.Document_no = newItemLedgEntry.Document_no
	itemApplnEntry.Valid = true
	err = itemApplnEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	if r.due() {
		return r.insertRegisters(run)
	}
	return nil
}

func (r *dateCompressItemLedger) onPostDataItem(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) error {
	r.window.Close()
	if r.dateComprReg.No_records_deleted > r.noOfDeleted {
		return r.insertRegisters(run)
	}
	return nil
}

// itemEntryAdjusted (ItemEntryAdjusted): an inbound entry can be compressed when the cost of the
// outbound entries applied to it (not transfers) and of the entries transferred from it is
// adjusted; a closed entry of quantity 0 when the entry that closed it is
func (r *dateCompressItemLedger) itemEntryAdjusted(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) bool {
	itemApplnEntry2 := gtables.NewItemApplicationEntry(run)
	itemLedgEntry3 := gtables.NewItemLedgerEntry(run)
	if itemLedgEntry.Quantity.IsPositive() {
		itemApplnEntry2.SetRange("applied_to_entry_no", itemLedgEntry.Entry_no)
		itemApplnEntry2.SetRange("transfered_from_entry_no", 0)
		itemApplnEntry2.SetFilter("entry_type", "<>"+strconv.Itoa(int(gtables.ItemApplicationEntry_Entry_type.Transfer)))
		itemApplnEntry2.SetFilter("quantity", "<0")
		for found := itemApplnEntry2.FindSet(); found; found = itemApplnEntry2.Next() {
			if itemLedgEntry3.Get(itemApplnEntry2.Applied_from_entry_no) && !itemLedgEntry3.Cost_is_adjusted {
				return false
			}
		}
		itemApplnEntry2.SetRange("applied_to_entry_no")
		itemApplnEntry2.SetRange("transfered_from_entry_no", itemLedgEntry.Entry_no)
		itemApplnEntry2.SetRange("entry_type")
		itemApplnEntry2.SetFilter("quantity", ">0")
		for found := itemApplnEntry2.FindSet(); found; found = itemApplnEntry2.Next() {
			if itemLedgEntry3.Get(itemApplnEntry2.Applied_to_entry_no) && !itemLedgEntry3.Cost_is_adjusted {
				return false
			}
		}
	} else {
		if itemLedgEntry.Quantity.IsZero() && itemLedgEntry.Closed_by_entry_no != 0 {
			if itemLedgEntry3.Get(itemLedgEntry.Closed_by_entry_no) {
				return itemLedgEntry3.Cost_is_adjusted
			}
		}
	}
	return true
}

// comprItemEntry (ComprItemEntry): the entry added to the new entry and deleted with its
// application entries
func (r *dateCompressItemLedger) comprItemEntry(itemLedgEntry *gtables.ItemLedgerEntry) error {
	newItemLedgEntry := r.newItemLedgEntry
	newItemLedgEntry.Quantity = newItemLedgEntry.Quantity.Add(itemLedgEntry.Quantity)
	newItemLedgEntry.Invoiced_quantity = newItemLedgEntry.Invoiced_quantity.Add(itemLedgEntry.Invoiced_quantity)
	newItemLedgEntry.Remaining_quantity = newItemLedgEntry.Remaining_quantity.Add(itemLedgEntry.Remaining_quantity)
	newItemLedgEntry.Amount = newItemLedgEntry.Amount.Add(itemLedgEntry.Amount)
	newItemLedgEntry.Discount_amount = newItemLedgEntry.Discount_amount.Add(itemLedgEntry.Discount_amount)
	newItemLedgEntry.Adjusted_cost_qty = newItemLedgEntry.Adjusted_cost_qty.Add(itemLedgEntry.Adjusted_cost_qty)
	newItemLedgEntry.Adjusted_cost_invoiced_qty = newItemLedgEntry.Adjusted_cost_invoiced_qty.Add(itemLedgEntry.Adjusted_cost_invoiced_qty)
	newItemLedgEntry.Cost_posted_to_gl = newItemLedgEntry.Cost_posted_to_gl.Add(itemLedgEntry.Cost_posted_to_gl)
	newItemLedgEntry.Add_currency_adj_cost_qty = newItemLedgEntry.Add_currency_adj_cost_qty.Add(itemLedgEntry.Add_currency_adj_cost_qty)
	newItemLedgEntry.Add_curr_adj_cost_inv_qty = newItemLedgEntry.Add_curr_adj_cost_inv_qty.Add(itemLedgEntry.Add_curr_adj_cost_inv_qty)
	newItemLedgEntry.Add_curr_cost_posted_to_gl = newItemLedgEntry.Add_curr_cost_posted_to_gl.Add(itemLedgEntry.Add_curr_cost_posted_to_gl)
	newItemLedgEntry.Invt_qty_not_adjusted = newItemLedgEntry.Invt_qty_not_adjusted.Add(itemLedgEntry.Invt_qty_not_adjusted)
	err := itemLedgEntry.DeleteOrError(false)
	if err != nil {
		return err
	}
	itemApplnEntry := gtables.NewItemApplicationEntry(itemLedgEntry)
	itemApplnEntry.SetCurrentKey("item_ledger_entry_no")
	itemApplnEntry.SetRange("item_ledger_entry_no", itemLedgEntry.Entry_no)
	itemApplnEntry.DeleteAll()
	r.deleted()
	r.window.Update(4, r.dateComprReg.No_records_deleted)
	return nil
}

// initRegisters (InitRegisters): the item register of the new entries and the Date Compr.
// Register row
func (r *dateCompressItemLedger) initRegisters(run *Run) {
	registerNo := r.init(r.sourceCodeSetup.Compress_item_ledger, r.lastEntryNo)
	itemLedgEntry := gtables.NewItemLedgerEntry(run)
	language := fcodeunits.CurrentLanguage()
	r.initRegister(registerNo, r.sourceCodeSetup.Compress_item_ledger, func(field string) string { return itemLedgEntry.FieldCaption(field, language) })
}

// insertRegisters (InsertRegisters): the registers inserted the first time, modified after
func (r *dateCompressItemLedger) insertRegisters(run *Run) error {
	err := r.save(r.newItemLedgEntry.Entry_no)
	if err != nil {
		return err
	}
	err = r.saveRegister(r.regExists)
	if err != nil {
		return err
	}
	r.regExists = true

	itemLedgEntry2 := gtables.NewItemLedgerEntry(run)
	if itemLedgEntry2.FindLast() && r.lastEntryNo != itemLedgEntry2.Entry_no {
		r.lastEntryNo = itemLedgEntry2.Entry_no
		r.initRegisters(run)
	}
	return nil
}
