// Package consolidation holds the business logic of the business units (table 220, page 240)
// that is not a report: the exchange rates of a business unit (Exch. Rates on page 240).
package consolidation

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func init() {
	functions.Register("table", gtables.BusinessUnitTableName, "ChangeExchangeRate", functions.ForArgs(ChangeExchangeRate, "int"), functions.WithDialog())
}

// The currency factors Exch. Rates changes (its menu items)
const (
	IncomeStatement  = 1 // Income Currency Factor
	BalanceSheet     = 2 // Balance Currency Factor
	LastBalanceSheet = 3 // Last Balance Currency Factor
)

// ChangeExchangeRate (Exch. Rates → Income Statement / Balance Sheet / Last Balance Sheet, the
// Change Exchange Rate dialog): the user enters the rate as an amount of the business unit's
// currency and the amount in the local currency it is worth; the factor is their quotient
func ChangeExchangeRate(rec *gtables.BusinessUnit, args functions.Args) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	which := args.Int(0)
	var factor types.Decimal
	switch which {
	case IncomeStatement:
		factor = rec.Income_currency_factor
	case BalanceSheet:
		factor = rec.Balance_currency_factor
	case LastBalanceSheet:
		factor = rec.Last_balance_currency_factor
	default:
		return fcodeunits.Result{}, system.Error(ts.Message("BU_EXCH_RATE_UNKNOWN", lang))
	}
	relational := types.NewDecimalFromInt(1)
	if !factor.IsZero() {
		relational = types.NewDecimalFromInt(1).Div(factor).Round(6)
	}
	answer := fcodeunits.RequestInput(ts.Message("BU_EXCH_RATE_TITLE", lang), []fcodeunits.InputField{
		{Name: "exchange_rate_amount", Label: system.StrSubstNo(ts.Message("BU_EXCH_RATE_AMOUNT", lang), rec.Currency_code), Type: "number", Required: true, Default: "1"},
		{Name: "relational_exch_rate_amount", Label: ts.Message("BU_EXCH_RATE_RELATIONAL", lang), Type: "number", Required: true, Default: relational.String()},
	})
	if answer == nil {
		return fcodeunits.Result{Success: true}, nil
	}
	exchangeRateAmount, err := types.NewDecimalFromString(answer["exchange_rate_amount"])
	if err != nil {
		return fcodeunits.Result{}, err
	}
	relationalExchRateAmount, err := types.NewDecimalFromString(answer["relational_exch_rate_amount"])
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !exchangeRateAmount.IsPositive() || !relationalExchRateAmount.IsPositive() {
		return fcodeunits.Result{}, system.Error(ts.Message("BU_EXCH_RATE_POSITIVE", lang))
	}
	factor = exchangeRateAmount.Div(relationalExchRateAmount)
	switch which {
	case IncomeStatement:
		rec.Income_currency_factor = factor
	case BalanceSheet:
		rec.Balance_currency_factor = factor
	case LastBalanceSheet:
		rec.Last_balance_currency_factor = factor
	}
	err = rec.ModifyOrError(true)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}
