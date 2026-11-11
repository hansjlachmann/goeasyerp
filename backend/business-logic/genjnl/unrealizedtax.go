package genjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Unrealized tax (codeunit 12 CustUnrealizedTax, VendUnrealizedTax): the tax of an invoice whose
// Tax Posting Setup (VAT) or tax jurisdiction (sales tax) has an Unrealized Tax Type is posted on
// the unrealized accounts and kept as the tax entry's Unrealized Amount/Base (insertTax). When the
// customer or vendor entry is paid — applied to — the part the type gives is realized: moved from
// the unrealized account to the tax account and written as a tax entry of its own (pointing at
// the unrealized one by Unrealized Tax Entry No.), and the unrealized entry's remaining amounts go
// down:
//   - Percentage: the share of the document paid;
//   - First: the tax first (the payment realizes tax until all of it is realized), First (Fully
//     Paid): all the tax once the payment covers it;
//   - Last: the tax last (after the base is paid), Last (Fully Paid): all when the entry is closed.
// A closed entry realizes everything still unrealized.

// unrealizedLedgEntry is what the realization reads of the customer or vendor entry
type unrealizedLedgEntry struct {
	transactionNo     int
	open              bool
	currencyCode      types.Code
	originalAmountLCY types.Decimal
	amount            types.Decimal
	remainingAmount   types.Decimal
}

func custUnrealizedLedgEntry(custLedgEntry2 *gtables.CustomerLedgerEntry) unrealizedLedgEntry {
	return unrealizedLedgEntry{custLedgEntry2.Transaction_no, custLedgEntry2.Open, custLedgEntry2.Currency_code,
		custLedgEntry2.Original_amount_lcy, custLedgEntry2.Amount, custLedgEntry2.Remaining_amount}
}

func vendUnrealizedLedgEntry(vendLedgEntry2 *gtables.VendorLedgerEntry) unrealizedLedgEntry {
	return unrealizedLedgEntry{vendLedgEntry2.Transaction_no, vendLedgEntry2.Open, vendLedgEntry2.Currency_code,
		vendLedgEntry2.Original_amount_lcy, vendLedgEntry2.Amount, vendLedgEntry2.Remaining_amount}
}

// custUnrealizedTax (CustUnrealizedTax) realizes the unrealized tax of a customer entry for the
// amount settled
func (p *GenJnlPostLine) custUnrealizedTax(custLedgEntry2 *gtables.CustomerLedgerEntry, settledAmount types.Decimal) error {
	return p.unrealizedTax(custUnrealizedLedgEntry(custLedgEntry2), settledAmount, false)
}

// vendUnrealizedTax (VendUnrealizedTax) realizes the unrealized tax of a vendor entry for the
// amount settled
func (p *GenJnlPostLine) vendUnrealizedTax(vendLedgEntry2 *gtables.VendorLedgerEntry, settledAmount types.Decimal) error {
	return p.unrealizedTax(vendUnrealizedLedgEntry(vendLedgEntry2), settledAmount, true)
}

// unrealizedTax is CustUnrealizedTax (vendor false) and VendUnrealizedTax (vendor true): the
// unrealized tax entries of the entry's transaction, each realized by its part
func (p *GenJnlPostLine) unrealizedTax(ledgEntry2 unrealizedLedgEntry, settledAmount types.Decimal, vendor bool) error {
	taxEntry2 := gtables.NewTaxEntry(p.src)
	taxEntry2.SetCurrentKey("transaction_no")
	taxEntry2.SetRange("transaction_no", ledgEntry2.transactionNo)
	// the entries first: the realized ones are inserted into the same table
	var taxEntries []*gtables.TaxEntry
	for found := taxEntry2.FindSet(); found; found = taxEntry2.Next() {
		e := gtables.NewTaxEntry(p.src)
		e.Copy(taxEntry2)
		taxEntries = append(taxEntries, e)
	}
	if len(taxEntries) == 0 {
		return nil
	}
	lastConnectionNo := 0
	for _, taxEntry2 := range taxEntries {
		if lastConnectionNo != taxEntry2.Sales_tax_connection_no {
			err := p.insertSummarizedTax()
			if err != nil {
				return err
			}
			lastConnectionNo = taxEntry2.Sales_tax_connection_no
		}
		if taxEntry2.Type == gtables.TaxEntry_Type.Blank || !taxEntry2.Amount.IsZero() || !taxEntry2.Base.IsZero() {
			continue
		}
		var taxPostingSetup *gtables.TaxPostingSetup
		var taxJurisdiction *gtables.TaxJurisdiction
		var unrealizedTaxType gtables.TaxPostingSetupUnrealized_tax_type
		if taxEntry2.Tax_calculation_type == gtables.TaxEntry_Tax_calculation_type.SalesTax {
			taxJurisdiction = gtables.NewTaxJurisdiction(p.src)
			err := taxJurisdiction.GetOrError(taxEntry2.Tax_jurisdiction_code)
			if err != nil {
				return err
			}
			unrealizedTaxType = gtables.TaxPostingSetupUnrealized_tax_type(taxJurisdiction.Unrealized_tax_type)
		} else {
			taxPostingSetup = gtables.NewTaxPostingSetup(p.src)
			err := taxPostingSetup.GetOrError(map[string]interface{}{
				"tax_bus_posting_group": taxEntry2.Tax_bus_posting_group, "tax_prod_posting_group": taxEntry2.Tax_prod_posting_group})
			if err != nil {
				return err
			}
			unrealizedTaxType = taxPostingSetup.Unrealized_tax_type
		}
		if unrealizedTaxType == gtables.TaxPostingSetup_Unrealized_tax_type.Blank ||
			taxEntry2.Remaining_unrealized_amount.IsZero() && taxEntry2.Remaining_unrealized_base.IsZero() {
			continue
		}
		// a reverse charge (vendor: also use tax) is realized all at once
		reverseCharge := taxEntry2.Tax_calculation_type == gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT ||
			vendor && taxEntry2.Tax_calculation_type == gtables.TaxEntry_Tax_calculation_type.SalesTax && taxEntry2.Use_tax
		taxPart := unrealizedTaxPart(unrealizedTaxType, ledgEntry2, taxEntry2, settledAmount, reverseCharge)
		if taxPart.IsZero() {
			continue
		}
		err := p.realizeTaxEntry(taxEntry2, taxPostingSetup, taxJurisdiction, taxPart, vendor)
		if err != nil {
			return err
		}
	}
	return p.insertSummarizedTax()
}

// unrealizedTaxPart is the part of the unrealized tax the payment realizes (1 = what remains)
func unrealizedTaxPart(unrealizedTaxType gtables.TaxPostingSetupUnrealized_tax_type, ledgEntry2 unrealizedLedgEntry, taxEntry2 *gtables.TaxEntry,
	settledAmount types.Decimal, reverseCharge bool) types.Decimal {
	one := types.NewDecimalFromInt(1)
	if !ledgEntry2.open {
		return one
	}
	// the share of the document paid, applied to the tax and its base
	paidAmount := func() types.Decimal {
		if ledgEntry2.amount.IsZero() {
			return types.Decimal{}
		}
		return taxEntry2.Unrealized_amount.Add(taxEntry2.Unrealized_base).Mul(ledgEntry2.amount.Sub(ledgEntry2.remainingAmount)).Div(ledgEntry2.amount).Abs()
	}
	switch unrealizedTaxType {
	case gtables.TaxPostingSetup_Unrealized_tax_type.Percentage:
		if ledgEntry2.currencyCode == "" {
			if ledgEntry2.originalAmountLCY.IsZero() {
				return one
			}
			return settledAmount.Neg().Div(ledgEntry2.originalAmountLCY)
		}
		if ledgEntry2.amount.IsZero() {
			return one
		}
		return settledAmount.Neg().Div(ledgEntry2.amount)
	case gtables.TaxPostingSetup_Unrealized_tax_type.First, gtables.TaxPostingSetup_Unrealized_tax_type.FirstFullyPaid:
		if reverseCharge {
			return one
		}
		paid := paidAmount()
		if !paid.Decimal.LessThan(taxEntry2.Unrealized_amount.Abs().Decimal) {
			return one
		}
		if unrealizedTaxType == gtables.TaxPostingSetup_Unrealized_tax_type.FirstFullyPaid {
			return types.Decimal{}
		}
		return paid.Sub(taxEntry2.Unrealized_amount.Sub(taxEntry2.Remaining_unrealized_amount).Abs()).Div(taxEntry2.Unrealized_amount).Abs()
	case gtables.TaxPostingSetup_Unrealized_tax_type.Last, gtables.TaxPostingSetup_Unrealized_tax_type.LastFullyPaid:
		if reverseCharge {
			return types.Decimal{}
		}
		paid := paidAmount()
		if paid.Decimal.LessThan(taxEntry2.Unrealized_base.Abs().Decimal) {
			return types.Decimal{}
		}
		if unrealizedTaxType == gtables.TaxPostingSetup_Unrealized_tax_type.LastFullyPaid || taxEntry2.Unrealized_amount.IsZero() {
			return types.Decimal{}
		}
		return paid.Sub(taxEntry2.Unrealized_base.Add(taxEntry2.Unrealized_amount.Sub(taxEntry2.Remaining_unrealized_amount)).Abs()).Div(taxEntry2.Unrealized_amount).Abs()
	}
	return types.Decimal{}
}

// realizeTaxEntry realizes a part of an unrealized tax entry: the tax from the unrealized account
// to the tax account (a reverse charge purchase also back from its unrealized reverse charge
// account), a tax entry of the realized amount and base, the unrealized entry's remaining
// amounts less it
func (p *GenJnlPostLine) realizeTaxEntry(taxEntry2 *gtables.TaxEntry, taxPostingSetup *gtables.TaxPostingSetup, taxJurisdiction *gtables.TaxJurisdiction,
	taxPart types.Decimal, vendor bool) error {
	var taxAccount, taxUnrealAccount, reverseAccount, reverseUnrealAccount types.Code
	reverse := false
	var err error
	if !vendor {
		switch taxEntry2.Tax_calculation_type {
		case gtables.TaxEntry_Tax_calculation_type.NormalVAT, gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT, gtables.TaxEntry_Tax_calculation_type.FullVAT:
			err = testFields(taxPostingSetup, "sales_vat_account", "sales_vat_unreal_account")
			taxAccount = taxPostingSetup.Sales_vat_account
			taxUnrealAccount = taxPostingSetup.Sales_vat_unreal_account
		case gtables.TaxEntry_Tax_calculation_type.SalesTax:
			err = testFields(taxJurisdiction, "tax_account_sales", "unreal_tax_acc_sales")
			taxAccount = taxJurisdiction.Tax_account_sales
			taxUnrealAccount = taxJurisdiction.Unreal_tax_acc_sales
		}
	} else {
		switch taxEntry2.Tax_calculation_type {
		case gtables.TaxEntry_Tax_calculation_type.NormalVAT, gtables.TaxEntry_Tax_calculation_type.FullVAT:
			err = testFields(taxPostingSetup, "purchase_vat_account", "purch_vat_unreal_account")
			taxAccount = taxPostingSetup.Purchase_vat_account
			taxUnrealAccount = taxPostingSetup.Purch_vat_unreal_account
		case gtables.TaxEntry_Tax_calculation_type.ReverseChargeVAT:
			err = testFields(taxPostingSetup, "purchase_vat_account", "purch_vat_unreal_account", "reverse_chrg_vat_acc", "reverse_chrg_vat_unreal_acc")
			taxAccount = taxPostingSetup.Purchase_vat_account
			taxUnrealAccount = taxPostingSetup.Purch_vat_unreal_account
			reverseAccount = taxPostingSetup.Reverse_chrg_vat_acc
			reverseUnrealAccount = taxPostingSetup.Reverse_chrg_vat_unreal_acc
			reverse = true
		case gtables.TaxEntry_Tax_calculation_type.SalesTax:
			if taxEntry2.Type == gtables.TaxEntry_Type.Purchase && taxEntry2.Use_tax {
				err = testFields(taxJurisdiction, "tax_account_purchases", "reverse_charge_purchases", "unreal_tax_acc_purchases", "unreal_rev_charge_purch")
				reverseAccount = taxJurisdiction.Reverse_charge_purchases
				reverseUnrealAccount = taxJurisdiction.Unreal_rev_charge_purch
				reverse = true
			} else {
				err = testFields(taxJurisdiction, "tax_account_purchases", "unreal_tax_acc_purchases")
			}
			taxAccount = taxJurisdiction.Tax_account_purchases
			taxUnrealAccount = taxJurisdiction.Unreal_tax_acc_purchases
		}
	}
	if err != nil {
		return err
	}

	var taxAmount, taxBase, taxAmountAddCurr, taxBaseAddCurr types.Decimal
	if taxPart.Equal(types.NewDecimalFromInt(1)) {
		taxAmount = taxEntry2.Remaining_unrealized_amount
		taxBase = taxEntry2.Remaining_unrealized_base
		taxAmountAddCurr = taxEntry2.Add_curr_rem_unreal_amount
		taxBaseAddCurr = taxEntry2.Add_curr_rem_unreal_base
	} else {
		taxAmount = system.Round(taxEntry2.Unrealized_amount.Mul(taxPart))
		taxBase = system.Round(taxEntry2.Unrealized_base.Mul(taxPart))
		taxAmountAddCurr = system.Round(taxEntry2.Add_currency_unrealized_amt.Mul(taxPart), p.addCurrRoundingPrecision())
		taxBaseAddCurr = system.Round(taxEntry2.Add_currency_unrealized_base.Mul(taxPart), p.addCurrRoundingPrecision())
	}

	err = p.postRealizedTaxGLEntry(taxUnrealAccount, taxAmount.Neg(), taxAmountAddCurr.Neg(), taxAccount, nil)
	if err != nil {
		return err
	}
	groups := taxEntry2
	if !vendor {
		groups = nil // the customer's realized tax carries no posting groups (classic)
	}
	err = p.postRealizedTaxGLEntry(taxAccount, taxAmount, taxAmountAddCurr, taxUnrealAccount, groups)
	if err != nil {
		return err
	}
	if reverse {
		err = p.postRealizedTaxGLEntry(reverseUnrealAccount, taxAmount, taxAmountAddCurr, reverseAccount, nil)
		if err != nil {
			return err
		}
		err = p.postRealizedTaxGLEntry(reverseAccount, taxAmount.Neg(), taxAmountAddCurr.Neg(), reverseUnrealAccount, taxEntry2)
		if err != nil {
			return err
		}
	}

	genJnlLine := p.genJnlLine
	taxEntry := gtables.NewTaxEntry(p.src)
	taxEntry.LockTable()
	taxEntry.TransferFields(taxEntry2)
	taxEntry.Entry_no = p.nextTaxEntryNo
	taxEntry.Posting_date = genJnlLine.Posting_date
	taxEntry.Document_no = genJnlLine.Document_no
	taxEntry.External_document_no = genJnlLine.External_document_no
	taxEntry.Document_type = gtables.TaxEntryDocument_type(genJnlLine.Document_type)
	taxEntry.Amount = taxAmount
	taxEntry.Base = taxBase
	taxEntry.Unrealized_amount = types.Decimal{}
	taxEntry.Unrealized_base = types.Decimal{}
	taxEntry.Remaining_unrealized_amount = types.Decimal{}
	taxEntry.Remaining_unrealized_base = types.Decimal{}
	taxEntry.Additional_currency_amount = taxAmountAddCurr
	taxEntry.Additional_currency_base = taxBaseAddCurr
	taxEntry.Add_currency_unrealized_amt = types.Decimal{}
	taxEntry.Add_currency_unrealized_base = types.Decimal{}
	taxEntry.Add_curr_rem_unreal_amount = types.Decimal{}
	taxEntry.Add_curr_rem_unreal_base = types.Decimal{}
	taxEntry.User_id = types.Code(system.UserId())
	taxEntry.Source_code = genJnlLine.Source_code
	taxEntry.Reason_code = genJnlLine.Reason_code
	taxEntry.Closed_by_entry_no = 0
	taxEntry.Closed = false
	taxEntry.Transaction_no = p.nextTransactionNo
	taxEntry.Sales_tax_connection_no = p.nextConnectionNo
	taxEntry.Unrealized_tax_entry_no = taxEntry2.Entry_no
	if p.acyCode() == "" {
		taxEntry.Additional_currency_amount = types.Decimal{}
		taxEntry.Additional_currency_base = types.Decimal{}
	}
	err = taxEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	p.nextTaxEntryNo = p.nextTaxEntryNo + 1
	p.recordApplicationTaxEntry(taxEntry.Entry_no)

	taxEntry2.Remaining_unrealized_amount = taxEntry2.Remaining_unrealized_amount.Sub(taxEntry.Amount)
	taxEntry2.Remaining_unrealized_base = taxEntry2.Remaining_unrealized_base.Sub(taxEntry.Base)
	taxEntry2.Add_curr_rem_unreal_amount = taxEntry2.Add_curr_rem_unreal_amount.Sub(taxEntry.Additional_currency_amount)
	taxEntry2.Add_curr_rem_unreal_base = taxEntry2.Add_curr_rem_unreal_base.Sub(taxEntry.Additional_currency_base)
	return taxEntry2.ModifyOrError(false)
}

// postRealizedTaxGLEntry posts a realized (or payment discount) tax G/L entry with its
// additional-currency amount given, the other account as balancing account and — groups given —
// the tax entry's posting groups; recorded for Unapply Entries
func (p *GenJnlPostLine) postRealizedTaxGLEntry(account types.Code, amount, amountAddCurr types.Decimal, balAccount types.Code, groups *gtables.TaxEntry) error {
	_, err := p.initGLEntry(account, amount, types.Decimal{}, false, true)
	if err != nil {
		return err
	}
	p.glEntry.Additional_currency_amount = amountAddCurr
	if groups != nil {
		setGLEntryGroupsFromTaxEntry(p.glEntry, groups)
	}
	p.glEntry.Bal_account_no = balAccount
	p.recordApplicationGL(account, amount, amountAddCurr, true)
	return p.summarizeTax()
}

// setGLEntryGroupsFromTaxEntry: the G/L entry carries the tax entry's general posting type and its
// general and tax posting groups
func setGLEntryGroupsFromTaxEntry(glEntry *gtables.GLEntry, taxEntry2 *gtables.TaxEntry) {
	glEntry.Gen_posting_type = gtables.GLEntryGen_posting_type(taxEntry2.Type)
	glEntry.Gen_bus_posting_group = taxEntry2.Gen_bus_posting_group
	glEntry.Gen_prod_posting_group = taxEntry2.Gen_prod_posting_group
	glEntry.Tax_bus_posting_group = taxEntry2.Tax_bus_posting_group
	glEntry.Tax_prod_posting_group = taxEntry2.Tax_prod_posting_group
	glEntry.Tax_area_code = taxEntry2.Tax_area_code
	glEntry.Tax_liable = taxEntry2.Tax_liable
	glEntry.Tax_group_code = taxEntry2.Tax_group_code
	glEntry.Use_tax = taxEntry2.Use_tax
}

// testFields: each field of the record must have a value (TESTFIELD one after the other)
func testFields(rec interface {
	TestField(field string, value ...any) error
}, fields ...string) error {
	for _, field := range fields {
		err := rec.TestField(field)
		if err != nil {
			return err
		}
	}
	return nil
}
