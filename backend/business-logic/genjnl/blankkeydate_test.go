package genjnl

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salestax"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// checkBlankKeyDate: a tax detail with a blank Effective Date (in its primary key) is found,
// changed and deleted by its key, and the tax calculation uses it
func checkBlankKeyDate(t *testing.T, src ftables.Source) {
	t.Helper()
	key := map[string]interface{}{"tax_jurisdiction_code": "NO", "tax_group_code": "TAXABLE", "tax_type": 0, "effective_date": types.Date{}}
	taxDetail := gtables.NewTaxDetail(src)
	must(t, taxDetail.GetOrError(key))
	if !taxDetail.Effective_date.IsZero() || !taxDetail.Tax_below_maximum.Equal(dec("25")) {
		t.Fatalf("tax detail: effective %v, %s %%", taxDetail.Effective_date, taxDetail.Tax_below_maximum)
	}
	tax, err := salestax.New(src).CalculateTax("DOMESTIC", "TAXABLE", true, date(t, 15, 10, 2026), dec("1000"), types.Decimal{}, types.Decimal{})
	must(t, err)
	if !tax.Equal(dec("250")) {
		t.Errorf("tax %s, want 250", tax)
	}
	taxDetail.Tax_below_maximum = dec("20")
	must(t, taxDetail.ModifyOrError(false))
	again := gtables.NewTaxDetail(src)
	must(t, again.GetOrError(key))
	if !again.Tax_below_maximum.Equal(dec("20")) {
		t.Errorf("modified rate %s", again.Tax_below_maximum)
	}
	blank := gtables.NewTaxDetail(src)
	blank.SetRange("effective_date", types.Date{})
	dated := gtables.NewTaxDetail(src)
	dated.SetFilter("effective_date", "<>''")
	if blank.Count() != 3 || dated.Count() != 0 {
		t.Errorf("blank effective dates %d, others %d", blank.Count(), dated.Count())
	}
	must(t, again.DeleteOrError(false))
	if gtables.NewTaxDetail(src).Get(key) {
		t.Error("the deleted tax detail is still there")
	}
}

func TestBlankKeyDate(t *testing.T) {
	checkBlankKeyDate(t, demoCompany(t))
}

func TestBlankKeyDatePostgres(t *testing.T) {
	checkBlankKeyDate(t, demoCompanyPostgres(t))
}
