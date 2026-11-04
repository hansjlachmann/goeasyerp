package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The classic Item code: numbers from the Item Nos. series of the Inventory Setup, Search
// Description, the base unit (an item unit of measure with quantity 1, sales and purchase units
// follow), the price/profit calculation and unit cost by costing method, the default tax group
// of the product posting group, and the delete of the item's units, variants, translations,
// vendors and comments
func TestItemTriggers(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.Item{}, &gtables.ItemUnitOfMeasure{}, &gtables.ItemVariant{}, &gtables.ItemTranslation{}, &gtables.ItemVendor{},
		&gtables.UnitOfMeasure{}, &gtables.UnitOfMeasureTranslation{}, &gtables.InventorySetup{}, &gtables.CommentLine{}, &gtables.Vendor{},
		&gtables.GenProductPostingGroup{}, &gtables.TaxProductPostingGroup{}, &gtables.TaxPostingSetup{}, &gtables.TaxBusinessPostingGroup{},
		&gtables.NoSeries{}, &gtables.NoSeriesLine{}, &gtables.NoSeriesRelationship{}, &gtables.Language{})
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	series := gtables.NewNoSeries(src)
	series.Code, series.Default_nos = "ITEM", true
	line := gtables.NewNoSeriesLine(src)
	line.InitRecord()
	line.Series_code, line.Line_no, line.Starting_no, line.Increment_by_no = "ITEM", 10000, "70000", 10
	invtSetup := gtables.NewInventorySetup(src)
	invtSetup.InitRecord()
	invtSetup.Item_nos = "ITEM"
	pcs := gtables.NewUnitOfMeasure(src)
	pcs.Code = "PCS"
	box := gtables.NewUnitOfMeasure(src)
	box.Code = "BOX"
	genProd := gtables.NewGenProductPostingGroup(src)
	genProd.InitRecord()
	genProd.Code, genProd.Def_tax_prod_posting_group, genProd.Auto_insert_default = "RETAIL", "STANDARD", true
	taxProd := gtables.NewTaxProductPostingGroup(src)
	taxProd.Code = "STANDARD"
	for _, err := range []error{series.InsertOrError(false), line.InsertOrError(false), invtSetup.InsertOrError(false), pcs.InsertOrError(false),
		box.InsertOrError(false), genProd.InsertOrError(false), taxProd.InsertOrError(false)} {
		if err != nil {
			t.Fatal(err)
		}
	}

	item := gtables.NewItem(src)
	item.InitRecord()
	must(t, item.Validate("description", types.Text("Office chair")))
	must(t, item.InsertOrError(true))
	if item.No != "70000" || item.No_series != "ITEM" || item.Search_description != "OFFICE CHAIR" || item.Reserve != gtables.Item_Reserve.Optional ||
		!item.Allow_invoice_disc || !item.Rounding_precision.Equal(types.NewDecimal(1)) {
		t.Fatalf("new item: no %q series %q search %q reserve %v", item.No, item.No_series, item.Search_description, item.Reserve)
	}

	// The base unit must be one of the item's units, with quantity 1
	if err := item.Validate("base_unit_of_measure", types.Code("PCS")); err == nil {
		t.Error("a base unit that is not an item unit of measure was accepted")
	}
	itemUoM := gtables.NewItemUnitOfMeasure(src)
	itemUoM.InitRecord()
	itemUoM.Item_no, itemUoM.Code = item.No, "PCS"
	must(t, itemUoM.InsertOrError(true))
	boxUoM := gtables.NewItemUnitOfMeasure(src)
	boxUoM.InitRecord()
	boxUoM.Item_no, boxUoM.Code = item.No, "BOX"
	if err := boxUoM.Validate("qty_per_unit_of_measure", types.Decimal{}); errCode(t, err) != apperrors.ErrItemUoMQtyNotPositive {
		t.Errorf("qty 0: %v", err)
	}
	must(t, boxUoM.Validate("qty_per_unit_of_measure", types.NewDecimal(10)))
	must(t, boxUoM.InsertOrError(true))
	must(t, item.Validate("base_unit_of_measure", types.Code("PCS")))
	if item.Sales_unit_of_measure != "PCS" || item.Purch_unit_of_measure != "PCS" {
		t.Errorf("sales %q purch %q", item.Sales_unit_of_measure, item.Purch_unit_of_measure)
	}
	if err := boxUoM.Validate("qty_per_unit_of_measure", types.NewDecimal(12)); err != nil {
		t.Errorf("another unit's quantity: %v", err)
	}
	must(t, item.ModifyOrError(true))
	itemUoM.Qty_per_unit_of_measure = types.NewDecimal(2)
	if err := itemUoM.Validate("qty_per_unit_of_measure", types.NewDecimal(2)); err == nil {
		t.Error("the base unit's quantity changed")
	}

	// Price/profit: the profit from price and cost, the price from cost and profit
	must(t, item.Validate("average_cost", types.NewDecimal(600)))
	if !item.Unit_cost.Equal(types.NewDecimal(600)) {
		t.Errorf("unit cost %s", item.Unit_cost)
	}
	must(t, item.Validate("unit_price", types.NewDecimal(1000)))
	if !item.Profit_percent.Equal(types.NewDecimal(40)) {
		t.Errorf("profit %% %s, want 40", item.Profit_percent)
	}
	must(t, item.Validate("price_profit_calculation", gtables.Item_Price_profit_calculation.PriceCostProfit))
	must(t, item.Validate("profit_percent", types.NewDecimal(50)))
	if !item.Unit_price.Equal(types.NewDecimal(1200)) {
		t.Errorf("unit price %s, want 1200", item.Unit_price)
	}
	// Standard costing: the unit cost is the standard cost, no indirect cost
	must(t, item.Validate("indirect_cost_percent", types.NewDecimal(5)))
	must(t, item.Validate("standard_cost", types.NewDecimal(700)))
	must(t, item.Validate("costing_method", gtables.Item_Costing_method.Standard))
	if !item.Unit_cost.Equal(types.NewDecimal(700)) || !item.Indirect_cost_percent.IsZero() || !item.Unit_price.Equal(types.NewDecimal(1400)) {
		t.Errorf("standard: unit cost %s indirect %s price %s", item.Unit_cost, item.Indirect_cost_percent, item.Unit_price)
	}
	// The product posting group's default tax group
	must(t, item.Validate("gen_prod_posting_group", types.Code("RETAIL")))
	if item.Tax_prod_posting_group != "STANDARD" {
		t.Errorf("tax prod. group %q", item.Tax_prod_posting_group)
	}
	// Prices including sales tax cannot be calculated
	setup := gtables.NewTaxPostingSetup(src)
	setup.Tax_prod_posting_group, setup.Tax_calculation_type = "STANDARD", gtables.TaxPostingSetup_Tax_calculation_type.SalesTax
	must(t, setup.InsertOrError(false))
	if err := item.Validate("price_includes_tax", true); errCode(t, err) != apperrors.ErrItemPriceInclTaxSalesTax {
		t.Errorf("price incl. sales tax: %v", err)
	}
	item.Price_includes_tax = false
	must(t, item.ModifyOrError(true))

	// Delete: the item's units, variants, translations, vendors and comments go with it
	variant := gtables.NewItemVariant(src)
	variant.Item_no, variant.Code = item.No, "BLACK"
	translation := gtables.NewItemTranslation(src)
	translation.Item_no, translation.Language_code, translation.Description = item.No, "NB-NO", "Kontorstol"
	itemVend := gtables.NewItemVendor(src)
	itemVend.Item_no, itemVend.Vendor_no = item.No, "V1"
	comment := gtables.NewCommentLine(src)
	comment.Table_name, comment.No, comment.Line_no, comment.Comment = gtables.CommentLine_Table_name.Item, item.No, 10000, "Note"
	for _, err := range []error{variant.InsertOrError(false), translation.InsertOrError(false), itemVend.InsertOrError(false), comment.InsertOrError(false)} {
		must(t, err)
	}
	must(t, item.DeleteOrError(true))
	for name, n := range map[string]int{"units": gtables.NewItemUnitOfMeasure(src).Count(), "variants": gtables.NewItemVariant(src).Count(),
		"translations": gtables.NewItemTranslation(src).Count(), "vendors": gtables.NewItemVendor(src).Count(), "comments": gtables.NewCommentLine(src).Count()} {
		if n != 0 {
			t.Errorf("%d %s left after the delete", n, name)
		}
	}
}

// An Item Vendor takes the vendor's currency
func TestItemVendorCurrency(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.ItemVendor{}, &gtables.Vendor{}, &gtables.Item{}, &gtables.Currency{})
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	vend := gtables.NewVendor(src)
	vend.InitRecord()
	vend.No, vend.Currency_code = "V1", "EUR"
	must(t, vend.InsertOrError(false))
	itemVend := gtables.NewItemVendor(src)
	itemVend.InitRecord()
	itemVend.Item_no = "1000"
	must(t, itemVend.Validate("vendor_no", types.Code("V1")))
	if itemVend.Currency_code != "EUR" {
		t.Errorf("currency %q", itemVend.Currency_code)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
