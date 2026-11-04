package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The resource journal's reports:
//   - 1102 Resource Journal - Test (Test Report on the resource journals): per batch its lines with
//     what posting would refuse under each line — the resource (missing, blocked), the general
//     posting setup, the recurring fields, the posting date (closing date, date order, allowed
//     range) and gaps in the batch's number series — and the batch's totals;
//   - 1103 Resource Register (the posting report of Post and Print): per register its resource
//     ledger entries.
const (
	ResJnlTestID        = 1102
	ResourceRegisterID  = 1103
	resJnlTestName      = "resource-journal-test"
	resourceRegisterKey = "resource-register"
)

func init() {
	Register(ResJnlTestID, resJnlTestName, func() Report { return &resJnlTest{} })
	Register(ResourceRegisterID, resourceRegisterKey, func() Report { return &resourceRegister{} })
}

// Report 1102 Resource Journal - Test
type resJnlTest struct {
	jnlTest
	batch    *gtables.ResJournalBatch
	template *gtables.ResJournalTemplate
}

func (r *resJnlTest) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.ResJournalLineTableName,
		// run from a journal (ReportPrint.PrintResJnlLine): its template and batch
		OnInitReport: func(run *Run) error {
			line, ok := run.Context().(*gtables.ResJournalLine)
			if ok && line.Journal_template_name != "" {
				run.SetRequestFilter("Res_Journal_Batch", "journal_template_name", line.Journal_template_name.String())
				run.SetRequestFilter("Res_Journal_Batch", "name", line.Journal_batch_name.String())
			}
			return nil
		},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Res_Journal_Batch", Table: gtables.ResJournalBatchTableName, Key: []string{"journal_template_name", "name"},
			RequestFilterFields: []string{"journal_template_name", "name"},
			OnAfterGetRecord: Trigger(func(batch *gtables.ResJournalBatch, run *Run) error {
				line := gtables.NewResJournalLine(run)
				line.SetRange("journal_template_name", batch.Journal_template_name)
				line.SetRange("journal_batch_name", batch.Name)
				r.batch = batch
				return jnlTestBatchHasLines(run, line.IsEmpty())
			}),
			DataItems: []*DataItem{{
				Name: "Res_Journal_Line", Table: gtables.ResJournalLineTableName,
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

// lineOnPreDataItem: a recurring journal's lines up to the work date (not filtered on the
// posting or expiration date); the batch's number series
func (r *resJnlTest) lineOnPreDataItem(line *gtables.ResJournalLine, run *Run) error {
	r.template = gtables.NewResJournalTemplate(run)
	if !r.template.Get(r.batch.Journal_template_name) {
		r.template.Init()
	}
	// the user's filters, before a recurring journal's own (GETFILTERS in OnPreReport)
	setFilterText(run, line)
	r.startBatch(run, line, r.template.Recurring, r.batch.No_series)
	return nil
}

func (r *resJnlTest) lineOnAfterGetRecord(line *gtables.ResJournalLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.errors = nil
	if tables.ResJnlLineEmptyLine(line) {
		return Skip()
	}
	caption := func(field string) string { return line.GetFieldCaption(field, language) }

	// MakeRecurringText: the texts as posting writes them
	if !line.Posting_date.IsZero() && line.Resource_no != "" && line.Recurring_method != gtables.ResJournalLine_Recurring_method.Blank {
		docNo, description := tables.RecurringText(run, line.Posting_date, line.Document_no, line.Description, 20, 50)
		line.Document_no, line.Description = docNo, description
	}

	if line.Resource_no == "" {
		r.addError(run.Text("MUST_BE_SPECIFIED", caption("resource_no")))
	} else {
		res := gtables.NewResource(run)
		if !res.Get(line.Resource_no) {
			r.addError(run.Text("RESOURCE_NOT_EXIST", line.Resource_no))
		} else {
			if res.Blocked {
				r.addError(run.Text("MUST_BE_FOR_RESOURCE", res.GetFieldCaption("blocked", language), formatValue(false, "", language), line.Resource_no))
			}
		}
	}

	r.checkGenPostingSetup(run, line.Gen_bus_posting_group, line.Gen_prod_posting_group, caption("gen_prod_posting_group"))

	r.checkRecurringLine(run, line, recurringLine{
		recurring:      r.template.Recurring,
		methodBlank:    line.Recurring_method == gtables.ResJournalLine_Recurring_method.Blank,
		methodVariable: line.Recurring_method == gtables.ResJournalLine_Recurring_method.Variable,
		frequency:      line.Recurring_frequency,
		quantityZero:   line.Quantity.IsZero(),
		quantityField:  "quantity",
	})

	r.checkPostingDate(run, line.Posting_date, caption("posting_date"))
	r.checkDocumentDate(run, line.Document_date, caption("document_date"))
	r.checkDocumentNo(run, r.batch.No_series, line.Document_no)
	return nil
}

// Report 1103 Resource Register
type resourceRegister struct {
	resReg *gtables.ResourceRegister
}

func (r *resourceRegister) Definition() *Definition {
	return &Definition{
		Options: []Option{{Name: "print_descriptions", Type: "bool", Default: "false"}},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Resource_Register", Table: gtables.ResourceRegisterTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "creation_date", "source_code", "journal_batch_name"},
			OnPreDataItem:       headWithFilters[*gtables.ResourceRegister](nil),
			OnAfterGetRecord: Trigger(func(resReg *gtables.ResourceRegister, run *Run) error {
				r.resReg = resReg
				sourceCode := gtables.NewSourceCode(run)
				sourceCodeText := ""
				if resReg.Source_code != "" {
					sourceCodeText = run.Text("SOURCE_CODE", resReg.Source_code)
					if !sourceCode.Get(resReg.Source_code) {
						sourceCode.Init()
					}
				}
				run.SetValue("register_text", run.Text("REGISTER_NO", resReg.No))
				run.SetValue("source_code_text", sourceCodeText)
				run.SetValue("source_code_description", sourceCode.Description.String())
				run.SetValue("no_of_entries", resReg.To_entry_no-resReg.From_entry_no+1)
				run.SetValue("entries_text", run.Text("NO_OF_ENTRIES", resReg.No))
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "Res_Ledger_Entry", Table: gtables.ResLedgerEntryTableName, Key: []string{"entry_no"},
				RequestFilterFields: []string{"entry_type", "posting_date", "work_type_code"},
				OnPreDataItem: Trigger(func(resLedgEntry *gtables.ResLedgerEntry, run *Run) error {
					resLedgEntry.SetRange("entry_no", r.resReg.From_entry_no, r.resReg.To_entry_no)
					return nil
				}),
				OnAfterGetRecord: Trigger(func(resLedgEntry *gtables.ResLedgerEntry, run *Run) error {
					resDescription := ""
					if run.OptionBool("print_descriptions") {
						resDescription = resLedgEntry.Description.String()
						if resDescription == "" {
							resource := gtables.NewResource(run)
							if !resource.Get(resLedgEntry.Resource_no) {
								resource.Init()
							}
							resDescription = resource.Name.String()
						}
					}
					run.SetValue("res_description", resDescription)
					return nil
				}),
			}},
		}},
	}
}
