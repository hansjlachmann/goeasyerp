package tables

import (
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// taxStatementNameLogic is the business logic of table 257 Tax Statement Name (see taxstatement.go).
type taxStatementNameLogic struct{}

func init() { gtables.RegisterTaxStatementNameLogic(taxStatementNameLogic{}) }

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
