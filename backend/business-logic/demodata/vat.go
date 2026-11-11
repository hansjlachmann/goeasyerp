package demodata

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// vatPercents: the VAT % of the demo's tax product posting groups
var vatPercents = map[types.Code]types.Decimal{
	"STANDARD": types.NewDecimalFromInt(25),
	"REDUCED":  types.NewDecimalFromInt(15),
}

// applyVATSetup makes the demo company a VAT company (Options.VAT), before anything is posted:
// the setup's vat_data (product group FULL, the reverse charge account 2345, the VAT return
// as the tax statement), the tax posting setups calculating VAT — Normal VAT for the domestic
// and blank business groups, Reverse Charge VAT for EXPORT, at the product group's VAT % —
// on the sales VAT account 2330, the purchase VAT account 1440 and the reverse charge account
// 2345, and the tax accounts named for VAT
func applyVATSetup(exec database.Executor, company string, dbType database.DBType, vatData []tableRecords, counts *Counts, p *progress) error {
	src := ftables.DBSource(exec, company, dbType)
	taxStmtLine := gtables.NewTaxStatementLine(src)
	taxStmtLine.SetRange("statement_template_name", types.Code("TAX"))
	taxStmtLine.SetRange("statement_name", types.Code("DEFAULT"))
	taxStmtLine.DeleteAll()
	err := insertRecords(exec, company, dbType, vatData, &counts.MasterData, p)
	if err != nil {
		return err
	}

	taxPostingSetup := gtables.NewTaxPostingSetup(src)
	taxPostingSetup.SetRange("tax_calculation_type", gtables.TaxPostingSetup_Tax_calculation_type.SalesTax)
	for found := taxPostingSetup.FindSet(); found; found = taxPostingSetup.Next() {
		setup := gtables.NewTaxPostingSetup(src)
		setup.Copy(taxPostingSetup)
		setup.Tax_calculation_type = gtables.TaxPostingSetup_Tax_calculation_type.NormalVAT
		if setup.Tax_bus_posting_group == "EXPORT" {
			setup.Tax_calculation_type = gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT
		}
		setup.Vat_percent = vatPercents[setup.Tax_prod_posting_group]
		setup.Sales_vat_account = "2330"
		setup.Purchase_vat_account = "1440"
		setup.Reverse_chrg_vat_acc = "2345"
		err = setup.ModifyOrError(true)
		if err != nil {
			return &InsertError{Table: gtables.TaxPostingSetupTableName, Key: setup.Tax_bus_posting_group.String() + "," + setup.Tax_prod_posting_group.String(), Err: err}
		}
	}

	for no, name := range map[types.Code]types.Text{"1440": "Purchase VAT", "2330": "Sales VAT", "2340": "VAT Settlement"} {
		glAcc := gtables.NewGLAccount(src)
		err = glAcc.GetOrError(no)
		if err != nil {
			return err
		}
		glAcc.Name = name
		glAcc.Search_name = types.NewCode(name.String())
		err = glAcc.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	taxStmtTemplate := gtables.NewTaxStatementTemplate(src)
	if taxStmtTemplate.Get(types.Code("TAX")) {
		taxStmtTemplate.Description = "VAT Statement"
		err = taxStmtTemplate.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	taxStmtName := gtables.NewTaxStatementName(src)
	if taxStmtName.Get(map[string]interface{}{"statement_template_name": "TAX", "name": "DEFAULT"}) {
		taxStmtName.Description = "VAT return"
		err = taxStmtName.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	// the reverse charge account in its place in the chart of accounts
	return tables.IndentChartOfAccounts(exec, company, dbType, nil)
}
