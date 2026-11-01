package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// fAReclassJournalTemplateLogic is the business logic of table 5622 FA Reclass. Journal
// Template: a journal that moves amounts from one fixed asset to another, with its page.
type fAReclassJournalTemplateLogic struct{}

func init() { gtables.RegisterFAReclassJournalTemplateLogic(fAReclassJournalTemplateLogic{}) }

// faReclassJournalPageID is the page of the FA reclass. journal templates (Form ID: FA Reclass.
// Journal)
const faReclassJournalPageID = 5636

// OnInsert: a template without a page gets the FA Reclass. Journal
func (fAReclassJournalTemplateLogic) OnInsert(rec *gtables.FAReclassJournalTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnDelete deletes the template's journal lines and batches
func (fAReclassJournalTemplateLogic) OnDelete(rec *gtables.FAReclassJournalTemplate) error {
	faReclassJnlLine := gtables.NewFAReclassJournalLine(rec)
	faReclassJnlLine.SetRange("journal_template_name", rec.Name)
	faReclassJnlLine.DeleteAll()
	faReclassJnlBatch := gtables.NewFAReclassJournalBatch(rec)
	faReclassJnlBatch.SetRange("journal_template_name", rec.Name)
	faReclassJnlBatch.DeleteAll()
	return nil
}

// OnValidate_Page_id: no page — the FA Reclass. Journal
func (fAReclassJournalTemplateLogic) OnValidate_Page_id(rec *gtables.FAReclassJournalTemplate) error {
	if rec.Page_id == 0 {
		rec.Page_id = faReclassJournalPageID
	}
	return nil
}
