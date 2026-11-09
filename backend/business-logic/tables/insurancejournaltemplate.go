package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// insuranceJournalTemplateLogic is the business logic of table 5633 Insurance Journal Template:
// an insurance journal with its page, source code and numbering.
//
// Not migrated yet: the test and posting reports (report prints).
type insuranceJournalTemplateLogic struct{}

func init() { gtables.RegisterInsuranceJournalTemplateLogic(insuranceJournalTemplateLogic{}) }

// insuranceJournalPageID is the page of the insurance journal templates (Form ID: Insurance
// Journal)
const insuranceJournalPageID = 5654

// OnInsert: a template without a page gets the Insurance Journal
func (insuranceJournalTemplateLogic) OnInsert(rec *gtables.InsuranceJournalTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnDelete deletes the template's journal lines and batches
func (insuranceJournalTemplateLogic) OnDelete(rec *gtables.InsuranceJournalTemplate) error {
	insuranceJnlLine := gtables.NewInsuranceJournalLine(rec)
	insuranceJnlLine.SetRange("journal_template_name", rec.Name)
	insuranceJnlLine.DeleteAll()
	insuranceJnlBatch := gtables.NewInsuranceJournalBatch(rec)
	insuranceJnlBatch.SetRange("journal_template_name", rec.Name)
	insuranceJnlBatch.DeleteAll()
	return nil
}

// OnValidate_Page_id: no page — the Insurance Journal; the Source Code Setup's insurance journal
// source code
func (insuranceJournalTemplateLogic) OnValidate_Page_id(rec *gtables.InsuranceJournalTemplate) error {
	if rec.Page_id == 0 {
		rec.Page_id = insuranceJournalPageID
		rec.Test_report_id = InsuranceJnlTestReportID
		rec.Posting_report_id = InsuranceRegisterReportID
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(rec)
	rec.Source_code = sourceCodeSetup.Insurance_journal
	return nil
}

// OnValidate_Source_code: the template's journal lines take the new source code
func (insuranceJournalTemplateLogic) OnValidate_Source_code(rec *gtables.InsuranceJournalTemplate) error {
	insuranceJnlLine := gtables.NewInsuranceJournalLine(rec)
	insuranceJnlLine.SetRange("journal_template_name", rec.Name)
	insuranceJnlLine.ModifyAll("source_code", rec.Source_code)
	return nil
}

// OnValidate_No_series: the posting no. series must differ
func (insuranceJournalTemplateLogic) OnValidate_No_series(rec *gtables.InsuranceJournalTemplate) error {
	if rec.No_series != "" && rec.No_series == rec.Posting_no_series {
		rec.Posting_no_series = ""
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series
func (insuranceJournalTemplateLogic) OnValidate_Posting_no_series(rec *gtables.InsuranceJournalTemplate) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.InsuranceJournalTemplateTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	return nil
}

// The journal's reports: 5622 Insurance Journal - Test, 5623 Insurance Register
const (
	InsuranceJnlTestReportID  = 5622
	InsuranceRegisterReportID = 5623
)
