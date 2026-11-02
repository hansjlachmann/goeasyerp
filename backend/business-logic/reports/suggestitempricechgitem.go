package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 791 Suggest Item Price Chg. (Item): suggests new item prices from the items' unit
// prices — for a price group, unit of measure, currency and starting date, multiplied by a
// factor above a lower limit and rounded by a rounding method — as Item Price Change lines,
// next to the price in effect on the starting date (only where a price starts on that date,
// unless Create New Prices).
const SuggestItemPriceChgItemID = 791

func init() {
	Register(SuggestItemPriceChgItemID, "suggest-item-price-chg-item", func() Report { return &suggestItemPriceChgItem{} })
}

type suggestItemPriceChgItem struct {
	createNewPrices bool
	unitPriceFactor types.Decimal
	priceLowerLimit types.Decimal
	toCurrency      *gtables.Currency
	roundingMethod  *gtables.RoundingMethod
	roundingCode    types.Code
	itemPriceChange *gtables.ItemPriceChange
	window          *system.Window
}

func (r *suggestItemPriceChgItem) Definition() *Definition {
	return &Definition{
		OptionsCaption: "COPY_TO",
		Options: []Option{
			{Name: "price_group_code", Type: "text", Relation: &OptionRelation{Table: "Price_Group", Columns: []string{"code", "description"}}},
			{Name: "unit_of_measure_code", Type: "text", Relation: &OptionRelation{Table: "Unit_of_Measure", Columns: []string{"code", "description"}}},
			{Name: "currency_code", Type: "text", Relation: &OptionRelation{Table: "Currency", Columns: []string{"code", "description"}}},
			{Name: "starting_date", Type: "date"},
			{Name: "price_lower_limit", Type: "number"},
			{Name: "unit_price_factor", Type: "number", Default: "1"},
			{Name: "rounding_method", Type: "text", Relation: &OptionRelation{Table: "Rounding_Method", Columns: []string{"code"}}},
			{Name: "create_new_prices", Type: "bool", Default: "false"},
		},
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Item", Table: "Item", Key: []string{"no"},
			RequestFilterFields: []string{"no", "vendor_no", "inventory_posting_group"},
			OnPreDataItem:       Trigger(r.itemOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.itemOnAfterGetRecord),
		}},
	}
}

func (r *suggestItemPriceChgItem) onPreReport(run *Run) error {
	r.createNewPrices = run.OptionBool("create_new_prices")
	r.unitPriceFactor = run.OptionDecimal("unit_price_factor")
	r.priceLowerLimit = run.OptionDecimal("price_lower_limit")
	r.roundingCode = types.Code(system.UpperCase(run.Option("rounding_method")))
	r.roundingMethod = gtables.NewRoundingMethod(run)

	r.toCurrency = gtables.NewCurrency(run)
	toCurrencyCode := types.Code(system.UpperCase(run.Option("currency_code")))
	if toCurrencyCode == "" {
		tables.InitRoundingPrecision(r.toCurrency)
	} else {
		err := r.toCurrency.GetOrError(toCurrencyCode)
		if err != nil {
			return err
		}
		err = r.toCurrency.TestField("unit_amount_rounding_precision")
		if err != nil {
			return err
		}
	}
	toPriceGr := gtables.NewPriceGroup(run)
	toPriceGrCode := types.Code(system.UpperCase(run.Option("price_group_code")))
	if toPriceGrCode != "" {
		err := toPriceGr.GetOrError(toPriceGrCode)
		if err != nil {
			return err
		}
	} else {
		if !toPriceGr.Get(toPriceGrCode) {
			toPriceGr.Init()
		}
	}
	toUnitOfMeasureCode := types.Code(system.UpperCase(run.Option("unit_of_measure_code")))
	if toUnitOfMeasureCode != "" {
		toUnitOfMeasure := gtables.NewUnitOfMeasure(run)
		err := toUnitOfMeasure.GetOrError(toUnitOfMeasureCode)
		if err != nil {
			return err
		}
	}
	r.itemPriceChange = gtables.NewItemPriceChange(run)
	r.itemPriceChange.Currency_code = r.toCurrency.Code
	r.itemPriceChange.Price_group_code = toPriceGrCode
	r.itemPriceChange.Unit_of_measure_code = toUnitOfMeasureCode
	r.itemPriceChange.Starting_date = run.OptionDate("starting_date")
	r.itemPriceChange.Price_includes_tax = toPriceGr.Price_includes_tax
	r.itemPriceChange.Allow_quantity_disc = toPriceGr.Allow_quantity_disc
	r.itemPriceChange.Allow_cust_item_disc = toPriceGr.Allow_cust_item_disc
	r.itemPriceChange.Allow_invoice_disc = toPriceGr.Allow_invoice_disc
	return nil
}

func (r *suggestItemPriceChgItem) itemOnPreDataItem(item *gtables.Item, run *Run) error {
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *suggestItemPriceChgItem) itemOnAfterGetRecord(item *gtables.Item, run *Run) error {
	r.window.Update(1, item.No)
	itemPriceChange := r.itemPriceChange
	itemPriceChange.Item_no = item.No
	if itemPriceChange.Unit_of_measure_code != item.Base_unit_of_measure && itemPriceChange.Unit_of_measure_code != "" {
		itemUnitOfMeasure := gtables.NewItemUnitOfMeasure(run)
		if !itemUnitOfMeasure.Get(map[string]interface{}{"item_no": itemPriceChange.Item_no, "code": itemPriceChange.Unit_of_measure_code}) {
			return Skip()
		}
	}
	currExchRate := tables.ExchangeRates(run)
	exchangeRate, err := currExchRate.ExchangeRate(system.WorkDate(), r.toCurrency.Code)
	if err != nil {
		return err
	}
	newUnitPrice, err := currExchRate.ExchangeAmtLCYToFCY(system.WorkDate(), r.toCurrency.Code, item.Unit_price, exchangeRate)
	if err != nil {
		return err
	}
	qtyPerUnitOfMeasure, err := tables.GetQtyPerUnitOfMeasure(item, itemPriceChange.Unit_of_measure_code)
	if err != nil {
		return err
	}
	itemPriceChange.New_unit_price = system.Round(newUnitPrice.Mul(qtyPerUnitOfMeasure), r.toCurrency.Unit_amount_rounding_precision)
	if itemPriceChange.New_unit_price.GreaterThan(r.priceLowerLimit) {
		itemPriceChange.New_unit_price = itemPriceChange.New_unit_price.Mul(r.unitPriceFactor)
	}
	itemPriceChange.New_unit_price = tables.ApplyRoundingMethod(r.roundingMethod, r.roundingCode, itemPriceChange.New_unit_price)

	found, priceAlreadyExists := setCurrentItemPrice(run, itemPriceChange)
	if !found {
		itemPriceChange.Tax_bus_posting_gr_price = item.Tax_bus_posting_gr_price
	}
	return insertItemPriceChange(run, itemPriceChange, priceAlreadyExists || r.createNewPrices)
}

// setCurrentItemPrice gives a suggested price change the price in effect on its starting date
// (the last Item Price of its item, price group, currency and unit of measure starting on or
// before it) and that price's settings: whether there is one, and whether it starts on the same
// date (PriceAlreadyExists). Without such a price the current price is 0.
func setCurrentItemPrice(run *Run, itemPriceChange *gtables.ItemPriceChange) (bool, bool) {
	itemPrice := gtables.NewItemPrice(run)
	itemPrice.SetRange("item_no", itemPriceChange.Item_no)
	itemPrice.SetRange("price_group_code", itemPriceChange.Price_group_code)
	itemPrice.SetRange("currency_code", itemPriceChange.Currency_code)
	itemPrice.SetRange("unit_of_measure_code", itemPriceChange.Unit_of_measure_code)
	itemPrice.SetRange("starting_date", types.Date{}, itemPriceChange.Starting_date)
	if itemPrice.FindLast() {
		itemPriceChange.Current_unit_price = itemPrice.Unit_price
		itemPriceChange.Price_includes_tax = itemPrice.Price_includes_tax
		itemPriceChange.Allow_quantity_disc = itemPrice.Allow_quantity_disc
		itemPriceChange.Allow_cust_item_disc = itemPrice.Allow_cust_item_disc
		itemPriceChange.Allow_invoice_disc = itemPrice.Allow_invoice_disc
		itemPriceChange.Tax_bus_posting_gr_price = itemPrice.Tax_bus_posting_gr_price
		return true, itemPrice.Starting_date == itemPriceChange.Starting_date
	}
	itemPriceChange.Current_unit_price = types.Decimal{}
	return false, false
}

// insertItemPriceChange inserts the suggested price change, or modifies the one of its key
// (the end of the suggest reports' OnAfterGetRecord: ItemPriceChange2 := ItemPriceChange; IF
// FIND('=') THEN MODIFY ELSE INSERT) — when save is set (a price exists, or new prices are
// created)
func insertItemPriceChange(run *Run, itemPriceChange *gtables.ItemPriceChange, save bool) error {
	if !save {
		return nil
	}
	itemPriceChange2 := gtables.NewItemPriceChange(run)
	key := map[string]interface{}{"item_no": itemPriceChange.Item_no, "price_group_code": itemPriceChange.Price_group_code,
		"unit_of_measure_code": itemPriceChange.Unit_of_measure_code, "currency_code": itemPriceChange.Currency_code,
		"starting_date": itemPriceChange.Starting_date}
	exists := itemPriceChange2.Get(key)
	itemPriceChange2.TransferFields(itemPriceChange)
	if exists {
		return itemPriceChange2.ModifyOrError(false)
	}
	return itemPriceChange2.InsertOrError(false)
}
