package genjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The additional reporting currency (General Ledger Setup 68): every G/L entry also gets its
// amount in that currency (Additional-Currency Amount, with its debit and credit) — the line's
// source currency amount when the line is in that currency, else the LCY amount exchanged at the
// posting date's rate. Where the LCY amounts of a transaction balance but the exchanged ones do
// not, the difference goes to the currency's residual gains or losses account (an entry with
// amount 0). The balance check also checks the additional-currency amounts.

// addCurrency is the posting's view of the additional reporting currency (classic Currency2,
// CurrencyDate, CurrencyFactor, UseCurrFactorOnly)
type addCurrency struct {
	currency          *gtables.Currency
	currencyDate      types.Date
	currencyFactor    types.Decimal
	useCurrFactorOnly bool
	// residuals: the transaction's LCY and additional-currency totals so far
	totalAmount           types.Decimal
	totalAddCurrAmount    types.Decimal
	nonAddCurrCodeOccured bool
}

// acyCode is the additional reporting currency ("" without one)
func (p *GenJnlPostLine) acyCode() types.Code {
	return p.glSetup.Additional_reporting_currency
}

// getCurrencyExchRate (GetCurrencyExchRate): the additional currency (its rounding and residual
// accounts are required) and its rate on the line's posting date (the day before for a reversing
// entry); a fixed asset line with its own FA Add.-Currency Factor uses that factor alone
func (p *GenJnlPostLine) getCurrencyExchRate() error {
	if p.acyCode() == "" {
		return nil
	}
	acy := &p.addCurr
	if acy.currency == nil || acy.currency.Code != p.acyCode() {
		acy.currency = gtables.NewCurrency(p.src)
		err := acy.currency.GetOrError(p.acyCode())
		if err != nil {
			return err
		}
		for _, field := range []string{"amount_rounding_precision", "residual_gains_account", "residual_losses_account"} {
			err = acy.currency.TestField(field)
			if err != nil {
				return err
			}
		}
	}
	newCurrencyDate := p.genJnlLine.Posting_date.Normal()
	if p.genJnlLine.Reversing_entry {
		newCurrencyDate = newCurrencyDate.AddDays(-1)
	}
	if !newCurrencyDate.Equal(acy.currencyDate) || acy.useCurrFactorOnly {
		acy.useCurrFactorOnly = false
		acy.currencyDate = newCurrencyDate
		factor, err := tables.ExchangeRates(p.src).ExchangeRate(acy.currencyDate, p.acyCode())
		if err != nil {
			return err
		}
		acy.currencyFactor = factor
	}
	if !p.genJnlLine.Fa_add_currency_factor.IsZero() && !p.genJnlLine.Fa_add_currency_factor.Equal(acy.currencyFactor) {
		acy.useCurrFactorOnly = true
		acy.currencyDate = types.Date{}
		acy.currencyFactor = p.genJnlLine.Fa_add_currency_factor
	}
	return nil
}

// exchangeAmtLCYToAddCurr (ExchangeAmtUSDToFCY2): an LCY amount in the additional currency,
// rounded to its precision
func (p *GenJnlPostLine) exchangeAmtLCYToAddCurr(amount types.Decimal) (types.Decimal, error) {
	acy := &p.addCurr
	rates := tables.ExchangeRates(p.src)
	if acy.useCurrFactorOnly {
		return system.Round(rates.ExchangeAmtLCYToFCYOnlyFactor(amount, acy.currencyFactor), acy.currency.Amount_rounding_precision), nil
	}
	exchanged, err := rates.ExchangeAmtLCYToFCY(acy.currencyDate, p.acyCode(), amount, acy.currencyFactor)
	if err != nil {
		return types.Decimal{}, err
	}
	return system.Round(exchanged, acy.currency.Amount_rounding_precision), nil
}

// calcLCYToAddCurr (CalcUSDToAddCurr): an LCY amount in the additional currency (0 without one)
func (p *GenJnlPostLine) calcLCYToAddCurr(amountLCY types.Decimal) (types.Decimal, error) {
	if p.acyCode() == "" {
		return types.Decimal{}, nil
	}
	return p.exchangeAmtLCYToAddCurr(amountLCY)
}

// calcAddCurrFactor (CalcAddCurrFactor): the additional-currency amount per LCY of a pair of
// amounts (the rate when the additional-currency amount is 0)
func (p *GenJnlPostLine) calcAddCurrFactor(numerator, denominator types.Decimal) (types.Decimal, error) {
	if denominator.IsZero() {
		return types.Decimal{}, nil
	}
	if !numerator.IsZero() {
		return numerator.Div(denominator), nil
	}
	return p.calcLCYToAddCurr(types.NewDecimalFromInt(1))
}

// glCalcAddCurrency (GLCalcAddCurrency): the G/L entry's additional-currency amount — the given
// one when the line is in the additional currency (and it may be used), else its LCY amount
// exchanged; a line of one of the two amounts only keeps what it set
func (p *GenJnlPostLine) glCalcAddCurrency(addCurrAmount types.Decimal, useAddCurrAmount bool) error {
	if p.acyCode() == "" || p.genJnlLine.Additional_currency_posting != gtables.GenJournalLine_Additional_currency_posting.None {
		return nil
	}
	if p.genJnlLine.Source_currency_code == p.acyCode() && useAddCurrAmount {
		p.glEntry.Additional_currency_amount = addCurrAmount
		return nil
	}
	amount, err := p.exchangeAmtLCYToAddCurr(p.glEntry.Amount)
	if err != nil {
		return err
	}
	p.glEntry.Additional_currency_amount = amount
	return nil
}

// checkNonAddCurrCodeOccurred (CheckNonAddCurrCodeOccurred): an amount of another currency than
// the additional one took part in the transaction (residuals are possible from then on)
func (p *GenJnlPostLine) checkNonAddCurrCodeOccurred(currencyCode types.Code) bool {
	p.addCurr.nonAddCurrCodeOccured = p.addCurr.nonAddCurrCodeOccured || p.acyCode() != currencyCode
	return p.addCurr.nonAddCurrCodeOccured
}

// handleAddCurrResidualGLEntry (HandleAddCurrResidualGLEntry): when the transaction's LCY
// amounts balance and its additional-currency amounts do not, the difference goes to the
// additional currency's residual gains or losses account (an entry of amount 0)
func (p *GenJnlPostLine) handleAddCurrResidualGLEntry() error {
	if p.acyCode() == "" {
		return nil
	}
	acy := &p.addCurr
	acy.totalAddCurrAmount = acy.totalAddCurrAmount.Add(p.glEntry.Additional_currency_amount)
	acy.totalAmount = acy.totalAmount.Add(p.glEntry.Amount)
	genJnlLine := p.genJnlLine
	if genJnlLine.Additional_currency_posting != gtables.GenJournalLine_Additional_currency_posting.None ||
		!acy.totalAmount.IsZero() || acy.totalAddCurrAmount.IsZero() || !p.checkNonAddCurrCodeOccurred(genJnlLine.Source_currency_code) {
		return nil
	}
	ts := i18n.GetInstance()
	language := fcodeunits.CurrentLanguage()
	origGLEntry := p.glEntry
	glEntry := gtables.NewGLEntry(p.src)
	glEntry.Init()
	glEntry.Posting_date = genJnlLine.Posting_date
	glEntry.Document_date = genJnlLine.Document_date
	glEntry.Document_type = gtables.GLEntryDocument_type(genJnlLine.Document_type)
	glEntry.Document_no = genJnlLine.Document_no
	glEntry.Description = types.Text(system.CopyStr(system.StrSubstNo(ts.Message("JNL_ACY_RESIDUAL", language),
		glEntry.FieldCaption("additional_currency_amount", language)), 1, glEntry.MaxStrLen("description")))
	glEntry.Business_unit_code = genJnlLine.Business_unit_code
	glEntry.Source_code = genJnlLine.Source_code
	glEntry.Journal_batch_name = genJnlLine.Journal_batch_name
	glEntry.Reason_code = genJnlLine.Reason_code
	glEntry.Entry_no = p.nextEntryNo
	glEntry.Transaction_no = p.nextTransactionNo
	if acy.totalAddCurrAmount.IsNegative() {
		glEntry.Gl_account_no = acy.currency.Residual_losses_account
	} else {
		glEntry.Gl_account_no = acy.currency.Residual_gains_account
	}
	glEntry.User_id = types.Code(system.UserId())
	glEntry.No_series = genJnlLine.Posting_no_series
	glEntry.System_created_entry = true
	glEntry.Prior_year_entry = glEntry.Posting_date.Before(p.fiscalYearStartDate)
	glEntry.Additional_currency_amount = acy.totalAddCurrAmount.Neg()
	p.glEntry = glEntry
	err := p.insertGLEntry(false)
	if err != nil {
		return err
	}
	p.glEntry = origGLEntry
	acy.totalAddCurrAmount = types.Decimal{}
	return nil
}

// calcAddCurrExchRateGainLossAmt (CalcAddCurrExchRateGainLossAmt): the line's exchange rate gains
// and losses of other currencies than the additional one, in the additional currency
func (p *GenJnlPostLine) calcAddCurrExchRateGainLossAmt() (types.Decimal, error) {
	var total types.Decimal
	found := false
	for _, currencyCode := range p.gainsLossesOrder {
		if currencyCode == p.acyCode() {
			continue
		}
		buffer := p.gainsLossesBuffer[currencyCode]
		total = total.Add(buffer.realGains).Add(buffer.realLosses).Add(buffer.unrealGains).Add(buffer.unrealLosses)
		found = true
	}
	if !found || p.acyCode() == "" {
		return types.Decimal{}, nil
	}
	return p.exchangeAmtLCYToAddCurr(total)
}

// pmtDiscAddCurr: a payment discount in the additional currency — the discount itself when the
// application is in that currency, else its LCY amount exchanged
func (p *GenJnlPostLine) pmtDiscAddCurr(applnCurrencyCode types.Code, pmtDisc, pmtDiscLCY types.Decimal) (types.Decimal, error) {
	if p.acyCode() != "" && applnCurrencyCode == p.acyCode() {
		return pmtDisc, nil
	}
	return p.calcLCYToAddCurr(pmtDiscLCY)
}
