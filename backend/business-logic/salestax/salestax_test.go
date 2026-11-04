package salestax_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salestax"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

func date(t *testing.T, day, month, year int) types.Date {
	t.Helper()
	d, err := system.DMY2Date(day, month, year)
	must(t, err)
	return d
}

// taxCompany: tax area AREA with the state STATE (10 %, from 1 July 8 % before) and the city
// CITY (5 % tax on tax, up to 500 then 2 %), calculated after the state
func taxCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "tax", database.DBTypeSQLite))
	src := ftables.DBSource(db, "tax", database.DBTypeSQLite)
	for _, j := range []string{"STATE", "CITY"} {
		jurisdiction := gtables.NewTaxJurisdiction(src)
		jurisdiction.Code = types.Code(j)
		must(t, jurisdiction.InsertOrError(false))
	}
	for _, l := range []struct {
		jurisdiction string
		order        int
	}{{"STATE", 2}, {"CITY", 1}} {
		line := gtables.NewTaxAreaLine(src)
		line.Tax_area = "AREA"
		line.Tax_jurisdiction_code = types.Code(l.jurisdiction)
		line.Calculation_order = l.order
		must(t, line.InsertOrError(false))
	}
	for _, d := range []struct {
		jurisdiction, group string
		effective           types.Date
		below, above, max   string
		taxOnTax            bool
	}{
		{"STATE", "TAXABLE", types.Date{}, "8", "0", "0", false},
		{"STATE", "TAXABLE", date(t, 1, 7, 2026), "10", "0", "0", false},
		{"CITY", "", types.Date{}, "5", "2", "500", true},
	} {
		taxDetail := gtables.NewTaxDetail(src)
		taxDetail.Tax_jurisdiction_code = types.Code(d.jurisdiction)
		taxDetail.Tax_group_code = types.Code(d.group)
		taxDetail.Effective_date = d.effective
		taxDetail.Tax_below_maximum = dec(d.below)
		taxDetail.Tax_above_maximum = dec(d.above)
		taxDetail.Maximum_amount_qty = dec(d.max)
		taxDetail.Calculate_tax_on_tax = d.taxOnTax
		must(t, taxDetail.InsertOrError(false))
	}
	return src
}

// The state's rate in effect on the date, the city's tax on the state's tax up to its maximum
func TestCalculateTax(t *testing.T) {
	src := taxCompany(t)
	cases := []struct {
		name   string
		date   types.Date
		amount string
		want   string
	}{
		{"new state rate, city below its maximum", date(t, 15, 10, 2026), "400", "62"}, // 40 + 5 % of 440
		{"old state rate", date(t, 15, 3, 2026), "400", "53.6"},                        // 32 + 5 % of 432
		{"city above its maximum", date(t, 15, 10, 2026), "1000", "137"},               // 100 + 5 % of 500 + 2 % of 600
		{"not liable elsewhere", date(t, 15, 10, 2026), "0", "0"},
	}
	for _, c := range cases {
		got, err := salestax.New(src).CalculateTax("AREA", "TAXABLE", true, c.date, dec(c.amount), types.Decimal{}, types.Decimal{})
		must(t, err)
		if !got.Equal(dec(c.want)) {
			t.Errorf("%s: tax %s, want %s", c.name, got, c.want)
		}
	}
	got, err := salestax.New(src).CalculateTax("AREA", "TAXABLE", false, date(t, 15, 10, 2026), dec("400"), types.Decimal{}, types.Decimal{})
	must(t, err)
	if !got.IsZero() {
		t.Errorf("not tax liable: tax %s", got)
	}
	// the maximum is used up within one calculation (one document)
	calc := salestax.New(src)
	_, err = calc.CalculateTax("AREA", "TAXABLE", true, date(t, 15, 10, 2026), dec("400"), types.Decimal{}, types.Decimal{})
	must(t, err)
	got, err = calc.CalculateTax("AREA", "TAXABLE", true, date(t, 15, 10, 2026), dec("400"), types.Decimal{}, types.Decimal{})
	must(t, err)
	// 40 + 5 % of the 60 left of the city's 500 + 2 % of 380
	if !got.Equal(dec("50.6")) {
		t.Errorf("second amount of the document: tax %s, want 50.6", got)
	}
	calc.ClearMaximums()
	got, err = calc.CalculateTax("AREA", "TAXABLE", true, date(t, 15, 10, 2026), dec("400"), types.Decimal{}, types.Decimal{})
	must(t, err)
	if !got.Equal(dec("62")) {
		t.Errorf("after ClearMaximums: tax %s, want 62", got)
	}
}

// ReverseCalculateTax takes the tax out of an amount that includes it, also above the maximum
func TestReverseCalculateTax(t *testing.T) {
	src := taxCompany(t)
	for _, c := range []struct{ total, want string }{{"462", "400"}, {"1137", "1000"}, {"-462", "-400"}} {
		got, err := salestax.New(src).ReverseCalculateTax("AREA", "TAXABLE", true, date(t, 15, 10, 2026), dec(c.total), types.Decimal{}, types.Decimal{})
		must(t, err)
		if !got.Equal(dec(c.want)) {
			t.Errorf("total %s: amount %s, want %s", c.total, got, c.want)
		}
	}
	got, err := salestax.New(src).ReverseCalculateTax("", "TAXABLE", true, date(t, 15, 10, 2026), dec("462"), types.Decimal{}, types.Decimal{})
	must(t, err)
	if !got.Equal(dec("462")) {
		t.Errorf("no tax area: %s", got)
	}
}

// InitSalesTaxLines splits the tax per jurisdiction; a total that is not the line's tax is refused
func TestSalesTaxLines(t *testing.T) {
	src := taxCompany(t)
	calc := salestax.New(src)
	must(t, calc.InitSalesTaxLines("AREA", "TAXABLE", true, dec("400"), types.Decimal{}, date(t, 15, 10, 2026), dec("62")))
	got := map[string][2]string{}
	for {
		taxDetail, taxAmount, taxBase, ok := calc.GetSalesTaxLine()
		if !ok {
			break
		}
		got[taxDetail.Tax_jurisdiction_code.String()] = [2]string{taxAmount.String(), taxBase.String()}
	}
	if len(got) != 2 || !dec(got["STATE"][0]).Equal(dec("40")) || !dec(got["STATE"][1]).Equal(dec("400")) ||
		!dec(got["CITY"][0]).Equal(dec("22")) || !dec(got["CITY"][1]).Equal(dec("440")) {
		t.Errorf("tax lines %v", got)
	}
	err := salestax.New(src).InitSalesTaxLines("AREA", "TAXABLE", true, dec("400"), types.Decimal{}, date(t, 15, 10, 2026), dec("61"))
	if err == nil {
		t.Error("a wrong tax total was accepted")
	}
}

// Tax on tax needs the jurisdictions in distinct calculation orders
func TestCalculationOrder(t *testing.T) {
	src := taxCompany(t)
	line := gtables.NewTaxAreaLine(src)
	must(t, line.GetOrError(map[string]interface{}{"tax_area": "AREA", "tax_jurisdiction_code": "CITY"}))
	line.Calculation_order = 2
	must(t, line.ModifyOrError(false))
	_, err := salestax.New(src).CalculateTax("AREA", "TAXABLE", true, date(t, 15, 10, 2026), dec("400"), types.Decimal{}, types.Decimal{})
	if err == nil {
		t.Error("equal calculation orders with tax on tax were accepted")
	}
}
