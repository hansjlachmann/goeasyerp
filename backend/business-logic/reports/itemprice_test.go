package reports_test

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Items 1000 (price 200, PCS; a BOX of 10) and 2000 (price 1234, Average costing, average cost
// 50); price group RETAIL (prices include tax); an item price of 1000 for RETAIL from
// 2026-01-01 at 180; rounding method R rounds to whole amounts, from 1000 up to tens
func itemPriceCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "c1", database.DBTypeSQLite))
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	for _, code := range []types.Code{"PCS", "BOX"} {
		unitOfMeasure := gtables.NewUnitOfMeasure(src)
		unitOfMeasure.Code = code
		mustOK(t, unitOfMeasure.InsertOrError(false))
	}
	for _, i := range []struct {
		no, price string
		costing   gtables.ItemCosting_method
	}{{"1000", "200", gtables.Item_Costing_method.FIFO}, {"2000", "1234", gtables.Item_Costing_method.Average}} {
		item := gtables.NewItem(src)
		item.InitRecord()
		item.No, item.Unit_price, item.Base_unit_of_measure, item.Costing_method = types.Code(i.no), types.MustDecimal(i.price), "PCS", i.costing
		item.Average_cost = types.MustDecimal("50")
		mustOK(t, item.InsertOrError(false))
		itemUnitOfMeasure := gtables.NewItemUnitOfMeasure(src)
		itemUnitOfMeasure.Item_no, itemUnitOfMeasure.Code, itemUnitOfMeasure.Qty_per_unit_of_measure = types.Code(i.no), "PCS", types.NewDecimalFromInt(1)
		mustOK(t, itemUnitOfMeasure.InsertOrError(false))
	}
	itemUnitOfMeasure := gtables.NewItemUnitOfMeasure(src)
	itemUnitOfMeasure.Item_no, itemUnitOfMeasure.Code, itemUnitOfMeasure.Qty_per_unit_of_measure = "1000", "BOX", types.NewDecimalFromInt(10)
	mustOK(t, itemUnitOfMeasure.InsertOrError(false))
	priceGroup := gtables.NewPriceGroup(src)
	priceGroup.Code, priceGroup.Price_includes_tax, priceGroup.Allow_invoice_disc = "RETAIL", true, false
	mustOK(t, priceGroup.InsertOrError(false))
	itemPrice := gtables.NewItemPrice(src)
	itemPrice.Item_no, itemPrice.Price_group_code, itemPrice.Starting_date, itemPrice.Unit_price = "1000", "RETAIL", types.NewDate(2026, 1, 1), types.MustDecimal("180")
	itemPrice.Allow_quantity_disc, itemPrice.Allow_cust_item_disc, itemPrice.Allow_invoice_disc = true, true, true
	mustOK(t, itemPrice.InsertOrError(false))
	for _, m := range []struct {
		min, precision string
		typ            gtables.RoundingMethodType
	}{{"0", "1", gtables.RoundingMethod_Type.Nearest}, {"1000", "10", gtables.RoundingMethod_Type.Up}} {
		roundingMethod := gtables.NewRoundingMethod(src)
		roundingMethod.Code, roundingMethod.Minimum_amount, roundingMethod.Precision, roundingMethod.Type = "R", types.MustDecimal(m.min), types.MustDecimal(m.precision), m.typ
		mustOK(t, roundingMethod.InsertOrError(false))
	}
	return src
}

func itemPriceChanges(src ftables.Source) map[string]string {
	got := map[string]string{}
	itemPriceChange := gtables.NewItemPriceChange(src)
	for found := itemPriceChange.FindSet(); found; found = itemPriceChange.Next() {
		key := strings.Join([]string{itemPriceChange.Item_no.String(), itemPriceChange.Price_group_code.String(), itemPriceChange.Unit_of_measure_code.String(), itemPriceChange.Starting_date.String()}, "/")
		got[key] = itemPriceChange.Current_unit_price.String() + "→" + itemPriceChange.New_unit_price.String()
	}
	return got
}

// Suggest Item Price Chg. (Item): the items' unit prices for a price group and date — where a
// price starts on that date, or all with Create New Prices; a unit of measure multiplies the
// price; Implement makes them item prices
func TestSuggestItemPriceChgItem(t *testing.T) {
	src := itemPriceCompany(t)
	runReport(t, src, reports.SuggestItemPriceChgItemID, map[string]string{"price_group_code": "RETAIL", "starting_date": "2026-01-01", "unit_price_factor": "1"})
	// only 1000 has a RETAIL price starting on 2026-01-01
	if got := itemPriceChanges(src); len(got) != 1 || got["1000/RETAIL//2026-01-01"] != "180→200" {
		t.Errorf("suggestions where a price exists = %v", got)
	}
	runReport(t, src, reports.SuggestItemPriceChgItemID, map[string]string{"price_group_code": "RETAIL", "unit_of_measure_code": "BOX", "starting_date": "2026-06-01",
		"unit_price_factor": "2", "price_lower_limit": "1000", "rounding_method": "R", "create_new_prices": "true"})
	// 1000 BOX: 200 × 10 = 2000 × 2 = 4000; 2000 has no BOX: left out
	got := itemPriceChanges(src)
	if got["1000/RETAIL/BOX/2026-06-01"] != "0→4000" || len(got) != 2 {
		t.Errorf("suggestions with BOX = %v", got)
	}
	itemPriceChange := gtables.NewItemPriceChange(src)
	mustOK(t, itemPriceChange.GetOrError(map[string]interface{}{"item_no": "1000", "price_group_code": "RETAIL", "unit_of_measure_code": "BOX", "currency_code": "", "starting_date": types.NewDate(2026, 6, 1)}))
	if !itemPriceChange.Price_includes_tax || itemPriceChange.Allow_invoice_disc {
		t.Errorf("a new price takes the price group's settings: incl. tax %v, invoice disc. %v", itemPriceChange.Price_includes_tax, itemPriceChange.Allow_invoice_disc)
	}

	runReport(t, src, reports.ImplementItemPriceChangeID, nil)
	itemPrice := gtables.NewItemPrice(src)
	mustOK(t, itemPrice.GetOrError(map[string]interface{}{"item_no": "1000", "price_group_code": "RETAIL", "unit_of_measure_code": "BOX", "currency_code": "", "starting_date": types.NewDate(2026, 6, 1)}))
	if !itemPrice.Unit_price.Equal(types.MustDecimal("4000")) || !itemPrice.Price_includes_tax {
		t.Errorf("item price after Implement: %s, incl. tax %v", itemPrice.Unit_price, itemPrice.Price_includes_tax)
	}
	mustOK(t, itemPrice.GetOrError(map[string]interface{}{"item_no": "1000", "price_group_code": "RETAIL", "unit_of_measure_code": "", "currency_code": "", "starting_date": types.NewDate(2026, 1, 1)}))
	if !itemPrice.Unit_price.Equal(types.MustDecimal("200")) {
		t.Errorf("the existing price after Implement: %s, want 200", itemPrice.Unit_price)
	}
}

// Suggest Item Price Chg.(Price): the existing prices copied to another price group and date,
// multiplied; ALL keeps each price's own
func TestSuggestItemPriceChgPrice(t *testing.T) {
	src := itemPriceCompany(t)
	runReport(t, src, reports.SuggestItemPriceChgPriceID, map[string]string{"price_group_code": "ALL", "unit_of_measure_code": "ALL", "currency_code": "ALL",
		"starting_date": "2026-07-01", "unit_price_factor": "1.5", "create_new_prices": "true"})
	if got := itemPriceChanges(src); len(got) != 1 || got["1000/RETAIL//2026-07-01"] != "180→270" {
		t.Errorf("suggestions from the prices = %v", got)
	}
	gtables.NewItemPriceChange(src).DeleteAll()
	// to BOX: 180 per PCS × 10
	runReport(t, src, reports.SuggestItemPriceChgPriceID, map[string]string{"price_group_code": "ALL", "unit_of_measure_code": "BOX", "currency_code": "ALL",
		"unit_price_factor": "1", "create_new_prices": "true"})
	if got := itemPriceChanges(src); got["1000/RETAIL/BOX/2026-01-01"] != "0→1800" {
		t.Errorf("suggestions to BOX = %v", got)
	}
}

// Adjust Item Costs/Prices: Unit Price × 1.1 rounded by the method (220; 1357.4 → 1360); the
// average cost of an Average item with ledger entries cannot be changed
func TestAdjustItemCostsPrices(t *testing.T) {
	src := itemPriceCompany(t)
	runReport(t, src, reports.AdjustItemCostsPricesID, map[string]string{"adjust_field": "1", "adjustment_factor": "1.1", "rounding_method": "R"})
	for no, want := range map[string]string{"1000": "220", "2000": "1360"} {
		item := gtables.NewItem(src)
		mustOK(t, item.GetOrError(types.Code(no)))
		if !item.Unit_price.Equal(types.MustDecimal(want)) {
			t.Errorf("%s unit price %s, want %s", no, item.Unit_price, want)
		}
	}
	itemLedgEntry := gtables.NewItemLedgerEntry(src)
	itemLedgEntry.Entry_no, itemLedgEntry.Item_no = 1, "2000"
	mustOK(t, itemLedgEntry.InsertOrError(false))
	_, err := reports.Execute(reports.AdjustItemCostsPricesID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Options: map[string]string{"adjust_field": "4", "adjustment_factor": "2"}})
	if err == nil || !strings.Contains(err.Error(), "2000") {
		t.Errorf("average cost of an Average item with entries: %v, want an error naming 2000", err)
	}
}
