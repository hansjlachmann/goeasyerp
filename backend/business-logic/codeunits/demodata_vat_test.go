package codeunits

import (
	"database/sql"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/taxstatement"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func TestDemoDataParameter(t *testing.T) {
	for parameter, want := range map[string]struct {
		size demodata.Size
		vat  bool
	}{"": {demodata.Small, false}, "vat": {demodata.Small, true}, "LARGE VAT": {demodata.Large, true}, "vat,heavy": {demodata.Heavy, true}, "SMALL": {demodata.Small, false}} {
		size, vat, err := demodata.ParseParameter(parameter)
		if err != nil || size != want.size || vat != want.vat {
			t.Errorf("%q: %s %v %v", parameter, size, vat, err)
		}
	}
	for _, parameter := range []string{"SMALL LARGE", "MEDIUM", "VAT X"} {
		_, _, err := demodata.ParseParameter(parameter)
		if err == nil {
			t.Errorf("%q accepted", parameter)
		}
	}
}

// Codeunit 901 with VAT: the demo's postings calculate VAT — Normal VAT at home, Reverse Charge
// VAT abroad (a purchase's VAT on the purchase VAT and the reverse charge account, a sale none),
// no sales tax —, the G/L balances, the VAT return adds up and the general journals post
func TestDemoDataVATCompany(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "vat", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	opts := demodata.Options{VAT: true, Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: openItemJnlPostLine, BOMJnlPostLine: openBOMJnlPostLine, SalesPost: openSalesPost, PurchPost: openPurchPost,
		GenJnlPostLine: openGenJnlPostLine, ProjectJnlPostLine: openProjectJnlPostLine, Journals: true,
		ReminderIssue: openReminderIssue, FinChrgMemoIssue: openFinChrgMemoIssue, AdjustExchRates: adjustExchRates}
	if _, err := demodata.Create(db, "vat", database.DBTypeSQLite, opts); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "vat", database.DBTypeSQLite)

	setup := gtables.NewTaxPostingSetup(src)
	setup.SetRange("tax_calculation_type", gtables.TaxPostingSetup_Tax_calculation_type.SalesTax)
	if n := setup.Count(); n != 0 {
		t.Errorf("%d tax posting setups still calculate sales tax", n)
	}
	count := func(calcType gtables.TaxEntryTax_calculation_type, entryType gtables.TaxEntryType) (int, types.Decimal) {
		taxEntry := gtables.NewTaxEntry(src)
		taxEntry.SetRange("tax_calculation_type", calcType)
		taxEntry.SetRange("type", entryType)
		if err := taxEntry.CalcSums("amount"); err != nil {
			t.Fatal(err)
		}
		return taxEntry.Count(), taxEntry.Amount
	}
	if n, _ := count(gtables.TaxEntry_Tax_calculation_type.SalesTax, gtables.TaxEntry_Type.Sale); n != 0 {
		t.Errorf("%d sales tax entries", n)
	}
	if n, amount := count(gtables.TaxEntry_Tax_calculation_type.NormalVAT, gtables.TaxEntry_Type.Sale); n == 0 || !amount.IsNegative() {
		t.Errorf("sales VAT: %d entries, %s", n, amount)
	}
	if n, amount := count(gtables.TaxEntry_Tax_calculation_type.NormalVAT, gtables.TaxEntry_Type.Purchase); n == 0 || !amount.IsPositive() {
		t.Errorf("purchase VAT: %d entries, %s", n, amount)
	}
	n, reverseCharge := count(gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT, gtables.TaxEntry_Type.Purchase)
	if n == 0 || !reverseCharge.IsPositive() {
		t.Errorf("reverse charge purchases: %d entries, %s", n, reverseCharge)
	}
	if n, amount := count(gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT, gtables.TaxEntry_Type.Sale); n == 0 || !amount.IsZero() {
		t.Errorf("reverse charge sales: %d entries, VAT %s", n, amount)
	}
	// the reverse charge account holds the purchases' reverse charge VAT
	glAcc := gtables.NewGLAccount(src)
	if err := glAcc.GetOrError(types.Code("2345")); err != nil {
		t.Fatal(err)
	}
	glAcc.CalcFields("balance")
	if !glAcc.Balance.Equal(reverseCharge.Neg()) {
		t.Errorf("reverse charge account %s, want %s", glAcc.Balance, reverseCharge.Neg())
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L balance check: %+v", r)
	}

	// the VAT return: VAT payable is the output less the input VAT of the open entries
	line := gtables.NewTaxStatementLine(src)
	line.SetRange("statement_template_name", types.Code("TAX"))
	line.SetRange("statement_name", types.Code("DEFAULT"))
	line.SetRange("row_no", types.Code("30"))
	if !line.FindFirst() {
		t.Fatal("no VAT payable row")
	}
	calc := &taxstatement.Calculator{Src: src}
	payable, err := calc.LineTotal(line)
	if err != nil {
		t.Fatal(err)
	}
	open := gtables.NewTaxEntry(src)
	open.SetRange("closed", false)
	open.SetFilter("type", "1|2")
	if err := open.CalcSums("amount"); err != nil {
		t.Fatal(err)
	}
	// the reverse charge's output VAT is not a tax entry: its input VAT counts once (row 23 is
	// left out of the total)
	if want := open.Amount.Sub(reverseCharge); !payable.Equal(want) || payable.IsZero() {
		t.Errorf("VAT payable %s, want %s", payable, want)
	}

	fullVAT := gtables.NewGenJournalLine(src)
	fullVAT.SetRange("journal_template_name", types.Code("GENERAL"))
	fullVAT.SetRange("tax_prod_posting_group", types.Code("FULL"))
	if !fullVAT.FindFirst() || fullVAT.Tax_calculation_type != gtables.GenJournalLine_Tax_calculation_type.FullVAT {
		t.Error("no Full VAT line in the general journal")
	}
	for _, template := range []string{"GENERAL", "SALES", "PURCHASES", "CASHRCPT", "PAYMENT", "ASSETS"} {
		tx, err := ftables.BeginTransaction(db)
		if err != nil {
			t.Fatal(err)
		}
		txSrc := ftables.DBSource(tx, "vat", database.DBTypeSQLite)
		l := gtables.NewGenJournalLine(txSrc)
		l.SetRange("journal_template_name", template)
		if l.FindFirst() {
			_, err = genjnl.NewGenJnlPostBatch(txSrc).Run(l)
			if err != nil {
				t.Errorf("general journal %s: %v", template, err)
			}
			if template == "GENERAL" {
				// the import VAT: no G/L amount on 1440 but the VAT, a Full VAT tax entry
				taxEntry := gtables.NewTaxEntry(txSrc)
				taxEntry.SetRange("tax_calculation_type", gtables.TaxEntry_Tax_calculation_type.FullVAT)
				if !taxEntry.FindFirst() || !taxEntry.Amount.Equal(types.NewDecimalFromInt(2500)) || !taxEntry.Base.IsZero() {
					t.Errorf("Full VAT entry: %d found, amount %s base %s", taxEntry.Count(), taxEntry.Amount, taxEntry.Base)
				}
			}
		}
		_ = ftables.Rollback(tx)
	}
}
