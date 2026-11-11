package reports_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/glcheck"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Calc. and Post Tax Settlement after a sale, a purchase and a purchase with use tax: the test
// report posts nothing; the posted run closes every open purchase and sales tax entry by a
// closed settlement entry per jurisdiction and use tax, empties the tax accounts of the postings
// into the settlement account (use tax against its reverse charge) and keeps the G/L balanced
func TestCalcAndPostTaxSettlement(t *testing.T) {
	src := listCompany(t)
	post := genjnl.NewGenJnlPostLine(src)
	for i, l := range []struct {
		accNo  types.Code
		amount string
		useTax bool
	}{{"3110", "-1250", false}, {"4110", "500", false}, {"4110", "1000", true}} {
		line := gtables.NewGenJournalLine(src)
		line.Init()
		mustOK(t, line.Validate("posting_date", types.NewDate(2026, 10, 15)))
		line.Document_no = types.Code("TX" + strconv.Itoa(i+1))
		mustOK(t, line.Validate("account_no", l.accNo))
		mustOK(t, line.Validate("bal_account_no", types.Code("1210")))
		amount, err := types.NewDecimalFromString(l.amount)
		mustOK(t, err)
		mustOK(t, line.Validate("amount", amount))
		if l.useTax {
			mustOK(t, line.Validate("use_tax", true))
		}
		mustOK(t, post.Run(line))
	}
	open := gtables.NewTaxEntry(src)
	open.SetRange("closed", false)
	open.SetRange("type", gtables.TaxEntry_Type.Purchase, gtables.TaxEntry_Type.Sale)
	openCount := open.Count()
	if openCount == 0 {
		t.Fatal("no open tax entries in the demo")
	}
	// the settled tax: the open entries less the use tax (settled against its reverse charge)
	var want types.Decimal
	groups := map[string]bool{}
	for found := open.FindSet(); found; found = open.Next() {
		groups[string(open.Tax_jurisdiction_code)+"/"+open.Type.String()+"/"+map[bool]string{true: "use"}[open.Use_tax]+"/"+string(open.Tax_bus_posting_group)+"/"+string(open.Tax_prod_posting_group)] = true
		if !(open.Type == gtables.TaxEntry_Type.Purchase && open.Use_tax) {
			want = want.Add(open.Amount)
		}
	}
	options := map[string]string{"posting_date": "2026-10-31", "document_no": "TAXSET-1", "settlement_account": "2340", "show_tax_entries": "true"}

	// the test report
	result, err := reports.Execute(reports.CalcAndPostTaxSettlementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Options: options})
	mustOK(t, err)
	texts := printedTexts(result, 0)
	if !strings.Contains(texts, "Test Report (not posted)") || !strings.Contains(texts, amountText(want)) {
		t.Errorf("test report: total %s not printed: %s", amountText(want), texts)
	}
	if open.Count() != openCount {
		t.Fatal("the test report closed entries")
	}

	options["post"] = "true"
	_, err = reports.Execute(reports.CalcAndPostTaxSettlementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Options: options})
	mustOK(t, err)
	if n := open.Count(); n != 0 {
		t.Errorf("%d tax entries still open", n)
	}
	settlement := gtables.NewTaxEntry(src)
	settlement.SetRange("type", gtables.TaxEntry_Type.Settlement)
	if n := settlement.Count(); n != len(groups) {
		t.Errorf("%d settlement entries for %d groups", n, len(groups))
	}
	for found := settlement.FindSet(); found; found = settlement.Next() {
		if !settlement.Closed || settlement.Document_no != "TAXSET-1" || settlement.Tax_jurisdiction_code == "" || settlement.Tax_area_code != "" {
			t.Errorf("settlement entry %d: closed %v, document %s, jurisdiction %q, area %q", settlement.Entry_no, settlement.Closed, settlement.Document_no, settlement.Tax_jurisdiction_code, settlement.Tax_area_code)
		}
		closedBy := gtables.NewTaxEntry(src)
		closedBy.SetRange("closed_by_entry_no", settlement.Entry_no)
		if closedBy.Count() == 0 {
			t.Errorf("settlement entry %d closes no entries", settlement.Entry_no)
		}
	}

	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("document_no", types.Code("TAXSET-1"))
	glEntry.SetRange("gl_account_no", types.Code("2340"))
	mustOK(t, glEntry.CalcSums("amount"))
	if !want.Equal(types.NewDecimalFromInt(-150)) || !glEntry.Amount.Equal(want) {
		t.Errorf("settlement account %s, want %s (-150)", glEntry.Amount, want)
	}
	for _, acc := range []types.Code{"2330", "1440"} {
		gl := gtables.NewGLEntry(src)
		gl.SetRange("gl_account_no", acc)
		gl.SetFilter("document_no", "TX1|TX2|TX3|TAXSET-1")
		mustOK(t, gl.CalcSums("amount"))
		if !gl.Amount.IsZero() {
			t.Errorf("tax account %s: %s after the settlement", acc, gl.Amount)
		}
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L not balanced: %+v", r)
	}
}

// VAT settlement: a sale and a purchase with Normal VAT and a reverse charge purchase (groups
// VATDOM and VATEU): the settlement closes the entries per tax posting setup and type, empties
// the sales and purchase VAT accounts into the settlement account and the reverse charge account
// against the purchase VAT it balances (not into the settlement)
func TestCalcAndPostVATSettlement(t *testing.T) {
	src := listCompany(t)
	for _, code := range []types.Code{"VATDOM", "VATEU"} {
		group := gtables.NewTaxBusinessPostingGroup(src)
		group.Code = code
		mustOK(t, group.InsertOrError(false))
	}
	product := gtables.NewTaxProductPostingGroup(src)
	product.Code = "VAT25"
	mustOK(t, product.InsertOrError(false))
	for bus, calcType := range map[types.Code]gtables.TaxPostingSetupTax_calculation_type{
		"VATDOM": gtables.TaxPostingSetup_Tax_calculation_type.NormalVAT, "VATEU": gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT} {
		setup := gtables.NewTaxPostingSetup(src)
		setup.Tax_bus_posting_group = bus
		setup.Tax_prod_posting_group = "VAT25"
		setup.Tax_calculation_type = calcType
		setup.Vat_percent = types.NewDecimalFromInt(25)
		setup.Sales_vat_account = "2330"
		setup.Purchase_vat_account = "1440"
		setup.Reverse_chrg_vat_acc = "2350"
		mustOK(t, setup.InsertOrError(false))
	}
	post := genjnl.NewGenJnlPostLine(src)
	for i, l := range []struct {
		accNo  types.Code
		bus    types.Code
		amount string
	}{{"3110", "VATDOM", "-1250"}, {"4110", "VATDOM", "500"}, {"4120", "VATEU", "1000"}} {
		line := gtables.NewGenJournalLine(src)
		line.Init()
		mustOK(t, line.Validate("posting_date", types.NewDate(2026, 10, 15)))
		line.Document_no = types.Code("VX" + strconv.Itoa(i+1))
		mustOK(t, line.Validate("account_no", l.accNo))
		mustOK(t, line.Validate("tax_bus_posting_group", l.bus))
		mustOK(t, line.Validate("tax_prod_posting_group", types.Code("VAT25")))
		mustOK(t, line.Validate("bal_account_no", types.Code("1210")))
		amount, err := types.NewDecimalFromString(l.amount)
		mustOK(t, err)
		mustOK(t, line.Validate("amount", amount))
		mustOK(t, post.Run(line))
	}
	options := map[string]string{"posting_date": "2026-10-31", "document_no": "VATSET-1", "settlement_account": "2340", "post": "true"}
	_, err := reports.Execute(reports.CalcAndPostTaxSettlementID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF, Options: options,
		Filters: map[string][]ftables.FieldFilter{"Tax_Posting_Setup": {{Field: "tax_bus_posting_group", Expression: "VATDOM|VATEU"}}}})
	mustOK(t, err)

	open := gtables.NewTaxEntry(src)
	open.SetRange("closed", false)
	open.SetFilter("tax_bus_posting_group", "VATDOM|VATEU")
	if n := open.Count(); n != 0 {
		t.Errorf("%d VAT entries still open", n)
	}
	glAmount := func(acc types.Code, docs string) types.Decimal {
		gl := gtables.NewGLEntry(src)
		gl.SetRange("gl_account_no", acc)
		gl.SetFilter("document_no", docs)
		mustOK(t, gl.CalcSums("amount"))
		return gl.Amount
	}
	// sales VAT -250, purchase VAT 100 + reverse charge 250: the settlement account gets the
	// net VAT payable -150; the reverse charge's 250 is settled against its own account
	if got := glAmount("2340", "VATSET-1"); !got.Equal(types.NewDecimalFromInt(-150)) {
		t.Errorf("settlement account %s, want -150", got)
	}
	for _, acc := range []types.Code{"2330", "1440", "2350"} {
		if got := glAmount(acc, "VX1|VX2|VX3|VATSET-1"); !got.IsZero() {
			t.Errorf("VAT account %s: %s after the settlement", acc, got)
		}
	}
	settlement := gtables.NewTaxEntry(src)
	settlement.SetRange("type", gtables.TaxEntry_Type.Settlement)
	settlement.SetRange("document_no", types.Code("VATSET-1"))
	for found := settlement.FindSet(); found; found = settlement.Next() {
		if !settlement.Closed || settlement.Tax_jurisdiction_code != "" || !isVATCalc(settlement.Tax_calculation_type) {
			t.Errorf("settlement entry %d: closed %v jurisdiction %q calc %v", settlement.Entry_no, settlement.Closed, settlement.Tax_jurisdiction_code, settlement.Tax_calculation_type)
		}
	}
	if settlement.Count() != 3 { // VATDOM sale, VATDOM purchase, VATEU purchase
		t.Errorf("%d settlement entries, want 3", settlement.Count())
	}
	if r := glcheck.Check(src, nil); !r.Balanced() {
		t.Errorf("G/L not balanced: %+v", r)
	}
}

func isVATCalc(c gtables.TaxEntryTax_calculation_type) bool {
	return c == gtables.TaxEntry_Tax_calculation_type.NormalVAT || c == gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT ||
		c == gtables.TaxEntry_Tax_calculation_type.FullVAT
}
