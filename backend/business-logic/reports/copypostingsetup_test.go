package reports_test

import (
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// answerYes answers every question of the run with Yes
func answerYes(t *testing.T) {
	d := fcodeunits.OpenDialogWithID("", "copy-posting-setup-test")
	fcodeunits.SetCurrentDialog(d)
	t.Cleanup(fcodeunits.ClearCurrentDialog)
	go func() {
		for ev := range d.Events() {
			if ev.EventType == "confirm" {
				for !d.RespondToConfirm(ev.ConfirmID, true) {
					time.Sleep(time.Millisecond)
				}
			}
		}
	}()
}

// Copy - General Posting Setup (87) / Copy - Tax Posting Setup (85): the setup line the page is
// on gets the ticked groups of accounts of another line
func TestCopyPostingSetup(t *testing.T) {
	src := listCompany(t)
	answerYes(t)

	genPostingSetup := gtables.NewGeneralPostingSetup(src)
	genPostingSetup.SetFilter("sales_account", "<>''")
	genPostingSetup.SetFilter("cogs_account", "<>''")
	genPostingSetup.SetFilter("purch_account", "<>''")
	if !genPostingSetup.FindFirst() {
		t.Fatal("no general posting setup with sales, purchase and COGS accounts")
	}
	target := gtables.NewGeneralPostingSetup(src)
	target.Init()
	target.Gen_bus_posting_group = "COPYTEST"
	target.Gen_prod_posting_group = genPostingSetup.Gen_prod_posting_group
	mustOK(t, target.InsertOrError(false))
	_, err := reports.Execute(reports.CopyGenPostingSetupID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: target,
		Options: map[string]string{"gen_bus_posting_group": genPostingSetup.Gen_bus_posting_group.String(),
			"gen_prod_posting_group": genPostingSetup.Gen_prod_posting_group.String(),
			"sales":                  "true", "purch": "false", "invt": "true", "projects": "true"},
	})
	mustOK(t, err)
	mustOK(t, target.GetOrError(map[string]interface{}{"gen_bus_posting_group": target.Gen_bus_posting_group, "gen_prod_posting_group": target.Gen_prod_posting_group}))
	if target.Sales_account != genPostingSetup.Sales_account || target.Cogs_account != genPostingSetup.Cogs_account {
		t.Errorf("sales/inventory accounts not copied: %s %s, want %s %s", target.Sales_account, target.Cogs_account, genPostingSetup.Sales_account, genPostingSetup.Cogs_account)
	}
	if target.Purch_account != "" {
		t.Errorf("purchase account copied without Purchase Accounts: %s", target.Purch_account)
	}

	taxPostingSetup := gtables.NewTaxPostingSetup(src)
	if !taxPostingSetup.FindLast() {
		t.Fatal("no tax posting setup")
	}
	taxPostingSetup.Sales_vat_account = genPostingSetup.Sales_account
	mustOK(t, taxPostingSetup.ModifyOrError(false))
	taxTarget := gtables.NewTaxPostingSetup(src)
	taxTarget.Init()
	taxTarget.Tax_bus_posting_group = "COPYTEST"
	taxTarget.Tax_prod_posting_group = taxPostingSetup.Tax_prod_posting_group
	mustOK(t, taxTarget.InsertOrError(false))
	_, err = reports.Execute(reports.CopyTaxPostingSetupID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: taxTarget,
		Options: map[string]string{"tax_bus_posting_group": taxPostingSetup.Tax_bus_posting_group.String(),
			"tax_prod_posting_group": taxPostingSetup.Tax_prod_posting_group.String(),
			"tax_setup":              "true", "sales": "true", "purch": "true"},
	})
	mustOK(t, err)
	mustOK(t, taxTarget.GetOrError(map[string]interface{}{"tax_bus_posting_group": taxTarget.Tax_bus_posting_group, "tax_prod_posting_group": taxTarget.Tax_prod_posting_group}))
	if taxTarget.Sales_vat_account != taxPostingSetup.Sales_vat_account || taxTarget.Tax_calculation_type != taxPostingSetup.Tax_calculation_type {
		t.Errorf("tax setup not copied: %s, want %s", taxTarget.Sales_vat_account, taxPostingSetup.Sales_vat_account)
	}
}
