package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fAJournalTemplateLogic is the business logic of table 5619 FA Journal Template: a fixed asset
// journal with its page, source code and numbering.
//
// Not migrated yet: the test and posting reports (report prints), recurring FA journals (page
// 5634).
type fAJournalTemplateLogic struct{}

func init() { gtables.RegisterFAJournalTemplateLogic(fAJournalTemplateLogic{}) }

// faJournalPageID is the page of the FA journal templates (Form ID: Fixed Asset Journal)
const faJournalPageID = 5629

// OnInsert: a template without a page gets the Fixed Asset Journal
func (fAJournalTemplateLogic) OnInsert(rec *gtables.FAJournalTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnDelete deletes the template's journal lines and batches
func (fAJournalTemplateLogic) OnDelete(rec *gtables.FAJournalTemplate) error {
	faJnlLine := gtables.NewFAJournalLine(rec)
	faJnlLine.SetRange("journal_template_name", rec.Name)
	faJnlLine.DeleteAll()
	faJnlBatch := gtables.NewFAJournalBatch(rec)
	faJnlBatch.SetRange("journal_template_name", rec.Name)
	faJnlBatch.DeleteAll()
	return nil
}

// OnValidate_Page_id: no page — the journal's page (by Recurring)
func (fAJournalTemplateLogic) OnValidate_Page_id(rec *gtables.FAJournalTemplate) error {
	if rec.Page_id == 0 {
		return rec.Validate("recurring", rec.Recurring)
	}
	return nil
}

// OnValidate_Source_code: the template's journal lines take the new source code
func (fAJournalTemplateLogic) OnValidate_Source_code(rec *gtables.FAJournalTemplate) error {
	faJnlLine := gtables.NewFAJournalLine(rec)
	faJnlLine.SetRange("journal_template_name", rec.Name)
	faJnlLine.ModifyAll("source_code", rec.Source_code)
	return nil
}

// OnValidate_Recurring: the journal's page and the Source Code Setup's fixed asset journal source
// code; recurring journals have no No. Series
func (fAJournalTemplateLogic) OnValidate_Recurring(rec *gtables.FAJournalTemplate) error {
	rec.Page_id = faJournalPageID
	rec.Test_report_id = FAJnlTestReportID
	rec.Posting_report_id = FARegisterReportID
	if rec.Recurring {
		rec.Page_id = RecurringFAJnlPageID
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(rec)
	rec.Source_code = sourceCodeSetup.Fixed_asset_journal
	if rec.Recurring {
		return rec.TestField("no_series", types.Code(""))
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (fAJournalTemplateLogic) OnValidate_No_series(rec *gtables.FAJournalTemplate) error {
	if rec.No_series != "" {
		if rec.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.FAJournalTemplateTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			rec.Posting_no_series = ""
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series
func (fAJournalTemplateLogic) OnValidate_Posting_no_series(rec *gtables.FAJournalTemplate) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.FAJournalTemplateTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	return nil
}

// RecurringFAJnlPageID is page 5634 Recurring Fixed Asset Journal, the page of a recurring template
const RecurringFAJnlPageID = 5634

// The journal's reports: 5602 Fixed Asset Journal - Test, 5603 Fixed Asset Register
const (
	FAJnlTestReportID  = 5602
	FARegisterReportID = 5603
)
