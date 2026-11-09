package genjnl

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// postCust posts a customer line: the customer ledger entry (applied to the customer's open
// entries), the G/L entry on the posting group's receivables account, and the payment discount
// and application rounding the application gave
func (p *GenJnlPostLine) postCust(genJnlLine *gtables.GenJournalLine) error {
	cust := gtables.NewCustomer(p.src)
	err := cust.GetOrError(genJnlLine.Account_no)
	if err != nil {
		return err
	}
	err = cust.TestField("blocked", false)
	if err != nil {
		return err
	}
	if genJnlLine.Posting_group == "" {
		err = cust.TestField("customer_posting_group")
		if err != nil {
			return err
		}
		genJnlLine.Posting_group = cust.Customer_posting_group
	}
	custPostingGr := gtables.NewCustomerPostingGroup(p.src)
	err = custPostingGr.GetOrError(genJnlLine.Posting_group)
	if err != nil {
		return err
	}
	err = custPostingGr.TestField("receivables_account")
	if err != nil {
		return err
	}
	p.custPosting = true
	err = p.testPostingType()
	if err != nil {
		return err
	}
	custLedgEntry := gtables.NewCustomerLedgerEntry(p.src)
	custLedgEntry.LockTable()
	custLedgEntry.Init()
	custLedgEntry.Customer_no = genJnlLine.Account_no
	custLedgEntry.Posting_date = genJnlLine.Posting_date
	custLedgEntry.Document_date = genJnlLine.Document_date
	custLedgEntry.Document_type = gtables.CustomerLedgerEntryDocument_type(genJnlLine.Document_type)
	custLedgEntry.Document_no = genJnlLine.Document_no
	custLedgEntry.External_document_no = genJnlLine.External_document_no
	custLedgEntry.Description = genJnlLine.Description
	custLedgEntry.Currency_code = genJnlLine.Currency_code
	custLedgEntry.Sales_lcy = genJnlLine.Sales_purch_lcy
	custLedgEntry.Profit_lcy = genJnlLine.Profit_lcy
	custLedgEntry.Inv_discount_lcy = genJnlLine.Inv_discount_lcy
	custLedgEntry.Sell_to_customer_no = genJnlLine.Sell_to_buy_from_no
	custLedgEntry.Customer_posting_group = genJnlLine.Posting_group
	custLedgEntry.Dimension_1_code = genJnlLine.Dimension_1_code
	custLedgEntry.Dimension_2_code = genJnlLine.Dimension_2_code
	custLedgEntry.Salesperson_code = genJnlLine.Salespers_purch_code
	custLedgEntry.Source_code = genJnlLine.Source_code
	custLedgEntry.On_hold = genJnlLine.On_hold
	custLedgEntry.Applies_to_doc_type = gtables.CustomerLedgerEntryApplies_to_doc_type(genJnlLine.Applies_to_doc_type)
	custLedgEntry.Applies_to_doc_no = genJnlLine.Applies_to_doc_no
	custLedgEntry.Due_date = genJnlLine.Due_date
	custLedgEntry.Pmt_discount_date = genJnlLine.Pmt_discount_date
	custLedgEntry.Applies_to_id = genJnlLine.Applies_to_id
	custLedgEntry.Journal_batch_name = genJnlLine.Journal_batch_name
	custLedgEntry.Reason_code = genJnlLine.Reason_code
	custLedgEntry.Entry_no = p.nextEntryNo
	custLedgEntry.Transaction_no = p.nextTransactionNo
	custLedgEntry.Amount = genJnlLine.Amount
	custLedgEntry.Remaining_amount = genJnlLine.Amount
	custLedgEntry.Original_amount_lcy = genJnlLine.Amount_lcy
	custLedgEntry.Remaining_amt_lcy = genJnlLine.Amount_lcy
	custLedgEntry.Amount_lcy = genJnlLine.Amount_lcy
	custLedgEntry.User_id = types.Code(system.UserId())
	custLedgEntry.Open = !custLedgEntry.Remaining_amount.IsZero()
	custLedgEntry.Positive = custLedgEntry.Remaining_amount.IsPositive()
	custLedgEntry.Bal_account_type = gtables.CustomerLedgerEntryBal_account_type(genJnlLine.Bal_account_type)
	custLedgEntry.Bal_account_no = genJnlLine.Bal_account_no
	custLedgEntry.No_series = genJnlLine.Posting_no_series
	if !genJnlLine.Amount_lcy.IsZero() {
		if p.glSetup.Pmt_disc_excl_tax {
			custLedgEntry.Pmt_disc_possible = genJnlLine.Sales_purch_lcy.Mul(genJnlLine.Amount).Div(genJnlLine.Amount_lcy)
		} else {
			custLedgEntry.Pmt_disc_possible = genJnlLine.Amount
		}
		custLedgEntry.Pmt_disc_possible = system.Round(
			custLedgEntry.Pmt_disc_possible.Mul(genJnlLine.Payment_discount_percent).Div(types.NewDecimalFromInt(100)),
			p.currency.Amount_rounding_precision)
	}
	// Check the document no.
	if genJnlLine.Document_type == gtables.GenJournalLine_Document_type.Invoice ||
		genJnlLine.Document_type == gtables.GenJournalLine_Document_type.CreditMemo ||
		genJnlLine.Document_type == gtables.GenJournalLine_Document_type.FinanceChargeMemo ||
		genJnlLine.Document_type == gtables.GenJournalLine_Document_type.Reminder {
		oldCustLedgEntry := gtables.NewCustomerLedgerEntry(p.src)
		oldCustLedgEntry.SetCurrentKey("document_type", "document_no")
		oldCustLedgEntry.SetRange("document_type", custLedgEntry.Document_type)
		oldCustLedgEntry.SetRange("document_no", custLedgEntry.Document_no)
		if oldCustLedgEntry.FindFirst() {
			return apperrors.JnlSalesDocExists(custLedgEntry.Document_type, custLedgEntry.Document_no.String())
		}
	}
	salesSetup := gtables.GetSalesReceivablesSetup(p.src)
	if salesSetup.Ext_doc_no_mandatory {
		err = genJnlLine.TestField("external_document_no")
		if err != nil {
			return err
		}
	}

	// Post customer entry
	p.resetApplicationTotals()
	p.startEntryApplication(gtables.EntryApplication_Account_type.Customer, genJnlLine.Account_no)
	err = p.applyCustLedgEntry(custLedgEntry, cust, genJnlLine)
	if err != nil {
		return err
	}
	custUpdateDebitCredit(genJnlLine.Correction, custLedgEntry)
	err = custLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	// Post to Receivables Account, payment discount, application rounding
	return p.postApplicationGLEntries(genJnlLine, custPostingGr, custPostingGr.Receivables_account, custPostingGr.Payment_disc_acc, custPostingGr.Curr_appln_rounding_account)
}

// applyCustLedgEntry applies a new customer ledger entry (a payment) to the customer's open
// entries: the one of the line's Applies-to Doc. No., the entries marked with its Applies-to
// ID, or — Apply to Oldest — the open entries by due date. Each application takes the payment
// discount still possible, closes what it pays and records the exchange-rate differences.
func (p *GenJnlPostLine) applyCustLedgEntry(newCustLedgEntry *gtables.CustomerLedgerEntry, cust *gtables.Customer, genJnlLine *gtables.GenJournalLine) error {
	if newCustLedgEntry.Remaining_amount.IsZero() {
		return nil
	}
	applnCurrencyCode := newCustLedgEntry.Currency_code
	applnPostingDate := newCustLedgEntry.Posting_date
	applnRoundingPrecision, err := p.applnRoundingPrecision(newCustLedgEntry.Currency_code)
	if err != nil {
		return err
	}
	oldCustLedgEntry := gtables.NewCustomerLedgerEntry(p.src)
	positiveFilter := false
	if newCustLedgEntry.Applies_to_doc_no != "" {
		// Find the entry to be applied to
		oldCustLedgEntry.SetCurrentKey("document_type", "document_no", "customer_no")
		oldCustLedgEntry.SetRange("document_type", newCustLedgEntry.Applies_to_doc_type)
		oldCustLedgEntry.SetRange("document_no", newCustLedgEntry.Applies_to_doc_no)
		oldCustLedgEntry.SetRange("customer_no", newCustLedgEntry.Customer_no)
		oldCustLedgEntry.SetRange("open", true)
		if !oldCustLedgEntry.FindFirst() {
			return apperrors.JnlAppliesToNotFound(gtables.CustomerLedgerEntryTableName, newCustLedgEntry.Applies_to_doc_type, newCustLedgEntry.Applies_to_doc_no.String())
		}
		err = oldCustLedgEntry.TestField("positive", !newCustLedgEntry.Positive)
		if err != nil {
			return err
		}
		_, err = CheckCurrency(p.src, newCustLedgEntry.Currency_code, oldCustLedgEntry.Currency_code, gtables.GenJournalLine_Account_type.Customer, true)
		if err != nil {
			return err
		}
	} else {
		if newCustLedgEntry.Applies_to_id == "" &&
			!(cust.Application_method == gtables.Customer_Application_method.ApplyToOldest && genJnlLine.Allow_application) {
			return nil
		}
		// Find the first old entry (Invoice) which the new entry (Payment) should apply to
		oldCustLedgEntry.SetCurrentKey("customer_no", "open", "positive", "due_date", "currency_code")
		oldCustLedgEntry.SetRange("customer_no", newCustLedgEntry.Customer_no)
		oldCustLedgEntry.SetRange("open", true)
		oldCustLedgEntry.SetFilter("entry_no", "<>"+itoa(newCustLedgEntry.Entry_no))
		oldCustLedgEntry.SetRange("applies_to_id", newCustLedgEntry.Applies_to_id)
		// Management of posting in multiple currencies: the entries of the currency, or of the
		// currencies the setup lets the entry be applied to
		salesSetup := gtables.GetSalesReceivablesSetup(p.src)
		if salesSetup.Appln_between_currencies == gtables.SalesReceivablesSetup_Appln_between_currencies.None {
			oldCustLedgEntry.SetRange("currency_code", newCustLedgEntry.Currency_code)
		} else {
			oldCustLedgEntry.ClearMarks()
			for found := oldCustLedgEntry.FindSetBuffered(); found; found = oldCustLedgEntry.Next() {
				ok, err := CheckCurrency(p.src, newCustLedgEntry.Currency_code, oldCustLedgEntry.Currency_code, gtables.GenJournalLine_Account_type.Customer, false)
				if err != nil {
					return err
				}
				if ok {
					oldCustLedgEntry.Mark(true)
				}
			}
			oldCustLedgEntry.MarkedOnly(true)
		}
		oldCustLedgEntry.SetRange("positive", newCustLedgEntry.Remaining_amount.IsPositive())
		if oldCustLedgEntry.FindFirst() {
			tempAmount := newCustLedgEntry.Remaining_amount
			oldCustLedgEntry.SetRange("positive")
			for found := oldCustLedgEntry.FindSet(); found; found = oldCustLedgEntry.Next() {
				remaining := oldCustLedgEntry.Remaining_amount
				pmtDiscPossible := oldCustLedgEntry.Pmt_disc_possible
				if newCustLedgEntry.Currency_code != oldCustLedgEntry.Currency_code {
					remaining, err = ExchAmount(p.src, remaining, oldCustLedgEntry.Currency_code, applnCurrencyCode, applnPostingDate)
					if err != nil {
						return err
					}
					pmtDiscPossible, err = ExchAmount(p.src, pmtDiscPossible, oldCustLedgEntry.Currency_code, applnCurrencyCode, applnPostingDate)
					if err != nil {
						return err
					}
				}
				if newCustLedgEntry.Document_type == gtables.CustomerLedgerEntry_Document_type.Payment &&
					oldCustLedgEntry.Document_type == gtables.CustomerLedgerEntry_Document_type.Invoice &&
					!oldCustLedgEntry.Pmt_discount_date.Before(newCustLedgEntry.Posting_date) {
					remaining = remaining.Sub(pmtDiscPossible)
				}
				tempAmount = tempAmount.Add(remaining)
			}
			oldCustLedgEntry.SetRange("positive", tempAmount.IsNegative())
			positiveFilter = true
		} else {
			oldCustLedgEntry.SetRange("positive")
		}
		if !oldCustLedgEntry.FindFirst() {
			return nil
		}
	}

	// Apply the new entry (Payment) to the old entries (Invoices) one at a time
	found := true
	for found {
		oldCustLedgEntry2 := gtables.NewCustomerLedgerEntry(p.src)
		oldCustLedgEntry2.Copy(oldCustLedgEntry)
		// the application register: the entries as they were before the application
		p.recordCustEntry(newCustLedgEntry, true)
		p.recordCustEntry(oldCustLedgEntry2, false)
		// Management of posting in multiple currencies: the old entry's amounts in the
		// application's currency
		if applnCurrencyCode != oldCustLedgEntry2.Currency_code {
			oldCustLedgEntry2.Remaining_amount, err = ExchAmount(p.src, oldCustLedgEntry2.Remaining_amount, oldCustLedgEntry2.Currency_code, applnCurrencyCode, applnPostingDate)
			if err != nil {
				return err
			}
			oldCustLedgEntry2.Pmt_disc_possible, err = ExchAmount(p.src, oldCustLedgEntry2.Pmt_disc_possible, oldCustLedgEntry2.Currency_code, applnCurrencyCode, applnPostingDate)
			if err != nil {
				return err
			}
		}

		// Calculate payment discount
		if newCustLedgEntry.Document_type == gtables.CustomerLedgerEntry_Document_type.Payment &&
			oldCustLedgEntry.Document_type == gtables.CustomerLedgerEntry_Document_type.Invoice &&
			!oldCustLedgEntry.Pmt_discount_date.Before(newCustLedgEntry.Posting_date) &&
			(positiveFilter ||
				!newCustLedgEntry.Remaining_amount.Abs().Add(applnRoundingPrecision).Decimal.LessThan(
					oldCustLedgEntry2.Remaining_amount.Sub(oldCustLedgEntry2.Pmt_disc_possible).Abs().Decimal)) {
			postUnrealAmount := newCustLedgEntry.Remaining_amt_lcy.Add(custLedgApplAmtLCY(newCustLedgEntry.Remaining_amount.Neg(), newCustLedgEntry))
			pmtDisc := oldCustLedgEntry2.Pmt_disc_possible.Neg()
			pmtDiscLCY := custLedgApplAmtLCY(pmtDisc, newCustLedgEntry)
			newCustLedgEntry.Amount = newCustLedgEntry.Amount.Add(pmtDisc)
			newCustLedgEntry.Original_amount_lcy = newCustLedgEntry.Original_amount_lcy.Add(pmtDiscLCY)
			newCustLedgEntry.Remaining_amount = newCustLedgEntry.Remaining_amount.Add(pmtDisc)
			newCustLedgEntry.Remaining_amt_lcy = newCustLedgEntry.Remaining_amt_lcy.Add(pmtDiscLCY)
			deltaUnrealAmount := postUnrealAmount.Sub(newCustLedgEntry.Remaining_amt_lcy.Add(custLedgApplAmtLCY(newCustLedgEntry.Remaining_amount.Neg(), newCustLedgEntry)))
			newCustLedgEntry.Original_amount_lcy = newCustLedgEntry.Original_amount_lcy.Sub(deltaUnrealAmount)
			newCustLedgEntry.Remaining_amt_lcy = newCustLedgEntry.Remaining_amt_lcy.Sub(deltaUnrealAmount)
			newCustLedgEntry.Amount_lcy = newCustLedgEntry.Amount_lcy.Add(pmtDiscLCY).Sub(deltaUnrealAmount)
			oldCustLedgEntry.Pmt_disc_given_lcy = pmtDiscLCY.Sub(deltaUnrealAmount).Neg()
			p.totalPmtDiscLCY = p.totalPmtDiscLCY.Sub(pmtDiscLCY.Sub(deltaUnrealAmount))
			pmtDiscAddCurr, err := p.pmtDiscAddCurr(applnCurrencyCode, pmtDisc, pmtDiscLCY)
			if err != nil {
				return err
			}
			deltaUnrealAmountAddCurr, err := p.calcLCYToAddCurr(deltaUnrealAmount)
			if err != nil {
				return err
			}
			p.totalPmtDiscAddCurr = p.totalPmtDiscAddCurr.Sub(pmtDiscAddCurr.Sub(deltaUnrealAmountAddCurr))
		}

		// Management of currency rounding
		err = p.custCurrencyApplnRounding(newCustLedgEntry, oldCustLedgEntry, oldCustLedgEntry2, applnCurrencyCode, applnRoundingPrecision)
		if err != nil {
			return err
		}

		// Find amount for application
		var appliedAmount types.Decimal
		if positiveFilter {
			appliedAmount = oldCustLedgEntry2.Remaining_amount.Neg()
		} else {
			if newCustLedgEntry.Remaining_amount.Abs().Decimal.LessThan(oldCustLedgEntry2.Remaining_amount.Abs().Decimal) {
				appliedAmount = newCustLedgEntry.Remaining_amount
			} else {
				appliedAmount = oldCustLedgEntry2.Remaining_amount.Neg()
			}
		}
		var oldAppliedAmount types.Decimal
		if appliedAmount.Equal(oldCustLedgEntry2.Remaining_amount.Neg()) {
			oldAppliedAmount = oldCustLedgEntry.Remaining_amount.Neg()
		} else {
			oldAppliedAmount, err = ExchAmount(p.src, appliedAmount, applnCurrencyCode, oldCustLedgEntry2.Currency_code, applnPostingDate)
			if err != nil {
				return err
			}
		}
		var appliedAmountLCY types.Decimal
		if applnCurrencyCode != "" {
			appliedAmountLCY = custLedgApplAmtLCY(oldAppliedAmount, oldCustLedgEntry)
		} else {
			appliedAmountLCY = custLedgApplAmtLCY(appliedAmount, newCustLedgEntry)
		}
		realCurrencyCode := newCustLedgEntry.Currency_code
		if oldCustLedgEntry.Currency_code != "" {
			realCurrencyCode = oldCustLedgEntry.Currency_code
		}

		// Update the old entry
		p.applCustLedgEntryAmt(oldCustLedgEntry, oldAppliedAmount, appliedAmountLCY, realCurrencyCode)
		if !oldCustLedgEntry.Open {
			oldCustLedgEntry.Closed_by_entry_no = newCustLedgEntry.Entry_no
			oldCustLedgEntry.Closed_at_date = newCustLedgEntry.Posting_date
			oldCustLedgEntry.Closed_by_amount = oldAppliedAmount.Neg()
			oldCustLedgEntry.Closed_by_amount_lcy = appliedAmountLCY.Neg()
			oldCustLedgEntry.Closed_by_currency_code = newCustLedgEntry.Currency_code
			oldCustLedgEntry.Closed_by_currency_amount = appliedAmount.Neg()
			p.updateCalcInterest(oldCustLedgEntry)
			updateCalcInterest2(oldCustLedgEntry, newCustLedgEntry)
		}
		custUpdateDebitCredit(
			oldCustLedgEntry.Debit_amount.IsNegative() || oldCustLedgEntry.Credit_amount.IsNegative() ||
				oldCustLedgEntry.Debit_amount_lcy.IsNegative() || oldCustLedgEntry.Credit_amount_lcy.IsNegative(),
			oldCustLedgEntry)
		oldCustLedgEntry.Applies_to_id = ""
		err = oldCustLedgEntry.ModifyOrError(false)
		if err != nil {
			return err
		}

		// Update the new entry
		p.applCustLedgEntryAmt(newCustLedgEntry, appliedAmount.Neg(), appliedAmountLCY.Neg(), realCurrencyCode)
		if !newCustLedgEntry.Open && oldCustLedgEntry.Open {
			newCustLedgEntry.Closed_by_entry_no = oldCustLedgEntry.Entry_no
			newCustLedgEntry.Closed_at_date = newCustLedgEntry.Posting_date
			newCustLedgEntry.Closed_by_amount = appliedAmount
			newCustLedgEntry.Closed_by_amount_lcy = appliedAmountLCY
			newCustLedgEntry.Closed_by_currency_code = oldCustLedgEntry.Currency_code
			newCustLedgEntry.Closed_by_currency_amount = oldAppliedAmount
			newCustLedgEntry.Applies_to_id = ""
		}

		// Find the next old entry for application of the new entry
		completed := false
		if newCustLedgEntry.Applies_to_doc_no != "" {
			completed = true
		} else if positiveFilter {
			if oldCustLedgEntry.Next() {
				completed = false
			} else {
				oldCustLedgEntry.SetRange("positive")
				positiveFilter = false
				oldCustLedgEntry.FindFirst()
				completed = !oldCustLedgEntry.Remaining_amount.Mul(newCustLedgEntry.Remaining_amount).IsNegative()
			}
		} else {
			if newCustLedgEntry.Open {
				completed = !oldCustLedgEntry.Next()
			} else {
				completed = true
			}
		}
		found = !completed
	}
	// Blank the Applies-to ID field
	if !newCustLedgEntry.Open {
		newCustLedgEntry.Applies_to_id = ""
		p.updateCalcInterest(newCustLedgEntry)
	}
	return nil
}

// updateCalcInterest (UpdateCalcInterest): a closed entry earns interest when it was closed after
// its due date — by the entry that closed it, or by those it closed
func (p *GenJnlPostLine) updateCalcInterest(custLedgEntry *gtables.CustomerLedgerEntry) {
	custLedgEntry2 := gtables.NewCustomerLedgerEntry(p.src)
	if custLedgEntry2.Get(custLedgEntry.Closed_by_entry_no) {
		updateCalcInterest2(custLedgEntry, custLedgEntry2)
	}
	custLedgEntry2 = gtables.NewCustomerLedgerEntry(p.src)
	custLedgEntry2.SetRange("closed_by_entry_no", custLedgEntry.Entry_no)
	for found := custLedgEntry2.FindSet(); found; found = custLedgEntry2.Next() {
		updateCalcInterest2(custLedgEntry, custLedgEntry2)
	}
}

// updateCalcInterest2 (UpdateCalcInterest2)
func updateCalcInterest2(custLedgEntry, custLedgEntry2 *gtables.CustomerLedgerEntry) {
	if custLedgEntry.Due_date.Before(custLedgEntry2.Document_date) {
		custLedgEntry.Calculate_interest = true
	}
}

// applCustLedgEntryAmt applies an amount to a customer ledger entry: its remaining amounts and
// Open, and the realized and unrealized exchange-rate gains and losses the application gives
func (p *GenJnlPostLine) applCustLedgEntryAmt(custLedgEntry *gtables.CustomerLedgerEntry, appliedAmount, appliedAmountLCY types.Decimal, currencyCode types.Code) {
	postUnrealAmount := custLedgEntry.Remaining_amt_lcy.Add(custLedgApplAmtLCY(custLedgEntry.Remaining_amount.Neg(), custLedgEntry))
	expectedApplAmtLCY := custLedgApplAmtLCY(appliedAmount, custLedgEntry)
	deltaRealAmount := expectedApplAmtLCY.Sub(appliedAmountLCY)
	var deltaUnrealAmount types.Decimal
	if !custLedgEntry.Remaining_amount.IsZero() {
		deltaUnrealAmount = system.Round(postUnrealAmount.Mul(custLedgEntry.Remaining_amount.Add(appliedAmount).Div(custLedgEntry.Remaining_amount))).Sub(postUnrealAmount)
	}
	custLedgEntry.Amount_lcy = custLedgEntry.Amount_lcy.Add(deltaRealAmount).Add(deltaUnrealAmount)
	custLedgEntry.Remaining_amt_lcy = custLedgEntry.Remaining_amt_lcy.Add(expectedApplAmtLCY).Add(deltaUnrealAmount)
	p.addGainsLosses(currencyCode, custLedgEntry.Currency_code, deltaRealAmount, deltaUnrealAmount)
	custLedgEntry.Remaining_amount = custLedgEntry.Remaining_amount.Add(appliedAmount)
	custLedgEntry.Open = !custLedgEntry.Remaining_amount.IsZero()
}

// custLedgApplAmtLCY is the LCY part of an amount applied to a customer ledger entry, at the
// entry's own exchange rate
func custLedgApplAmtLCY(appliedAmount types.Decimal, custLedgEntry *gtables.CustomerLedgerEntry) types.Decimal {
	if custLedgEntry.Amount.IsZero() {
		return types.Decimal{}
	}
	return system.Round(custLedgEntry.Original_amount_lcy.Mul(custLedgEntry.Remaining_amount.Add(appliedAmount).Div(custLedgEntry.Amount))).
		Sub(system.Round(custLedgEntry.Original_amount_lcy.Mul(custLedgEntry.Remaining_amount.Div(custLedgEntry.Amount))))
}

// custCurrencyApplnRounding: a payment in another currency that leaves less than the
// application rounding precision takes the difference (Curr. Appln. Rounding Account)
func (p *GenJnlPostLine) custCurrencyApplnRounding(newCustLedgEntry, oldCustLedgEntry, oldCustLedgEntry2 *gtables.CustomerLedgerEntry, applnCurrencyCode types.Code, applnRoundingPrecision types.Decimal) error {
	if newCustLedgEntry.Document_type != gtables.CustomerLedgerEntry_Document_type.Payment ||
		applnCurrencyCode == oldCustLedgEntry.Currency_code {
		return nil
	}
	applnRounding := newCustLedgEntry.Remaining_amount.Add(oldCustLedgEntry2.Remaining_amount).Neg()
	applnRoundingLCY := custLedgApplAmtLCY(applnRounding, newCustLedgEntry)
	if applnRounding.IsZero() || applnRounding.Abs().Decimal.GreaterThan(applnRoundingPrecision.Decimal) {
		return nil
	}
	postUnrealAmount := newCustLedgEntry.Remaining_amt_lcy.Add(custLedgApplAmtLCY(newCustLedgEntry.Remaining_amount.Neg(), newCustLedgEntry))
	newCustLedgEntry.Amount = newCustLedgEntry.Amount.Add(applnRounding)
	newCustLedgEntry.Original_amount_lcy = newCustLedgEntry.Original_amount_lcy.Add(applnRoundingLCY)
	newCustLedgEntry.Remaining_amount = newCustLedgEntry.Remaining_amount.Add(applnRounding)
	newCustLedgEntry.Remaining_amt_lcy = newCustLedgEntry.Remaining_amt_lcy.Add(applnRoundingLCY)
	deltaUnrealAmount := postUnrealAmount.Sub(newCustLedgEntry.Remaining_amt_lcy.Add(custLedgApplAmtLCY(newCustLedgEntry.Remaining_amount.Neg(), newCustLedgEntry)))
	newCustLedgEntry.Original_amount_lcy = newCustLedgEntry.Original_amount_lcy.Sub(deltaUnrealAmount)
	newCustLedgEntry.Remaining_amt_lcy = newCustLedgEntry.Remaining_amt_lcy.Sub(deltaUnrealAmount)
	newCustLedgEntry.Amount_lcy = newCustLedgEntry.Amount_lcy.Add(applnRoundingLCY).Sub(deltaUnrealAmount)
	oldCustLedgEntry.Rounding_currency = newCustLedgEntry.Currency_code
	oldCustLedgEntry.Rounding_amount = applnRounding
	oldCustLedgEntry.Rounding_amount_lcy = applnRoundingLCY
	p.totalApplnRoundingLCY = p.totalApplnRoundingLCY.Sub(applnRoundingLCY.Sub(deltaUnrealAmount))
	deltaUnrealAmountAddCurr, err := p.calcLCYToAddCurr(deltaUnrealAmount)
	if err != nil {
		return err
	}
	p.totalApplnRoundingAddCurr = p.totalApplnRoundingAddCurr.Sub(applnRounding.Sub(deltaUnrealAmountAddCurr))
	return nil
}

// custUpdateDebitCredit: Debit/Credit Amount (LCY) from the amounts (a correction the other way
// round)
func custUpdateDebitCredit(correction bool, custLedgEntry *gtables.CustomerLedgerEntry) {
	if (custLedgEntry.Amount.IsPositive() || custLedgEntry.Amount_lcy.IsPositive()) && !correction ||
		(custLedgEntry.Amount.IsNegative() || custLedgEntry.Amount_lcy.IsNegative()) && correction {
		custLedgEntry.Debit_amount = custLedgEntry.Amount
		custLedgEntry.Credit_amount = types.Decimal{}
		custLedgEntry.Debit_amount_lcy = custLedgEntry.Amount_lcy
		custLedgEntry.Credit_amount_lcy = types.Decimal{}
	} else {
		custLedgEntry.Debit_amount = types.Decimal{}
		custLedgEntry.Credit_amount = custLedgEntry.Amount.Neg()
		custLedgEntry.Debit_amount_lcy = types.Decimal{}
		custLedgEntry.Credit_amount_lcy = custLedgEntry.Amount_lcy.Neg()
	}
}
