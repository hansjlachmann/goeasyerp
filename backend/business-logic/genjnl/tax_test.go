package genjnl

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// A sale on a taxable G/L account takes the tax out of the amount: the revenue without tax, the
// tax on the jurisdiction's sales tax account, a tax entry; the register covers it
func TestPostSalesTax(t *testing.T) {
	src := demoCompany(t)
	line := addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00001", accNo: "3110", amount: "-1250", balNo: "1210"})
	if line.Tax_calculation_type != gtables.GenJournalLine_Tax_calculation_type.SalesTax || !line.Tax_amount.Equal(dec("-250")) ||
		!line.Tax_percent.Equal(dec("25")) || !line.Tax_base_amount.Equal(dec("-1000")) {
		t.Fatalf("line tax: type %v amount %s %% %s base %s", line.Tax_calculation_type, line.Tax_amount, line.Tax_percent, line.Tax_base_amount)
	}
	sales0 := glAmount(src, "3110", "G00001")
	result, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	if !glAmount(src, "3110", "G00001").Sub(sales0).Equal(dec("-1000")) || !glAmount(src, "2330", "G00001").Equal(dec("-250")) ||
		!glAmount(src, "1210", "G00001").Equal(dec("1250")) {
		t.Errorf("G/L: 3110 %s, 2330 %s, 1210 %s", glAmount(src, "3110", "G00001"), glAmount(src, "2330", "G00001"), glAmount(src, "1210", "G00001"))
	}
	revenue := gtables.NewGLEntry(src)
	revenue.SetRange("gl_account_no", "3110")
	revenue.SetRange("document_no", "G00001")
	if !revenue.FindFirst() || !revenue.Tax_amount.Equal(dec("-250")) || revenue.Tax_group_code != "TAXABLE" || revenue.Tax_area_code != "DOMESTIC" {
		t.Errorf("revenue entry: tax %s group %q area %q", revenue.Tax_amount, revenue.Tax_group_code, revenue.Tax_area_code)
	}
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", "G00001")
	if taxEntry.Count() != 1 || !taxEntry.FindFirst() {
		t.Fatalf("%d tax entries", taxEntry.Count())
	}
	if !taxEntry.Amount.Equal(dec("-250")) || !taxEntry.Base.Equal(dec("-1000")) || taxEntry.Type != gtables.TaxEntry_Type.Sale ||
		taxEntry.Tax_jurisdiction_code != "NO" || taxEntry.Tax_group_used != "TAXABLE" || taxEntry.Transaction_no != revenue.Transaction_no {
		t.Errorf("tax entry %+v", taxEntry)
	}
	glReg := gtables.NewGLRegister(src)
	must(t, glReg.GetOrError(result.RegisterNo))
	if glReg.From_tax_entry_no != taxEntry.Entry_no || glReg.To_tax_entry_no != taxEntry.Entry_no {
		t.Errorf("register tax entries %d..%d, entry %d", glReg.From_tax_entry_no, glReg.To_tax_entry_no, taxEntry.Entry_no)
	}
}

// A purchase with use tax posts the full amount on the account; the tax goes on the purchase
// tax account and, as a liability, on the reverse charge account
func TestPostUseTax(t *testing.T) {
	src := demoCompany(t)
	line := newJnlLine(src, "GENERAL", "DEFAULT", 10000)
	must(t, line.Validate("posting_date", date(t, 15, 10, 2026)))
	line.Document_no = "G00001"
	must(t, line.Validate("account_no", types.Code("4110")))
	must(t, line.Validate("bal_account_no", types.Code("1210")))
	must(t, line.Validate("amount", dec("1000")))
	must(t, line.Validate("use_tax", true))
	if !line.Tax_amount.IsZero() || !line.Tax_base_amount.Equal(dec("1000")) {
		t.Fatalf("use tax line: tax %s base %s", line.Tax_amount, line.Tax_base_amount)
	}
	must(t, line.InsertOrError(true))
	purchTax0 := glAmount(src, "1440", "G00001")
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	if !glAmount(src, "4110", "G00001").Equal(dec("1000")) || !glAmount(src, "1440", "G00001").Sub(purchTax0).Equal(dec("250")) ||
		!glAmount(src, "2330", "G00001").Equal(dec("-250")) || !glAmount(src, "1210", "G00001").Equal(dec("-1000")) {
		t.Errorf("G/L: 4110 %s, 1440 %s, 2330 %s, 1210 %s", glAmount(src, "4110", "G00001"), glAmount(src, "1440", "G00001"),
			glAmount(src, "2330", "G00001"), glAmount(src, "1210", "G00001"))
	}
	taxEntry := gtables.NewTaxEntry(src)
	taxEntry.SetRange("document_no", "G00001")
	if !taxEntry.FindFirst() || !taxEntry.Amount.Equal(dec("250")) || !taxEntry.Base.Equal(dec("1000")) || !taxEntry.Use_tax ||
		taxEntry.Type != gtables.TaxEntry_Type.Purchase {
		t.Errorf("tax entry %+v", taxEntry)
	}
}

// A line without tax (export, not liable) writes a zero tax entry and no tax G/L entry
func TestPostTaxNotLiable(t *testing.T) {
	src := demoCompany(t)
	addLine(t, src, "GENERAL", "DEFAULT", lineSpec{lineNo: 10000, docNo: "G00001", accNo: "3120", amount: "-500", balNo: "1210"})
	_, err := postBatch(t, src, "GENERAL", "DEFAULT")
	must(t, err)
	if !glAmount(src, "3120", "G00001").Equal(dec("-500")) || !glAmount(src, "2330", "G00001").IsZero() {
		t.Errorf("G/L: 3120 %s, 2330 %s", glAmount(src, "3120", "G00001"), glAmount(src, "2330", "G00001"))
	}
}
