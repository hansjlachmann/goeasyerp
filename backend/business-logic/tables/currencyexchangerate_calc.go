package tables

import (
	"github.com/shopspring/decimal"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The exchange calculations of table 330 (its procedures): amounts between the local currency
// (LCY) and foreign currencies (FCY) at the rate valid on a date — the latest rate whose
// Starting Date is on or before it. A rate is "Exchange Rate Amount" units of the currency for
// "Relational Exch. Rate Amount" units of the local currency, or of its Relational Currency
// (then converted on through that currency's rate). Fix Exchange Rate Amount decides which side
// of a relational rate is fixed when a currency factor is given. Factor is the currency factor
// of the document (FCY per LCY); with a Relational Currency the factor applies to the chain.
//
// Use: rates := tables.ExchangeRates(rec) — any record, for its database and company.

// CurrExchRate runs the exchange calculations in one company
type CurrExchRate struct {
	rate gtables.CurrencyExchangeRate // the database and company to read rates from
}

// ExchangeRates returns the exchange calculations for the company of a record
func ExchangeRates(other interface {
	GetDB() database.Executor
	GetCompany() string
	GetDBType() database.DBType
}) *CurrExchRate {
	c := &CurrExchRate{}
	c.rate.InitWithDBType(other.GetDB(), other.GetCompany(), other.GetDBType())
	return c
}

// rateAmounts is a rate as the calculation uses it (the adjustment amounts in place of the
// normal ones for adjustments)
type rateAmounts struct {
	exch, rel decimal.Decimal
	relCur    types.Code
	fix       gtables.CurrencyExchangeRateFix_exchange_rate_amount
	currency  types.Code
}

// FindCurrency returns the rate of a currency valid on a date (FindCurrency): the last one with
// a Starting Date on or before it
func (c *CurrExchRate) FindCurrency(date types.Date, currencyCode types.Code) (*gtables.CurrencyExchangeRate, error) {
	currExchRate := gtables.NewCurrencyExchangeRate(&c.rate)
	currExchRate.SetRange("currency_code", currencyCode)
	currExchRate.SetRange("starting_date", types.Date{}, date)
	if !currExchRate.FindLast() {
		return nil, apperrors.NoExchangeRate(currencyCode.String(), system.Format(date))
	}
	return currExchRate, nil
}

// find reads a rate and checks its amounts (TestField), taking the adjustment amounts when adjmt
func (c *CurrExchRate) find(date types.Date, currencyCode types.Code, adjmt bool) (rateAmounts, error) {
	currExchRate, err := c.FindCurrency(date, currencyCode)
	if err != nil {
		return rateAmounts{}, err
	}
	exchField, relField := "exchange_rate_amount", "relational_exch_rate_amount"
	exch, rel := currExchRate.Exchange_rate_amount, currExchRate.Relational_exch_rate_amount
	if adjmt {
		exchField, relField = "adjustment_exch_rate_amount", "relational_adjmt_exch_rate_amt"
		exch, rel = currExchRate.Adjustment_exch_rate_amount, currExchRate.Relational_adjmt_exch_rate_amt
	}
	err = currExchRate.TestField(exchField)
	if err != nil {
		return rateAmounts{}, err
	}
	err = currExchRate.TestField(relField)
	if err != nil {
		return rateAmounts{}, err
	}
	return rateAmounts{exch: exch.Decimal, rel: rel.Decimal, relCur: currExchRate.Relational_currency_code, fix: currExchRate.Fix_exchange_rate_amount, currency: currExchRate.Currency_code}, nil
}

var exchFix = gtables.CurrencyExchangeRate_Fix_exchange_rate_amount

func exchDec(d decimal.Decimal) types.Decimal { return types.Decimal{Decimal: d} }

// ExchangeAmtLCYToFCY converts an amount in the local currency to a currency on a date
// (ExchangeAmtUSDToFCY); a blank currency is the local currency
func (c *CurrExchRate) ExchangeAmtLCYToFCY(date types.Date, currencyCode types.Code, amount, factor types.Decimal) (types.Decimal, error) {
	if currencyCode == "" {
		return amount, nil
	}
	r1, err := c.find(date, currencyCode, false)
	if err != nil {
		return amount, err
	}
	a, f := amount.Decimal, factor.Decimal
	if r1.relCur == "" {
		if r1.fix == exchFix.Both {
			return exchDec(a.Div(r1.rel).Mul(r1.exch)), nil
		}
		return exchDec(a.Mul(f)), nil
	}
	rel1, exch1 := r1.rel, r1.exch
	r2, err := c.find(date, r1.relCur, false)
	if err != nil {
		return amount, err
	}
	switch r1.fix {
	case exchFix.RelationalCurrency:
		exch1 = f.Mul(rel1).Mul(r2.rel).Div(r2.exch)
	case exchFix.Currency:
		rel1 = exch1.Mul(r2.exch).Div(f.Mul(r2.rel))
	case exchFix.Both:
		switch r2.fix {
		case exchFix.RelationalCurrency:
			r2.exch = f.Mul(rel1).Mul(r2.rel).Div(exch1)
		case exchFix.Currency:
			r2.rel = exch1.Mul(r2.exch).Div(f.Mul(rel1))
		}
	}
	a = a.Div(rel1).Mul(exch1)
	a = a.Div(r2.rel).Mul(r2.exch)
	return exchDec(a), nil
}

// ExchangeAmtFCYToLCY converts an amount in a currency to the local currency on a date
// (ExchangeAmtFCYToUSD)
func (c *CurrExchRate) ExchangeAmtFCYToLCY(date types.Date, currencyCode types.Code, amount, factor types.Decimal) (types.Decimal, error) {
	return c.exchangeAmtFCYToLCY(date, currencyCode, amount, factor, false)
}

// ExchangeAmtFCYToLCYAdjmt is ExchangeAmtFCYToLCY with the adjustment exchange rates
func (c *CurrExchRate) ExchangeAmtFCYToLCYAdjmt(date types.Date, currencyCode types.Code, amount, factor types.Decimal) (types.Decimal, error) {
	return c.exchangeAmtFCYToLCY(date, currencyCode, amount, factor, true)
}

func (c *CurrExchRate) exchangeAmtFCYToLCY(date types.Date, currencyCode types.Code, amount, factor types.Decimal, adjmt bool) (types.Decimal, error) {
	if currencyCode == "" {
		return amount, nil
	}
	r1, err := c.find(date, currencyCode, adjmt)
	if err != nil {
		return amount, err
	}
	a, f := amount.Decimal, factor.Decimal
	if r1.relCur == "" {
		if r1.fix == exchFix.Both {
			return exchDec(a.Div(r1.exch).Mul(r1.rel)), nil
		}
		return exchDec(a.Div(f)), nil
	}
	rel1, exch1 := r1.rel, r1.exch
	r2, err := c.find(date, r1.relCur, adjmt)
	if err != nil {
		return amount, err
	}
	switch r1.fix {
	case exchFix.RelationalCurrency:
		exch1 = rel1.Mul(r2.rel).Div(r2.exch.Mul(f))
	case exchFix.Currency:
		rel1 = f.Mul(exch1).Mul(r2.exch).Div(r2.rel)
	case exchFix.Both:
		switch r2.fix {
		case exchFix.RelationalCurrency:
			r2.exch = rel1.Mul(r2.rel).Div(exch1.Mul(f))
		case exchFix.Currency:
			r2.rel = f.Mul(exch1).Mul(r2.exch).Div(rel1)
		case exchFix.Both:
			a = a.Div(exch1).Mul(rel1)
			a = a.Div(r2.exch).Mul(r2.rel)
			return exchDec(a), nil
		}
	}
	a = a.Div(rel1).Mul(exch1)
	a = a.Div(r2.rel).Mul(r2.exch)
	return exchDec(a), nil
}

// ExchangeRate returns the currency factor of a currency on a date (FCY per LCY; 1 for the
// local currency)
func (c *CurrExchRate) ExchangeRate(date types.Date, currencyCode types.Code) (types.Decimal, error) {
	return c.exchangeRate(date, currencyCode, false)
}

// ExchangeRateAdjmt is ExchangeRate with the adjustment exchange rates
func (c *CurrExchRate) ExchangeRateAdjmt(date types.Date, currencyCode types.Code) (types.Decimal, error) {
	return c.exchangeRate(date, currencyCode, true)
}

func (c *CurrExchRate) exchangeRate(date types.Date, currencyCode types.Code, adjmt bool) (types.Decimal, error) {
	if currencyCode == "" {
		return types.NewDecimalFromInt(1), nil
	}
	r1, err := c.find(date, currencyCode, adjmt)
	if err != nil {
		return types.ZeroDecimal(), err
	}
	if r1.relCur == "" {
		return exchDec(r1.exch.Div(r1.rel)), nil
	}
	r2, err := c.find(date, r1.relCur, adjmt)
	if err != nil {
		return types.ZeroDecimal(), err
	}
	return exchDec(r1.exch.Mul(r2.exch).Div(r1.rel.Mul(r2.rel))), nil
}

// ExchangeAmtLCYToFCYOnlyFactor converts with a currency factor only (no rate is read)
func (c *CurrExchRate) ExchangeAmtLCYToFCYOnlyFactor(amount, factor types.Decimal) types.Decimal {
	return exchDec(factor.Decimal.Mul(amount.Decimal))
}

// ExchangeAmtFCYToFCY converts an amount from one currency to another on a date; a blank code
// is the local currency (through the relational currencies where the rates name them)
func (c *CurrExchRate) ExchangeAmtFCYToFCY(date types.Date, fromCurrency, toCurrency types.Code, amount types.Decimal) (types.Decimal, error) {
	if fromCurrency == toCurrency {
		return amount, nil
	}
	a := amount.Decimal
	if toCurrency == "" {
		from, err := c.find(date, fromCurrency, false)
		if err != nil {
			return amount, err
		}
		a = a.Div(from.exch).Mul(from.rel)
		if from.relCur == "" {
			return exchDec(a), nil
		}
		via, err := c.find(date, from.relCur, false)
		if err != nil {
			return amount, err
		}
		return exchDec(a.Div(via.exch).Mul(via.rel)), nil
	}
	if fromCurrency == "" {
		to, err := c.find(date, toCurrency, false)
		if err != nil {
			return amount, err
		}
		a = a.Div(to.rel).Mul(to.exch)
		if to.relCur == "" {
			return exchDec(a), nil
		}
		via, err := c.find(date, to.relCur, false)
		if err != nil {
			return amount, err
		}
		return exchDec(a.Div(via.rel).Mul(via.exch)), nil
	}
	from, err := c.find(date, fromCurrency, false)
	if err != nil {
		return amount, err
	}
	to, err := c.find(date, toCurrency, false)
	if err != nil {
		return amount, err
	}
	switch {
	case from.currency == to.relCur:
		return exchDec(a.Div(to.rel).Mul(to.exch)), nil
	case from.relCur == to.currency:
		return exchDec(a.Div(from.exch).Mul(from.rel)), nil
	case from.relCur == to.relCur:
		a = a.Div(from.exch).Mul(from.rel)
		return exchDec(a.Div(to.rel).Mul(to.exch)), nil
	case from.relCur == "" && to.relCur != "":
		via, err := c.find(date, to.relCur, false)
		if err != nil {
			return amount, err
		}
		a = a.Mul(from.rel).Div(from.exch)
		a = a.Div(via.rel).Mul(via.exch)
		return exchDec(a.Div(to.rel).Mul(to.exch)), nil
	case from.relCur != "" && to.relCur == "":
		via, err := c.find(date, from.relCur, false)
		if err != nil {
			return amount, err
		}
		a = a.Div(from.exch).Mul(from.rel)
		a = a.Div(via.exch).Mul(via.rel)
		return exchDec(a.Div(to.rel).Mul(to.exch)), nil
	}
	return types.ZeroDecimal(), nil // both relational to different currencies: as the classic code (no case)
}
