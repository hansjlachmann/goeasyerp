package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// currencyExchangeRateLogic is the business logic of table 330 Currency_Exchange_Rate.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/currencyexchangerate_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type currencyExchangeRateLogic struct{}

func init() { gtables.RegisterCurrencyExchangeRateLogic(currencyExchangeRateLogic{}) }

// OnInsert runs before a new record is inserted
func (currencyExchangeRateLogic) OnInsert(rec *gtables.CurrencyExchangeRate) error {
	return nil
}

// OnModify runs before a record is modified
func (currencyExchangeRateLogic) OnModify(rec *gtables.CurrencyExchangeRate) error {
	return nil
}

// OnDelete runs before a record is deleted
func (currencyExchangeRateLogic) OnDelete(rec *gtables.CurrencyExchangeRate) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (currencyExchangeRateLogic) OnRename(rec *gtables.CurrencyExchangeRate) error {
	return nil
}

// OnValidate_Currency_code: a currency is not quoted in itself
func (currencyExchangeRateLogic) OnValidate_Currency_code(rec *gtables.CurrencyExchangeRate) error {
	return checkNotSameCurrency(rec)
}

// OnValidate_Relational_currency_code: a currency is not quoted in itself
func (currencyExchangeRateLogic) OnValidate_Relational_currency_code(rec *gtables.CurrencyExchangeRate) error {
	return checkNotSameCurrency(rec)
}

func checkNotSameCurrency(rec *gtables.CurrencyExchangeRate) error {
	if rec.Currency_code == rec.Relational_currency_code {
		return apperrors.SameCurrency(gtables.CurrencyExchangeRateTableName, "relational_currency_code")
	}
	return nil
}

// OnValidate_Starting_date: a new starting date takes the amounts of the rate before it (the
// rate is entered as a change of the previous one)
func (currencyExchangeRateLogic) OnValidate_Starting_date(rec *gtables.CurrencyExchangeRate) error {
	xRec := rec.XRec()
	if !xRec.Starting_date.Equal(rec.Starting_date) {
		currExchRate := gtables.NewCurrencyExchangeRate(rec)
		currExchRate.SetRange("currency_code", rec.Currency_code)
		currExchRate.SetRange("starting_date", types.Date{}, rec.Starting_date.AddDays(-1))
		if currExchRate.FindLast() {
			rec.Exchange_rate_amount = currExchRate.Exchange_rate_amount
			rec.Adjustment_exch_rate_amount = currExchRate.Adjustment_exch_rate_amount
			rec.Relational_currency_code = currExchRate.Relational_currency_code
			rec.Relational_exch_rate_amount = currExchRate.Relational_exch_rate_amount
			rec.Fix_exchange_rate_amount = currExchRate.Fix_exchange_rate_amount
			rec.Relational_adjmt_exch_rate_amt = currExchRate.Relational_adjmt_exch_rate_amt
		}
	}
	return nil
}

// The amounts must not be 0 (TestField)
func (currencyExchangeRateLogic) OnValidate_Exchange_rate_amount(rec *gtables.CurrencyExchangeRate) error {
	return rec.TestField("exchange_rate_amount")
}

func (currencyExchangeRateLogic) OnValidate_Adjustment_exch_rate_amount(rec *gtables.CurrencyExchangeRate) error {
	return rec.TestField("adjustment_exch_rate_amount")
}

func (currencyExchangeRateLogic) OnValidate_Relational_exch_rate_amount(rec *gtables.CurrencyExchangeRate) error {
	return rec.TestField("relational_exch_rate_amount")
}

func (currencyExchangeRateLogic) OnValidate_Relational_adjmt_exch_rate_amt(rec *gtables.CurrencyExchangeRate) error {
	return rec.TestField("relational_adjmt_exch_rate_amt")
}
