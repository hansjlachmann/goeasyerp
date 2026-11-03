package reports

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 1191 Suggest Res. Price Chg. (Res.): suggests new resource prices from the resources'
// unit prices — in a currency, for a work type, multiplied by a factor above a lower limit and
// rounded by a rounding method — as Resource Price Change lines, next to the current price
// (only where a price exists, unless Create New Prices).
const SuggestResPriceChgResID = 1191

func init() {
	Register(SuggestResPriceChgResID, "suggest-res-price-chg-res", func() Report { return &suggestResPriceChgRes{} })
}

type suggestResPriceChgRes struct {
	createNewPrices bool
	unitPriceFactor types.Decimal
	priceLowerLimit types.Decimal
	toCurrency      *gtables.Currency
	toWorkTypeCode  types.Code
	toProjectNo     types.Code
	roundingMethod  *gtables.RoundingMethod
	roundingCode    types.Code
	resPriceChg     *gtables.ResourcePriceChange
	window          *system.Window
}

func (r *suggestResPriceChgRes) Definition() *Definition {
	return &Definition{
		OptionsCaption: "COPY_TO",
		Options: []Option{
			{Name: "project_no", Type: "text", Relation: &OptionRelation{Table: "Project", Columns: []string{"no", "description"}}},
			{Name: "work_type_code", Type: "text", Relation: &OptionRelation{Table: "Work_Type", Columns: []string{"code", "description"}}},
			{Name: "currency_code", Type: "text", Relation: &OptionRelation{Table: "Currency", Columns: []string{"code", "description"}}},
			{Name: "unit_price_factor", Type: "number", Default: "1"},
			{Name: "rounding_method", Type: "text", Relation: &OptionRelation{Table: "Rounding_Method", Columns: []string{"code"}}},
			{Name: "price_lower_limit", Type: "number"},
			{Name: "create_new_prices", Type: "bool", Default: "false"},
		},
		OnPreReport: r.onPreReport,
		DataItems: []*DataItem{{
			Name: "Resource", Table: "Resource", Key: []string{"no"},
			RequestFilterFields: []string{"no", "resource_group_no"},
			OnPreDataItem:       Trigger(r.resourceOnPreDataItem),
			OnAfterGetRecord:    Trigger(r.resourceOnAfterGetRecord),
		}},
	}
}

func (r *suggestResPriceChgRes) onPreReport(run *Run) error {
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
	r.toWorkTypeCode = types.Code(system.UpperCase(run.Option("work_type_code")))
	if r.toWorkTypeCode != "" {
		toWorkType := gtables.NewWorkType(run)
		err := toWorkType.GetOrError(r.toWorkTypeCode)
		if err != nil {
			return err
		}
	}

	r.toProjectNo = types.Code(system.UpperCase(run.Option("project_no")))
	if r.toProjectNo != "" {
		toProject := gtables.NewProject(run)
		err := toProject.GetOrError(r.toProjectNo)
		if err != nil {
			return err
		}
	}

	r.resPriceChg = gtables.NewResourcePriceChange(run)
	r.resPriceChg.Project_no = r.toProjectNo
	r.resPriceChg.Currency_code = r.toCurrency.Code
	r.resPriceChg.Work_type_code = r.toWorkTypeCode
	return nil
}

func (r *suggestResPriceChgRes) resourceOnPreDataItem(resource *gtables.Resource, run *Run) error {
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *suggestResPriceChgRes) resourceOnAfterGetRecord(resource *gtables.Resource, run *Run) error {
	r.window.Update(1, resource.No)
	resPriceChg := r.resPriceChg
	resPriceChg.Type = gtables.ResourcePriceChange_Type.Resource
	resPriceChg.Code = resource.No

	currExchRate := tables.ExchangeRates(run)
	exchangeRate, err := currExchRate.ExchangeRate(system.WorkDate(), r.toCurrency.Code)
	if err != nil {
		return err
	}
	newUnitPrice, err := currExchRate.ExchangeAmtLCYToFCY(system.WorkDate(), r.toCurrency.Code, resource.Unit_price, exchangeRate)
	if err != nil {
		return err
	}
	resPriceChg.New_unit_price = system.Round(newUnitPrice, r.toCurrency.Unit_amount_rounding_precision)
	if resPriceChg.New_unit_price.GreaterThan(r.priceLowerLimit) {
		resPriceChg.New_unit_price = resPriceChg.New_unit_price.Mul(r.unitPriceFactor)
	}
	resPriceChg.New_unit_price = tables.ApplyRoundingMethod(r.roundingMethod, r.roundingCode, resPriceChg.New_unit_price)

	return insertResPriceChange(run, resPriceChg, r.createNewPrices)
}

// insertResPriceChange gives a suggested price change the current price of its key and
// inserts or modifies it — only where a price exists, unless new prices are created (the
// end of the suggest reports' OnAfterGetRecord)
func insertResPriceChange(run *Run, resPriceChg *gtables.ResourcePriceChange, createNewPrices bool) error {
	priceAlreadyExists := false
	resourcePrice := gtables.NewResourcePrice(run)
	resourcePrice.SetRange("project_no", resPriceChg.Project_no)
	resourcePrice.SetRange("type", resPriceChg.Type)
	resourcePrice.SetRange("code", resPriceChg.Code)
	resourcePrice.SetRange("currency_code", resPriceChg.Currency_code)
	resourcePrice.SetRange("work_type_code", resPriceChg.Work_type_code)
	if resourcePrice.FindLast() {
		resPriceChg.Current_unit_price = resourcePrice.Unit_price
		priceAlreadyExists = true
	} else {
		resPriceChg.Current_unit_price = types.Decimal{}
		priceAlreadyExists = false
	}
	if priceAlreadyExists || createNewPrices {
		resPriceChg2 := gtables.NewResourcePriceChange(run)
		key := map[string]interface{}{"project_no": resPriceChg.Project_no, "type": resPriceChg.Type, "code": resPriceChg.Code, "work_type_code": resPriceChg.Work_type_code, "currency_code": resPriceChg.Currency_code}
		if resPriceChg2.Get(key) {
			resPriceChg2.Current_unit_price = resPriceChg.Current_unit_price
			resPriceChg2.New_unit_price = resPriceChg.New_unit_price
			return resPriceChg2.ModifyOrError(false)
		}
		resPriceChg2.Init()
		resPriceChg2.Project_no = resPriceChg.Project_no
		resPriceChg2.Type = resPriceChg.Type
		resPriceChg2.Code = resPriceChg.Code
		resPriceChg2.Work_type_code = resPriceChg.Work_type_code
		resPriceChg2.Currency_code = resPriceChg.Currency_code
		resPriceChg2.Current_unit_price = resPriceChg.Current_unit_price
		resPriceChg2.New_unit_price = resPriceChg.New_unit_price
		return resPriceChg2.InsertOrError(false)
	}
	return nil
}
