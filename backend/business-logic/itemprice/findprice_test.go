package itemprice_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemprice"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
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

// Item 1000 (unit price 100) with prices: all customers from 1 Jan (90), from 1 July (95),
// price group WHOLESALE (80), WHOLESALE in EUR (9), all customers per BOX (850)
func priceCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "c1", database.DBTypeSQLite))
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	item := gtables.NewItem(src)
	item.InitRecord()
	item.No, item.Unit_price, item.Allow_invoice_disc = "1000", dec("100"), false
	must(t, item.InsertOrError(false))
	priceGr := gtables.NewPriceGroup(src)
	priceGr.InitRecord()
	priceGr.Code, priceGr.Allow_invoice_disc, priceGr.Price_includes_tax = "WHOLESALE", false, true
	must(t, priceGr.InsertOrError(false))
	for _, p := range []struct {
		group, currency, uom string
		start                types.Date
		price                string
	}{
		{"", "", "", types.NewDate(2026, 1, 1), "90"},
		{"", "", "", types.NewDate(2026, 7, 1), "95"},
		{"WHOLESALE", "", "", types.Date{}, "80"},
		{"WHOLESALE", "EUR", "", types.Date{}, "9"},
		{"", "", "BOX", types.Date{}, "850"},
	} {
		itemPrice := gtables.NewItemPrice(src)
		itemPrice.InitRecord()
		itemPrice.Item_no, itemPrice.Currency_code, itemPrice.Unit_of_measure_code = "1000", types.Code(p.currency), types.Code(p.uom)
		itemPrice.Starting_date, itemPrice.Unit_price = p.start, dec(p.price)
		must(t, itemPrice.Validate("price_group_code", types.Code(p.group)))
		must(t, itemPrice.InsertOrError(true))
	}
	return src
}

func TestFindPrice(t *testing.T) {
	src := priceCompany(t)
	cases := []struct {
		name, group, currency, uom string
		date                       types.Date
		want                       string
		inclTax                    bool
	}{
		{"before any price: the item's unit price", "", "", "", types.NewDate(2025, 12, 31), "100", false},
		{"from 1 January", "", "", "", types.NewDate(2026, 3, 1), "90", false},
		{"the latest starting date", "", "", "", types.NewDate(2026, 10, 1), "95", false},
		{"the price group's price (with its tax setting)", "WHOLESALE", "", "", types.NewDate(2026, 10, 1), "80", true},
		{"the price group's price in the currency", "WHOLESALE", "EUR", "", types.NewDate(2026, 10, 1), "9", true},
		{"another currency: the group's price without currency", "WHOLESALE", "GBP", "", types.NewDate(2026, 10, 1), "80", true},
		{"per unit of measure", "", "", "BOX", types.NewDate(2026, 10, 1), "850", false},
	}
	for _, c := range cases {
		rec := gtables.NewItemPrice(src)
		rec.Item_no, rec.Price_group_code, rec.Currency_code = "1000", types.Code(c.group), types.Code(c.currency)
		rec.Unit_of_measure_code, rec.Starting_date = types.Code(c.uom), c.date
		must(t, itemprice.FindPrice(rec))
		if !rec.Unit_price.Equal(dec(c.want)) || rec.Price_includes_tax != c.inclTax {
			t.Errorf("%s: price %s incl. tax %v, want %s %v", c.name, rec.Unit_price, rec.Price_includes_tax, c.want, c.inclTax)
		}
	}
	// no item price: the item's invoice discount setting
	rec := gtables.NewItemPrice(src)
	rec.Item_no, rec.Starting_date = "1000", types.NewDate(2025, 1, 1)
	must(t, itemprice.FindPrice(rec))
	if rec.Allow_invoice_disc || !rec.Allow_quantity_disc {
		t.Errorf("item defaults: invoice disc %v quantity disc %v", rec.Allow_invoice_disc, rec.Allow_quantity_disc)
	}
}

// Quantity discounts are keyed by a decimal (Minimum Quantity): get, modify and delete by it
func TestSalesQtyDiscKey(t *testing.T) {
	src := priceCompany(t)
	for _, q := range []string{"0", "10", "12.5"} {
		disc := gtables.NewItemSalesQtyDisc(src)
		disc.Code, disc.Minimum_quantity, disc.Discount_percent = "1000", dec(q), dec("5")
		must(t, disc.InsertOrError(true))
	}
	disc := gtables.NewItemSalesQtyDisc(src)
	must(t, disc.GetOrError(map[string]interface{}{"code": "1000", "minimum_quantity": "12.5"}))
	disc.Discount_percent = dec("7.5")
	must(t, disc.ModifyOrError(true))
	again := gtables.NewItemSalesQtyDisc(src)
	must(t, again.GetOrError(map[string]interface{}{"code": "1000", "minimum_quantity": dec("12.5")}))
	if !again.Discount_percent.Equal(dec("7.5")) {
		t.Errorf("discount %s", again.Discount_percent)
	}
	must(t, again.DeleteOrError(true))
	if gtables.NewItemSalesQtyDisc(src).Count() != 2 {
		t.Errorf("%d discounts left", gtables.NewItemSalesQtyDisc(src).Count())
	}
}
