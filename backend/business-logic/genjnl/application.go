package genjnl

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// gainsLosses is the gains-and-losses posting buffer of one currency (classic table 343): the
// exchange-rate differences of the applications of a line, posted to the currency's accounts
type gainsLosses struct {
	realGains, realLosses, unrealGains, unrealLosses types.Decimal
}

// resetApplicationTotals: a customer or vendor line starts its application totals
func (p *GenJnlPostLine) resetApplicationTotals() {
	p.totalPmtDiscLCY = types.Decimal{}
	p.totalApplnRoundingLCY = types.Decimal{}
	p.totalPmtDiscAddCurr = types.Decimal{}
	p.totalApplnRoundingAddCurr = types.Decimal{}
	p.realExchRateGains = types.Decimal{}
	p.realExchRateLosses = types.Decimal{}
	p.unrealExchRateGains = types.Decimal{}
	p.unrealExchRateLosses = types.Decimal{}
}

// applicationLCY is what the line's application posts elsewhere than on the receivables or
// payables account: exchange-rate gains and losses, payment discount, application rounding
func (p *GenJnlPostLine) applicationLCY() types.Decimal {
	return p.realExchRateGains.Add(p.realExchRateLosses).Add(p.unrealExchRateGains).Add(p.unrealExchRateLosses).
		Add(p.totalPmtDiscLCY).Add(p.totalApplnRoundingLCY)
}

// applicationAddCurr is applicationLCY in the additional currency (the exchange-rate gains and
// losses of the other currencies exchanged)
func (p *GenJnlPostLine) applicationAddCurr() (types.Decimal, error) {
	gainsLosses, err := p.calcAddCurrExchRateGainLossAmt()
	if err != nil {
		return types.Decimal{}, err
	}
	return gainsLosses.Add(p.totalPmtDiscAddCurr).Add(p.totalApplnRoundingAddCurr), nil
}

// postApplicationGLEntries (PostCust/PostVend after the ledger entry) posts the line's G/L
// entries on the receivables or payables account, the payment discount account and the
// application rounding account of the posting group
func (p *GenJnlPostLine) postApplicationGLEntries(genJnlLine *gtables.GenJournalLine, postingGroup interface {
	TestField(field string, value ...any) error
}, account, pmtDiscAccount, roundingAccount types.Code) error {
	addCurr, err := p.applicationAddCurr()
	if err != nil {
		return err
	}
	// the application register: what the application moved off the receivables or payables account
	p.recordApplicationGL(account, p.applicationLCY().Neg(), addCurr.Neg(), true)
	_, err = p.initGLEntry(account, genJnlLine.Amount_lcy.Sub(p.applicationLCY()), genJnlLine.Source_currency_amount.Sub(addCurr), true, true)
	if err != nil {
		return err
	}
	p.glEntry.Bal_account_type = gtables.GLEntryBal_account_type(genJnlLine.Bal_account_type)
	p.glEntry.Bal_account_no = genJnlLine.Bal_account_no
	err = p.insertGLEntry(true)
	if err != nil {
		return err
	}
	acy := p.acyCode() != ""
	// Post payment discount
	if !p.totalPmtDiscLCY.IsZero() || !p.totalPmtDiscAddCurr.IsZero() && acy {
		err = postingGroup.TestField("payment_disc_acc")
		if err != nil {
			return err
		}
		p.recordApplicationGL(pmtDiscAccount, p.totalPmtDiscLCY, p.totalPmtDiscAddCurr, true)
		_, err = p.initGLEntry(pmtDiscAccount, p.totalPmtDiscLCY, p.totalPmtDiscAddCurr, true, true)
		if err != nil {
			return err
		}
		err = p.insertGLEntry(true)
		if err != nil {
			return err
		}
	}
	// Post to Curr. Appln. Rounding Account
	if !p.totalApplnRoundingLCY.IsZero() || !p.totalApplnRoundingAddCurr.IsZero() && acy {
		err = postingGroup.TestField("curr_appln_rounding_account")
		if err != nil {
			return err
		}
		p.recordApplicationGL(roundingAccount, p.totalApplnRoundingLCY, p.totalApplnRoundingAddCurr, true)
		_, err = p.initGLEntry(roundingAccount, p.totalApplnRoundingLCY, p.totalApplnRoundingAddCurr, true, true)
		if err != nil {
			return err
		}
		return p.insertGLEntry(true)
	}
	return nil
}

// initGainsLosses: each line starts with an empty gains-and-losses buffer
func (p *GenJnlPostLine) initGainsLosses() {
	p.gainsLossesBuffer = map[types.Code]*gainsLosses{}
	p.gainsLossesOrder = nil
}

func (p *GenJnlPostLine) gainsLossesOf(currencyCode types.Code) *gainsLosses {
	buffer, ok := p.gainsLossesBuffer[currencyCode]
	if !ok {
		buffer = &gainsLosses{}
		p.gainsLossesBuffer[currencyCode] = buffer
		p.gainsLossesOrder = append(p.gainsLossesOrder, currencyCode)
	}
	return buffer
}

// addGainsLosses records an application's exchange-rate differences: the realized one in the
// application's currency, the unrealized one in the entry's
func (p *GenJnlPostLine) addGainsLosses(currencyCode, entryCurrencyCode types.Code, deltaRealAmount, deltaUnrealAmount types.Decimal) {
	buffer := p.gainsLossesOf(currencyCode)
	if deltaRealAmount.IsPositive() {
		p.realExchRateGains = p.realExchRateGains.Sub(deltaRealAmount)
		buffer.realGains = buffer.realGains.Sub(deltaRealAmount)
	} else {
		p.realExchRateLosses = p.realExchRateLosses.Sub(deltaRealAmount)
		buffer.realLosses = buffer.realLosses.Sub(deltaRealAmount)
	}
	buffer = p.gainsLossesOf(entryCurrencyCode)
	if deltaUnrealAmount.IsNegative() {
		p.unrealExchRateGains = p.unrealExchRateGains.Sub(deltaUnrealAmount)
		buffer.unrealGains = buffer.unrealGains.Sub(deltaUnrealAmount)
	} else {
		p.unrealExchRateLosses = p.unrealExchRateLosses.Sub(deltaUnrealAmount)
		buffer.unrealLosses = buffer.unrealLosses.Sub(deltaUnrealAmount)
	}
}

// postGainsLosses posts the line's exchange-rate gains and losses per currency to the
// currency's gains and losses accounts — all as unrealized when a currency uses the same
// accounts for both (PostCurrencyGainsLosses)
func (p *GenJnlPostLine) postGainsLosses() error {
	for _, currencyCode := range p.gainsLossesOrder {
		buffer := p.gainsLossesBuffer[currencyCode]
		if buffer.realGains.IsZero() && buffer.realLosses.IsZero() && buffer.unrealGains.IsZero() && buffer.unrealLosses.IsZero() {
			continue
		}
		currency := gtables.NewCurrency(p.src)
		err := currency.GetOrError(currencyCode)
		if err != nil {
			return err
		}
		if currency.Realized_gains_acc == currency.Unrealized_gains_acc && currency.Realized_losses_acc == currency.Unrealized_losses_acc {
			buffer.unrealGains = buffer.unrealGains.Add(buffer.realGains)
			buffer.unrealLosses = buffer.unrealLosses.Add(buffer.realLosses)
			buffer.realGains = types.Decimal{}
			buffer.realLosses = types.Decimal{}
		}
		isAddCurr := currency.Code == p.acyCode()
		postings := []struct {
			amount  types.Decimal
			field   string
			account types.Code
		}{
			{buffer.unrealGains, "unrealized_gains_acc", currency.Unrealized_gains_acc},
			{buffer.unrealLosses, "unrealized_losses_acc", currency.Unrealized_losses_acc},
			{buffer.realGains, "realized_gains_acc", currency.Realized_gains_acc},
			{buffer.realLosses, "realized_losses_acc", currency.Realized_losses_acc},
		}
		for _, posting := range postings {
			if posting.amount.IsZero() {
				continue
			}
			p.checkNonAddCurrCodeOccurred(currency.Code)
			err = currency.TestField(posting.field)
			if err != nil {
				return err
			}
			p.recordApplicationGL(posting.account, posting.amount, types.Decimal{}, isAddCurr)
			_, err = p.initGLEntry(posting.account, posting.amount, types.Decimal{}, isAddCurr, true)
			if err != nil {
				return err
			}
			err = p.insertGLEntry(true)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// testPostingType: a customer line cannot post a purchase, a vendor line not a sale
func (p *GenJnlPostLine) testPostingType() error {
	switch {
	case p.custPosting && p.purchPostingType:
		return apperrors.JnlCustPurchaseType()
	case p.vendPosting && p.salesPostingType:
		return apperrors.JnlVendSalesType()
	}
	return nil
}

// newTransaction: a new transaction starts without customer, vendor or sales/purchase postings
func (p *GenJnlPostLine) newTransaction() {
	p.custPosting = false
	p.vendPosting = false
	p.salesPostingType = false
	p.purchPostingType = false
	p.firstNewTaxEntryNo = p.nextTaxEntryNo
}

// applnRoundingPrecision is the application rounding precision of a currency (the local
// currency's from the General Ledger Setup)
func (p *GenJnlPostLine) applnRoundingPrecision(currencyCode types.Code) (types.Decimal, error) {
	if currencyCode == "" {
		return p.glSetup.Appln_rounding_precision, nil
	}
	currency3 := gtables.NewCurrency(p.src)
	err := currency3.GetOrError(currencyCode)
	if err != nil {
		return types.Decimal{}, err
	}
	return currency3.Appln_rounding_precision, nil
}

// CheckCurrency (codeunit 225 CheckCurrency): may an entry in compareCurrencyCode be applied in
// applnCurrencyCode — the same currency always; others as the setup's Appln. between Currencies
// allows (Sales & Receivables Setup for customers, Purchases & Payables Setup for vendors): None
// no, EMU between EMU currencies (the local currency counts as one when the General Ledger Setup
// says it is an EMU currency), All any. With message the refusal is an error, else false.
func CheckCurrency(src ftables.Source, applnCurrencyCode, compareCurrencyCode types.Code, accType gtables.GenJournalLineAccount_type, message bool) (bool, error) {
	if applnCurrencyCode == compareCurrencyCode {
		return true, nil
	}
	var currencyAppln int
	switch accType {
	case gtables.GenJournalLine_Account_type.Customer:
		salesSetup := gtables.GetSalesReceivablesSetup(src)
		currencyAppln = int(salesSetup.Appln_between_currencies)
	case gtables.GenJournalLine_Account_type.Vendor:
		purchSetup := gtables.GetPurchasesPayablesSetup(src)
		currencyAppln = int(purchSetup.Appln_between_currencies)
	default:
		return true, nil
	}
	switch currencyAppln {
	case int(gtables.SalesReceivablesSetup_Appln_between_currencies.None):
		if message {
			return false, apperrors.JnlApplnSameCurrency()
		}
		return false, nil
	case int(gtables.SalesReceivablesSetup_Appln_between_currencies.EMU):
		glSetup := gtables.GetGeneralLedgerSetup(src)
		currency := gtables.NewCurrency(src)
		currencyEMU := glSetup.Emu_currency
		if currency.Get(applnCurrencyCode) {
			currencyEMU = currency.Emu_currency
		}
		currency2 := gtables.NewCurrency(src)
		currency2EMU := glSetup.Emu_currency
		if currency2.Get(compareCurrencyCode) {
			currency2EMU = currency2.Emu_currency
		}
		if !currencyEMU || !currency2EMU {
			if message {
				return false, apperrors.JnlApplnSameCurrencyOrEMU()
			}
			return false, nil
		}
	}
	return true, nil
}

// ExchAmount (ExchAmount): an amount in one currency converted to another on a date, rounded to
// the currency's precision (blank: the local currency)
func ExchAmount(src ftables.Source, amount types.Decimal, fromCurrencyCode, toCurrencyCode types.Code, usePostingDate types.Date) (types.Decimal, error) {
	if fromCurrencyCode == toCurrencyCode || amount.IsZero() {
		return amount, nil
	}
	amount, err := tables.ExchangeRates(src).ExchangeAmtFCYToFCY(usePostingDate, fromCurrencyCode, toCurrencyCode, amount)
	if err != nil {
		return types.Decimal{}, err
	}
	if toCurrencyCode != "" {
		toCurrency := gtables.NewCurrency(src)
		err = toCurrency.GetOrError(toCurrencyCode)
		if err != nil {
			return types.Decimal{}, err
		}
		return system.Round(amount, toCurrency.Amount_rounding_precision), nil
	}
	return system.Round(amount), nil
}

func itoa(n int) string { return strconv.Itoa(n) }
