package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 702 Inventory Posting - Test (Test Report on the item journals — also the transfer,
// phys. inventory and recurring ones): per batch its lines with what posting would refuse under
// each line — the item (missing, blocked, a Reserve Always item not reserved), the recurring
// fields, the posting date (closing date, date order, allowed range), the general posting
// setup, the location (Location Mandatory), discounts, serial numbers, a transfer's amounts and
// quantities, a phys. inventory line's quantities, invoiced quantities and the shipment entry,
// a Specific item's serial number and gaps in the batch's number series — and per entry type
// the batch's amounts with their total
const ItemJnlTestID = 702

func init() {
	Register(ItemJnlTestID, "inventory-posting-test", func() Report { return &itemJnlTest{} })
}

type itemJnlTest struct {
	jnlTest
	batch        *gtables.ItemJournalBatch
	template     *gtables.ItemJournalTemplate
	invtSetup    *gtables.InventorySetup
	qtyError     bool
	noOfEntries  [5]int
	totalAmounts [5]types.Decimal
}

func (r *itemJnlTest) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.ItemJournalLineTableName,
		// run from a journal (ReportPrint.PrintItemJnlLine): its template and batch
		OnInitReport: func(run *Run) error {
			line, ok := run.Context().(*gtables.ItemJournalLine)
			if ok && line.Journal_template_name != "" {
				run.SetRequestFilter("Item_Journal_Batch", "journal_template_name", line.Journal_template_name.String())
				run.SetRequestFilter("Item_Journal_Batch", "name", line.Journal_batch_name.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			r.invtSetup = gtables.GetInventorySetup(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Item_Journal_Batch", Table: gtables.ItemJournalBatchTableName, Key: []string{"journal_template_name", "name"},
			RequestFilterFields: []string{"journal_template_name", "name"},
			OnAfterGetRecord: Trigger(func(batch *gtables.ItemJournalBatch, run *Run) error {
				line := gtables.NewItemJournalLine(run)
				line.SetRange("journal_template_name", batch.Journal_template_name)
				line.SetRange("journal_batch_name", batch.Name)
				r.batch = batch
				return jnlTestBatchHasLines(run, line.IsEmpty())
			}),
			DataItems: []*DataItem{{
				Name: "Item_Journal_Line", Table: gtables.ItemJournalLineTableName,
				Key:                 []string{"journal_template_name", "journal_batch_name", "line_no"},
				Link:                map[string]string{"journal_template_name": "journal_template_name", "journal_batch_name": "name"},
				RequestFilterFields: []string{"posting_date"},
				OnPreDataItem:       Trigger(r.lineOnPreDataItem),
				OnAfterGetRecord:    Trigger(r.lineOnAfterGetRecord),
				DataItems:           []*DataItem{r.errorLoop()},
			}},
		}},
	}
}

// lineOnPreDataItem: a recurring journal's lines up to the work date; the batch's number series;
// the entry types' amounts start at 0
func (r *itemJnlTest) lineOnPreDataItem(line *gtables.ItemJournalLine, run *Run) error {
	r.template = gtables.NewItemJournalTemplate(run)
	if !r.template.Get(r.batch.Journal_template_name) {
		r.template.Init()
	}
	// the user's filters, before a recurring journal's own (GETFILTERS in OnPreReport)
	setFilterText(run, line)
	r.startBatch(run, line, r.template.Recurring, r.batch.No_series)
	r.noOfEntries = [5]int{}
	r.totalAmounts = [5]types.Decimal{}
	r.setEntryTypeTotals(run)
	return nil
}

// setEntryTypeTotals: the amounts per entry type (printed for the types the batch has) and their
// total — purchases and positive adjustments in, sales and negative adjustments out
func (r *itemJnlTest) setEntryTypeTotals(run *Run) {
	language := fcodeunits.CurrentLanguage()
	keys := []string{"purchase", "sale", "positive_adjmt", "negative_adjmt", "transfer"}
	for i, key := range keys {
		run.SetValue("entry_type_"+key, optionCaption(gtables.ItemJournalLineTableName, "entry_type", key, language))
		run.SetValue("no_of_entries_"+key, r.noOfEntries[i])
		run.SetValue("total_amount_"+key, r.totalAmounts[i])
	}
	total := r.totalAmounts[0].Sub(r.totalAmounts[1]).Add(r.totalAmounts[2]).Sub(r.totalAmounts[3]).Add(r.totalAmounts[4])
	run.SetValue("total_amount", total)
}

func (r *itemJnlTest) lineOnAfterGetRecord(line *gtables.ItemJournalLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.errors = nil
	if line.Item_no == "" && line.Quantity.IsZero() {
		return Skip()
	}
	r.noOfEntries[line.Entry_type] = 1
	r.totalAmounts[line.Entry_type] = r.totalAmounts[line.Entry_type].Add(line.Amount)
	r.setEntryTypeTotals(run)

	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	entryTypeCaption := optionCaption(gtables.ItemJournalLineTableName, "entry_type", line.Entry_type.String(), language)
	itemExists := false
	r.qtyError = false

	// MakeRecurringText: the texts as posting writes them
	if !line.Posting_date.IsZero() && line.Item_no != "" && line.Recurring_method != gtables.ItemJournalLine_Recurring_method.Blank {
		docNo, description := tables.RecurringText(run, line.Posting_date, line.Document_no, line.Description, 20, 50)
		line.Document_no, line.Description = docNo, description
	}

	item := gtables.NewItem(run)
	if line.Item_no == "" {
		r.addError(run.Text("MUST_BE_SPECIFIED", caption("item_no")))
	} else {
		if !item.Get(line.Item_no) {
			r.addError(run.Text("ITEM_NOT_EXIST", line.Item_no))
		} else {
			itemExists = true
			if item.Blocked {
				r.addError(run.Text("MUST_BE_FOR_ITEM", item.GetFieldCaption("blocked", language), formatValue(false, "", language), line.Item_no))
			}
			if item.Reserve == gtables.Item_Reserve.Always {
				signedQty := tables.ItemJnlLineSigned(line, line.Quantity)
				if signedQty.IsNegative() && line.Negative_reserved_quantity.Abs().LessThan(line.Quantity.Abs()) {
					r.addError(run.Text("MUST_BE", caption("negative_reserved_quantity"), formatValue(signedQty, "", language)))
				}
			}
		}
	}

	r.checkRecurringLine(run, line, recurringLine{
		recurring:      r.template.Recurring,
		methodBlank:    line.Recurring_method == gtables.ItemJournalLine_Recurring_method.Blank,
		methodVariable: line.Recurring_method == gtables.ItemJournalLine_Recurring_method.Variable,
		frequency:      line.Recurring_frequency,
		quantityZero:   line.Quantity.IsZero(),
		quantityField:  "quantity",
	})

	r.checkPostingDate(run, line.Posting_date, caption("posting_date"))
	r.checkDocumentDate(run, line.Document_date, caption("document_date"))

	r.checkGenPostingSetup(run, line.Gen_bus_posting_group, line.Gen_prod_posting_group, caption("gen_prod_posting_group"))

	if r.invtSetup.Location_mandatory {
		if line.Location_code == "" {
			r.addError(run.Text("MUST_BE_SPECIFIED", caption("location_code")))
		}
		if line.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer {
			if line.New_location_code == "" {
				r.addError(run.Text("MUST_BE_SPECIFIED", caption("new_location_code")))
			}
		}
	}

	if line.Entry_type == gtables.ItemJournalLine_Entry_type.PositiveAdjmt || line.Entry_type == gtables.ItemJournalLine_Entry_type.NegativeAdjmt {
		if !line.Discount_amount.IsZero() {
			r.addError(run.Text("MUST_BE_ZERO", caption("discount_amount")))
		}
	}

	if line.Serial_no != "" {
		if line.Quantity.Equal(line.Invoiced_quantity) || line.Invoiced_quantity.IsZero() {
			if !line.Quantity.Abs().Equal(types.MustDecimal("1")) && !r.qtyError {
				r.qtyError = true
				r.addError(run.Text("SERIAL_QTY_ONE", caption("quantity"), caption("serial_no")))
			}
		} else {
			if !line.Invoiced_quantity.Abs().Equal(types.MustDecimal("1")) && !r.qtyError {
				r.qtyError = true
				r.addError(run.Text("SERIAL_QTY_ONE", caption("invoiced_quantity"), caption("serial_no")))
			}
		}
	}

	if line.Entry_type == gtables.ItemJournalLine_Entry_type.Transfer {
		if !line.Amount.IsZero() {
			r.addError(run.Text("MUST_BE_ZERO_WHEN", caption("amount"), caption("entry_type"), entryTypeCaption))
		}
		if !line.Discount_amount.IsZero() {
			r.addError(run.Text("MUST_BE_ZERO_WHEN", caption("discount_amount"), caption("entry_type"), entryTypeCaption))
		}
		if line.Quantity.IsNegative() {
			r.addError(run.Text("MUST_NOT_BE_NEGATIVE_WHEN", caption("quantity"), caption("entry_type"), entryTypeCaption))
		}
		if !line.Quantity.Equal(line.Invoiced_quantity) {
			r.addError(run.Text("MUST_HAVE_SAME_VALUE_WHEN", caption("invoiced_quantity"), caption("quantity"), caption("entry_type"), entryTypeCaption))
		}
	}

	r.checkPhysInventory(run, line)

	if line.Quantity.IsZero() && !line.Invoiced_quantity.IsZero() {
		if line.Item_shpt_entry_no == 0 {
			r.addError(run.Text("MUST_BE_SPECIFIED", caption("item_shpt_entry_no")))
		}
	} else {
		if !line.Quantity.Equal(line.Invoiced_quantity) {
			if !line.Invoiced_quantity.IsZero() && !r.qtyError {
				r.qtyError = true
				r.addError(run.Text("MUST_BE_ZERO", caption("invoiced_quantity")))
			}
		}
		if line.Item_shpt_entry_no != 0 {
			r.addError(run.Text("CANNOT_BE_SPECIFIED", caption("item_shpt_entry_no")))
		}
	}

	if !line.Quantity.IsZero() && (!line.Drop_shipment || line.Applies_to_entry != 0) {
		if line.Applies_to_entry == 0 {
			if itemExists {
				if item.Costing_method == gtables.Item_Costing_method.Specific {
					if line.Serial_no == "" {
						r.addError(run.Text("MUST_BE_SPECIFIED", caption("serial_no")))
					}
				}
			}
		}
	}

	r.checkDocumentNo(run, r.batch.No_series, line.Document_no)
	return nil
}

// checkPhysInventory: a line that is no phys. inventory line has no calculated or counted
// quantity; a phys. inventory line is a positive or negative adjustment of the difference
func (r *itemJnlTest) checkPhysInventory(run *Run, line *gtables.ItemJournalLine) {
	language := fcodeunits.CurrentLanguage()
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	entryTypeCaption := func(key string) string {
		return optionCaption(gtables.ItemJournalLineTableName, "entry_type", key, language)
	}
	physInventory := formatValue(line.Phys_inventory, "", language)
	if !line.Phys_inventory {
		if !line.Qty_calculated.IsZero() {
			r.addError(run.Text("MUST_BE_ZERO_WHEN", caption("qty_calculated"), caption("phys_inventory"), physInventory))
		}
		if !line.Qty_phys_inventory.IsZero() {
			r.addError(run.Text("MUST_BE_ZERO_WHEN", caption("qty_phys_inventory"), caption("phys_inventory"), physInventory))
		}
	} else {
		if line.Entry_type != gtables.ItemJournalLine_Entry_type.PositiveAdjmt && line.Entry_type != gtables.ItemJournalLine_Entry_type.NegativeAdjmt {
			r.addError(run.Text("MUST_BE_OR_WHEN", caption("entry_type"), entryTypeCaption("positive_adjmt"), entryTypeCaption("negative_adjmt"),
				caption("phys_inventory"), physInventory))
		}
		if line.Entry_type == gtables.ItemJournalLine_Entry_type.PositiveAdjmt && !line.Qty_phys_inventory.Sub(line.Qty_calculated).Equal(line.Quantity) {
			r.addError(run.Text("MUST_EQUAL_DIFFERENCE", caption("quantity"), caption("qty_phys_inventory"), caption("qty_calculated"),
				caption("entry_type"), entryTypeCaption("positive_adjmt"), caption("phys_inventory"), physInventory))
		}
		if line.Entry_type == gtables.ItemJournalLine_Entry_type.NegativeAdjmt && !line.Qty_calculated.Sub(line.Qty_phys_inventory).Equal(line.Quantity) {
			r.addError(run.Text("MUST_EQUAL_DIFFERENCE", caption("quantity"), caption("qty_calculated"), caption("qty_phys_inventory"),
				caption("entry_type"), entryTypeCaption("negative_adjmt"), caption("phys_inventory"), physInventory))
		}
	}
}
