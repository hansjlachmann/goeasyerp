package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
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
	batch                            *gtables.ProjectJournalBatch
	template                         *gtables.ProjectJournalTemplate
	noSeries                         *gtables.NoSeries
	errors                           []string
	lastPostingDate                  types.Date
	lastDocNo                        types.Code
	allowPostingFrom, allowPostingTo types.Date
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
				// a batch without lines prints nothing (PrintOnlyIfDetail)
				line := gtables.NewProjectJournalLine(run)
				line.SetRange("journal_template_name", batch.Journal_template_name)
				line.SetRange("journal_batch_name", batch.Name)
				if line.IsEmpty() {
					return Skip()
				}
				r.batch = batch
				run.SetPageNo(1)
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "Project_Journal_Line", Table: gtables.ProjectJournalLineTableName,
				Key:                 []string{"journal_template_name", "journal_batch_name", "line_no"},
				Link:                map[string]string{"journal_template_name": "journal_template_name", "journal_batch_name": "name"},
				RequestFilterFields: []string{"posting_date"},
				OnPreDataItem:       Trigger(r.lineOnPreDataItem),
				OnAfterGetRecord:    Trigger(r.lineOnAfterGetRecord),
				DataItems: []*DataItem{{
					Name: "Error_Loop", Table: IntegerTable,
					OnPreDataItem: Trigger(func(rec *Integer, run *Run) error {
						rec.SetRange("number", 1, len(r.errors))
						return nil
					}),
					OnAfterGetRecord: Trigger(func(rec *Integer, run *Run) error {
						run.SetValue("error_text", r.errors[rec.Number-1])
						return nil
					}),
				}},
			}},
		}},
	}
}

func (r *projectJnlTest) addError(text string) {
	r.errors = append(r.errors, text)
}

// lineOnPreDataItem: a recurring journal's lines up to the work date (not filtered on the
// posting or expiration date); the batch's number series
func (r *projectJnlTest) lineOnPreDataItem(line *gtables.ProjectJournalLine, run *Run) error {
	language := fcodeunits.CurrentLanguage()
	r.errors = nil
	r.template = gtables.NewProjectJournalTemplate(run)
	if !r.template.Get(r.batch.Journal_template_name) {
		r.template.Init()
	}
	if r.template.Recurring {
		if line.GetFilter("posting_date") != "" {
			r.addError(run.Text("CANNOT_BE_FILTERED", line.GetFieldCaption("posting_date", language)))
		}
		line.SetRange("posting_date", types.Date{}, system.WorkDate())
		if line.GetFilter("expiration_date") != "" {
			r.addError(run.Text("CANNOT_BE_FILTERED", line.GetFieldCaption("expiration_date", language)))
		}
		line.SetFilter("expiration_date", "''|"+system.WorkDate().String()+"..")
	}
	setFilterText(run, line)
	r.noSeries = nil
	if r.batch.No_series != "" {
		r.noSeries = gtables.NewNoSeries(run)
		if !r.noSeries.Get(r.batch.No_series) {
			r.noSeries.Init()
		}
	}
	r.lastPostingDate = types.Date{}
	r.lastDocNo = ""
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
		if line.Gen_prod_posting_group == "" {
			mustBeSpecified("gen_prod_posting_group")
		} else {
			genPostingSetup := gtables.NewGeneralPostingSetup(run)
			if !genPostingSetup.Get(map[string]interface{}{"gen_bus_posting_group": line.Gen_bus_posting_group, "gen_prod_posting_group": line.Gen_prod_posting_group}) {
				r.addError(run.Text("SETUP_NOT_EXIST", genPostingSetup.TableCaption(language), line.Gen_bus_posting_group, line.Gen_prod_posting_group))
			}
		}
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

	r.checkRecurringLine(run, line, mustBeSpecified)

	if line.Posting_date.IsZero() {
		mustBeSpecified("posting_date")
	} else {
		if line.Posting_date.IsClosing() {
			r.addError(run.Text("NOT_CLOSING_DATE", caption("posting_date")))
		}
		if r.noSeries != nil && r.noSeries.Date_order && line.Posting_date.Before(r.lastPostingDate) {
			r.addError(run.Text("NOT_IN_DATE_ORDER"))
		}
		r.lastPostingDate = line.Posting_date
		if r.dateNotAllowed(run, line.Posting_date) {
			r.addError(run.Text("DATE_NOT_ALLOWED", formatDate(line.Posting_date, language)))
		}
	}
	if !line.Document_date.IsZero() && line.Document_date.IsClosing() {
		r.addError(run.Text("NOT_CLOSING_DATE", caption("document_date")))
	}
	if r.batch.No_series != "" {
		if r.lastDocNo != "" && line.Document_no != r.lastDocNo && line.Document_no.String() != system.IncStr(r.lastDocNo.String()) {
			r.addError(run.Text("GAP_IN_NO_SERIES"))
		}
		r.lastDocNo = line.Document_no
	}
	return nil
}

// checkGLAccount: the account exists and is not blocked
func (r *projectJnlTest) checkGLAccount(run *Run, no types.Code) {
	glAcc := gtables.NewGLAccount(run)
	if !glAcc.Get(no) {
		r.addError(run.Text("GL_ACCOUNT_NOT_EXIST", no))
	} else if glAcc.Blocked {
		language := fcodeunits.CurrentLanguage()
		r.addError(run.Text("MUST_BE_FOR_ACCOUNT", glAcc.GetFieldCaption("blocked", language), formatValue(false, "", language), no))
	}
}

// checkRecurringLine (CheckRecurringLine): a recurring journal's line has a recurring method and
// frequency (a variable one a quantity); another journal's line has neither
func (r *projectJnlTest) checkRecurringLine(run *Run, line *gtables.ProjectJournalLine, mustBeSpecified func(string)) {
	language := fcodeunits.CurrentLanguage()
	if r.template.Recurring {
		if line.Recurring_method == gtables.ProjectJournalLine_Recurring_method.Blank {
			mustBeSpecified("recurring_method")
		}
		if line.Recurring_frequency == "" {
			mustBeSpecified("recurring_frequency")
		}
		if line.Recurring_method == gtables.ProjectJournalLine_Recurring_method.Variable && line.Quantity.IsZero() {
			mustBeSpecified("quantity")
		}
		return
	}
	if line.Recurring_method != gtables.ProjectJournalLine_Recurring_method.Blank {
		r.addError(run.Text("CANNOT_BE_SPECIFIED", line.GetFieldCaption("recurring_method", language)))
	}
	if line.Recurring_frequency != "" {
		r.addError(run.Text("CANNOT_BE_SPECIFIED", line.GetFieldCaption("recurring_frequency", language)))
	}
}

// dateNotAllowed: the posting date is outside the user's allowed posting dates (User Setup,
// else General Ledger Setup; no end: no limit)
func (r *projectJnlTest) dateNotAllowed(run *Run, postingDate types.Date) bool {
	if r.allowPostingFrom.IsZero() && r.allowPostingTo.IsZero() {
		if system.UserId() != "" {
			userSetup := gtables.NewUserSetup(run)
			if userSetup.Get(types.Code(system.UserId())) {
				r.allowPostingFrom = userSetup.Allow_posting_from
				r.allowPostingTo = userSetup.Allow_posting_to
			}
		}
		if r.allowPostingFrom.IsZero() && r.allowPostingTo.IsZero() {
			glSetup := gtables.GetGeneralLedgerSetup(run)
			r.allowPostingFrom = glSetup.Allow_posting_from
			r.allowPostingTo = glSetup.Allow_posting_to
		}
		if r.allowPostingTo.IsZero() {
			r.allowPostingTo, _ = system.DMY2Date(31, 12, 9999)
		}
	}
	return postingDate.Before(r.allowPostingFrom) || r.allowPostingTo.Before(postingDate)
}
