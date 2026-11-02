package tables

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// bomJournalTemplateLogic is the business logic of table 88 BOM Journal Template: a BOM
// journal with its page, source code and numbering.
//
// Not migrated yet: the test and posting reports (report prints), recurring BOM journals (page
// 287).
type bomJournalTemplateLogic struct{}

func init() { gtables.RegisterBOMJournalTemplateLogic(bomJournalTemplateLogic{}) }

// bomJournalPageID is the page of the BOM journal templates (Form ID: BOM Journal)
const bomJournalPageID = 109

// OnInsert: a template without a page gets the BOM journal's
func (bomJournalTemplateLogic) OnInsert(rec *gtables.BOMJournalTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnDelete deletes the template's journal lines and batches
func (bomJournalTemplateLogic) OnDelete(rec *gtables.BOMJournalTemplate) error {
	bomJnlLine := gtables.NewBOMJournalLine(rec)
	bomJnlLine.SetRange("journal_template_name", rec.Name)
	bomJnlLine.DeleteAll()
	bomJnlBatch := gtables.NewBOMJournalBatch(rec)
	bomJnlBatch.SetRange("journal_template_name", rec.Name)
	bomJnlBatch.DeleteAll()
	return nil
}

// OnValidate_Page_id: no page — the journal's page (by Recurring)
func (bomJournalTemplateLogic) OnValidate_Page_id(rec *gtables.BOMJournalTemplate) error {
	if rec.Page_id == 0 {
		return rec.Validate("recurring", rec.Recurring)
	}
	return nil
}

// OnValidate_Source_code: the template's journal lines take the new source code
func (bomJournalTemplateLogic) OnValidate_Source_code(rec *gtables.BOMJournalTemplate) error {
	bomJnlLine := gtables.NewBOMJournalLine(rec)
	bomJnlLine.SetRange("journal_template_name", rec.Name)
	bomJnlLine.ModifyAll("source_code", rec.Source_code)
	return nil
}

// OnValidate_Recurring: the journal's page and the Source Code Setup's BOM journal source
// code; recurring journals have no No. Series
func (bomJournalTemplateLogic) OnValidate_Recurring(rec *gtables.BOMJournalTemplate) error {
	rec.Page_id = bomJournalPageID
	if rec.Recurring {
		rec.Page_id = RecurringBOMJnlPageID
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(rec)
	rec.Source_code = sourceCodeSetup.Bom_journal
	if rec.Recurring {
		return rec.TestField("no_series", types.Code(""))
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (bomJournalTemplateLogic) OnValidate_No_series(rec *gtables.BOMJournalTemplate) error {
	if rec.No_series != "" {
		if rec.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.BOMJournalTemplateTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			rec.Posting_no_series = ""
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series
func (bomJournalTemplateLogic) OnValidate_Posting_no_series(rec *gtables.BOMJournalTemplate) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.BOMJournalTemplateTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	return nil
}

// RecurringBOMJnlPageID is page 287 Recurring BOM Jnl., the page of a recurring template
const RecurringBOMJnlPageID = 287
