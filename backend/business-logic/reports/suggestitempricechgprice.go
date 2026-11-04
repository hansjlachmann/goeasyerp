package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 792 Suggest Item Price Chg.(Price): suggests new item prices from the existing item
// prices — copied to a price group, unit of measure and currency (ALL: each price's own) and a
// starting date (blank: each price's own), multiplied by a factor above a lower limit and
// rounded by a rounding method — as Item Price Change lines.
const SuggestItemPriceChgPriceID = 792

func init() {
	Register(SuggestItemPriceChgPriceID, "suggest-item-price-chg-price", func() Report { return &suggestItemPriceChgPrice{} })
}

type suggestItemPriceChgPrice struct {
	createNewPrices      bool
	unitPriceFactor      types.Decimal
	priceLowerLimit      types.Decimal
	replacePriceGroup    bool
	replaceUnitOfMeasure bool
	replaceCurrency      bool
	replaceStartingDate  bool
	toPriceGr            *gtables.PriceGroup
	toUnitOfMeasureCode  types.Code
	toCurrencyCode       types.Code
	toDate               types.Date
	roundingMethod       *gtables.RoundingMethod
	roundingCode         types.Code
	itemPriceChange      *gtables.ItemPriceChange
	item                 *gtables.Item
	window               *system.Window
}

func (r *suggestItemPriceChgPrice) Definition() *Definition {
	return &Definition{
		OptionsCaption: "COPY_TO",
		Options: []Option{
			{Name: "price_group_code", Type: "text", Default: all, Relation: &OptionRelation{Table: "Price_Group", Columns: []string{"code", "description"}}},
			{Name: "unit_of_measure_code", Type: "text", Default: all, Relation: &OptionRelation{Table: "Unit_of_Measure", Columns: []string{"code", "description"}}},
			{Name: "currency_code", Type: "text", Default: all, Relation: &OptionRelation{Table: "Currency", Columns: []string{"code", "description"}}},
			{Name: "starting_date", Type: "date"},
			{Name: "price_lower_limit", Type: "number"},
			{Name: "unit_price_factor", Type: "number", Default: "1"},
			{Name: "rounding_method", Type: "text", Relation: &OptionRelation{Table: "Rounding_Method", Columns: []string{"code"}}},
			{Name: "create_new_prices", Type: "bool", Default: "true"},
		},
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Item", Table: "Item", Key: []string{"no"},
			RequestFilterFields: []string{"no", "vendor_no", "inventory_posting_group"},
			OnPreDataItem:       Trigger(r.itemOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.itemOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name: "Item_Price", Table: "Item_Price", Key: []string{"item_no", "price_group_code", "unit_of_measure_code", "currency_code", "starting_date"},
				Link:                map[string]string{"item_no": "no"},
				RequestFilterFields: []string{"price_group_code", "unit_of_measure_code", "currency_code", "starting_date"},
				OnPreDataItem:       Trigger(r.itemPriceOnPreDataItem),
				OnAfterGetRecord:    Trigger(r.itemPriceOnAfterGetRecord),
			}},
		}},
	}
}

// onPreReport: ALL keeps each price's price group, unit of measure and currency; a blank
// starting date keeps each price's starting date (the classic ALL)
func (r *suggestItemPriceChgPrice) onPreReport(run *Run) error {
	r.createNewPrices = run.OptionBool("create_new_prices")
	r.unitPriceFactor = run.OptionDecimal("unit_price_factor")
	r.priceLowerLimit = run.OptionDecimal("price_lower_limit")
	r.roundingCode = types.Code(system.UpperCase(run.Option("rounding_method")))
	r.roundingMethod = gtables.NewRoundingMethod(run)
	toPriceGrCode := types.Code(system.UpperCase(run.Option("price_group_code")))
	r.toUnitOfMeasureCode = types.Code(system.UpperCase(run.Option("unit_of_measure_code")))
	r.toCurrencyCode = types.Code(system.UpperCase(run.Option("currency_code")))
	r.toDate = run.OptionDate("starting_date")

	r.replacePriceGroup = toPriceGrCode != all
	r.replaceUnitOfMeasure = r.toUnitOfMeasureCode != all
	r.replaceCurrency = r.toCurrencyCode != all
	r.replaceStartingDate = !r.toDate.IsZero()

	r.toPriceGr = gtables.NewPriceGroup(run)
	if r.replacePriceGroup {
		err := r.toPriceGr.GetOrError(toPriceGrCode)
		if err != nil {
			return err
		}
	}
	if r.replaceUnitOfMeasure && r.toUnitOfMeasureCode != "" {
		toUnitOfMeasure := gtables.NewUnitOfMeasure(run)
		err := toUnitOfMeasure.GetOrError(r.toUnitOfMeasureCode)
		if err != nil {
			return err
		}
	}
	if r.replaceCurrency && r.toCurrencyCode != "" {
		toCurrency := gtables.NewCurrency(run)
		err := toCurrency.GetOrError(r.toCurrencyCode)
		if err != nil {
			return err
		}
	}
	r.itemPriceChange = gtables.NewItemPriceChange(run)
	return nil
}

func (r *suggestItemPriceChgPrice) itemOnPreDataItem(item *gtables.Item, run *Run) error {
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *suggestItemPriceChgPrice) itemOnAfterGetRecord(item *gtables.Item, run *Run) error {
	r.window.Update(1, item.No)
	r.item = item
	return nil
}

// itemPriceOnPreDataItem: a copy to one currency reads that currency's prices, when the item
// has any
func (r *suggestItemPriceChgPrice) itemPriceOnPreDataItem(itemPrice *gtables.ItemPrice, run *Run) error {
	if !r.replaceCurrency {
		r.itemPriceChange.Currency_code = itemPrice.Currency_code
	} else {
		itemPrice.SetRange("currency_code", r.toCurrencyCode)
		if !itemPrice.FindFirst() {
			itemPrice.SetRange("currency_code")
		}
		r.itemPriceChange.Currency_code = r.toCurrencyCode
	}
	return nil
}

func (r *suggestItemPriceChgPrice) itemPriceOnAfterGetRecord(itemPrice *gtables.ItemPrice, run *Run) error {
	itemPriceChange := r.itemPriceChange
	itemPriceChange.Item_no = itemPrice.Item_no
	itemPriceChange.New_unit_price = itemPrice.Unit_price

	if !r.replaceStartingDate {
		itemPriceChange.Starting_date = itemPrice.Starting_date
	} else {
		itemPriceChange.Starting_date = r.toDate
	}

	if !r.replacePriceGroup {
		itemPriceChange.Price_group_code = itemPrice.Price_group_code
	} else {
		itemPriceChange.Price_group_code = r.toPriceGr.Code
	}
	if itemPriceChange.Price_group_code == itemPrice.Price_group_code {
		itemPriceChange.Price_includes_tax = itemPrice.Price_includes_tax
		itemPriceChange.Allow_quantity_disc = itemPrice.Allow_quantity_disc
		itemPriceChange.Allow_cust_item_disc = itemPrice.Allow_cust_item_disc
		itemPriceChange.Allow_invoice_disc = itemPrice.Allow_invoice_disc
		itemPriceChange.Tax_bus_posting_gr_price = itemPrice.Tax_bus_posting_gr_price
	} else {
		itemPriceChange.Price_includes_tax = r.toPriceGr.Price_includes_tax
		itemPriceChange.Allow_quantity_disc = r.toPriceGr.Allow_quantity_disc
		itemPriceChange.Allow_cust_item_disc = r.toPriceGr.Allow_cust_item_disc
		itemPriceChange.Allow_invoice_disc = r.toPriceGr.Allow_invoice_disc
		itemPriceChange.Tax_bus_posting_gr_price = r.toPriceGr.Tax_bus_posting_gr_price
	}

	if !r.replaceUnitOfMeasure {
		itemPriceChange.Unit_of_measure_code = itemPrice.Unit_of_measure_code
	} else {
		itemPriceChange.Unit_of_measure_code = r.toUnitOfMeasureCode
		if itemPriceChange.Unit_of_measure_code != "" && itemPriceChange.Unit_of_measure_code != r.item.Base_unit_of_measure {
			itemUnitOfMeasure := gtables.NewItemUnitOfMeasure(run)
			if !itemUnitOfMeasure.Get(map[string]interface{}{"item_no": itemPrice.Item_no, "code": itemPriceChange.Unit_of_measure_code}) {
				return Skip()
			}
		}
		toQtyPerUnitOfMeasure, err := tables.GetQtyPerUnitOfMeasure(r.item, itemPriceChange.Unit_of_measure_code)
		if err != nil {
			return err
		}
		fromQtyPerUnitOfMeasure, err := tables.GetQtyPerUnitOfMeasure(r.item, itemPrice.Unit_of_measure_code)
		if err != nil {
			return err
		}
		itemPriceChange.New_unit_price = itemPriceChange.New_unit_price.Mul(toQtyPerUnitOfMeasure).Div(fromQtyPerUnitOfMeasure)
	}

	currExchRate := tables.ExchangeRates(run)
	if itemPrice.Currency_code != itemPriceChange.Currency_code {
		if itemPrice.Currency_code != "" {
			fromCurrency := gtables.NewCurrency(run)
			err := fromCurrency.GetOrError(itemPrice.Currency_code)
			if err != nil {
				return err
			}
			exchangeRate, err := currExchRate.ExchangeRate(system.WorkDate(), itemPrice.Currency_code)
			if err != nil {
				return err
			}
			itemPriceChange.New_unit_price, err = currExchRate.ExchangeAmtFCYToLCY(system.WorkDate(), itemPrice.Currency_code, itemPriceChange.New_unit_price, exchangeRate)
			if err != nil {
				return err
			}
		}
		if itemPriceChange.Currency_code != "" {
			exchangeRate, err := currExchRate.ExchangeRate(system.WorkDate(), itemPriceChange.Currency_code)
			if err != nil {
				return err
			}
			itemPriceChange.New_unit_price, err = currExchRate.ExchangeAmtLCYToFCY(system.WorkDate(), itemPriceChange.Currency_code, itemPriceChange.New_unit_price, exchangeRate)
			if err != nil {
				return err
			}
		}
	}

	currency2 := gtables.NewCurrency(run)
	if itemPriceChange.Currency_code == "" {
		tables.InitRoundingPrecision(currency2)
	} else {
		err := currency2.GetOrError(itemPriceChange.Currency_code)
		if err != nil {
			return err
		}
		err = currency2.TestField("unit_amount_rounding_precision")
		if err != nil {
			return err
		}
	}
	itemPriceChange.New_unit_price = system.Round(itemPriceChange.New_unit_price, currency2.Unit_amount_rounding_precision)
	if itemPriceChange.New_unit_price.GreaterThan(r.priceLowerLimit) {
		itemPriceChange.New_unit_price = itemPriceChange.New_unit_price.Mul(r.unitPriceFactor)
	}
	itemPriceChange.New_unit_price = tables.ApplyRoundingMethod(r.roundingMethod, r.roundingCode, itemPriceChange.New_unit_price)

	_, priceAlreadyExists := setCurrentItemPrice(run, itemPriceChange)
	return insertItemPriceChange(run, itemPriceChange, priceAlreadyExists || r.createNewPrices)
}
