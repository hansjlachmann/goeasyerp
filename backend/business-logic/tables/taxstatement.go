package tables

import (
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Tax statements: table 255 Tax Statement Template, 257 Tax Statement Name and 256 Tax Statement
// Line. A template has statements, a statement its lines; a line totals G/L accounts (Account
// Totaling, a filter), tax entries (Tax Entry Totaling: the type, posting groups, jurisdiction
// and use tax, and which amount) or other rows (Row Totaling, a filter on Row No.), or is a text.
// The amounts are calculated by `business-logic/taxstatement` (report 12, the preview).

// TaxStatementPageID is page 317 Tax Statement, the page of a template without one
const TaxStatementPageID = 317

// TaxStatementReportID is report 12 Tax Statement
const TaxStatementReportID = 12

type taxStatementTemplateLogic struct{}
type taxStatementNameLogic struct{}
type taxStatementLineLogic struct{}

func init() {
	gtables.RegisterTaxStatementTemplateLogic(taxStatementTemplateLogic{})
	gtables.RegisterTaxStatementNameLogic(taxStatementNameLogic{})
	gtables.RegisterTaxStatementLineLogic(taxStatementLineLogic{})
}

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

// OnDelete: the statement's lines go with it
func (taxStatementNameLogic) OnDelete(rec *gtables.TaxStatementName) error {
	line := gtables.NewTaxStatementLine(rec)
	line.SetRange("statement_template_name", rec.Statement_template_name)
	line.SetRange("statement_name", rec.Name)
	line.DeleteAll()
	return nil
}

// OnRename: the statement's lines follow its new key
func (taxStatementNameLogic) OnRename(rec *gtables.TaxStatementName) error {
	oldTemplate, _ := rec.OldValue("statement_template_name").(types.Code)
	oldName, _ := rec.OldValue("name").(types.Code)
	line := gtables.NewTaxStatementLine(rec)
	line.SetRange("statement_template_name", oldTemplate)
	line.SetRange("statement_name", oldName)
	var lines []*gtables.TaxStatementLine
	for found := line.FindSetBuffered(); found; found = line.Next() {
		l := gtables.NewTaxStatementLine(rec)
		l.Copy(line)
		lines = append(lines, l)
	}
	for _, l := range lines {
		l.Statement_template_name = rec.Statement_template_name
		l.Statement_name = rec.Name
		err := l.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnValidate_Type: a line of another type starts again — only its key, row no. and description
// stay
func (taxStatementLineLogic) OnValidate_Type(rec *gtables.TaxStatementLine) error {
	xRec := rec.XRec()
	if rec.Type == xRec.Type {
		return nil
	}
	template, name, lineNo, rowNo, description, newType := rec.Statement_template_name, rec.Statement_name, rec.Line_no, rec.Row_no, rec.Description, rec.Type
	rec.Init()
	rec.Statement_template_name, rec.Statement_name, rec.Line_no = template, name, lineNo
	rec.Row_no, rec.Description, rec.Type = rowNo, description, newType
	return nil
}

// OnValidate_Account_totaling: the filter may only take posting accounts
func (taxStatementLineLogic) OnValidate_Account_totaling(rec *gtables.TaxStatementLine) error {
	if rec.Account_totaling == "" {
		return nil
	}
	glAcc := gtables.NewGLAccount(rec)
	glAcc.SetFilter("no", rec.Account_totaling.String())
	glAcc.SetFilter("account_type", "<>0")
	if glAcc.FindFirst() {
		return glAcc.TestField("account_type", gtables.GLAccount_Account_type.Posting)
	}
	return nil
}

// OnValidate_Calculate_with: a row total cannot reverse its sign
func (taxStatementLineLogic) OnValidate_Calculate_with(rec *gtables.TaxStatementLine) error {
	if rec.Calculate_with == gtables.TaxStatementLine_Calculate_with.OppositeSign && rec.Type == gtables.TaxStatementLine_Type.RowTotaling {
		return rec.TestField("type", gtables.TaxStatementLine_Type.AccountTotaling)
	}
	return nil
}
