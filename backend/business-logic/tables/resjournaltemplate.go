package tables

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// resJournalTemplateLogic is the business logic of table 206 Res. Journal Template: a resource
// journal with its page, source code and numbering.
//
// Not migrated yet: the test and posting reports (report prints), recurring resource journals
// (page 290).
type resJournalTemplateLogic struct{}

func init() { gtables.RegisterResJournalTemplateLogic(resJournalTemplateLogic{}) }

// resJournalPageID is the page of the resource journal templates (Form ID: Resource Journal)
const resJournalPageID = 207

// OnInsert: a template without a page gets the resource journal's
func (resJournalTemplateLogic) OnInsert(rec *gtables.ResJournalTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnDelete deletes the template's journal lines and batches
func (resJournalTemplateLogic) OnDelete(rec *gtables.ResJournalTemplate) error {
	resJnlLine := gtables.NewResJournalLine(rec)
	resJnlLine.SetRange("journal_template_name", rec.Name)
	resJnlLine.DeleteAll()
	resJnlBatch := gtables.NewResJournalBatch(rec)
	resJnlBatch.SetRange("journal_template_name", rec.Name)
	resJnlBatch.DeleteAll()
	return nil
}

// OnValidate_Page_id: no page — the journal's page (by Recurring)
func (resJournalTemplateLogic) OnValidate_Page_id(rec *gtables.ResJournalTemplate) error {
	if rec.Page_id == 0 {
		return rec.Validate("recurring", rec.Recurring)
	}
	return nil
}

// OnValidate_Source_code: the template's journal lines take the new source code
func (resJournalTemplateLogic) OnValidate_Source_code(rec *gtables.ResJournalTemplate) error {
	resJnlLine := gtables.NewResJournalLine(rec)
	resJnlLine.SetRange("journal_template_name", rec.Name)
	resJnlLine.ModifyAll("source_code", rec.Source_code)
	return nil
}

// OnValidate_Recurring: the journal's page and the Source Code Setup's resource journal source
// code; recurring journals have no No. Series
func (resJournalTemplateLogic) OnValidate_Recurring(rec *gtables.ResJournalTemplate) error {
	rec.Page_id = resJournalPageID
	if rec.Recurring {
		rec.Page_id = RecurringResJnlPageID
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(rec)
	rec.Source_code = sourceCodeSetup.Resource_journal
	if rec.Recurring {
		return rec.TestField("no_series", types.Code(""))
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (resJournalTemplateLogic) OnValidate_No_series(rec *gtables.ResJournalTemplate) error {
	if rec.No_series != "" {
		if rec.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.ResJournalTemplateTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			rec.Posting_no_series = ""
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series
func (resJournalTemplateLogic) OnValidate_Posting_no_series(rec *gtables.ResJournalTemplate) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.ResJournalTemplateTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	return nil
}

// RecurringResJnlPageID is page 290 Recurring Resource Jnl., the page of a recurring template
const RecurringResJnlPageID = 290
