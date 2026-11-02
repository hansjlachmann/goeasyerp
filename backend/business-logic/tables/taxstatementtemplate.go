package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// taxStatementTemplateLogic is the business logic of table 255 Tax Statement Template (see taxstatement.go).
type taxStatementTemplateLogic struct{}

func init() { gtables.RegisterTaxStatementTemplateLogic(taxStatementTemplateLogic{}) }

// OnInsert: a template without a page gets the Tax Statement page and report
func (taxStatementTemplateLogic) OnInsert(rec *gtables.TaxStatementTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnDelete: the template's statements and lines go with it
func (taxStatementTemplateLogic) OnDelete(rec *gtables.TaxStatementTemplate) error {
	line := gtables.NewTaxStatementLine(rec)
	line.SetRange("statement_template_name", rec.Name)
	line.DeleteAll()
	name := gtables.NewTaxStatementName(rec)
	name.SetRange("statement_template_name", rec.Name)
	name.DeleteAll()
	return nil
}

// OnValidate_Page_id: no page — the Tax Statement page; the report is Tax Statement
func (taxStatementTemplateLogic) OnValidate_Page_id(rec *gtables.TaxStatementTemplate) error {
	if rec.Page_id == 0 {
		rec.Page_id = TaxStatementPageID
	}
	rec.Tax_statement_report_id = TaxStatementReportID
	return nil
}
