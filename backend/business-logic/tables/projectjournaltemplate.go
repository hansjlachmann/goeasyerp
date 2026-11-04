package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// projectJournalTemplateLogic is the business logic of table 209 Project Journal Template: a project
// journal with its page, source code and numbering.
// The type (Recurring) sets the page, the source code and the test and posting reports.
type projectJournalTemplateLogic struct{}

func init() { gtables.RegisterProjectJournalTemplateLogic(projectJournalTemplateLogic{}) }

// projectJournalPageID is the page of the project journal templates (Form ID: Project Journal)
const projectJournalPageID = 201

// OnInsert: a template without a page gets the project journal's
func (projectJournalTemplateLogic) OnInsert(rec *gtables.ProjectJournalTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnDelete deletes the template's journal lines and batches
func (projectJournalTemplateLogic) OnDelete(rec *gtables.ProjectJournalTemplate) error {
	projectJnlLine := gtables.NewProjectJournalLine(rec)
	projectJnlLine.SetRange("journal_template_name", rec.Name)
	projectJnlLine.DeleteAll()
	projectJnlBatch := gtables.NewProjectJournalBatch(rec)
	projectJnlBatch.SetRange("journal_template_name", rec.Name)
	projectJnlBatch.DeleteAll()
	return nil
}

// OnValidate_Page_id: no page — the journal's page (by Recurring)
func (projectJournalTemplateLogic) OnValidate_Page_id(rec *gtables.ProjectJournalTemplate) error {
	if rec.Page_id == 0 {
		return rec.Validate("recurring", rec.Recurring)
	}
	return nil
}

// OnValidate_Source_code: the template's journal lines take the new source code
func (projectJournalTemplateLogic) OnValidate_Source_code(rec *gtables.ProjectJournalTemplate) error {
	projectJnlLine := gtables.NewProjectJournalLine(rec)
	projectJnlLine.SetRange("journal_template_name", rec.Name)
	projectJnlLine.ModifyAll("source_code", rec.Source_code)
	return nil
}

// OnValidate_Recurring: the journal's page and the Source Code Setup's project journal source
// code; recurring journals have no No. Series
func (projectJournalTemplateLogic) OnValidate_Recurring(rec *gtables.ProjectJournalTemplate) error {
	rec.Page_id = projectJournalPageID
	rec.Test_report_id = ProjectJnlTestReportID
	rec.Posting_report_id = ProjectRegisterReportID
	if rec.Recurring {
		rec.Page_id = RecurringProjectJnlPageID
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(rec)
	rec.Source_code = sourceCodeSetup.Project_journal
	if rec.Recurring {
		return rec.TestField("no_series", types.Code(""))
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (projectJournalTemplateLogic) OnValidate_No_series(rec *gtables.ProjectJournalTemplate) error {
	if rec.No_series != "" {
		if rec.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.ProjectJournalTemplateTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			rec.Posting_no_series = ""
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series
func (projectJournalTemplateLogic) OnValidate_Posting_no_series(rec *gtables.ProjectJournalTemplate) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.ProjectJournalTemplateTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	return nil
}

// RecurringProjectJnlPageID is page 289 Recurring Project Jnl., the page of a recurring template
const RecurringProjectJnlPageID = 289

// The journal's reports: 1202 Project Journal - Test, 1203 Project Register
const (
	ProjectJnlTestReportID  = 1202
	ProjectRegisterReportID = 1203
)
