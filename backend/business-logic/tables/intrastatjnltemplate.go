package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Intrastat page and report numbers
const (
	IntrastatJournalPageID       = 311 // Intrastat Journal
	IntrastatChecklistReportID   = 502 // Intrastat - Checklist
	IntrastatJnlTemplateListPage = 326 // Intrastat Jnl. Template List
)

// intrastatJnlTemplateLogic is the business logic of table 261 Intrastat Jnl. Template.
type intrastatJnlTemplateLogic struct{}

func init() { gtables.RegisterIntrastatJnlTemplateLogic(intrastatJnlTemplateLogic{}) }

// OnInsert: the template's page and checklist
func (intrastatJnlTemplateLogic) OnInsert(rec *gtables.IntrastatJnlTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnDelete: the template's lines and batches go with it
func (intrastatJnlTemplateLogic) OnDelete(rec *gtables.IntrastatJnlTemplate) error {
	intraJnlLine := gtables.NewIntrastatJnlLine(rec)
	intraJnlLine.SetRange("journal_template_name", rec.Name)
	intraJnlLine.DeleteAll()
	intraJnlBatch := gtables.NewIntrastatJnlBatch(rec)
	intraJnlBatch.SetRange("journal_template_name", rec.Name)
	intraJnlBatch.DeleteAll()
	return nil
}

// OnValidate_Page_id: no page — the Intrastat Journal; the checklist is Intrastat - Checklist
func (intrastatJnlTemplateLogic) OnValidate_Page_id(rec *gtables.IntrastatJnlTemplate) error {
	if rec.Page_id == 0 {
		rec.Page_id = IntrastatJournalPageID
	}
	rec.Checklist_report_id = IntrastatChecklistReportID
	return nil
}
