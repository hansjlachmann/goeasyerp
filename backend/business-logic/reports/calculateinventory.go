package reports

import (
	"sort"

	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 790 Calculate Inventory: fills a phys. inventory journal batch with a line per item
// (and variant, and by choice dimension 1, dimension 2, location and bin) with the quantity on inventory
// as Qty. (Calculated) and Qty. (Phys. Inventory) — the user enters the counted quantity, posting
// adjusts the inventory by the difference. Run from the Phys. Inventory Journal with its line
// as the context (classic SetItemJnlLine).
const CalculateInventoryID = 790

func init() {
	Register(CalculateInventoryID, "calculate-inventory", func() Report { return &calculateInventory{} })
}

type calculateInventory struct {
	itemJnlLine          *gtables.ItemJournalLine
	postingDate          types.Date
	nextDocNo            types.Code
	nextLineNo           int
	zeroQty              bool
	byDept, byProj       bool
	byLocation           bool
	byBin                bool
	quantityOnHandBuffer map[quantityOnHandKey]types.Decimal
	sourceCodeSetup      *gtables.SourceCodeSetup
	window               *system.Window
}

// quantityOnHandKey is a line of the result (QuantityOnHandBuffer, table 307)
type quantityOnHandKey struct {
	itemNo, variantCode, deptCode, projCode, locationCode, binCode types.Code
}

func (r *calculateInventory) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "posting_date", Type: "date", Required: true},
			{Name: "document_no", Type: "text"},
			{Name: "items_not_on_inventory", Type: "bool", Default: "false"},
			{Name: "by_dimension_1", Type: "bool", Default: "false"},
			{Name: "by_dimension_2", Type: "bool", Default: "false"},
			{Name: "by_location", Type: "bool", Default: "false"},
			{Name: "by_bin", Type: "bool", Default: "false"},
		},
		OnInitReport: r.onInitReport,
		ContextTable: gtables.ItemJournalLineTableName,
		DataItems: []*DataItem{{
			Name:                "item",
			Table:               gtables.ItemTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "dimension_1_filter", "dimension_2_filter", "location_filter", "bin_filter"},
			OnPreDataItem:       Trigger(r.itemOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.itemOnAfterGetRecord),
			OnPostDataItem:      Trigger(r.itemOnPostDataItem),
		}},
	}
}

// The posting date starts as the work date, the document no. as the batch series' next number
// (ValidatePostingDate)
func (r *calculateInventory) onInitReport(run *Run) error {
	itemJnlLine, ok := run.Context().(*gtables.ItemJournalLine)
	if !ok {
		return system.Error(run.Text("NO_JOURNAL"))
	}
	r.itemJnlLine = itemJnlLine
	// the journal's template and batch: the page's filters when it has no line yet
	if r.itemJnlLine.Journal_template_name == "" {
		r.itemJnlLine.Journal_template_name = itemJnlFilter(r.itemJnlLine, "journal_template_name")
	}
	if r.itemJnlLine.Journal_batch_name == "" {
		r.itemJnlLine.Journal_batch_name = itemJnlFilter(r.itemJnlLine, "journal_batch_name")
	}
	run.SetOption("posting_date", system.WorkDate().String())
	itemJnlBatch := gtables.NewItemJournalBatch(run)
	err := itemJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": itemJnlLine.Journal_template_name, "name": itemJnlLine.Journal_batch_name})
	if err != nil {
		return err
	}
	if itemJnlBatch.No_series != "" {
		noSeriesMgt := noseries.New(run)
		nextDocNo, err := noSeriesMgt.GetNextNo(itemJnlBatch.No_series, system.WorkDate(), false)
		if err != nil {
			return err
		}
		run.SetOption("document_no", nextDocNo.String())
	}
	return nil
}

func (r *calculateInventory) itemOnPreDataItem(item *gtables.Item, run *Run) error {
	r.postingDate = run.OptionDate("posting_date")
	r.nextDocNo = types.Code(system.UpperCase(run.Option("document_no")))
	r.zeroQty = run.OptionBool("items_not_on_inventory")
	r.byDept = run.OptionBool("by_dimension_1")
	r.byProj = run.OptionBool("by_dimension_2")
	r.byLocation = run.OptionBool("by_location")
	r.byBin = run.OptionBool("by_bin")
	if r.postingDate.IsZero() {
		return system.Error(run.Text("ENTER_POSTING_DATE"))
	}
	itemJnlTemplate := gtables.NewItemJournalTemplate(run)
	err := itemJnlTemplate.GetOrError(r.itemJnlLine.Journal_template_name)
	if err != nil {
		return err
	}
	itemJnlBatch := gtables.NewItemJournalBatch(run)
	err = itemJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": r.itemJnlLine.Journal_template_name, "name": r.itemJnlLine.Journal_batch_name})
	if err != nil {
		return err
	}
	if r.nextDocNo == "" {
		if itemJnlBatch.No_series != "" {
			itemJnlLine := gtables.NewItemJournalLine(run)
			itemJnlLine.SetRange("journal_template_name", r.itemJnlLine.Journal_template_name)
			itemJnlLine.SetRange("journal_batch_name", r.itemJnlLine.Journal_batch_name)
			if !itemJnlLine.FindFirst() {
				noSeriesMgt := noseries.New(run)
				r.nextDocNo, err = noSeriesMgt.GetNextNo(itemJnlBatch.No_series, r.postingDate, false)
				if err != nil {
					return err
				}
			}
		}
		if r.nextDocNo == "" {
			return system.Error(run.Text("ENTER_DOCUMENT_NO"))
		}
	}
	r.nextLineNo = 0
	r.window = system.OpenWindow(run.Text("WINDOW"))
	r.quantityOnHandBuffer = map[quantityOnHandKey]types.Decimal{}
	return nil
}

// itemOnAfterGetRecord adds up the item's ledger entries per variant (and dimension 1, dimension 2,
// location, bin) within the item's filters
func (r *calculateInventory) itemOnAfterGetRecord(item *gtables.Item, run *Run) error {
	r.window.Update(1, item.No)
	itemLedgEntry := gtables.NewItemLedgerEntry(run)
	itemLedgEntry.SetCurrentKey("item_no", "variant_code", "drop_shipment", "location_code", "posting_date")
	itemLedgEntry.SetRange("item_no", item.No)
	copyItemFilter(item.Variant_filter, itemLedgEntry, "variant_code")
	copyItemFilter(item.Dimension_1_filter, itemLedgEntry, "dimension_1_code")
	copyItemFilter(item.Dimension_2_filter, itemLedgEntry, "dimension_2_code")
	copyItemFilter(item.Location_filter, itemLedgEntry, "location_code")
	copyItemFilter(item.Bin_filter, itemLedgEntry, "bin_code")
	for found := itemLedgEntry.FindSetBuffered(); found; found = itemLedgEntry.Next() {
		key := quantityOnHandKey{itemNo: itemLedgEntry.Item_no, variantCode: itemLedgEntry.Variant_code}
		if r.byDept {
			key.deptCode = itemLedgEntry.Dimension_1_code
		}
		if r.byProj {
			key.projCode = itemLedgEntry.Dimension_2_code
		}
		if r.byLocation {
			key.locationCode = itemLedgEntry.Location_code
		}
		if r.byBin {
			key.binCode = itemLedgEntry.Bin_code
		}
		r.quantityOnHandBuffer[key] = r.quantityOnHandBuffer[key].Add(itemLedgEntry.Quantity)
	}
	return nil
}

// copyItemFilter: an Item FlowFilter as the ledger entries' filter (COPYFILTER)
func copyItemFilter(expression string, itemLedgEntry *gtables.ItemLedgerEntry, field string) {
	if expression != "" {
		itemLedgEntry.SetFilter(field, expression)
	}
}

// itemOnPostDataItem writes the journal lines in the buffer's order
func (r *calculateInventory) itemOnPostDataItem(item *gtables.Item, run *Run) error {
	defer r.window.Close()
	var keys []quantityOnHandKey
	for key := range r.quantityOnHandBuffer {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.itemNo != b.itemNo {
			return a.itemNo < b.itemNo
		}
		if a.variantCode != b.variantCode {
			return a.variantCode < b.variantCode
		}
		if a.deptCode != b.deptCode {
			return a.deptCode < b.deptCode
		}
		if a.projCode != b.projCode {
			return a.projCode < b.projCode
		}
		if a.locationCode != b.locationCode {
			return a.locationCode < b.locationCode
		}
		return a.binCode < b.binCode
	})
	for _, key := range keys {
		err := r.insertItemJnlLine(run, key, r.quantityOnHandBuffer[key])
		if err != nil {
			return err
		}
	}
	return nil
}

// insertItemJnlLine writes a phys. inventory line of the batch: a positive adjustment of 0 with
// the quantity on inventory as calculated and counted quantity
func (r *calculateInventory) insertItemJnlLine(run *Run, key quantityOnHandKey, quantity2 types.Decimal) error {
	itemJnlLine := gtables.NewItemJournalLine(run)
	if r.nextLineNo == 0 {
		itemJnlLine.LockTable()
		itemJnlLine.SetRange("journal_template_name", r.itemJnlLine.Journal_template_name)
		itemJnlLine.SetRange("journal_batch_name", r.itemJnlLine.Journal_batch_name)
		if itemJnlLine.FindLast() {
			r.nextLineNo = itemJnlLine.Line_no
		}
		r.sourceCodeSetup = gtables.GetSourceCodeSetup(run)
	}
	r.nextLineNo = r.nextLineNo + 10000
	if quantity2.IsZero() && !r.zeroQty {
		return nil
	}
	itemJnlLine.Init()
	itemJnlLine.Journal_template_name = r.itemJnlLine.Journal_template_name
	itemJnlLine.Journal_batch_name = r.itemJnlLine.Journal_batch_name
	itemJnlLine.Line_no = r.nextLineNo
	steps := []struct {
		field string
		value interface{}
	}{
		{"posting_date", r.postingDate},
		{"entry_type", gtables.ItemJournalLine_Entry_type.PositiveAdjmt},
		{"document_no", r.nextDocNo},
		{"item_no", key.itemNo},
		{"variant_code", key.variantCode},
		{"dimension_1_code", key.deptCode},
		{"dimension_2_code", key.projCode},
		{"location_code", key.locationCode},
		{"bin_code", key.binCode},
		{"source_code", r.sourceCodeSetup.Phys_inventory_journal},
	}
	for _, step := range steps {
		err := itemJnlLine.Validate(step.field, step.value)
		if err != nil {
			return err
		}
	}
	itemJnlLine.Qty_phys_inventory = quantity2
	itemJnlLine.Phys_inventory = true
	err := itemJnlLine.Validate("qty_calculated", quantity2)
	if err != nil {
		return err
	}
	err = itemJnlLine.Validate("unit_amount", types.Decimal{})
	if err != nil {
		return err
	}
	itemLedgEntry := gtables.NewItemLedgerEntry(run)
	itemLedgEntry.SetCurrentKey("item_no")
	itemLedgEntry.SetRange("item_no", key.itemNo)
	if itemLedgEntry.FindLast() {
		itemJnlLine.Last_item_ledger_entry_no = itemLedgEntry.Entry_no
	} else {
		itemJnlLine.Last_item_ledger_entry_no = 0
	}
	return itemJnlLine.InsertOrError(true)
}

// itemJnlFilter is the exact value a journal line is filtered on (filter group 2, the page's
// filters)
func itemJnlFilter(rec *gtables.ItemJournalLine, field string) types.Code {
	rec.FilterGroup(2)
	value := rec.GetFilter(field)
	rec.FilterGroup(0)
	return types.Code(value)
}
