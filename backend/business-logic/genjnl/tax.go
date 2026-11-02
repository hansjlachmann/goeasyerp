package genjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/salestax"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Sales tax of the journal lines (codeunit 12 InitTax, PostTax, InsertTax): a G/L line with a
// general posting type takes the tax out of its amount (a sale includes it; a purchase with
// use tax pays it on top), writes a tax entry per jurisdiction and posts the tax to the
// jurisdiction's accounts. A settlement line (Manual VAT Entry, posting type Settlement, the
// jurisdiction in Tax Area Code) writes one closed tax entry of its amount.
//
// Not migrated yet: the VAT calculation types (Normal, Reverse Charge, Full VAT), unrealized tax
// (realized when the entries are applied), the additional reporting currency.

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
	if genJnlLine.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Blank {
		return nil
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
	if genJnlLine.Tax_calculation_type != gtables.GenJournalLine_Tax_calculation_type.SalesTax {
		return apperrors.JnlTaxNotYet(jnlLine, "tax_calculation_type", genJnlLine.Tax_calculation_type)
	}
	switch genJnlLine.Vat_posting {
	case gtables.GenJournalLine_Vat_posting.AutomaticVATEntry:
		p.setGLEntryTaxFields(genJnlLine)
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
	case gtables.GenJournalLine_Vat_posting.ManualVATEntry:
		if genJnlLine.Gen_posting_type != gtables.GenJournalLine_Gen_posting_type.Settlement {
			p.setGLEntryTaxFields(genJnlLine)
			p.glEntry.Tax_amount = genJnlLine.Tax_amount_lcy
		}
	}
	return nil
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

// postTax writes the line's tax entries — one per jurisdiction of its tax area — and their G/L
// entries
func (p *GenJnlPostLine) postTax(genJnlLine *gtables.GenJournalLine) error {
	if genJnlLine.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Blank ||
		genJnlLine.Tax_calculation_type != gtables.GenJournalLine_Tax_calculation_type.SalesTax {
		return nil
	}
	glEntryAmount := p.glEntry.Amount
	glEntryTaxAmount := p.glEntry.Tax_amount
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
		return p.insertTax(genJnlLine, taxDetail, glEntryAmount, glEntryTaxAmount, genJnlLine.Tax_base_amount_lcy)
	}
	p.salesTaxCalculate = salestax.New(p.src)
	err := p.salesTaxCalculate.InitSalesTaxLines(genJnlLine.Tax_area_code, genJnlLine.Tax_group_code, genJnlLine.Tax_liable,
		salesTaxBaseAmount, genJnlLine.Quantity, genJnlLine.Posting_date, glEntryTaxAmount)
	if err != nil {
		return err
	}
	for {
		taxDetail, taxAmount, taxBase, ok := p.salesTaxCalculate.GetSalesTaxLine()
		if !ok {
			break
		}
		err = p.insertTax(genJnlLine, taxDetail, salesTaxBaseAmount, taxAmount, taxBase)
		if err != nil {
			return err
		}
	}
	return p.insertSummarizedTax()
}

// insertTax writes the tax entry of one jurisdiction and posts its tax to the jurisdiction's
// accounts (use tax: the purchase tax account and the reverse charge account)
func (p *GenJnlPostLine) insertTax(genJnlLine *gtables.GenJournalLine, taxDetail *gtables.TaxDetail, glEntryAmount, glEntryTaxAmount, glEntryBaseAmount types.Decimal) error {
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
	taxJurisdiction := gtables.NewTaxJurisdiction(p.src)
	err := taxJurisdiction.GetOrError(taxDetail.Tax_jurisdiction_code)
	if err != nil {
		return err
	}
	settlement := genJnlLine.Gen_posting_type == gtables.GenJournalLine_Gen_posting_type.Settlement
	if !settlement {
		taxEntry.Tax_group_used = taxDetail.Tax_group_code
		taxEntry.Tax_type = gtables.TaxEntryTax_type(taxDetail.Tax_type)
		taxEntry.Tax_on_tax = taxDetail.Calculate_tax_on_tax
	}
	taxEntry.Tax_jurisdiction_code = taxDetail.Tax_jurisdiction_code
	unrealizedTax := taxJurisdiction.Unrealized_tax_type > 0 &&
		(genJnlLine.Document_type == gtables.GenJournalLine_Document_type.Invoice ||
			genJnlLine.Document_type == gtables.GenJournalLine_Document_type.CreditMemo ||
			genJnlLine.Document_type == gtables.GenJournalLine_Document_type.FinanceChargeMemo ||
			genJnlLine.Document_type == gtables.GenJournalLine_Document_type.Reminder)
	if unrealizedTax {
		return apperrors.JnlUnrealizedTaxNotYet()
	}
	taxEntry.Amount = glEntryTaxAmount
	if settlement {
		// the settled amount is the G/L entry's
		taxEntry.Amount = glEntryAmount
		taxEntry.Closed = true
	}
	taxEntry.Base = glEntryBaseAmount
	err = taxEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	p.nextTaxEntryNo = p.nextTaxEntryNo + 1

	// Tax for G/L entry/entries
	if glEntryTaxAmount.IsZero() {
		return nil
	}
	switch genJnlLine.Gen_posting_type {
	case gtables.GenJournalLine_Gen_posting_type.Purchase:
		if genJnlLine.Use_tax {
			err = p.postTaxGLEntry(taxJurisdiction, "tax_account_purchases", taxJurisdiction.Tax_account_purchases, glEntryTaxAmount)
			if err != nil {
				return err
			}
			return p.postTaxGLEntry(taxJurisdiction, "reverse_charge_purchases", taxJurisdiction.Reverse_charge_purchases, glEntryTaxAmount.Neg())
		}
		return p.postTaxGLEntry(taxJurisdiction, "tax_account_purchases", taxJurisdiction.Tax_account_purchases, glEntryTaxAmount)
	case gtables.GenJournalLine_Gen_posting_type.Sale:
		return p.postTaxGLEntry(taxJurisdiction, "tax_account_sales", taxJurisdiction.Tax_account_sales, glEntryTaxAmount)
	}
	return nil
}

// postTaxGLEntry posts tax on a jurisdiction's account (summarized when the General Ledger
// Setup says so)
func (p *GenJnlPostLine) postTaxGLEntry(taxJurisdiction *gtables.TaxJurisdiction, field string, account types.Code, amount types.Decimal) error {
	err := taxJurisdiction.TestField(field)
	if err != nil {
		return err
	}
	_, err = p.initGLEntry(account, amount, true)
	if err != nil {
		return err
	}
	return p.summarizeTax()
}

// summarizeTax: with Summarize G/L Entries the tax G/L entries of a line on the same account are
// added up (written by insertSummarizedTax), otherwise each is written
func (p *GenJnlPostLine) summarizeTax() error {
	if !p.glSetup.Summarize_gl_entries {
		return p.insertGLEntry()
	}
	for _, glEntryTmpTax := range p.glEntryTmpTax {
		if glEntryTmpTax.Gl_account_no == p.glEntry.Gl_account_no && glEntryTmpTax.Bal_account_no == p.glEntry.Bal_account_no {
			glEntryTmpTax.Amount = glEntryTmpTax.Amount.Add(p.glEntry.Amount)
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
			err := p.insertGLEntry()
			if err != nil {
				return err
			}
		}
		p.glEntryTmpTax = nil
	}
	p.nextConnectionNo = p.nextConnectionNo + 1
	return nil
}
