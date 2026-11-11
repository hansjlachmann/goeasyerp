package genjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The tax adjustment of a payment discount (codeunit 12 CashDiscPost): with the General Ledger
// Setup's Adjust for Payment Disc. (and not Pmt. Disc. Excl. Tax), a discount taken on an invoice
// whose Tax Posting Setup (VAT) or tax jurisdiction (sales tax) has Adjust for Payment Discount is
// split over the invoice's tax entries in proportion to the discount: per tax entry the base part
// goes to the General Posting Setup's Sales / Purch. Payment Disc. Account and the tax part reduces
// the tax — a tax entry of the discount's base and tax (document: the payment) and the tax
// account. What is not split (setups without the adjustment) stays on the posting group's Payment
// Disc. Acc.

// pendingPmtDiscGLEntry is the payment discount G/L entry of a tax connection, written once its
// tax is known (classic GLEntry2)
type pendingPmtDiscGLEntry struct {
	glEntry *gtables.GLEntry
}

// cashDiscPost (CashDiscPost) splits a payment discount pmtDiscLCY2 (and *pmtDiscAddCurr2 in the
// additional currency) of a document — its original amount, currency and transaction — over the
// document's tax entries; it returns what is left of the discount (0 when every tax entry of the
// document is adjusted). vendor: the vendor's purchase accounts, else the customer's sales ones.
func (p *GenJnlPostLine) cashDiscPost(originalAmountLCY, originalAmount types.Decimal, originalCurrencyCode types.Code, currentTransactionNo int,
	vendor bool, pmtDiscLCY2 types.Decimal, pmtDiscAddCurr2 *types.Decimal) (types.Decimal, error) {
	if originalAmountLCY.IsZero() {
		return pmtDiscLCY2, nil
	}
	var originalAmountAddCurr types.Decimal
	if p.acyCode() == "" || p.acyCode() == originalCurrencyCode {
		originalAmountAddCurr = originalAmount
	} else {
		var err error
		originalAmountAddCurr, err = p.calcLCYToAddCurr(originalAmountLCY)
		if err != nil {
			return pmtDiscLCY2, err
		}
	}
	addCurrPrecision := p.addCurrRoundingPrecision()
	pmtDiscRounding := pmtDiscLCY2
	pmtDiscRoundingAddCurr := *pmtDiscAddCurr2
	pmtDiscFactorLCY := pmtDiscLCY2.Div(originalAmountLCY)
	var pmtDiscFactorAddCurr types.Decimal
	if !originalAmountAddCurr.IsZero() {
		pmtDiscFactorAddCurr = pmtDiscAddCurr2.Div(originalAmountAddCurr)
	}

	taxEntry2 := gtables.NewTaxEntry(p.src)
	taxEntry2.SetCurrentKey("transaction_no")
	taxEntry2.SetRange("transaction_no", currentTransactionNo)
	if currentTransactionNo == p.nextTransactionNo {
		taxEntry2.SetRange("entry_no", 0, p.firstNewTaxEntryNo-1)
	}
	// the entries first: the adjustments are inserted into the same table
	var taxEntries []*gtables.TaxEntry
	for found := taxEntry2.FindSet(); found; found = taxEntry2.Next() {
		e := gtables.NewTaxEntry(p.src)
		e.Copy(taxEntry2)
		taxEntries = append(taxEntries, e)
	}
	if len(taxEntries) == 0 {
		return pmtDiscLCY2, nil
	}

	var glEntry2 *pendingPmtDiscGLEntry
	var totalTaxAmount types.Decimal
	var taxBase, taxBaseAddCurr types.Decimal
	lastConnectionNo := 0
	for _, taxEntry2 := range taxEntries {
		taxPostingSetup := gtables.NewTaxPostingSetup(p.src)
		err := taxPostingSetup.GetOrError(map[string]interface{}{
			"tax_bus_posting_group": taxEntry2.Tax_bus_posting_group, "tax_prod_posting_group": taxEntry2.Tax_prod_posting_group})
		if err != nil {
			return pmtDiscLCY2, err
		}
		var taxJurisdiction *gtables.TaxJurisdiction
		adjustForPaymentDiscount := taxPostingSetup.Adjust_for_payment_discount
		if taxEntry2.Tax_calculation_type == gtables.TaxEntry_Tax_calculation_type.SalesTax {
			taxJurisdiction = gtables.NewTaxJurisdiction(p.src)
			err = taxJurisdiction.GetOrError(taxEntry2.Tax_jurisdiction_code)
			if err != nil {
				return pmtDiscLCY2, err
			}
			adjustForPaymentDiscount = taxJurisdiction.Adjust_for_payment_discount
		}
		if !adjustForPaymentDiscount {
			continue
		}
		if lastConnectionNo != taxEntry2.Sales_tax_connection_no {
			genPostingSetup := gtables.NewGeneralPostingSetup(p.src)
			err = genPostingSetup.GetOrError(map[string]interface{}{
				"gen_bus_posting_group": taxEntry2.Gen_bus_posting_group, "gen_prod_posting_group": taxEntry2.Gen_prod_posting_group})
			if err != nil {
				return pmtDiscLCY2, err
			}
			var paymentDiscAccount types.Code
			if !vendor {
				err = genPostingSetup.TestField("sales_payment_disc_account")
				paymentDiscAccount = genPostingSetup.Sales_payment_disc_account
			} else {
				err = genPostingSetup.TestField("purch_payment_disc_account")
				paymentDiscAccount = genPostingSetup.Purch_payment_disc_account
			}
			if err != nil {
				return pmtDiscLCY2, err
			}

			if lastConnectionNo != 0 {
				err = p.insertPmtDiscGLEntry(glEntry2, totalTaxAmount)
				if err != nil {
					return pmtDiscLCY2, err
				}
			}

			switch taxEntry2.Tax_calculation_type {
			case gtables.TaxEntry_Tax_calculation_type.NormalVAT, gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT, gtables.TaxEntry_Tax_calculation_type.FullVAT:
				taxBase = taxEntry2.Base.Add(taxEntry2.Unrealized_base)
				taxBaseAddCurr = taxEntry2.Additional_currency_base.Add(taxEntry2.Add_currency_unrealized_base)
			case gtables.TaxEntry_Tax_calculation_type.SalesTax:
				// the base of the connection's first jurisdiction (tax on tax has a larger one)
				taxEntry := connectionBaseEntry(taxEntries, taxEntry2)
				taxBase = taxEntry.Base.Add(taxEntry.Unrealized_base)
				taxBaseAddCurr = taxEntry.Additional_currency_base.Add(taxEntry.Add_currency_unrealized_base)
			}

			pmtDiscRounding = pmtDiscRounding.Add(taxBase.Mul(pmtDiscFactorLCY))
			taxBase = system.Round(pmtDiscRounding.Sub(pmtDiscLCY2))
			pmtDiscLCY2 = pmtDiscLCY2.Add(taxBase)

			pmtDiscRoundingAddCurr = pmtDiscRoundingAddCurr.Add(taxBaseAddCurr.Mul(pmtDiscFactorAddCurr))
			taxBaseAddCurr = system.Round(pmtDiscRoundingAddCurr.Sub(*pmtDiscAddCurr2), addCurrPrecision)
			*pmtDiscAddCurr2 = pmtDiscAddCurr2.Add(taxBaseAddCurr)

			_, err = p.initGLEntry(paymentDiscAccount, taxBase, types.Decimal{}, false, true)
			if err != nil {
				return pmtDiscLCY2, err
			}
			p.glEntry.Additional_currency_amount = taxBaseAddCurr
			setGLEntryGroupsFromTaxEntry(p.glEntry, taxEntry2)
			glEntry2 = &pendingPmtDiscGLEntry{glEntry: p.glEntry}

			totalTaxAmount = types.Decimal{}
			lastConnectionNo = taxEntry2.Sales_tax_connection_no
		}

		var taxAmount, taxAmountAddCurr types.Decimal
		taxEntryAmount := taxEntry2.Amount.Add(taxEntry2.Unrealized_amount)
		taxEntryAmountAddCurr := taxEntry2.Additional_currency_amount.Add(taxEntry2.Add_currency_unrealized_amt)
		// the reverse charge and use tax are on top of the amount: their share alone, not rounded
		// into the discount
		onTop := taxEntry2.Tax_calculation_type == gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT ||
			taxEntry2.Tax_calculation_type == gtables.TaxEntry_Tax_calculation_type.SalesTax && taxEntry2.Type == gtables.TaxEntry_Type.Purchase && taxEntry2.Use_tax
		if onTop {
			taxAmount = system.Round(taxEntryAmount.Mul(pmtDiscFactorLCY))
			taxAmountAddCurr = system.Round(taxEntryAmountAddCurr.Mul(pmtDiscFactorAddCurr), addCurrPrecision)
		} else {
			pmtDiscRounding = pmtDiscRounding.Add(taxEntryAmount.Mul(pmtDiscFactorLCY))
			taxAmount = system.Round(pmtDiscRounding.Sub(pmtDiscLCY2))
			pmtDiscLCY2 = pmtDiscLCY2.Add(taxAmount)

			pmtDiscRoundingAddCurr = pmtDiscRoundingAddCurr.Add(taxEntryAmountAddCurr.Mul(pmtDiscFactorAddCurr))
			taxAmountAddCurr = system.Round(pmtDiscRoundingAddCurr.Sub(*pmtDiscAddCurr2), addCurrPrecision)
			*pmtDiscAddCurr2 = pmtDiscAddCurr2.Add(taxAmountAddCurr)
		}
		totalTaxAmount = totalTaxAmount.Add(taxAmount)

		// Tax for Tax entry
		if taxEntry2.Type != gtables.TaxEntry_Type.Blank {
			genJnlLine := p.genJnlLine
			taxEntry := gtables.NewTaxEntry(p.src)
			taxEntry.LockTable()
			taxEntry.TransferFields(taxEntry2)
			taxEntry.Entry_no = p.nextTaxEntryNo
			taxEntry.Posting_date = genJnlLine.Posting_date
			taxEntry.Document_no = genJnlLine.Document_no
			taxEntry.External_document_no = genJnlLine.External_document_no
			taxEntry.Document_type = gtables.TaxEntryDocument_type(genJnlLine.Document_type)
			taxEntry.Source_code = genJnlLine.Source_code
			taxEntry.Reason_code = genJnlLine.Reason_code
			taxEntry.Transaction_no = p.nextTransactionNo
			taxEntry.Sales_tax_connection_no = p.nextConnectionNo
			taxEntry.Unrealized_amount = types.Decimal{}
			taxEntry.Unrealized_base = types.Decimal{}
			taxEntry.Remaining_unrealized_amount = types.Decimal{}
			taxEntry.Remaining_unrealized_base = types.Decimal{}
			taxEntry.Add_curr_rem_unreal_amount = types.Decimal{}
			taxEntry.Add_curr_rem_unreal_base = types.Decimal{}
			taxEntry.Unrealized_tax_entry_no = 0
			taxEntry.User_id = types.Code(system.UserId())
			taxEntry.Closed_by_entry_no = 0
			taxEntry.Closed = false
			taxEntry.Internal_ref_no = ""
			taxEntry.Amount = taxAmount
			taxEntry.Additional_currency_amount = taxAmountAddCurr
			taxEntry.Add_currency_unrealized_amt = types.Decimal{}
			taxEntry.Add_currency_unrealized_base = types.Decimal{}
			if taxEntry2.Tax_on_tax {
				taxEntry.Base = system.Round(taxEntry2.Base.Add(taxEntry2.Unrealized_base).Mul(pmtDiscFactorLCY))
				taxEntry.Additional_currency_base = system.Round(
					taxEntry2.Additional_currency_base.Add(taxEntry2.Add_currency_unrealized_base).Mul(pmtDiscFactorAddCurr), addCurrPrecision)
			} else {
				taxEntry.Base = taxBase
				taxEntry.Additional_currency_base = taxBaseAddCurr
			}
			if p.acyCode() == "" {
				taxEntry.Additional_currency_base = types.Decimal{}
				taxEntry.Additional_currency_amount = types.Decimal{}
			}
			err = taxEntry.InsertOrError(false)
			if err != nil {
				return pmtDiscLCY2, err
			}
			p.nextTaxEntryNo = p.nextTaxEntryNo + 1
			p.recordApplicationTaxEntry(taxEntry.Entry_no)
		}

		// Tax for G/L entry/entries
		if !taxAmount.IsZero() || !taxAmountAddCurr.IsZero() && p.genJnlLine.Source_currency_code == p.acyCode() {
			err = p.postPmtDiscTax(taxEntry2, taxPostingSetup, taxJurisdiction, taxAmount, taxAmountAddCurr)
			if err != nil {
				return pmtDiscLCY2, err
			}
		}
	}

	if lastConnectionNo != 0 {
		err := p.insertPmtDiscGLEntry(glEntry2, totalTaxAmount)
		if err != nil {
			return pmtDiscLCY2, err
		}
	}
	return pmtDiscLCY2, nil
}

// connectionBaseEntry: of the tax entries of a sales tax connection (one line's jurisdictions),
// the one with the smallest base — the line's own base, before tax on tax
func connectionBaseEntry(taxEntries []*gtables.TaxEntry, taxEntry2 *gtables.TaxEntry) *gtables.TaxEntry {
	baseEntry := taxEntry2
	for _, e := range taxEntries {
		if e.Transaction_no != taxEntry2.Transaction_no || e.Sales_tax_connection_no != taxEntry2.Sales_tax_connection_no {
			continue
		}
		if baseEntry.Base.IsNegative() && e.Base.Decimal.GreaterThan(baseEntry.Base.Decimal) ||
			!baseEntry.Base.IsNegative() && e.Base.Decimal.LessThan(baseEntry.Base.Decimal) {
			baseEntry = e
		}
	}
	return baseEntry
}

// postPmtDiscTax posts the tax a payment discount takes off: on the purchase VAT account (reverse
// charge: and back on the reverse charge account), the sales VAT account (reverse charge sale:
// none) or the jurisdiction's accounts
func (p *GenJnlPostLine) postPmtDiscTax(taxEntry2 *gtables.TaxEntry, taxPostingSetup *gtables.TaxPostingSetup, taxJurisdiction *gtables.TaxJurisdiction,
	taxAmount, taxAmountAddCurr types.Decimal) error {
	switch taxEntry2.Type {
	case gtables.TaxEntry_Type.Purchase:
		switch taxEntry2.Tax_calculation_type {
		case gtables.TaxEntry_Tax_calculation_type.NormalVAT, gtables.TaxEntry_Tax_calculation_type.FullVAT:
			err := taxPostingSetup.TestField("purchase_vat_account")
			if err != nil {
				return err
			}
			return p.postRealizedTaxGLEntry(taxPostingSetup.Purchase_vat_account, taxAmount, taxAmountAddCurr, "", nil)
		case gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT:
			err := testFields(taxPostingSetup, "purchase_vat_account", "reverse_chrg_vat_acc")
			if err != nil {
				return err
			}
			err = p.postRealizedTaxGLEntry(taxPostingSetup.Purchase_vat_account, taxAmount, taxAmountAddCurr, "", nil)
			if err != nil {
				return err
			}
			return p.postRealizedTaxGLEntry(taxPostingSetup.Reverse_chrg_vat_acc, taxAmount.Neg(), taxAmountAddCurr.Neg(), "", nil)
		case gtables.TaxEntry_Tax_calculation_type.SalesTax:
			if taxEntry2.Use_tax {
				err := testFields(taxJurisdiction, "tax_account_purchases", "reverse_charge_purchases")
				if err != nil {
					return err
				}
				err = p.postRealizedTaxGLEntry(taxJurisdiction.Tax_account_purchases, taxAmount, taxAmountAddCurr, "", nil)
				if err != nil {
					return err
				}
				return p.postRealizedTaxGLEntry(taxJurisdiction.Reverse_charge_purchases, taxAmount.Neg(), taxAmountAddCurr.Neg(), "", nil)
			}
			err := taxJurisdiction.TestField("tax_account_purchases")
			if err != nil {
				return err
			}
			return p.postRealizedTaxGLEntry(taxJurisdiction.Tax_account_purchases, taxAmount, taxAmountAddCurr, "", nil)
		}
	case gtables.TaxEntry_Type.Sale:
		switch taxEntry2.Tax_calculation_type {
		case gtables.TaxEntry_Tax_calculation_type.NormalVAT, gtables.TaxEntry_Tax_calculation_type.FullVAT:
			err := taxPostingSetup.TestField("sales_vat_account")
			if err != nil {
				return err
			}
			return p.postRealizedTaxGLEntry(taxPostingSetup.Sales_vat_account, taxAmount, taxAmountAddCurr, "", nil)
		case gtables.TaxEntry_Tax_calculation_type.SalesTax:
			err := taxJurisdiction.TestField("tax_account_sales")
			if err != nil {
				return err
			}
			return p.postRealizedTaxGLEntry(taxJurisdiction.Tax_account_sales, taxAmount, taxAmountAddCurr, "", nil)
		}
	}
	return nil
}

// insertPmtDiscGLEntry writes a connection's payment discount G/L entry with the tax the discount
// took off (its Tax Amount), then the connection's summarized tax G/L entries
func (p *GenJnlPostLine) insertPmtDiscGLEntry(glEntry2 *pendingPmtDiscGLEntry, totalTaxAmount types.Decimal) error {
	p.glEntry = glEntry2.glEntry
	p.glEntry.Entry_no = p.nextEntryNo
	p.glEntry.Tax_amount = totalTaxAmount
	p.recordApplicationGL(p.glEntry.Gl_account_no, p.glEntry.Amount, p.glEntry.Additional_currency_amount, true)
	err := p.insertGLEntry(true)
	if err != nil {
		return err
	}
	return p.insertSummarizedTax()
}
