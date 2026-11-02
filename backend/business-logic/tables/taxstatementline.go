package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// taxStatementLineLogic is the business logic of table 256 Tax Statement Line (see taxstatement.go).
type taxStatementLineLogic struct{}

func init() { gtables.RegisterTaxStatementLineLogic(taxStatementLineLogic{}) }

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
