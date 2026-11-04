package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The BOM journal's reports:
//   - 802 BOM Journal - Test (Test Report on the BOM journals): per batch its lines with what
//     posting would refuse under each line — the item (missing, blocked, Specific costing, a
//     Reserve Always item not reserved, not a bill of materials), its components (missing,
//     blocked, Specific costing), the recurring fields, the posting date (closing date, date
//     order, allowed range), the quantity and gaps in the batch's number series — and the
//     batch's total cost;
//   - 803 BOM Register (the posting report of Post and Print): per register its BOM ledger
//     entries with the item or resource entry each refers to.
const (
	BOMJnlTestID  = 802
	BOMRegisterID = 803
)

func init() {
	Register(BOMJnlTestID, "bom-journal-test", func() Report { return &bomJnlTest{} })
	Register(BOMRegisterID, "bom-register", func() Report { return &bomRegister{} })
}

// Report 802 BOM Journal - Test
type bomJnlTest struct {
	jnlTest
	batch    *gtables.BOMJournalBatch
	template *gtables.BOMJournalTemplate
	qtyError bool
}

func (r *bomJnlTest) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.BOMJournalLineTableName,
		// run from a journal: its template and batch
		OnInitReport: func(run *Run) error {
			line, ok := run.Context().(*gtables.BOMJournalLine)
			if ok && line.Journal_template_name != "" {
				run.SetRequestFilter("BOM_Journal_Batch", "journal_template_name", line.Journal_template_name.String())
				run.SetRequestFilter("BOM_Journal_Batch", "name", line.Journal_batch_name.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "BOM_Journal_Batch", Table: gtables.BOMJournalBatchTableName, Key: []string{"journal_template_name", "name"},
			RequestFilterFields: []string{"journal_template_name", "name"},
			OnAfterGetRecord: Trigger(func(batch *gtables.BOMJournalBatch, run *Run) error {
				line := gtables.NewBOMJournalLine(run)
				line.SetRange("journal_template_name", batch.Journal_template_name)
				line.SetRange("journal_batch_name", batch.Name)
				r.batch = batch
				return jnlTestBatchHasLines(run, line.IsEmpty())
			}),
			DataItems: []*DataItem{{
				Name: "BOM_Journal_Line", Table: gtables.BOMJournalLineTableName,
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

func (r *bomJnlTest) lineOnPreDataItem(line *gtables.BOMJournalLine, run *Run) error {
	r.template = gtables.NewBOMJournalTemplate(run)
	if !r.template.Get(r.batch.Journal_template_name) {
		r.template.Init()
	}
	// the user's filters, before a recurring journal's own (GETFILTERS in OnPreReport)
	setFilterText(run, line)
	r.startBatch(run, line, r.template.Recurring, r.batch.No_series)
	return nil
}

func (r *bomJnlTest) lineOnAfterGetRecord(line *gtables.BOMJournalLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.errors = nil
	if tables.BOMJnlLineEmptyLine(line) {
		return Skip()
	}
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	r.qtyError = false

	// MakeRecurringText: the texts as posting writes them
	if !line.Posting_date.IsZero() && line.Item_no != "" && line.Recurring_method != gtables.BOMJournalLine_Recurring_method.Blank {
		docNo, description := tables.RecurringText(run, line.Posting_date, line.Document_no, line.Description, 20, 50)
		line.Document_no, line.Description = docNo, description
	}

	if line.Item_no == "" {
		r.addError(run.Text("MUST_BE_SPECIFIED", caption("item_no")))
	} else {
		item := gtables.NewItem(run)
		if !item.Get(line.Item_no) {
			r.addError(run.Text("ITEM_NOT_EXIST", line.Item_no))
		} else {
			if item.Blocked {
				r.addError(run.Text("MUST_BE_FOR_ITEM", item.GetFieldCaption("blocked", language), formatValue(false, "", language), line.Item_no))
			}
			if item.Costing_method == gtables.Item_Costing_method.Specific {
				r.addError(run.Text("BOM_COSTING_METHOD", item.GetFieldCaption("costing_method", language), line.Item_no,
					item.GetOptionCaption("costing_method", "specific", language)))
			}
			if item.Reserve == gtables.Item_Reserve.Always {
				if line.Quantity.IsNegative() && line.Reserved_quantity.Abs().LessThan(line.Quantity.Abs()) {
					r.addError(run.Text("MUST_BE", caption("reserved_quantity"), formatValue(line.Quantity, "", language)))
				}
			}
			r.checkComponents(run, line)
		}
	}

	r.checkRecurringLine(run, line, recurringLine{
		recurring:      r.template.Recurring,
		methodBlank:    line.Recurring_method == gtables.BOMJournalLine_Recurring_method.Blank,
		methodVariable: line.Recurring_method == gtables.BOMJournalLine_Recurring_method.Variable,
		frequency:      line.Recurring_frequency,
		quantityZero:   line.Quantity.IsZero() && !r.qtyError,
		quantityField:  "quantity",
	})
	if r.template.Recurring && line.Recurring_method == gtables.BOMJournalLine_Recurring_method.Variable && line.Quantity.IsZero() {
		r.qtyError = true
	}

	r.checkPostingDate(run, line.Posting_date, caption("posting_date"))
	r.checkDocumentDate(run, line.Document_date, caption("document_date"))

	if line.Quantity.IsZero() && !r.qtyError {
		r.qtyError = true
		r.addError(run.Text("MUST_BE_SPECIFIED", caption("quantity")))
	}

	r.checkDocumentNo(run, r.batch.No_series, line.Document_no)
	return nil
}

// checkComponents: the item is a bill of materials, and each component with a quantity exists
// and is not blocked (an item: no Specific costing)
func (r *bomJnlTest) checkComponents(run *Run, line *gtables.BOMJournalLine) {
	language := fcodeunits.CurrentLanguage()
	bomComp := gtables.NewBOMComponent(run)
	bomComp.SetRange("parent_item_no", line.Item_no)
	if !bomComp.FindSetBuffered() {
		r.addError(run.Text("NOT_A_BOM", line.Item_no))
		return
	}
	for found := true; found; found = bomComp.Next() {
		compQty := system.Round(line.Quantity.Mul(bomComp.Quantity_per), types.MustDecimal("0.00001"), "=")
		if compQty.IsZero() {
			continue
		}
		switch bomComp.Type {
		case gtables.BOMComponent_Type.Item:
			item := gtables.NewItem(run)
			if !item.Get(bomComp.No) {
				r.addError(run.Text("ITEM_NOT_EXIST", bomComp.No))
			} else {
				if item.Blocked {
					r.addError(run.Text("MUST_BE_FOR_ITEM", item.GetFieldCaption("blocked", language), formatValue(false, "", language), item.No))
				}
				if item.Costing_method == gtables.Item_Costing_method.Specific {
					r.addError(run.Text("COMP_COSTING_METHOD", item.GetFieldCaption("costing_method", language),
						item.GetOptionCaption("costing_method", "specific", language), item.No))
				}
			}
		case gtables.BOMComponent_Type.Resource:
			res := gtables.NewResource(run)
			if !res.Get(bomComp.No) {
				r.addError(run.Text("RESOURCE_NOT_EXIST", bomComp.No))
			} else {
				if res.Blocked {
					r.addError(run.Text("MUST_BE_FOR_RESOURCE", res.GetFieldCaption("blocked", language), formatValue(false, "", language), res.No))
				}
			}
		}
	}
}

// Report 803 BOM Register
type bomRegister struct {
	bomReg *gtables.BOMRegister
}

func (r *bomRegister) Definition() *Definition {
	return &Definition{
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "BOM_Register", Table: gtables.BOMRegisterTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "source_code"},
			OnPreDataItem:       headWithFilters[*gtables.BOMRegister](nil),
			OnAfterGetRecord: Trigger(func(bomReg *gtables.BOMRegister, run *Run) error {
				r.bomReg = bomReg
				sourceCode := gtables.NewSourceCode(run)
				sourceCodeText := ""
				if bomReg.Source_code != "" {
					sourceCodeText = run.Text("SOURCE_CODE", bomReg.Source_code)
					if !sourceCode.Get(bomReg.Source_code) {
						sourceCode.Init()
					}
				}
				run.SetValue("register_text", run.Text("REGISTER_NO", bomReg.No))
				run.SetValue("source_code_text", sourceCodeText)
				run.SetValue("source_code_description", sourceCode.Description.String())
				run.SetValue("no_of_entries", bomReg.To_entry_no-bomReg.From_entry_no+1)
				run.SetValue("entries_text", run.Text("NO_OF_ENTRIES", bomReg.No))
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "BOM_Ledger_Entry", Table: gtables.BOMLedgerEntryTableName, Key: []string{"entry_no"},
				RequestFilterFields: []string{"reference_entry_no", "posting_date"},
				OnPreDataItem: Trigger(func(bomLedgEntry *gtables.BOMLedgerEntry, run *Run) error {
					bomLedgEntry.SetRange("entry_no", r.bomReg.From_entry_no, r.bomReg.To_entry_no)
					return nil
				}),
				OnAfterGetRecord: Trigger(r.entryOnAfterGetRecord),
			}},
		}},
	}
}

// entryOnAfterGetRecord: the item or resource entry the BOM entry refers to — its date,
// document, entry type, number, description, quantity and cost (a resource's with the
// opposite sign: the cost the assembled item takes)
func (r *bomRegister) entryOnAfterGetRecord(bomLedgEntry *gtables.BOMLedgerEntry, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	switch bomLedgEntry.Reference_type {
	case gtables.BOMLedgerEntry_Reference_type.Item:
		itemLedgEntry := gtables.NewItemLedgerEntry(run)
		err := itemLedgEntry.GetOrError(bomLedgEntry.Reference_entry_no)
		if err != nil {
			return err
		}
		bomDescription := itemLedgEntry.Description.String()
		if bomDescription == "" {
			item := gtables.NewItem(run)
			if !item.Get(itemLedgEntry.Item_no) {
				item.Init()
			}
			bomDescription = item.Description.String()
		}
		run.SetValue("bom_date", itemLedgEntry.Posting_date)
		run.SetValue("bom_doc_no", itemLedgEntry.Document_no.String())
		run.SetValue("bom_entry_type", optionCaption(gtables.ItemLedgerEntryTableName, "entry_type", itemLedgEntry.Entry_type.String(), language))
		run.SetValue("bom_no", itemLedgEntry.Item_no.String())
		run.SetValue("bom_description", bomDescription)
		run.SetValue("bom_qty", itemLedgEntry.Quantity)
		run.SetValue("bom_unit_of_measure", "")
		run.SetValue("bom_unit_cost", itemLedgEntry.Unit_cost)
		run.SetValue("bom_total_cost", itemLedgEntry.Amount)
	case gtables.BOMLedgerEntry_Reference_type.Resource:
		resLedgEntry := gtables.NewResLedgerEntry(run)
		err := resLedgEntry.GetOrError(bomLedgEntry.Reference_entry_no)
		if err != nil {
			return err
		}
		run.SetValue("bom_date", resLedgEntry.Posting_date)
		run.SetValue("bom_doc_no", resLedgEntry.Document_no.String())
		run.SetValue("bom_entry_type", optionCaption(gtables.ResLedgerEntryTableName, "entry_type", resLedgEntry.Entry_type.String(), language))
		run.SetValue("bom_no", resLedgEntry.Resource_no.String())
		run.SetValue("bom_description", resLedgEntry.Description.String())
		run.SetValue("bom_qty", resLedgEntry.Quantity)
		run.SetValue("bom_unit_of_measure", resLedgEntry.Unit_of_measure_code.String())
		run.SetValue("bom_unit_cost", resLedgEntry.Unit_cost.Neg())
		run.SetValue("bom_total_cost", resLedgEntry.Total_cost.Neg())
	}
	return nil
}
