package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 1202 Project Journal - Test (Test Report on the project journals): per batch its lines
// with what posting would refuse under each line — the project (blocked, not on Order), the
// posting groups, the resource, item or G/L account, the recurring fields, the posting date
// (closing date, date order, allowed range) and gaps in the batch's number series — and the
// totals of the batch
const ProjectJnlTestID = 1202

func init() {
	Register(ProjectJnlTestID, "project-journal-test", func() Report { return &projectJnlTest{} })
}

type projectJnlTest struct {
	jnlTest
	batch    *gtables.ProjectJournalBatch
	template *gtables.ProjectJournalTemplate
}

func (r *projectJnlTest) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.ProjectJournalLineTableName,
		// run from a journal (ReportPrint.PrintJobJnlLine): its template and batch
		OnInitReport: func(run *Run) error {
			line, ok := run.Context().(*gtables.ProjectJournalLine)
			if ok && line.Journal_template_name != "" {
				run.SetRequestFilter("Project_Journal_Batch", "journal_template_name", line.Journal_template_name.String())
				run.SetRequestFilter("Project_Journal_Batch", "name", line.Journal_batch_name.String())
			}
			return nil
		},
		OnPreReport: projectHead,
		DataItems: []*DataItem{{
			Name: "Project_Journal_Batch", Table: gtables.ProjectJournalBatchTableName, Key: []string{"journal_template_name", "name"},
			RequestFilterFields: []string{"journal_template_name", "name"},
			OnAfterGetRecord: Trigger(func(batch *gtables.ProjectJournalBatch, run *Run) error {
				line := gtables.NewProjectJournalLine(run)
				line.SetRange("journal_template_name", batch.Journal_template_name)
				line.SetRange("journal_batch_name", batch.Name)
				r.batch = batch
				return jnlTestBatchHasLines(run, line.IsEmpty())
			}),
			DataItems: []*DataItem{{
				Name: "Project_Journal_Line", Table: gtables.ProjectJournalLineTableName,
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
func (r *projectJnlTest) lineOnPreDataItem(line *gtables.ProjectJournalLine, run *Run) error {
	r.template = gtables.NewProjectJournalTemplate(run)
	if !r.template.Get(r.batch.Journal_template_name) {
		r.template.Init()
	}
	// the user's filters, before a recurring journal's own (GETFILTERS in OnPreReport)
	setFilterText(run, line)
	r.startBatch(run, line, r.template.Recurring, r.batch.No_series)
	return nil
}

func (r *projectJnlTest) lineOnAfterGetRecord(line *gtables.ProjectJournalLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.errors = nil
	if tables.ProjectJnlLineEmptyLine(line) {
		return Skip()
	}
	caption := func(field string) string { return line.GetFieldCaption(field, language) }
	mustBeSpecified := func(field string) { r.addError(run.Text("MUST_BE_SPECIFIED", caption(field))) }

	// MakeRecurringText: the texts as posting writes them
	if !line.Posting_date.IsZero() && line.No != "" && line.Recurring_method != gtables.ProjectJournalLine_Recurring_method.Blank {
		docNo, description := tables.RecurringText(run, line.Posting_date, line.Document_no, line.Description, 20, 50)
		line.Document_no, line.Description = docNo, description
	}

	if line.Project_no == "" {
		mustBeSpecified("project_no")
	} else {
		project := gtables.NewProject(run)
		if !project.Get(line.Project_no) {
			r.addError(run.Text("PROJECT_NOT_EXIST", line.Project_no))
		} else {
			if project.Blocked {
				r.addError(run.Text("MUST_BE_FOR_PROJECT", project.GetFieldCaption("blocked", language), formatValue(false, "", language), line.Project_no))
			}
			if project.Status != gtables.Project_Status.Order {
				r.addError(run.Text("MUST_BE_FOR_PROJECT", project.GetFieldCaption("status", language),
					project.GetOptionCaption("status", "order", language), line.Project_no))
			}
		}
	}

	if line.Type != gtables.ProjectJournalLine_Type.AccountGL {
		r.checkGenPostingSetup(run, line.Gen_bus_posting_group, line.Gen_prod_posting_group, caption("gen_prod_posting_group"))
	}

	if line.No == "" {
		mustBeSpecified("no")
	} else if !line.Post_project_entry_only {
		switch line.Type {
		case gtables.ProjectJournalLine_Type.Resource:
			res := gtables.NewResource(run)
			if !res.Get(line.No) {
				r.addError(run.Text("RESOURCE_NOT_EXIST", line.No))
			} else if res.Blocked {
				r.addError(run.Text("MUST_BE_FOR_RESOURCE", res.GetFieldCaption("blocked", language), formatValue(false, "", language), line.No))
			}
		case gtables.ProjectJournalLine_Type.Item:
			item := gtables.NewItem(run)
			if !item.Get(line.No) {
				r.addError(run.Text("ITEM_NOT_EXIST", line.No))
			} else if item.Blocked {
				r.addError(run.Text("MUST_BE_FOR_ITEM", item.GetFieldCaption("blocked", language), formatValue(false, "", language), line.No))
			}
		case gtables.ProjectJournalLine_Type.AccountGL:
			if line.Balancing_acc_no != "" {
				r.checkGLAccount(run, line.No)
				r.checkGLAccount(run, line.Balancing_acc_no)
			}
		}
	}

	r.checkRecurringLine(run, line, recurringLine{
		recurring:      r.template.Recurring,
		methodBlank:    line.Recurring_method == gtables.ProjectJournalLine_Recurring_method.Blank,
		methodVariable: line.Recurring_method == gtables.ProjectJournalLine_Recurring_method.Variable,
		frequency:      line.Recurring_frequency,
		quantityZero:   line.Quantity.IsZero(),
		quantityField:  "quantity",
	})

	r.checkPostingDate(run, line.Posting_date, caption("posting_date"))
	r.checkDocumentDate(run, line.Document_date, caption("document_date"))
	r.checkDocumentNo(run, r.batch.No_series, line.Document_no)
	return nil
}
