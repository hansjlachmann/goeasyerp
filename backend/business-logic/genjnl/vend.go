package genjnl

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// postVend posts a vendor line: the vendor ledger entry (applied to the vendor's open
// entries), the G/L entry on the posting group's payables account, and the payment discount
// and application rounding the application gave
func (p *GenJnlPostLine) postVend(genJnlLine *gtables.GenJournalLine) error {
	vend := gtables.NewVendor(p.src)
	err := vend.GetOrError(genJnlLine.Account_no)
	if err != nil {
		return err
	}
	err = vend.TestField("blocked", false)
	if err != nil {
		return err
	}
	if genJnlLine.Posting_group == "" {
		err = vend.TestField("vendor_posting_group")
		if err != nil {
			return err
		}
		genJnlLine.Posting_group = vend.Vendor_posting_group
	}
	vendPostingGr := gtables.NewVendorPostingGroup(p.src)
	err = vendPostingGr.GetOrError(genJnlLine.Posting_group)
	if err != nil {
		return err
	}
	err = vendPostingGr.TestField("payables_account")
	if err != nil {
		return err
	}
	p.vendPosting = true
	err = p.testPostingType()
	if err != nil {
		return err
	}
	vendLedgEntry := gtables.NewVendorLedgerEntry(p.src)
	vendLedgEntry.LockTable()
	vendLedgEntry.Init()
	vendLedgEntry.Vendor_no = genJnlLine.Account_no
	vendLedgEntry.Posting_date = genJnlLine.Posting_date
	vendLedgEntry.Document_date = genJnlLine.Document_date
	vendLedgEntry.Document_type = gtables.VendorLedgerEntryDocument_type(genJnlLine.Document_type)
	vendLedgEntry.Document_no = genJnlLine.Document_no
	vendLedgEntry.External_document_no = genJnlLine.External_document_no
	vendLedgEntry.Description = genJnlLine.Description
	vendLedgEntry.Currency_code = genJnlLine.Currency_code
	vendLedgEntry.Purchase_lcy = genJnlLine.Sales_purch_lcy
	vendLedgEntry.Inv_discount_lcy = genJnlLine.Inv_discount_lcy
	vendLedgEntry.Buy_from_vendor_no = genJnlLine.Sell_to_buy_from_no
	vendLedgEntry.Vendor_posting_group = genJnlLine.Posting_group
	vendLedgEntry.Dimension_1_code = genJnlLine.Dimension_1_code
	vendLedgEntry.Dimension_2_code = genJnlLine.Dimension_2_code
	vendLedgEntry.Purchaser_code = genJnlLine.Salespers_purch_code
	vendLedgEntry.Source_code = genJnlLine.Source_code
	vendLedgEntry.On_hold = genJnlLine.On_hold
	vendLedgEntry.Applies_to_doc_type = gtables.VendorLedgerEntryApplies_to_doc_type(genJnlLine.Applies_to_doc_type)
	vendLedgEntry.Applies_to_doc_no = genJnlLine.Applies_to_doc_no
	vendLedgEntry.Due_date = genJnlLine.Due_date
	vendLedgEntry.Pmt_discount_date = genJnlLine.Pmt_discount_date
	vendLedgEntry.Applies_to_id = genJnlLine.Applies_to_id
	vendLedgEntry.Journal_batch_name = genJnlLine.Journal_batch_name
	vendLedgEntry.Reason_code = genJnlLine.Reason_code
	vendLedgEntry.Entry_no = p.nextEntryNo
	vendLedgEntry.Transaction_no = p.nextTransactionNo
	vendLedgEntry.Amount = genJnlLine.Amount
	vendLedgEntry.Remaining_amount = genJnlLine.Amount
	vendLedgEntry.Original_amount_lcy = genJnlLine.Amount_lcy
	vendLedgEntry.Remaining_amt_lcy = genJnlLine.Amount_lcy
	vendLedgEntry.Amount_lcy = genJnlLine.Amount_lcy
	vendLedgEntry.User_id = types.Code(system.UserId())
	vendLedgEntry.Open = !vendLedgEntry.Remaining_amount.IsZero()
	vendLedgEntry.Positive = vendLedgEntry.Remaining_amount.IsPositive()
	vendLedgEntry.Bal_account_type = gtables.VendorLedgerEntryBal_account_type(genJnlLine.Bal_account_type)
	vendLedgEntry.Bal_account_no = genJnlLine.Bal_account_no
	vendLedgEntry.No_series = genJnlLine.Posting_no_series
	if !genJnlLine.Amount_lcy.IsZero() {
		if p.glSetup.Pmt_disc_excl_tax {
			vendLedgEntry.Pmt_disc_possible = genJnlLine.Sales_purch_lcy.Mul(genJnlLine.Amount).Div(genJnlLine.Amount_lcy)
		} else {
			vendLedgEntry.Pmt_disc_possible = genJnlLine.Amount
		}
		vendLedgEntry.Pmt_disc_possible = system.Round(
			vendLedgEntry.Pmt_disc_possible.Mul(genJnlLine.Payment_discount_percent).Div(types.NewDecimalFromInt(100)),
			p.currency.Amount_rounding_precision)
	}
	purchSetup := gtables.GetPurchasesPayablesSetup(p.src)
	// Check the document no.
	if genJnlLine.Document_type == gtables.GenJournalLine_Document_type.Invoice ||
		genJnlLine.Document_type == gtables.GenJournalLine_Document_type.CreditMemo ||
		genJnlLine.Document_type == gtables.GenJournalLine_Document_type.FinanceChargeMemo ||
		genJnlLine.Document_type == gtables.GenJournalLine_Document_type.Reminder {
		oldVendLedgEntry := gtables.NewVendorLedgerEntry(p.src)
		oldVendLedgEntry.SetCurrentKey("document_type", "document_no")
		oldVendLedgEntry.SetRange("document_type", vendLedgEntry.Document_type)
		oldVendLedgEntry.SetRange("document_no", vendLedgEntry.Document_no)
		if oldVendLedgEntry.FindFirst() {
			return apperrors.JnlPurchDocExists(vendLedgEntry.Document_type, vendLedgEntry.Document_no.String())
		}
		if purchSetup.Ext_doc_no_mandatory || vendLedgEntry.External_document_no != "" {
			// Test vendor number
			err = genJnlLine.TestField("external_document_no")
			if err != nil {
				return err
			}
			oldVendLedgEntry.Reset()
			oldVendLedgEntry.SetCurrentKey("document_type", "external_document_no", "vendor_no")
			oldVendLedgEntry.SetRange("document_type", vendLedgEntry.Document_type)
			oldVendLedgEntry.SetRange("external_document_no", vendLedgEntry.External_document_no)
			oldVendLedgEntry.SetRange("vendor_no", vendLedgEntry.Vendor_no)
			if oldVendLedgEntry.FindFirst() {
				return apperrors.JnlPurchDocExistsForVendor(vendLedgEntry.Document_type, vendLedgEntry.External_document_no.String())
			}
		}
	}

	// Post vendor entry
	p.resetApplicationTotals()
	err = p.applyVendLedgEntry(vendLedgEntry, vend, genJnlLine)
	if err != nil {
		return err
	}
	vendUpdateDebitCredit(genJnlLine.Correction, vendLedgEntry)
	err = vendLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	// Post to Payables Account, payment discount, application rounding
	return p.postApplicationGLEntries(genJnlLine, vendPostingGr, vendPostingGr.Payables_account, vendPostingGr.Payment_disc_acc, vendPostingGr.Curr_appln_rounding_account)
}

// applyVendLedgEntry applies a new vendor ledger entry (a payment) to the vendor's open
// entries: the one of the line's Applies-to Doc. No., the entries marked with its Applies-to
// ID, or — Apply to Oldest — the open entries by due date. Each application takes the payment
// discount still possible, closes what it pays and records the exchange-rate differences.
func (p *GenJnlPostLine) applyVendLedgEntry(newVendLedgEntry *gtables.VendorLedgerEntry, vend *gtables.Vendor, genJnlLine *gtables.GenJournalLine) error {
	if newVendLedgEntry.Remaining_amount.IsZero() {
		return nil
	}
	applnCurrencyCode := newVendLedgEntry.Currency_code
	applnRoundingPrecision, err := p.applnRoundingPrecision(newVendLedgEntry.Currency_code)
	if err != nil {
		return err
	}
	oldVendLedgEntry := gtables.NewVendorLedgerEntry(p.src)
	positiveFilter := false
	if newVendLedgEntry.Applies_to_doc_no != "" {
		// Find the entry to be applied to
		oldVendLedgEntry.SetCurrentKey("document_type", "document_no", "vendor_no")
		oldVendLedgEntry.SetRange("document_type", newVendLedgEntry.Applies_to_doc_type)
		oldVendLedgEntry.SetRange("document_no", newVendLedgEntry.Applies_to_doc_no)
		oldVendLedgEntry.SetRange("vendor_no", newVendLedgEntry.Vendor_no)
		oldVendLedgEntry.SetRange("open", true)
		if !oldVendLedgEntry.FindFirst() {
			return apperrors.JnlAppliesToNotFound(gtables.VendorLedgerEntryTableName, newVendLedgEntry.Applies_to_doc_type, newVendLedgEntry.Applies_to_doc_no.String())
		}
		err = oldVendLedgEntry.TestField("positive", !newVendLedgEntry.Positive)
		if err != nil {
			return err
		}
		err = CheckCurrency(newVendLedgEntry.Currency_code, oldVendLedgEntry.Currency_code)
		if err != nil {
			return err
		}
	} else {
		if newVendLedgEntry.Applies_to_id == "" &&
			!(vend.Application_method == gtables.Vendor_Application_method.ApplyToOldest && genJnlLine.Allow_application) {
			return nil
		}
		// Find the first old entry (Invoice) which the new entry (Payment) should apply to
		oldVendLedgEntry.SetCurrentKey("vendor_no", "open", "positive", "due_date", "currency_code")
		oldVendLedgEntry.SetRange("vendor_no", newVendLedgEntry.Vendor_no)
		oldVendLedgEntry.SetRange("open", true)
		oldVendLedgEntry.SetFilter("entry_no", "<>"+itoa(newVendLedgEntry.Entry_no))
		oldVendLedgEntry.SetRange("applies_to_id", newVendLedgEntry.Applies_to_id)
		// Application between currencies is not supported yet: the entries of the currency
		oldVendLedgEntry.SetRange("currency_code", newVendLedgEntry.Currency_code)
		oldVendLedgEntry.SetRange("positive", newVendLedgEntry.Remaining_amount.IsPositive())
		if oldVendLedgEntry.FindFirst() {
			tempAmount := newVendLedgEntry.Remaining_amount
			oldVendLedgEntry.SetRange("positive")
			for found := oldVendLedgEntry.FindSet(); found; found = oldVendLedgEntry.Next() {
				remaining := oldVendLedgEntry.Remaining_amount
				if newVendLedgEntry.Document_type == gtables.VendorLedgerEntry_Document_type.Payment &&
					oldVendLedgEntry.Document_type == gtables.VendorLedgerEntry_Document_type.Invoice &&
					!oldVendLedgEntry.Pmt_discount_date.Before(newVendLedgEntry.Posting_date) {
					remaining = remaining.Sub(oldVendLedgEntry.Pmt_disc_possible)
				}
				tempAmount = tempAmount.Add(remaining)
			}
			oldVendLedgEntry.SetRange("positive", tempAmount.IsNegative())
			positiveFilter = true
		} else {
			oldVendLedgEntry.SetRange("positive")
		}
		if !oldVendLedgEntry.FindFirst() {
			return nil
		}
	}

	// Apply the new entry (Payment) to the old entries (Invoices) one at a time
	found := true
	for found {
		oldVendLedgEntry2 := gtables.NewVendorLedgerEntry(p.src)
		oldVendLedgEntry2.Copy(oldVendLedgEntry)

		// Calculate payment discount
		if newVendLedgEntry.Document_type == gtables.VendorLedgerEntry_Document_type.Payment &&
			oldVendLedgEntry.Document_type == gtables.VendorLedgerEntry_Document_type.Invoice &&
			!oldVendLedgEntry.Pmt_discount_date.Before(newVendLedgEntry.Posting_date) &&
			(positiveFilter ||
				!newVendLedgEntry.Remaining_amount.Abs().Add(applnRoundingPrecision).Decimal.LessThan(
					oldVendLedgEntry2.Remaining_amount.Sub(oldVendLedgEntry2.Pmt_disc_possible).Abs().Decimal)) {
			postUnrealAmount := newVendLedgEntry.Remaining_amt_lcy.Add(vendLedgApplAmtLCY(newVendLedgEntry.Remaining_amount.Neg(), newVendLedgEntry))
			pmtDisc := oldVendLedgEntry2.Pmt_disc_possible.Neg()
			pmtDiscLCY := vendLedgApplAmtLCY(pmtDisc, newVendLedgEntry)
			newVendLedgEntry.Amount = newVendLedgEntry.Amount.Add(pmtDisc)
			newVendLedgEntry.Original_amount_lcy = newVendLedgEntry.Original_amount_lcy.Add(pmtDiscLCY)
			newVendLedgEntry.Remaining_amount = newVendLedgEntry.Remaining_amount.Add(pmtDisc)
			newVendLedgEntry.Remaining_amt_lcy = newVendLedgEntry.Remaining_amt_lcy.Add(pmtDiscLCY)
			deltaUnrealAmount := postUnrealAmount.Sub(newVendLedgEntry.Remaining_amt_lcy.Add(vendLedgApplAmtLCY(newVendLedgEntry.Remaining_amount.Neg(), newVendLedgEntry)))
			newVendLedgEntry.Original_amount_lcy = newVendLedgEntry.Original_amount_lcy.Sub(deltaUnrealAmount)
			newVendLedgEntry.Remaining_amt_lcy = newVendLedgEntry.Remaining_amt_lcy.Sub(deltaUnrealAmount)
			newVendLedgEntry.Amount_lcy = newVendLedgEntry.Amount_lcy.Add(pmtDiscLCY).Sub(deltaUnrealAmount)
			oldVendLedgEntry.Pmt_disc_rcd_lcy = pmtDiscLCY.Sub(deltaUnrealAmount).Neg()
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
		err = p.vendCurrencyApplnRounding(newVendLedgEntry, oldVendLedgEntry, oldVendLedgEntry2, applnCurrencyCode, applnRoundingPrecision)
		if err != nil {
			return err
		}

		// Find amount for application
		var appliedAmount types.Decimal
		if positiveFilter {
			appliedAmount = oldVendLedgEntry2.Remaining_amount.Neg()
		} else {
			if newVendLedgEntry.Remaining_amount.Abs().Decimal.LessThan(oldVendLedgEntry2.Remaining_amount.Abs().Decimal) {
				appliedAmount = newVendLedgEntry.Remaining_amount
			} else {
				appliedAmount = oldVendLedgEntry2.Remaining_amount.Neg()
			}
		}
		oldAppliedAmount := appliedAmount
		if appliedAmount.Equal(oldVendLedgEntry2.Remaining_amount.Neg()) {
			oldAppliedAmount = oldVendLedgEntry.Remaining_amount.Neg()
		}
		var appliedAmountLCY types.Decimal
		if applnCurrencyCode != "" {
			appliedAmountLCY = vendLedgApplAmtLCY(oldAppliedAmount, oldVendLedgEntry)
		} else {
			appliedAmountLCY = vendLedgApplAmtLCY(appliedAmount, newVendLedgEntry)
		}
		realCurrencyCode := newVendLedgEntry.Currency_code
		if oldVendLedgEntry.Currency_code != "" {
			realCurrencyCode = oldVendLedgEntry.Currency_code
		}

		// Update the old entry
		p.applVendLedgEntryAmt(oldVendLedgEntry, oldAppliedAmount, appliedAmountLCY, realCurrencyCode)
		if !oldVendLedgEntry.Open {
			oldVendLedgEntry.Closed_by_entry_no = newVendLedgEntry.Entry_no
			oldVendLedgEntry.Closed_at_date = newVendLedgEntry.Posting_date
			oldVendLedgEntry.Closed_by_amount = oldAppliedAmount.Neg()
			oldVendLedgEntry.Closed_by_amount_lcy = appliedAmountLCY.Neg()
			oldVendLedgEntry.Closed_by_currency_code = newVendLedgEntry.Currency_code
			oldVendLedgEntry.Closed_by_currency_amount = appliedAmount.Neg()
		}
		vendUpdateDebitCredit(
			oldVendLedgEntry.Debit_amount.IsNegative() || oldVendLedgEntry.Credit_amount.IsNegative() ||
				oldVendLedgEntry.Debit_amount_lcy.IsNegative() || oldVendLedgEntry.Credit_amount_lcy.IsNegative(),
			oldVendLedgEntry)
		oldVendLedgEntry.Applies_to_id = ""
		err = oldVendLedgEntry.ModifyOrError(false)
		if err != nil {
			return err
		}

		// Update the new entry
		p.applVendLedgEntryAmt(newVendLedgEntry, appliedAmount.Neg(), appliedAmountLCY.Neg(), realCurrencyCode)
		if !newVendLedgEntry.Open && oldVendLedgEntry.Open {
			newVendLedgEntry.Closed_by_entry_no = oldVendLedgEntry.Entry_no
			newVendLedgEntry.Closed_at_date = newVendLedgEntry.Posting_date
			newVendLedgEntry.Closed_by_amount = appliedAmount
			newVendLedgEntry.Closed_by_amount_lcy = appliedAmountLCY
			newVendLedgEntry.Closed_by_currency_code = oldVendLedgEntry.Currency_code
			newVendLedgEntry.Closed_by_currency_amount = oldAppliedAmount
			newVendLedgEntry.Applies_to_id = ""
		}

		// Find the next old entry for application of the new entry
		completed := false
		if newVendLedgEntry.Applies_to_doc_no != "" {
			completed = true
		} else if positiveFilter {
			if oldVendLedgEntry.Next() {
				completed = false
			} else {
				oldVendLedgEntry.SetRange("positive")
				positiveFilter = false
				oldVendLedgEntry.FindFirst()
				completed = !oldVendLedgEntry.Remaining_amount.Mul(newVendLedgEntry.Remaining_amount).IsNegative()
			}
		} else {
			if newVendLedgEntry.Open {
				completed = !oldVendLedgEntry.Next()
			} else {
				completed = true
			}
		}
		found = !completed
	}
	// Blank the Applies-to ID field
	if !newVendLedgEntry.Open {
		newVendLedgEntry.Applies_to_id = ""
	}
	return nil
}

// applVendLedgEntryAmt applies an amount to a vendor ledger entry: its remaining amounts and
// Open, and the realized and unrealized exchange-rate gains and losses the application gives
func (p *GenJnlPostLine) applVendLedgEntryAmt(vendLedgEntry *gtables.VendorLedgerEntry, appliedAmount, appliedAmountLCY types.Decimal, currencyCode types.Code) {
	postUnrealAmount := vendLedgEntry.Remaining_amt_lcy.Add(vendLedgApplAmtLCY(vendLedgEntry.Remaining_amount.Neg(), vendLedgEntry))
	expectedApplAmtLCY := vendLedgApplAmtLCY(appliedAmount, vendLedgEntry)
	deltaRealAmount := expectedApplAmtLCY.Sub(appliedAmountLCY)
	var deltaUnrealAmount types.Decimal
	if !vendLedgEntry.Remaining_amount.IsZero() {
		deltaUnrealAmount = system.Round(postUnrealAmount.Mul(vendLedgEntry.Remaining_amount.Add(appliedAmount).Div(vendLedgEntry.Remaining_amount))).Sub(postUnrealAmount)
	}
	vendLedgEntry.Amount_lcy = vendLedgEntry.Amount_lcy.Add(deltaRealAmount).Add(deltaUnrealAmount)
	vendLedgEntry.Remaining_amt_lcy = vendLedgEntry.Remaining_amt_lcy.Add(expectedApplAmtLCY).Add(deltaUnrealAmount)
	p.addGainsLosses(currencyCode, vendLedgEntry.Currency_code, deltaRealAmount, deltaUnrealAmount)
	vendLedgEntry.Remaining_amount = vendLedgEntry.Remaining_amount.Add(appliedAmount)
	vendLedgEntry.Open = !vendLedgEntry.Remaining_amount.IsZero()
}

// vendLedgApplAmtLCY is the LCY part of an amount applied to a vendor ledger entry, at the
// entry's own exchange rate
func vendLedgApplAmtLCY(appliedAmount types.Decimal, vendLedgEntry *gtables.VendorLedgerEntry) types.Decimal {
	if vendLedgEntry.Amount.IsZero() {
		return types.Decimal{}
	}
	return system.Round(vendLedgEntry.Original_amount_lcy.Mul(vendLedgEntry.Remaining_amount.Add(appliedAmount).Div(vendLedgEntry.Amount))).
		Sub(system.Round(vendLedgEntry.Original_amount_lcy.Mul(vendLedgEntry.Remaining_amount.Div(vendLedgEntry.Amount))))
}

// vendCurrencyApplnRounding: a payment in another currency that leaves less than the
// application rounding precision takes the difference (Curr. Appln. Rounding Account)
func (p *GenJnlPostLine) vendCurrencyApplnRounding(newVendLedgEntry, oldVendLedgEntry, oldVendLedgEntry2 *gtables.VendorLedgerEntry, applnCurrencyCode types.Code, applnRoundingPrecision types.Decimal) error {
	if newVendLedgEntry.Document_type != gtables.VendorLedgerEntry_Document_type.Payment ||
		applnCurrencyCode == oldVendLedgEntry.Currency_code {
		return nil
	}
	applnRounding := newVendLedgEntry.Remaining_amount.Add(oldVendLedgEntry2.Remaining_amount).Neg()
	applnRoundingLCY := vendLedgApplAmtLCY(applnRounding, newVendLedgEntry)
	if applnRounding.IsZero() || applnRounding.Abs().Decimal.GreaterThan(applnRoundingPrecision.Decimal) {
		return nil
	}
	postUnrealAmount := newVendLedgEntry.Remaining_amt_lcy.Add(vendLedgApplAmtLCY(newVendLedgEntry.Remaining_amount.Neg(), newVendLedgEntry))
	newVendLedgEntry.Amount = newVendLedgEntry.Amount.Add(applnRounding)
	newVendLedgEntry.Original_amount_lcy = newVendLedgEntry.Original_amount_lcy.Add(applnRoundingLCY)
	newVendLedgEntry.Remaining_amount = newVendLedgEntry.Remaining_amount.Add(applnRounding)
	newVendLedgEntry.Remaining_amt_lcy = newVendLedgEntry.Remaining_amt_lcy.Add(applnRoundingLCY)
	deltaUnrealAmount := postUnrealAmount.Sub(newVendLedgEntry.Remaining_amt_lcy.Add(vendLedgApplAmtLCY(newVendLedgEntry.Remaining_amount.Neg(), newVendLedgEntry)))
	newVendLedgEntry.Original_amount_lcy = newVendLedgEntry.Original_amount_lcy.Sub(deltaUnrealAmount)
	newVendLedgEntry.Remaining_amt_lcy = newVendLedgEntry.Remaining_amt_lcy.Sub(deltaUnrealAmount)
	newVendLedgEntry.Amount_lcy = newVendLedgEntry.Amount_lcy.Add(applnRoundingLCY).Sub(deltaUnrealAmount)
	oldVendLedgEntry.Rounding_currency = newVendLedgEntry.Currency_code
	oldVendLedgEntry.Rounding_amount = applnRounding
	oldVendLedgEntry.Rounding_amount_lcy = applnRoundingLCY
	p.totalApplnRoundingLCY = p.totalApplnRoundingLCY.Sub(applnRoundingLCY.Sub(deltaUnrealAmount))
	deltaUnrealAmountAddCurr, err := p.calcLCYToAddCurr(deltaUnrealAmount)
	if err != nil {
		return err
	}
	p.totalApplnRoundingAddCurr = p.totalApplnRoundingAddCurr.Sub(applnRounding.Sub(deltaUnrealAmountAddCurr))
	return nil
}

// vendUpdateDebitCredit: Debit/Credit Amount (LCY) from the amounts (a correction the other way
// round)
func vendUpdateDebitCredit(correction bool, vendLedgEntry *gtables.VendorLedgerEntry) {
	if (vendLedgEntry.Amount.IsPositive() || vendLedgEntry.Amount_lcy.IsPositive()) && !correction ||
		(vendLedgEntry.Amount.IsNegative() || vendLedgEntry.Amount_lcy.IsNegative()) && correction {
		vendLedgEntry.Debit_amount = vendLedgEntry.Amount
		vendLedgEntry.Credit_amount = types.Decimal{}
		vendLedgEntry.Debit_amount_lcy = vendLedgEntry.Amount_lcy
		vendLedgEntry.Credit_amount_lcy = types.Decimal{}
	} else {
		vendLedgEntry.Debit_amount = types.Decimal{}
		vendLedgEntry.Credit_amount = vendLedgEntry.Amount.Neg()
		vendLedgEntry.Debit_amount_lcy = types.Decimal{}
		vendLedgEntry.Credit_amount_lcy = vendLedgEntry.Amount_lcy.Neg()
	}
}
