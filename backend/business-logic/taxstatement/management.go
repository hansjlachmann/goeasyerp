package taxstatement

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// TemplateListPageID is page 319 Tax Statement Template List (the template to open a statement
// with, when there are several)
const TemplateListPageID = 319

// OpenStatementPage opens page 317 Tax Statement (codeunit 340 TemplateSelection and OpenStmt):
// the template in the page's parameters, the only template, a new TAX template when there is
// none — or the user picks one (page 319); then the statement in the parameters, else the
// template's first one — a new DEFAULT statement when it has none. Template and statement
// become the page's filters and parameters.
func OpenStatementPage(rec *gtables.TaxStatementLine, page *pages.Open) error {
	language := fcodeunits.CurrentLanguage()
	ts := i18n.GetInstance()
	template := gtables.NewTaxStatementTemplate(rec)
	if page.Param("template") != "" {
		err := template.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return err
		}
	} else {
		switch template.Count() {
		case 0:
			template.InitRecord()
			template.Name = "TAX"
			template.Description = types.Text(ts.Message("TAX_STMT_TEMPLATE_DESCRIPTION", language))
			err := template.InsertOrError(true)
			if err != nil {
				return err
			}
		case 1:
			template.FindFirst()
		default:
			page.SelectRecord(TemplateListPageID, "template", "name")
			return nil
		}
	}
	page.SetParam("template", template.Name.String())
	page.SetFilter("statement_template_name", template.Name.String())

	name := gtables.NewTaxStatementName(rec)
	name.SetRange("statement_template_name", template.Name)
	statement := types.Code(page.Param("statement"))
	if !name.Get(map[string]interface{}{"statement_template_name": template.Name, "name": statement}) {
		if !name.FindFirst() {
			name.InitRecord()
			name.Statement_template_name = template.Name
			name.Name = "DEFAULT"
			name.Description = types.Text(ts.Message("TAX_STMT_DEFAULT_NAME", language))
			err := name.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		statement = name.Name
	}
	page.SetParam("statement", statement.String())
	page.SetFilter("statement_name", statement.String())
	return nil
}
