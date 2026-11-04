package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 595, Adjust G/L Accounts for Add.-Reporting Currency: the G/L accounts whose Exchange
// Rate Adjustment says so get their LCY balance brought to the additional-currency balance at the
// adjustment rate (Adjust Amount: an LCY-only line) or the other way round (Adjust
// Additional-Currency Amount: a line of the additional-currency amount only); the open tax
// entries of the period are adjusted the same way per the General Ledger Setup's Tax Exchange
// Rate Adjustment, with their tax accounts. The totals go to the additional currency's Realized
// G/L Gains/Losses Account, with a register of account type G/L Account.

// addCurrAdjustment is the state of the G/L account adjustment
type addCurrAdjustment struct {
	glSetup                                   *gtables.GeneralLedgerSetup
	currency3                                 *gtables.Currency
	addCurrCurrencyFactor                     types.Decimal
	glAmtTotal, glAddCurrAmtTotal             types.Decimal
	glAddCurrBalAtDateTotal, glBalAtDateTotal types.Decimal
	glBalAtDateBase, glAddCurrBalAtDateBase   types.Decimal
	taxEntry2, taxEntryTotalBase              taxEntryAmounts
	taxEntry                                  *gtables.TaxEntry
	taxJurisdiction                           *gtables.TaxJurisdiction
	run                                       *Run
}

// taxEntryAmounts are the amounts of tax entries added up (classic TaxEntry2, TaxEntryTotalBase)
type taxEntryAmounts struct {
	base, amount, unrealizedAmount, unrealizedBase, remUnrealAmount, remUnrealBase                                types.Decimal
	addCurrBase, addCurrAmount, addCurrUnrealAmount, addCurrUnrealBase, addCurrRemUnrealAmt, addCurrRemUnrealBase types.Decimal
}

// accumulate adds a tax entry's amounts (sign -1: subtracts them)
func (t *taxEntryAmounts) accumulate(e *gtables.TaxEntry, sign int64) {
	s := types.NewDecimalFromInt(sign)
	t.base = t.base.Add(e.Base.Mul(s))
	t.amount = t.amount.Add(e.Amount.Mul(s))
	t.unrealizedAmount = t.unrealizedAmount.Add(e.Unrealized_amount.Mul(s))
	t.unrealizedBase = t.unrealizedBase.Add(e.Unrealized_base.Mul(s))
	t.remUnrealAmount = t.remUnrealAmount.Add(e.Remaining_unrealized_amount.Mul(s))
	t.remUnrealBase = t.remUnrealBase.Add(e.Remaining_unrealized_base.Mul(s))
	t.addCurrBase = t.addCurrBase.Add(e.Additional_currency_base.Mul(s))
	t.addCurrAmount = t.addCurrAmount.Add(e.Additional_currency_amount.Mul(s))
	t.addCurrUnrealAmount = t.addCurrUnrealAmount.Add(e.Add_currency_unrealized_amt.Mul(s))
	t.addCurrUnrealBase = t.addCurrUnrealBase.Add(e.Add_currency_unrealized_base.Mul(s))
	t.addCurrRemUnrealAmt = t.addCurrRemUnrealAmt.Add(e.Add_curr_rem_unreal_amount.Mul(s))
	t.addCurrRemUnrealBase = t.addCurrRemUnrealBase.Add(e.Add_curr_rem_unreal_base.Mul(s))
}

// checkAddCurrAdjustment (OnPreReport, Adjust G/L Accounts): an additional reporting currency
// whose realized G/L gains and losses accounts are not adjusted themselves, nor the tax accounts
// (their adjustment follows the setup's Tax Exchange Rate Adjustment); the adjustment rate of the
// additional currency
func (r *adjustExchangeRates) checkAddCurrAdjustment(run *Run) error {
	a := &r.addCurr
	a.run = run
	a.glSetup = gtables.GetGeneralLedgerSetup(run)
	err := a.glSetup.TestField("additional_reporting_currency")
	if err != nil {
		return err
	}
	a.currency3 = gtables.NewCurrency(run)
	err = a.currency3.GetOrError(a.glSetup.Additional_reporting_currency)
	if err != nil {
		return err
	}
	glAcc := gtables.NewGLAccount(run)
	err = a.currency3.TestField("realized_gl_gains_account")
	if err != nil {
		return err
	}
	err = glAcc.GetOrError(a.currency3.Realized_gl_gains_account)
	if err != nil {
		return err
	}
	err = glAcc.TestField("exchange_rate_adjustment", gtables.GLAccount_Exchange_rate_adjustment.NoAdjustment)
	if err != nil {
		return err
	}
	err = a.currency3.TestField("realized_gl_losses_account")
	if err != nil {
		return err
	}
	err = glAcc.GetOrError(a.currency3.Realized_gl_losses_account)
	if err != nil {
		return err
	}
	err = glAcc.TestField("exchange_rate_adjustment", gtables.GLAccount_Exchange_rate_adjustment.NoAdjustment)
	if err != nil {
		return err
	}
	taxPostingSetup2 := gtables.NewTaxPostingSetup(run)
	for found := taxPostingSetup2.FindSet(); found; found = taxPostingSetup2.Next() {
		if taxPostingSetup2.Tax_calculation_type != gtables.TaxPostingSetup_Tax_calculation_type.SalesTax {
			accounts := map[string]types.Code{
				"purchase_vat_account":        taxPostingSetup2.Purchase_vat_account,
				"reverse_chrg_vat_acc":        taxPostingSetup2.Reverse_chrg_vat_acc,
				"purch_vat_unreal_account":    taxPostingSetup2.Purch_vat_unreal_account,
				"reverse_chrg_vat_unreal_acc": taxPostingSetup2.Reverse_chrg_vat_unreal_acc,
				"sales_vat_account":           taxPostingSetup2.Sales_vat_account,
				"sales_vat_unreal_account":    taxPostingSetup2.Sales_vat_unreal_account,
			}
			for _, field := range []string{"purchase_vat_account", "reverse_chrg_vat_acc", "purch_vat_unreal_account",
				"reverse_chrg_vat_unreal_acc", "sales_vat_account", "sales_vat_unreal_account"} {
				err = r.checkExchRateAdjustment(run, accounts[field], taxPostingSetup2.TableCaption(), taxPostingSetup2.FieldCaption(field))
				if err != nil {
					return err
				}
			}
		}
	}
	taxJurisdiction2 := gtables.NewTaxJurisdiction(run)
	for found := taxJurisdiction2.FindSet(); found; found = taxJurisdiction2.Next() {
		accounts := map[string]types.Code{
			"tax_account_purchases":    taxJurisdiction2.Tax_account_purchases,
			"reverse_charge_purchases": taxJurisdiction2.Reverse_charge_purchases,
			"unreal_tax_acc_purchases": taxJurisdiction2.Unreal_tax_acc_purchases,
			"unreal_rev_charge_purch":  taxJurisdiction2.Unreal_rev_charge_purch,
			"tax_account_sales":        taxJurisdiction2.Tax_account_sales,
			"unreal_tax_acc_sales":     taxJurisdiction2.Unreal_tax_acc_sales,
		}
		for _, field := range []string{"tax_account_purchases", "reverse_charge_purchases", "unreal_tax_acc_purchases",
			"unreal_rev_charge_purch", "tax_account_sales", "unreal_tax_acc_sales"} {
			err = r.checkExchRateAdjustment(run, accounts[field], taxJurisdiction2.TableCaption(), taxJurisdiction2.FieldCaption(field))
			if err != nil {
				return err
			}
		}
	}
	a.addCurrCurrencyFactor, err = tables.ExchangeRates(run).ExchangeRateAdjmt(r.postingDate, a.glSetup.Additional_reporting_currency)
	return err
}

// checkExchRateAdjustment (CheckExchRateAdjustment): an account of the tax setup is not adjusted
// by its own Exchange Rate Adjustment
func (r *adjustExchangeRates) checkExchRateAdjustment(run *Run, accNo types.Code, setupTableName, setupFieldName string) error {
	if accNo == "" {
		return nil
	}
	glAcc := gtables.NewGLAccount(run)
	err := glAcc.GetOrError(accNo)
	if err != nil {
		return err
	}
	if glAcc.Exchange_rate_adjustment != gtables.GLAccount_Exchange_rate_adjustment.NoAdjustment {
		glAcc.Exchange_rate_adjustment = gtables.GLAccount_Exchange_rate_adjustment.NoAdjustment
		language := fcodeunits.CurrentLanguage()
		glSetup := r.addCurr.glSetup
		return system.Error(run.Text("EXCH_RATE_ADJMT_ACCOUNT",
			glAcc.FieldCaption("exchange_rate_adjustment"), glAcc.TableCaption(), glAcc.No,
			glAcc.GetOptionCaption("exchange_rate_adjustment", i18n.Key(glAcc.Exchange_rate_adjustment.String()), language),
			setupTableName, glSetup.FieldCaption("tax_exchange_rate_adjustment"), glSetup.TableCaption(), setupFieldName))
	}
	return nil
}

// taxPostingSetupOnPreDataItem: only with Adjust G/L Accounts and a Tax Exchange Rate Adjustment;
// the open tax entries of the period
func (r *adjustExchangeRates) taxPostingSetupOnPreDataItem(setup *gtables.TaxPostingSetup, run *Run) error {
	a := &r.addCurr
	if !r.adjGLAcc || a.glSetup.Tax_exchange_rate_adjustment == gtables.GeneralLedgerSetup_Tax_exchange_rate_adjustment.NoAdjustment {
		return Break()
	}
	a.taxEntry = gtables.NewTaxEntry(run)
	a.taxEntry.SetCurrentKey("type", "closed", "tax_bus_posting_group", "tax_prod_posting_group", "tax_jurisdiction_code", "use_tax", "posting_date")
	a.taxEntry.SetRange("closed", false)
	a.taxEntry.SetRange("posting_date", r.startDate, r.endDate)
	a.taxJurisdiction = gtables.NewTaxJurisdiction(run)
	return nil
}

// taxPostingSetupOnAfterGetRecord: the tax entries of the posting groups — VAT types by the
// setup's accounts, sales tax per jurisdiction — and their accounts
func (r *adjustExchangeRates) taxPostingSetupOnAfterGetRecord(setup *gtables.TaxPostingSetup, run *Run) error {
	a := &r.addCurr
	taxEntry := a.taxEntry
	taxEntry.SetRange("tax_bus_posting_group", setup.Tax_bus_posting_group)
	taxEntry.SetRange("tax_prod_posting_group", setup.Tax_prod_posting_group)
	if setup.Tax_calculation_type != gtables.TaxPostingSetup_Tax_calculation_type.SalesTax {
		taxEntry.SetRange("tax_jurisdiction_code")
		err := r.adjustTaxEntries(gtables.TaxEntry_Type.Purchase, false)
		if err != nil {
			return err
		}
		reverseCharge := setup.Tax_calculation_type == gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT
		err = r.adjustVATAccounts(setup, "purchase_vat_account", setup.Purchase_vat_account, "purch_vat_unreal_account", setup.Purch_vat_unreal_account, reverseCharge)
		if err != nil {
			return err
		}
		err = r.adjustTaxEntries(gtables.TaxEntry_Type.Sale, false)
		if err != nil {
			return err
		}
		return r.adjustVATAccounts(setup, "sales_vat_account", setup.Sales_vat_account, "sales_vat_unreal_account", setup.Sales_vat_unreal_account, false)
	}
	for found := a.taxJurisdiction.FindSet(); found; found = a.taxJurisdiction.Next() {
		taxEntry.SetRange("tax_jurisdiction_code", a.taxJurisdiction.Code)
		err := r.adjustTaxEntries(gtables.TaxEntry_Type.Purchase, false)
		if err != nil {
			return err
		}
		err = r.adjustPurchTax(false)
		if err != nil {
			return err
		}
		err = r.adjustTaxEntries(gtables.TaxEntry_Type.Purchase, true)
		if err != nil {
			return err
		}
		err = r.adjustPurchTax(true)
		if err != nil {
			return err
		}
		err = r.adjustTaxEntries(gtables.TaxEntry_Type.Sale, false)
		if err != nil {
			return err
		}
		err = r.adjustSalesTax()
		if err != nil {
			return err
		}
	}
	return nil
}

// adjustVATAccounts: the tax account and the unrealized tax account of a VAT posting setup (the
// reverse charge accounts the other way)
func (r *adjustExchangeRates) adjustVATAccounts(setup *gtables.TaxPostingSetup, accountField string, account types.Code, unrealAccountField string, unrealAccount types.Code, reverseCharge bool) error {
	t := r.addCurr.taxEntry2
	total := r.addCurr.taxEntryTotalBase
	if !t.amount.IsZero() || !t.addCurrAmount.IsZero() {
		err := setup.TestField(accountField)
		if err != nil {
			return err
		}
		err = r.adjustTaxAccount(account, t.amount, t.addCurrAmount, total.amount, total.addCurrAmount)
		if err != nil {
			return err
		}
		if reverseCharge {
			err = setup.TestField("reverse_chrg_vat_acc")
			if err != nil {
				return err
			}
			err = r.adjustTaxAccount(setup.Reverse_chrg_vat_acc, t.amount.Neg(), t.addCurrAmount.Neg(), total.amount.Neg(), total.addCurrAmount.Neg())
			if err != nil {
				return err
			}
		}
	}
	if !t.remUnrealAmount.IsZero() || !t.addCurrRemUnrealAmt.IsZero() {
		err := setup.TestField("unrealized_tax_type")
		if err != nil {
			return err
		}
		err = setup.TestField(unrealAccountField)
		if err != nil {
			return err
		}
		err = r.adjustTaxAccount(unrealAccount, t.remUnrealAmount, t.addCurrRemUnrealAmt, total.remUnrealAmount, total.addCurrRemUnrealAmt)
		if err != nil {
			return err
		}
		if reverseCharge {
			err = setup.TestField("reverse_chrg_vat_unreal_acc")
			if err != nil {
				return err
			}
			err = r.adjustTaxAccount(setup.Reverse_chrg_vat_unreal_acc, t.remUnrealAmount.Neg(), t.addCurrRemUnrealAmt.Neg(), total.remUnrealAmount.Neg(), total.addCurrRemUnrealAmt.Neg())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// adjustTaxEntries (AdjustTaxEntries): the tax entries of a type and use tax get their LCY or
// additional-currency amounts at the adjustment rate; taxEntry2 holds the change reversed
func (r *adjustExchangeRates) adjustTaxEntries(taxType gtables.TaxEntryType, useTax bool) error {
	a := &r.addCurr
	a.taxEntry2 = taxEntryAmounts{}
	taxEntry := a.taxEntry
	taxEntry.SetRange("type", taxType)
	taxEntry.SetRange("use_tax", useTax)
	for found := taxEntry.FindSet(); found; found = taxEntry.Next() {
		a.taxEntry2.accumulate(taxEntry, 1)
		a.taxEntryTotalBase.accumulate(taxEntry, 1)
		for _, pair := range [][2]*types.Decimal{
			{&taxEntry.Base, &taxEntry.Additional_currency_base},
			{&taxEntry.Amount, &taxEntry.Additional_currency_amount},
			{&taxEntry.Unrealized_amount, &taxEntry.Add_currency_unrealized_amt},
			{&taxEntry.Unrealized_base, &taxEntry.Add_currency_unrealized_base},
			{&taxEntry.Remaining_unrealized_amount, &taxEntry.Add_curr_rem_unreal_amount},
			{&taxEntry.Remaining_unrealized_base, &taxEntry.Add_curr_rem_unreal_base},
		} {
			err := r.adjustTaxAmount(pair[0], pair[1])
			if err != nil {
				return err
			}
		}
		err := taxEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
		a.taxEntry2.accumulate(taxEntry, -1)
	}
	return nil
}

// adjustTaxAmount (AdjustTaxAmount): the LCY amount from the additional-currency amount, or the
// additional-currency amount from the LCY amount, at the adjustment rate
func (r *adjustExchangeRates) adjustTaxAmount(amountLCY, amountAddCurr *types.Decimal) error {
	a := &r.addCurr
	rates := tables.ExchangeRates(a.run)
	switch a.glSetup.Tax_exchange_rate_adjustment {
	case gtables.GeneralLedgerSetup_Tax_exchange_rate_adjustment.AdjustAmount:
		amount, err := rates.ExchangeAmtFCYToLCYAdjmt(r.postingDate, a.glSetup.Additional_reporting_currency, *amountAddCurr, a.addCurrCurrencyFactor)
		if err != nil {
			return err
		}
		*amountLCY = system.Round(amount)
	case gtables.GeneralLedgerSetup_Tax_exchange_rate_adjustment.AdjustAdditionalCurrencyAmount:
		amount, err := rates.ExchangeAmtLCYToFCY(r.postingDate, a.glSetup.Additional_reporting_currency, *amountLCY, a.addCurrCurrencyFactor)
		if err != nil {
			return err
		}
		*amountAddCurr = system.Round(amount)
	}
	return nil
}

// adjustTaxAccount (AdjustTaxAccount): a tax account's line of the change
func (r *adjustExchangeRates) adjustTaxAccount(accNo types.Code, amountLCY, amountAddCurr, baseLCY, baseAddCurr types.Decimal) error {
	switch r.addCurr.glSetup.Tax_exchange_rate_adjustment {
	case gtables.GeneralLedgerSetup_Tax_exchange_rate_adjustment.AdjustAmount:
		return r.postGLAccAdjmt(accNo, gtables.GLAccount_Exchange_rate_adjustment.AdjustAmount, amountLCY.Neg(), baseLCY, baseAddCurr)
	case gtables.GeneralLedgerSetup_Tax_exchange_rate_adjustment.AdjustAdditionalCurrencyAmount:
		return r.postGLAccAdjmt(accNo, gtables.GLAccount_Exchange_rate_adjustment.AdjustAdditionalCurrencyAmount, amountAddCurr.Neg(), baseLCY, baseAddCurr)
	}
	return nil
}

// adjustPurchTax (AdjustPurchTax): the jurisdiction's purchase tax accounts (use tax: the reverse
// charge accounts the other way)
func (r *adjustExchangeRates) adjustPurchTax(useTax bool) error {
	a := &r.addCurr
	t := a.taxEntry2
	total := a.taxEntryTotalBase
	taxJurisdiction := a.taxJurisdiction
	if !t.amount.IsZero() || !t.addCurrAmount.IsZero() {
		err := taxJurisdiction.TestField("tax_account_purchases")
		if err != nil {
			return err
		}
		err = r.adjustTaxAccount(taxJurisdiction.Tax_account_purchases, t.amount, t.addCurrAmount, total.amount, total.addCurrAmount)
		if err != nil {
			return err
		}
		if useTax {
			err = taxJurisdiction.TestField("reverse_charge_purchases")
			if err != nil {
				return err
			}
			err = r.adjustTaxAccount(taxJurisdiction.Reverse_charge_purchases, t.amount.Neg(), t.addCurrAmount.Neg(), total.amount.Neg(), total.addCurrAmount.Neg())
			if err != nil {
				return err
			}
		}
	}
	if !t.remUnrealAmount.IsZero() || !t.addCurrRemUnrealAmt.IsZero() {
		err := taxJurisdiction.TestField("unrealized_tax_type")
		if err != nil {
			return err
		}
		err = taxJurisdiction.TestField("unreal_tax_acc_purchases")
		if err != nil {
			return err
		}
		err = r.adjustTaxAccount(taxJurisdiction.Unreal_tax_acc_purchases, t.remUnrealAmount, t.addCurrRemUnrealAmt, total.remUnrealAmount, t.addCurrRemUnrealAmt)
		if err != nil {
			return err
		}
		if useTax {
			err = taxJurisdiction.TestField("unreal_rev_charge_purch")
			if err != nil {
				return err
			}
			err = r.adjustTaxAccount(taxJurisdiction.Unreal_rev_charge_purch, t.remUnrealAmount.Neg(), t.addCurrRemUnrealAmt.Neg(), total.remUnrealAmount.Neg(), total.addCurrRemUnrealAmt.Neg())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// adjustSalesTax (AdjustSalesTax): the jurisdiction's sales tax accounts
func (r *adjustExchangeRates) adjustSalesTax() error {
	a := &r.addCurr
	t := a.taxEntry2
	total := a.taxEntryTotalBase
	taxJurisdiction := a.taxJurisdiction
	err := taxJurisdiction.TestField("tax_account_sales")
	if err != nil {
		return err
	}
	err = r.adjustTaxAccount(taxJurisdiction.Tax_account_sales, t.amount, t.addCurrAmount, total.amount, total.addCurrAmount)
	if err != nil {
		return err
	}
	if !t.remUnrealAmount.IsZero() || !t.addCurrRemUnrealAmt.IsZero() {
		err = taxJurisdiction.TestField("unrealized_tax_type")
		if err != nil {
			return err
		}
		err = taxJurisdiction.TestField("unreal_tax_acc_sales")
		if err != nil {
			return err
		}
		err = r.adjustTaxAccount(taxJurisdiction.Unreal_tax_acc_sales, t.remUnrealAmount, t.addCurrRemUnrealAmt, total.remUnrealAmount, total.addCurrRemUnrealAmt)
		if err != nil {
			return err
		}
	}
	return nil
}

// glAccOnPreDataItem: only with Adjust G/L Accounts; balances up to the period's end
func (r *adjustExchangeRates) glAccOnPreDataItem(glAcc *gtables.GLAccount, run *Run) error {
	if !r.adjGLAcc {
		return Break()
	}
	return glAcc.SetFlowFilter("date_filter", r.startDate.String()+".."+r.endDate.String())
}

// glAccOnAfterGetRecord: Adjust Amount brings the balance to the additional-currency balance at
// the adjustment rate, Adjust Additional-Currency Amount the additional-currency balance to the
// balance
func (r *adjustExchangeRates) glAccOnAfterGetRecord(glAcc *gtables.GLAccount, run *Run) error {
	a := &r.addCurr
	if glAcc.Exchange_rate_adjustment == gtables.GLAccount_Exchange_rate_adjustment.NoAdjustment {
		return Skip()
	}
	glAcc.CalcFields("balance_at_date", "add_currency_balance_at_date")
	rates := tables.ExchangeRates(run)
	switch glAcc.Exchange_rate_adjustment {
	case gtables.GLAccount_Exchange_rate_adjustment.AdjustAmount:
		amount, err := rates.ExchangeAmtFCYToLCYAdjmt(r.postingDate, a.glSetup.Additional_reporting_currency, glAcc.Add_currency_balance_at_date, a.addCurrCurrencyFactor)
		if err != nil {
			return err
		}
		return r.postGLAccAdjmt(glAcc.No, gtables.GLAccount_Exchange_rate_adjustment.AdjustAmount,
			system.Round(amount.Sub(glAcc.Balance_at_date)), glAcc.Balance_at_date, glAcc.Add_currency_balance_at_date)
	case gtables.GLAccount_Exchange_rate_adjustment.AdjustAdditionalCurrencyAmount:
		amount, err := rates.ExchangeAmtLCYToFCY(r.postingDate, a.glSetup.Additional_reporting_currency, glAcc.Balance_at_date, a.addCurrCurrencyFactor)
		if err != nil {
			return err
		}
		return r.postGLAccAdjmt(glAcc.No, gtables.GLAccount_Exchange_rate_adjustment.AdjustAdditionalCurrencyAmount,
			system.Round(amount.Sub(glAcc.Add_currency_balance_at_date), a.currency3.Amount_rounding_precision), glAcc.Balance_at_date, glAcc.Add_currency_balance_at_date)
	}
	return nil
}

// glAccOnPostDataItem: the LCY total and the additional-currency total to the additional
// currency's realized G/L gains or losses account, and the register
func (r *adjustExchangeRates) glAccOnPostDataItem(glAcc *gtables.GLAccount, run *Run) error {
	a := &r.addCurr
	if !a.glAmtTotal.IsZero() {
		accNo := a.currency3.Realized_gl_gains_account
		if a.glAmtTotal.IsPositive() {
			accNo = a.currency3.Realized_gl_losses_account
		}
		description := system.StrSubstNo(r.postingDescription, a.glSetup.Additional_reporting_currency, a.glAddCurrBalAtDateTotal)
		err := r.postGLAccAdjmtLine(accNo, gtables.GenJournalLine_Additional_currency_posting.AmountOnly, "", a.glAmtTotal.Neg(), description)
		if err != nil {
			return err
		}
	}
	if !a.glAddCurrAmtTotal.IsZero() {
		accNo := a.currency3.Realized_gl_gains_account
		if a.glAddCurrAmtTotal.IsPositive() {
			accNo = a.currency3.Realized_gl_losses_account
		}
		description := system.StrSubstNo(r.postingDescription, "", a.glBalAtDateTotal)
		err := r.postGLAccAdjmtLine(accNo, gtables.GenJournalLine_Additional_currency_posting.AdditionalCurrencyAmountOnly,
			a.glSetup.Additional_reporting_currency, a.glAddCurrAmtTotal.Neg(), description)
		if err != nil {
			return err
		}
	}
	r.exchRateAdjRegNo++
	reg := gtables.NewExchRateAdjmtReg(run)
	reg.No = r.exchRateAdjRegNo
	reg.Creation_date = r.postingDate
	reg.Account_type = gtables.ExchRateAdjmtReg_Account_type.GLAccount
	reg.Currency_code = a.glSetup.Additional_reporting_currency
	reg.Currency_factor = a.addCurrCurrencyFactor
	reg.Adjusted_base_lcy = a.glBalAtDateBase
	reg.Adjusted_amt_lcy = a.glAmtTotal
	reg.Adjusted_base_add_curr = a.glAddCurrBalAtDateBase
	reg.Adjusted_amt_add_curr = a.glAddCurrAmtTotal
	return reg.InsertOrError(false)
}

// postGLAccAdjmt (PostGLAccAdjmt): an account's adjustment — an LCY-only line (Adjust Amount) or
// a line of the additional-currency amount only — added to the totals
func (r *adjustExchangeRates) postGLAccAdjmt(glAccNo types.Code, exchRateAdjmt gtables.GLAccountExchange_rate_adjustment, amount, balanceAtDate, addCurrBalanceAtDate types.Decimal) error {
	a := &r.addCurr
	switch exchRateAdjmt {
	case gtables.GLAccount_Exchange_rate_adjustment.AdjustAmount:
		a.glAmtTotal = a.glAmtTotal.Add(amount)
		a.glAddCurrBalAtDateTotal = a.glAddCurrBalAtDateTotal.Add(addCurrBalanceAtDate)
		a.glBalAtDateBase = a.glBalAtDateBase.Add(balanceAtDate)
		if amount.IsZero() {
			return nil
		}
		description := system.StrSubstNo(r.postingDescription, a.glSetup.Additional_reporting_currency, addCurrBalanceAtDate)
		return r.postGLAccAdjmtLine(glAccNo, gtables.GenJournalLine_Additional_currency_posting.AmountOnly, "", amount, description)
	case gtables.GLAccount_Exchange_rate_adjustment.AdjustAdditionalCurrencyAmount:
		a.glAddCurrAmtTotal = a.glAddCurrAmtTotal.Add(amount)
		a.glBalAtDateTotal = a.glBalAtDateTotal.Add(balanceAtDate)
		a.glAddCurrBalAtDateBase = a.glAddCurrBalAtDateBase.Add(addCurrBalanceAtDate)
		if amount.IsZero() {
			return nil
		}
		description := system.StrSubstNo(r.postingDescription, "", balanceAtDate)
		return r.postGLAccAdjmtLine(glAccNo, gtables.GenJournalLine_Additional_currency_posting.AdditionalCurrencyAmountOnly,
			a.glSetup.Additional_reporting_currency, amount, description)
	}
	return nil
}

// postGLAccAdjmtLine posts a system-created line of one of the two amounts: Amount Only in LCY,
// Additional-Currency Amount Only in the additional currency (Amount (LCY) 0)
func (r *adjustExchangeRates) postGLAccAdjmtLine(glAccNo types.Code, addCurrPosting gtables.GenJournalLineAdditional_currency_posting, currencyCode types.Code, amount types.Decimal, description string) error {
	genJnlLine := gtables.NewGenJournalLine(r.addCurr.run)
	genJnlLine.Init()
	genJnlLine.Additional_currency_posting = addCurrPosting
	genJnlLine.Currency_code = currencyCode
	genJnlLine.Amount = amount
	if addCurrPosting == gtables.GenJournalLine_Additional_currency_posting.AmountOnly {
		genJnlLine.Amount_lcy = amount
	}
	genJnlLine.Document_no = r.postingDocNo
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.GLAccount
	genJnlLine.Account_no = glAccNo
	genJnlLine.Posting_date = r.postingDate
	genJnlLine.Description = types.Text(system.CopyStr(description, 1, genJnlLine.MaxStrLen("description")))
	genJnlLine.System_created_entry = true
	genJnlLine.Source_code = r.sourceCode
	return r.genJnlPostLine.Run(genJnlLine)
}
