package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 1192 Suggest Res. Price Chg. (Price): suggests new resource prices from the existing
// resource prices — copied to a currency and work type (ALL: each price's own), multiplied by
// a factor above a lower limit and rounded by a rounding method — as Resource Price Change
// lines.
const SuggestResPriceChgPriceID = 1192

func init() {
	Register(SuggestResPriceChgPriceID, "suggest-res-price-chg-price", func() Report { return &suggestResPriceChgPrice{} })
}

// all is the value of the Currency Code and Work Type Code options that keeps each price's own
const all = "ALL"

type suggestResPriceChgPrice struct {
	createNewPrices bool
	unitPriceFactor types.Decimal
	priceLowerLimit types.Decimal
	toCurrencyCode  types.Code
	toWorkTypeCode  types.Code
	toProjectNo     types.Code
	roundingMethod  *gtables.RoundingMethod
	roundingCode    types.Code
	window          *system.Window
}

func (r *suggestResPriceChgPrice) Definition() *Definition {
	return &Definition{
		OptionsCaption: "COPY_TO",
		Options: []Option{
			{Name: "project_no", Type: "text", Default: all, Relation: &OptionRelation{Table: "Project", Columns: []string{"no", "description"}}},
			{Name: "currency_code", Type: "text", Default: all, Relation: &OptionRelation{Table: "Currency", Columns: []string{"code", "description"}}},
			{Name: "work_type_code", Type: "text", Default: all, Relation: &OptionRelation{Table: "Work_Type", Columns: []string{"code", "description"}}},
			{Name: "unit_price_factor", Type: "number", Default: "1"},
			{Name: "rounding_method", Type: "text", Relation: &OptionRelation{Table: "Rounding_Method", Columns: []string{"code"}}},
			{Name: "price_lower_limit", Type: "number"},
			{Name: "create_new_prices", Type: "bool", Default: "true"},
		},
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Resource_Price", Table: "Resource_Price", Key: []string{"project_no", "type", "code", "work_type_code", "currency_code"},
			RequestFilterFields: []string{"project_no", "type", "code", "work_type_code", "currency_code"},
			OnPreDataItem:       Trigger(r.resourcePriceOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.resourcePriceOnAfterGetRecord),
		}},
	}
}

func (r *suggestResPriceChgPrice) onPreReport(run *Run) error {
	r.createNewPrices = run.OptionBool("create_new_prices")
	r.unitPriceFactor = run.OptionDecimal("unit_price_factor")
	r.priceLowerLimit = run.OptionDecimal("price_lower_limit")
	r.roundingCode = types.Code(system.UpperCase(run.Option("rounding_method")))
	r.roundingMethod = gtables.NewRoundingMethod(run)
	r.toCurrencyCode = types.Code(system.UpperCase(run.Option("currency_code")))
	r.toWorkTypeCode = types.Code(system.UpperCase(run.Option("work_type_code")))
	r.toProjectNo = types.Code(system.UpperCase(run.Option("project_no")))
	if r.toProjectNo != all && r.toProjectNo != "" {
		toProject := gtables.NewProject(run)
		err := toProject.GetOrError(r.toProjectNo)
		if err != nil {
			return err
		}
	}
	if r.toCurrencyCode != all {
		if r.toCurrencyCode != "" {
			toCurrency := gtables.NewCurrency(run)
			err := toCurrency.GetOrError(r.toCurrencyCode)
			if err != nil {
				return err
			}
		}
	}
	if r.toWorkTypeCode != all && r.toWorkTypeCode != "" {
		toWorkType := gtables.NewWorkType(run)
		err := toWorkType.GetOrError(r.toWorkTypeCode)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *suggestResPriceChgPrice) resourcePriceOnPreDataItem(resourcePrice *gtables.ResourcePrice, run *Run) error {
	r.window = system.OpenWindow(run.Text("WINDOW"))
	// a copy to the local currency reads the prices in local currency only
	if r.toCurrencyCode == "" {
		resourcePrice.SetRange("currency_code", types.Code(""))
	}
	return nil
}

func (r *suggestResPriceChgPrice) resourcePriceOnAfterGetRecord(resourcePrice *gtables.ResourcePrice, run *Run) error {
	r.window.Update(1, resourcePrice.Type)
	r.window.Update(2, resourcePrice.Code)
	resPriceChg := gtables.NewResourcePriceChange(run)
	resPriceChg.Type = gtables.ResourcePriceChangeType(resourcePrice.Type)
	resPriceChg.Code = resourcePrice.Code
	if r.toCurrencyCode == all {
		resPriceChg.Currency_code = resourcePrice.Currency_code
	} else {
		resPriceChg.Currency_code = r.toCurrencyCode
	}
	if r.toWorkTypeCode == all {
		resPriceChg.Work_type_code = resourcePrice.Work_type_code
	} else {
		resPriceChg.Work_type_code = r.toWorkTypeCode
	}
	if r.toProjectNo == all {
		resPriceChg.Project_no = resourcePrice.Project_no
	} else {
		resPriceChg.Project_no = r.toProjectNo
	}

	resPriceChg.New_unit_price = resourcePrice.Unit_price
	currExchRate := tables.ExchangeRates(run)
	if resPriceChg.Currency_code != resourcePrice.Currency_code {
		if resourcePrice.Currency_code != "" {
			exchangeRate, err := currExchRate.ExchangeRate(system.WorkDate(), resourcePrice.Currency_code)
			if err != nil {
				return err
			}
			resPriceChg.New_unit_price, err = currExchRate.ExchangeAmtFCYToLCY(system.WorkDate(), resourcePrice.Currency_code, resPriceChg.New_unit_price, exchangeRate)
			if err != nil {
				return err
			}
		}
		if resPriceChg.Currency_code != "" {
			exchangeRate, err := currExchRate.ExchangeRate(system.WorkDate(), resPriceChg.Currency_code)
			if err != nil {
				return err
			}
			resPriceChg.New_unit_price, err = currExchRate.ExchangeAmtLCYToFCY(system.WorkDate(), resPriceChg.Currency_code, resPriceChg.New_unit_price, exchangeRate)
			if err != nil {
				return err
			}
		}
	}
	currency2 := gtables.NewCurrency(run)
	if resPriceChg.Currency_code == "" {
		tables.InitRoundingPrecision(currency2)
	} else {
		err := currency2.GetOrError(resPriceChg.Currency_code)
		if err != nil {
			return err
		}
		err = currency2.TestField("unit_amount_rounding_precision")
		if err != nil {
			return err
		}
	}
	resPriceChg.New_unit_price = system.Round(resPriceChg.New_unit_price, currency2.Unit_amount_rounding_precision)
	if resPriceChg.New_unit_price.GreaterThan(r.priceLowerLimit) {
		resPriceChg.New_unit_price = resPriceChg.New_unit_price.Mul(r.unitPriceFactor)
	}
	resPriceChg.New_unit_price = tables.ApplyRoundingMethod(r.roundingMethod, r.roundingCode, resPriceChg.New_unit_price)

	return insertResPriceChange(run, resPriceChg, r.createNewPrices)
}
