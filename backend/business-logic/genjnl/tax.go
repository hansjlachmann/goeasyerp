package genjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salestax"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Tax of the journal lines (codeunit 12 InitTax, PostTax, InsertTax): a G/L line with a general
// posting type takes the tax out of its amount and writes tax entries, by the Tax Posting
// Setup's Tax Calculation Type:
//   - Normal VAT: the amount includes the VAT at the setup's VAT %; one tax entry, the VAT on the
//     setup's Sales / Purchase VAT Account;
//   - Reverse Charge VAT: a purchase owes the VAT on top of its amount (on the Purchase VAT
//     Account and back on the Reverse Chrg. VAT Acc.); a sale has none (a tax entry of its base);
//   - Full VAT: the whole amount is VAT, posted on the setup's VAT account (the line's account);
//   - Sales Tax: a sale includes it, a purchase with use tax pays it on top; a tax entry per
//     jurisdiction, the tax on the jurisdiction's accounts.
// Documents post with Manual VAT Entry (the tax amount the document calculated). A settlement
// line (Manual VAT Entry, posting type Settlement; sales tax: the jurisdiction in Tax Area Code)
// writes one closed tax entry of its amount.
//
// Unrealized tax (the setup's or jurisdiction's Unrealized Tax Type, General Ledger Setup
// Unrealized Tax): an invoice's tax goes onto the unrealized accounts and into the tax entry's
// Unrealized Amount/Base; it is realized as the document is paid (unrealizedtax.go). A payment
// discount reduces the VAT with Adjust for Payment Disc. (pmtdisctax.go, CashDiscPost).

// initTaxRegister: the first line takes the next tax entry number (Tax Entry locked); the
// register starts at it
func (p *GenJnlPostLine) initTaxRegister() error {
	taxEntry := gtables.NewTaxEntry(p.src)
	taxEntry.LockTable()
	if taxEntry.FindLast() {
		p.nextTaxEntryNo = taxEntry.Entry_no + 1
	} else {
		p.nextTaxEntryNo = 1
	}
	p.nextConnectionNo = 1
	p.firstNewTaxEntryNo = p.nextTaxEntryNo
	p.glReg.From_tax_entry_no = p.nextTaxEntryNo
	if p.salesTaxCalculate == nil {
		p.salesTaxCalculate = salestax.New(p.src)
	}
	return nil
}

// finishTaxRegister: the register's last tax entry (none: no tax entry range)
func (p *GenJnlPostLine) finishTaxRegister() {
	p.glReg.To_tax_entry_no = p.nextTaxEntryNo - 1
	if p.glReg.From_tax_entry_no == 0 || p.glReg.To_tax_entry_no < p.glReg.From_tax_entry_no {
		p.glReg.From_tax_entry_no = 0
		p.glReg.To_tax_entry_no = 0
	}
}

// initTax: a line with a general posting type keeps its posting groups and tax on the G/L
// entry and takes the tax out of the entry's amount
func (p *GenJnlPostLine) initTax(genJnlLine *gtables.GenJournalLine) error {
	p.addCurrGLEntryTaxAmt = types.Decimal{}
	if genJnlLine.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Blank {
		return p.glCalcAddCurrency(p.glEntry.Additional_currency_amount, true)
	}
	switch genJnlLine.Gen_posting_type {
	case gtables.GenJournalLine_Gen_posting_type.Sale:
		p.salesPostingType = true
	case gtables.GenJournalLine_Gen_posting_type.Purchase:
		p.purchPostingType = true
	}
	err := p.testPostingType()
	if err != nil {
		return err
	}
	p.taxPostingSetup = gtables.NewTaxPostingSetup(p.src)
	err = p.taxPostingSetup.GetOrError(map[string]interface{}{
		"tax_bus_posting_group": genJnlLine.Tax_bus_posting_group, "tax_prod_posting_group": genJnlLine.Tax_prod_posting_group})
	if err != nil {
		return err
	}
	err = genJnlLine.TestField("tax_calculation_type", gtables.GenJournalLineTax_calculation_type(p.taxPostingSetup.Tax_calculation_type))
	if err != nil {
		return err
	}
	switch genJnlLine.Vat_posting {
	case gtables.GenJournalLine_Vat_posting.AutomaticVATEntry:
		p.setGLEntryTaxFields(genJnlLine)
		switch genJnlLine.Tax_calculation_type {
		case gtables.GenJournalLine_Tax_calculation_type.NormalVAT:
			vatFactor := hundredPercent(p.taxPostingSetup.Vat_percent)
			p.glEntry.Amount = system.Round(genJnlLine.Amount_lcy.Div(vatFactor))
			p.glEntry.Tax_amount = genJnlLine.Amount_lcy.Sub(p.glEntry.Amount)
			p.glEntry.Additional_currency_amount = system.Round(genJnlLine.Source_currency_amount.Div(vatFactor), p.addCurrRoundingPrecision())
			if p.lineInAddCurr(genJnlLine) {
				p.addCurrGLEntryTaxAmt = genJnlLine.Source_currency_amount.Sub(p.glEntry.Additional_currency_amount)
			} else {
				p.addCurrGLEntryTaxAmt, err = p.calcLCYToAddCurr(p.glEntry.Tax_amount)
				if err != nil {
					return err
				}
			}
		case gtables.GenJournalLine_Tax_calculation_type.ReverseChargeVAT:
			switch genJnlLine.Gen_posting_type {
			case gtables.GenJournalLine_Gen_posting_type.Purchase:
				p.glEntry.Tax_amount = system.Round(p.glEntry.Amount.Mul(p.taxPostingSetup.Vat_percent).Div(types.NewDecimalFromInt(100)))
				p.addCurrGLEntryTaxAmt = system.Round(p.glEntry.Additional_currency_amount.Mul(p.taxPostingSetup.Vat_percent).Div(types.NewDecimalFromInt(100)),
					p.addCurrRoundingPrecision())
			case gtables.GenJournalLine_Gen_posting_type.Sale:
				p.glEntry.Tax_amount = types.Decimal{}
				p.addCurrGLEntryTaxAmt = types.Decimal{}
			}
		case gtables.GenJournalLine_Tax_calculation_type.FullVAT:
			switch genJnlLine.Gen_posting_type {
			case gtables.GenJournalLine_Gen_posting_type.Sale:
				err = p.taxPostingSetup.TestField("sales_vat_account")
				if err != nil {
					return err
				}
				err = genJnlLine.TestField("account_no", p.taxPostingSetup.Sales_vat_account)
				if err != nil {
					return err
				}
			case gtables.GenJournalLine_Gen_posting_type.Purchase:
				err = p.taxPostingSetup.TestField("purchase_vat_account")
				if err != nil {
					return err
				}
				err = genJnlLine.TestField("account_no", p.taxPostingSetup.Purchase_vat_account)
				if err != nil {
					return err
				}
			}
			p.glEntry.Amount = types.Decimal{}
			p.glEntry.Additional_currency_amount = types.Decimal{}
			p.glEntry.Tax_amount = genJnlLine.Amount_lcy
			if p.lineInAddCurr(genJnlLine) {
				p.addCurrGLEntryTaxAmt = genJnlLine.Source_currency_amount
			} else {
				p.addCurrGLEntryTaxAmt, err = p.calcLCYToAddCurr(genJnlLine.Amount_lcy)
				if err != nil {
					return err
				}
			}
		case gtables.GenJournalLine_Tax_calculation_type.SalesTax:
			if genJnlLine.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Purchase && genJnlLine.Use_tax {
				p.glEntry.Tax_amount, err = p.salesTaxCalculate.CalculateTax(genJnlLine.Tax_area_code, genJnlLine.Tax_group_code, genJnlLine.Tax_liable,
					genJnlLine.Posting_date, genJnlLine.Amount_lcy, genJnlLine.Quantity, types.Decimal{})
				if err != nil {
					return err
				}
				p.glEntry.Amount = genJnlLine.Amount_lcy
			} else {
				p.glEntry.Amount, err = p.salesTaxCalculate.ReverseCalculateTax(genJnlLine.Tax_area_code, genJnlLine.Tax_group_code, genJnlLine.Tax_liable,
					genJnlLine.Posting_date, genJnlLine.Amount_lcy, genJnlLine.Quantity, types.Decimal{})
				if err != nil {
					return err
				}
				p.glEntry.Tax_amount = genJnlLine.Amount_lcy.Sub(p.glEntry.Amount)
			}
		}
	case gtables.GenJournalLine_Vat_posting.ManualVATEntry:
		if genJnlLine.Gen_posting_type != gtables.GenJournalLine_Gen_posting_type.Settlement {
			p.setGLEntryTaxFields(genJnlLine)
			p.glEntry.Tax_amount = genJnlLine.Tax_amount_lcy
			if p.lineInAddCurr(genJnlLine) {
				p.addCurrGLEntryTaxAmt = genJnlLine.Source_curr_tax_amount
			} else {
				p.addCurrGLEntryTaxAmt, err = p.calcLCYToAddCurr(genJnlLine.Tax_amount_lcy)
				if err != nil {
					return err
				}
			}
		}
	}
	return p.glCalcAddCurrency(p.glEntry.Additional_currency_amount, true)
}

// hundredPercent: 1 + percent/100 (an amount including the percentage over the amount)
func hundredPercent(percent types.Decimal) types.Decimal {
	return types.NewDecimalFromInt(1).Add(percent.Div(types.NewDecimalFromInt(100)))
}

// lineInAddCurr: the line is in the additional reporting currency (classic "Source Currency
// Code" = GLSetup."Additional Reporting Currency", only when there is one)
func (p *GenJnlPostLine) lineInAddCurr(genJnlLine *gtables.GenJournalLine) bool {
	return p.acyCode() != "" && genJnlLine.Source_currency_code == p.acyCode()
}

// addCurrRoundingPrecision: the additional currency's Amount Rounding Precision (Currency2;
// without one the amounts in it are not used)
func (p *GenJnlPostLine) addCurrRoundingPrecision() types.Decimal {
	if p.acyCode() == "" || p.addCurr.currency == nil {
		return types.MustDecimal("0.01")
	}
	return p.addCurr.currency.Amount_rounding_precision
}

// isVAT: a VAT calculation type (Normal VAT, Reverse Charge VAT, Full VAT), not sales tax
func isVAT(taxCalculationType gtables.GenJournalLineTax_calculation_type) bool {
	return taxCalculationType == gtables.GenJournalLine_Tax_calculation_type.NormalVAT ||
		taxCalculationType == gtables.GenJournalLine_Tax_calculation_type.ReverseChargeVAT ||
		taxCalculationType == gtables.GenJournalLine_Tax_calculation_type.FullVAT
}

// setGLEntryTaxFields: the G/L entry keeps the line's posting groups and tax
func (p *GenJnlPostLine) setGLEntryTaxFields(genJnlLine *gtables.GenJournalLine) {
	p.glEntry.Gen_bus_posting_group = genJnlLine.Gen_bus_posting_group
	p.glEntry.Gen_prod_posting_group = genJnlLine.Gen_prod_posting_group
	p.glEntry.Tax_bus_posting_group = genJnlLine.Tax_bus_posting_group
	p.glEntry.Tax_prod_posting_group = genJnlLine.Tax_prod_posting_group
	p.glEntry.Tax_area_code = genJnlLine.Tax_area_code
	p.glEntry.Tax_liable = genJnlLine.Tax_liable
	p.glEntry.Tax_group_code = genJnlLine.Tax_group_code
	p.glEntry.Use_tax = genJnlLine.Use_tax
}

// postTax writes the line's tax entries — VAT: one; sales tax: one per jurisdiction of its tax
// area — and their G/L entries
func (p *GenJnlPostLine) postTax(genJnlLine *gtables.GenJournalLine) error {
	if genJnlLine.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Blank {
		return nil
	}
	glEntryAmount := p.glEntry.Amount
	glEntryTaxAmount := p.glEntry.Tax_amount
	glEntryAddCurrAmount := p.glEntry.Additional_currency_amount
	if isVAT(genJnlLine.Tax_calculation_type) {
		srcCurrTaxBaseAmount := genJnlLine.Source_curr_tax_base_amount
		if genJnlLine.Vat_posting == gtables.GenJournalLine_Vat_posting.AutomaticVATEntry {
			genJnlLine.Tax_base_amount_lcy = glEntryAmount
			// the base in the additional currency is the G/L entry's amount in it, as the LCY
			// base is the G/L entry's amount
			srcCurrTaxBaseAmount = glEntryAddCurrAmount
		}
		addCurrencyFactor, err := p.calcAddCurrFactor(glEntryAddCurrAmount.Add(p.addCurrGLEntryTaxAmt), glEntryAmount.Add(glEntryTaxAmount))
		if err != nil {
			return err
		}
		err = p.insertTax(genJnlLine, nil, glEntryAmount, glEntryTaxAmount, genJnlLine.Tax_base_amount_lcy, addCurrencyFactor,
			genJnlLine.Source_currency_code, glEntryAddCurrAmount, p.addCurrGLEntryTaxAmt, srcCurrTaxBaseAmount)
		if err != nil {
			return err
		}
		p.nextConnectionNo = p.nextConnectionNo + 1
		return nil
	}
	if genJnlLine.Tax_calculation_type != gtables.GenJournalLine_Tax_calculation_type.SalesTax {
		return nil
	}
	var salesTaxBaseAmount types.Decimal
	switch genJnlLine.Vat_posting {
	case gtables.GenJournalLine_Vat_posting.AutomaticVATEntry:
		salesTaxBaseAmount = glEntryAmount
	case gtables.GenJournalLine_Vat_posting.ManualVATEntry:
		salesTaxBaseAmount = genJnlLine.Tax_base_amount_lcy
	}
	if genJnlLine.Vat_posting == gtables.GenJournalLine_Vat_posting.ManualVATEntry &&
		genJnlLine.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Settlement {
		// a settlement (Calc. and Post Tax Settlement): one closed tax entry of the jurisdiction
		// in the line's Tax Area Code
		taxDetail := gtables.NewTaxDetail(p.src)
		taxDetail.Tax_jurisdiction_code = genJnlLine.Tax_area_code
		genJnlLine.Tax_area_code = ""
		addCurrencyFactor, err := p.calcAddCurrFactor(glEntryAddCurrAmount, glEntryAmount)
		if err != nil {
			return err
		}
		return p.insertTax(genJnlLine, taxDetail, glEntryAmount, glEntryTaxAmount, genJnlLine.Tax_base_amount_lcy, addCurrencyFactor,
			"", types.Decimal{}, types.Decimal{}, types.Decimal{})
	}
	addCurrencyFactor, err := p.calcAddCurrFactor(glEntryAddCurrAmount, glEntryAmount)
	if err != nil {
		return err
	}
	p.salesTaxCalculate = salestax.New(p.src)
	err = p.salesTaxCalculate.InitSalesTaxLines(genJnlLine.Tax_area_code, genJnlLine.Tax_group_code, genJnlLine.Tax_liable,
		salesTaxBaseAmount, genJnlLine.Quantity, genJnlLine.Posting_date, glEntryTaxAmount)
	if err != nil {
		return err
	}
	for {
		taxDetail, taxAmount, taxBase, ok := p.salesTaxCalculate.GetSalesTaxLine()
		if !ok {
			break
		}
		err = p.insertTax(genJnlLine, taxDetail, salesTaxBaseAmount, taxAmount, taxBase, addCurrencyFactor,
			"", types.Decimal{}, types.Decimal{}, types.Decimal{})
		if err != nil {
			return err
		}
	}
	return p.insertSummarizedTax()
}

// insertTax writes the tax entry (VAT: the line's; sales tax: one jurisdiction's — taxDetail)
// and posts its tax: VAT on the Tax Posting Setup's accounts, sales tax on the jurisdiction's
// (use tax: the purchase tax account and the reverse charge account). The additional-currency
// amounts are given when the line is in that currency (srcCurrCode), else exchanged at the
// factor.
func (p *GenJnlPostLine) insertTax(genJnlLine *gtables.GenJournalLine, taxDetail *gtables.TaxDetail, glEntryAmount, glEntryTaxAmount, glEntryBaseAmount, addCurrencyFactor types.Decimal,
	srcCurrCode types.Code, srcCurrGLEntryAmt, srcCurrGLEntryTaxAmt, srcCurrGLEntryBaseAmt types.Decimal) error {
	taxEntry := gtables.NewTaxEntry(p.src)
	taxEntry.Init()
	taxEntry.Entry_no = p.nextTaxEntryNo
	taxEntry.Gen_bus_posting_group = genJnlLine.Gen_bus_posting_group
	taxEntry.Gen_prod_posting_group = genJnlLine.Gen_prod_posting_group
	taxEntry.Tax_bus_posting_group = genJnlLine.Tax_bus_posting_group
	taxEntry.Tax_prod_posting_group = genJnlLine.Tax_prod_posting_group
	taxEntry.Tax_area_code = genJnlLine.Tax_area_code
	taxEntry.Tax_liable = genJnlLine.Tax_liable
	taxEntry.Tax_group_code = genJnlLine.Tax_group_code
	taxEntry.Use_tax = genJnlLine.Use_tax
	taxEntry.Posting_date = genJnlLine.Posting_date
	taxEntry.Document_no = genJnlLine.Document_no
	taxEntry.External_document_no = genJnlLine.External_document_no
	taxEntry.Document_type = gtables.TaxEntryDocument_type(genJnlLine.Document_type)
	taxEntry.Type = gtables.TaxEntryType(genJnlLine.Gen_posting_type)
	taxEntry.Tax_calculation_type = gtables.TaxEntryTax_calculation_type(genJnlLine.Tax_calculation_type)
	taxEntry.Source_code = genJnlLine.Source_code
	taxEntry.Reason_code = genJnlLine.Reason_code
	taxEntry.Sell_to_buy_from_no = genJnlLine.Sell_to_buy_from_no
	taxEntry.Eu_3_party_trade = genJnlLine.Eu_3_party_trade
	taxEntry.Transaction_no = p.nextTransactionNo
	taxEntry.Sales_tax_connection_no = p.nextConnectionNo
	taxEntry.User_id = types.Code(system.UserId())
	taxEntry.No_series = genJnlLine.Posting_no_series
	if genJnlLine.Sell_to_buy_from_no != "" {
		switch taxEntry.Type {
		case gtables.TaxEntry_Type.Purchase:
			vend := gtables.NewVendor(p.src)
			err := vend.GetOrError(genJnlLine.Sell_to_buy_from_no)
			if err != nil {
				return err
			}
			taxEntry.Country_code = vend.Country_code
		case gtables.TaxEntry_Type.Sale:
			cust := gtables.NewCustomer(p.src)
			err := cust.GetOrError(genJnlLine.Sell_to_buy_from_no)
			if err != nil {
				return err
			}
			taxEntry.Country_code = cust.Country_code
		}
	}
	settlement := genJnlLine.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Settlement
	var taxJurisdiction *gtables.TaxJurisdiction
	if genJnlLine.Tax_calculation_type == gtables.GenJournalLine_Tax_calculation_type.SalesTax {
		taxJurisdiction = gtables.NewTaxJurisdiction(p.src)
		err := taxJurisdiction.GetOrError(taxDetail.Tax_jurisdiction_code)
		if err != nil {
			return err
		}
		if !settlement {
			taxEntry.Tax_group_used = taxDetail.Tax_group_code
			taxEntry.Tax_type = gtables.TaxEntryTax_type(taxDetail.Tax_type)
			taxEntry.Tax_on_tax = taxDetail.Calculate_tax_on_tax
		}
		taxEntry.Tax_jurisdiction_code = taxDetail.Tax_jurisdiction_code
	}

	// the amounts in the additional currency: at the G/L entry's factor, unless the line is in it
	if p.acyCode() != "" && p.acyCode() != srcCurrCode {
		precision := p.addCurrRoundingPrecision()
		srcCurrGLEntryAmt = system.Round(addCurrencyFactor.Mul(glEntryAmount), precision)
		srcCurrGLEntryTaxAmt = system.Round(addCurrencyFactor.Mul(glEntryTaxAmount), precision)
		srcCurrGLEntryBaseAmt = system.Round(addCurrencyFactor.Mul(glEntryBaseAmount), precision)
	}

	unrealizedTax := p.isUnrealizedTax(genJnlLine, taxJurisdiction)

	// Tax for Tax entry
	taxAmount := glEntryTaxAmount
	srcCurrTaxAmount := srcCurrGLEntryTaxAmt
	if settlement {
		// the settled amount is the G/L entry's
		taxAmount = glEntryAmount
		srcCurrTaxAmount = srcCurrGLEntryAmt
		taxEntry.Closed = true
	}
	taxBase := glEntryBaseAmount
	srcCurrTaxBase := srcCurrGLEntryBaseAmt
	if unrealizedTax {
		// realized when the customer or vendor entry is paid (custUnrealizedTax/vendUnrealizedTax)
		taxEntry.Amount = types.Decimal{}
		taxEntry.Base = types.Decimal{}
		taxEntry.Unrealized_amount = taxAmount
		taxEntry.Unrealized_base = taxBase
		taxEntry.Remaining_unrealized_amount = taxEntry.Unrealized_amount
		taxEntry.Remaining_unrealized_base = taxEntry.Unrealized_base
	} else {
		taxEntry.Amount = taxAmount
		taxEntry.Base = taxBase
	}
	if p.acyCode() != "" {
		if unrealizedTax {
			taxEntry.Add_currency_unrealized_base = srcCurrTaxBase
			taxEntry.Add_currency_unrealized_amt = srcCurrTaxAmount
		} else {
			taxEntry.Additional_currency_base = srcCurrTaxBase
			taxEntry.Additional_currency_amount = srcCurrTaxAmount
		}
	}
	taxEntry.Add_curr_rem_unreal_amount = taxEntry.Add_currency_unrealized_amt
	taxEntry.Add_curr_rem_unreal_base = taxEntry.Add_currency_unrealized_base
	err := taxEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	p.nextTaxEntryNo = p.nextTaxEntryNo + 1

	// Tax for G/L entry/entries
	if glEntryTaxAmount.IsZero() && (srcCurrGLEntryTaxAmt.IsZero() || p.acyCode() == "" || srcCurrCode != p.acyCode()) {
		return nil
	}
	if isVAT(genJnlLine.Tax_calculation_type) {
		return p.postVATGLEntries(genJnlLine, glEntryTaxAmount, srcCurrGLEntryTaxAmt, unrealizedTax)
	}
	switch genJnlLine.Gen_posting_type {
	case gtables.GenJournalLine_Gen_posting_type.Purchase:
		if genJnlLine.Use_tax {
			if unrealizedTax {
				err = p.postTaxGLEntry(taxJurisdiction, "unreal_tax_acc_purchases", taxJurisdiction.Unreal_tax_acc_purchases, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
				if err != nil {
					return err
				}
				return p.postTaxGLEntry(taxJurisdiction, "unreal_rev_charge_purch", taxJurisdiction.Unreal_rev_charge_purch, glEntryTaxAmount.Neg(), srcCurrGLEntryTaxAmt.Neg())
			}
			err = p.postTaxGLEntry(taxJurisdiction, "tax_account_purchases", taxJurisdiction.Tax_account_purchases, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
			if err != nil {
				return err
			}
			return p.postTaxGLEntry(taxJurisdiction, "reverse_charge_purchases", taxJurisdiction.Reverse_charge_purchases, glEntryTaxAmount.Neg(), srcCurrGLEntryTaxAmt.Neg())
		}
		if unrealizedTax {
			return p.postTaxGLEntry(taxJurisdiction, "unreal_tax_acc_purchases", taxJurisdiction.Unreal_tax_acc_purchases, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
		}
		return p.postTaxGLEntry(taxJurisdiction, "tax_account_purchases", taxJurisdiction.Tax_account_purchases, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
	case gtables.GenJournalLine_Gen_posting_type.Sale:
		if unrealizedTax {
			return p.postTaxGLEntry(taxJurisdiction, "unreal_tax_acc_sales", taxJurisdiction.Unreal_tax_acc_sales, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
		}
		return p.postTaxGLEntry(taxJurisdiction, "tax_account_sales", taxJurisdiction.Tax_account_sales, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
	}
	return nil
}

// isUnrealizedTax: the tax of an invoice, credit memo, finance charge memo or reminder whose Tax
// Posting Setup (VAT) or tax jurisdiction (sales tax) has an Unrealized Tax Type is unrealized —
// posted on the unrealized accounts and realized when the document is paid
func (p *GenJnlPostLine) isUnrealizedTax(genJnlLine *gtables.GenJournalLine, taxJurisdiction *gtables.TaxJurisdiction) bool {
	unrealizedTaxType := false
	if isVAT(gtables.GenJournalLineTax_calculation_type(p.taxPostingSetup.Tax_calculation_type)) {
		unrealizedTaxType = p.taxPostingSetup.Unrealized_tax_type > 0
	} else if taxJurisdiction != nil && p.taxPostingSetup.Tax_calculation_type == gtables.TaxPostingSetup_Tax_calculation_type.SalesTax {
		unrealizedTaxType = taxJurisdiction.Unrealized_tax_type > 0
	}
	return unrealizedTaxType &&
		(genJnlLine.Document_type == gtables.GenJournalLine_Document_type.Invoice ||
			genJnlLine.Document_type == gtables.GenJournalLine_Document_type.CreditMemo ||
			genJnlLine.Document_type == gtables.GenJournalLine_Document_type.FinanceChargeMemo ||
			genJnlLine.Document_type == gtables.GenJournalLine_Document_type.Reminder)
}

// postVATGLEntries posts the VAT of a line on the Tax Posting Setup's accounts: a purchase on
// the Purchase VAT Account (reverse charge: and back on the Reverse Chrg. VAT Acc.), a sale on
// the Sales VAT Account (reverse charge: none) — unrealized VAT on their unrealized accounts
func (p *GenJnlPostLine) postVATGLEntries(genJnlLine *gtables.GenJournalLine, glEntryTaxAmount, srcCurrGLEntryTaxAmt types.Decimal, unrealizedTax bool) error {
	taxPostingSetup := p.taxPostingSetup
	switch genJnlLine.Gen_posting_type {
	case gtables.GenJournalLine_Gen_posting_type.Purchase:
		switch taxPostingSetup.Tax_calculation_type {
		case gtables.TaxPostingSetup_Tax_calculation_type.NormalVAT, gtables.TaxPostingSetup_Tax_calculation_type.FullVAT:
			if unrealizedTax {
				return p.postVATGLEntry("purch_vat_unreal_account", taxPostingSetup.Purch_vat_unreal_account, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
			}
			return p.postVATGLEntry("purchase_vat_account", taxPostingSetup.Purchase_vat_account, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
		case gtables.TaxPostingSetup_Tax_calculation_type.ReverseChargeVAT:
			if unrealizedTax {
				err := p.postVATGLEntry("purch_vat_unreal_account", taxPostingSetup.Purch_vat_unreal_account, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
				if err != nil {
					return err
				}
				return p.postVATGLEntry("reverse_chrg_vat_unreal_acc", taxPostingSetup.Reverse_chrg_vat_unreal_acc, glEntryTaxAmount.Neg(), srcCurrGLEntryTaxAmt.Neg())
			}
			err := p.postVATGLEntry("purchase_vat_account", taxPostingSetup.Purchase_vat_account, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
			if err != nil {
				return err
			}
			return p.postVATGLEntry("reverse_chrg_vat_acc", taxPostingSetup.Reverse_chrg_vat_acc, glEntryTaxAmount.Neg(), srcCurrGLEntryTaxAmt.Neg())
		}
	case gtables.GenJournalLine_Gen_posting_type.Sale:
		switch taxPostingSetup.Tax_calculation_type {
		case gtables.TaxPostingSetup_Tax_calculation_type.NormalVAT, gtables.TaxPostingSetup_Tax_calculation_type.FullVAT:
			if unrealizedTax {
				return p.postVATGLEntry("sales_vat_unreal_account", taxPostingSetup.Sales_vat_unreal_account, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
			}
			return p.postVATGLEntry("sales_vat_account", taxPostingSetup.Sales_vat_account, glEntryTaxAmount, srcCurrGLEntryTaxAmt)
		}
	}
	return nil
}

// postVATGLEntry posts VAT on one of the Tax Posting Setup's accounts (it must have one)
func (p *GenJnlPostLine) postVATGLEntry(field string, account types.Code, amount, amountAddCurr types.Decimal) error {
	err := p.taxPostingSetup.TestField(field)
	if err != nil {
		return err
	}
	_, err = p.initGLEntry(account, amount, amountAddCurr, true, true)
	if err != nil {
		return err
	}
	return p.insertGLEntry(true)
}

// postTaxGLEntry posts tax on a jurisdiction's account (summarized when the General Ledger
// Setup says so)
func (p *GenJnlPostLine) postTaxGLEntry(taxJurisdiction *gtables.TaxJurisdiction, field string, account types.Code, amount, amountAddCurr types.Decimal) error {
	err := taxJurisdiction.TestField(field)
	if err != nil {
		return err
	}
	_, err = p.initGLEntry(account, amount, amountAddCurr, true, true)
	if err != nil {
		return err
	}
	return p.summarizeTax()
}

// summarizeTax: with Summarize G/L Entries the tax G/L entries of a line on the same account are
// added up (written by insertSummarizedTax), otherwise each is written
func (p *GenJnlPostLine) summarizeTax() error {
	if !p.glSetup.Summarize_gl_entries {
		return p.insertGLEntry(true)
	}
	for _, glEntryTmpTax := range p.glEntryTmpTax {
		if glEntryTmpTax.Gl_account_no == p.glEntry.Gl_account_no && glEntryTmpTax.Bal_account_no == p.glEntry.Bal_account_no {
			glEntryTmpTax.Amount = glEntryTmpTax.Amount.Add(p.glEntry.Amount)
			glEntryTmpTax.Additional_currency_amount = glEntryTmpTax.Additional_currency_amount.Add(p.glEntry.Additional_currency_amount)
			return nil
		}
	}
	p.glEntryTmpTax = append(p.glEntryTmpTax, p.glEntry)
	return nil
}

// insertSummarizedTax writes the summarized tax G/L entries; the next line's tax entries get a
// new connection number
func (p *GenJnlPostLine) insertSummarizedTax() error {
	if p.glSetup.Summarize_gl_entries {
		for _, glEntryTmpTax := range p.glEntryTmpTax {
			p.glEntry = glEntryTmpTax
			p.glEntry.Entry_no = p.nextEntryNo
			err := p.insertGLEntry(true)
			if err != nil {
				return err
			}
		}
		p.glEntryTmpTax = nil
	}
	p.nextConnectionNo = p.nextConnectionNo + 1
	return nil
}
