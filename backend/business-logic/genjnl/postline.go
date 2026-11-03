package genjnl

import (
	"fmt"

	"github.com/hansjlachmann/openerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/salestax"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// GenJnlPostLine is codeunit 12 Gen. Jnl.-Post Line: it posts journal lines one by one to the
// ledgers — G/L entries (with the G/L Register of the posting), bank account and check ledger
// entries, customer and vendor entries. One instance posts a whole batch: it keeps the next
// entry and transaction numbers and the register; open it on the posting's transaction.
//
// A fixed asset line posts its FA or maintenance entry (codeunit 5632, fajnl) and the G/L entries
// of its accounts.
//
// With an additional reporting currency every G/L entry also carries its amount in that currency
// (addcurrency.go).
//
// Not migrated yet: computer checks and electronic payments (check printing), job ledger
// entries.
type GenJnlPostLine struct {
	src             ftables.Source
	genJnlLine      *gtables.GenJournalLine
	checkLine       bool
	genJnlCheckLine GenJnlCheckLine

	glSetup        *gtables.GeneralLedgerSetup
	genJnlTemplate *gtables.GenJournalTemplate
	currency       *gtables.Currency
	glReg          *gtables.GLRegister
	glEntry        *gtables.GLEntry
	glEntryTmp     []*gtables.GLEntry

	nextEntryNo         int
	nextTransactionNo   int
	nextCheckEntryNo    int
	lastDocType         gtables.GenJournalLineDocument_type
	lastDocNo           types.Code
	lastLineNo          int
	lastDate            types.Date
	currentBalance      types.Decimal
	fiscalYearStartDate types.Date
	balanceCheck        *balanceCheck
	faJnlPostLine       *fajnl.FAJnlPostLine
	addCurr             addCurrency

	// Customer and vendor application of the current line
	custPosting               bool
	vendPosting               bool
	salesPostingType          bool
	purchPostingType          bool
	totalPmtDiscLCY           types.Decimal
	totalApplnRoundingLCY     types.Decimal
	totalPmtDiscAddCurr       types.Decimal
	totalApplnRoundingAddCurr types.Decimal
	realExchRateGains         types.Decimal
	realExchRateLosses        types.Decimal
	unrealExchRateGains       types.Decimal
	unrealExchRateLosses      types.Decimal
	gainsLossesBuffer         map[types.Code]*gainsLosses
	gainsLossesOrder          []types.Code

	// Tax entries
	nextTaxEntryNo     int
	firstNewTaxEntryNo int
	nextConnectionNo   int
	taxPostingSetup    *gtables.TaxPostingSetup
	salesTaxCalculate  *salestax.SalesTaxCalculate
	glEntryTmpTax      []*gtables.GLEntry
	// the tax of the line's G/L entry in the additional currency (AddCurrGLEntryTaxAmt)
	addCurrGLEntryTaxAmt types.Decimal
}

// NewGenJnlPostLine opens the posting codeunit in the posting's database (its transaction)
func NewGenJnlPostLine(src ftables.Source) *GenJnlPostLine {
	return &GenJnlPostLine{src: src}
}

// Run checks and posts a journal line (OnRun)
func (p *GenJnlPostLine) Run(genJnlLine *gtables.GenJournalLine) error {
	p.genJnlLine = gtables.NewGenJournalLine(p.src)
	p.genJnlLine.Copy(genJnlLine)
	p.checkLine = true
	return p.code()
}

// RunWithoutCheck posts a journal line its caller checked (Gen. Jnl.-Post Batch)
func (p *GenJnlPostLine) RunWithoutCheck(genJnlLine2 *gtables.GenJournalLine) error {
	p.genJnlLine = gtables.NewGenJournalLine(p.src)
	p.genJnlLine.Copy(genJnlLine2)
	p.checkLine = false
	return p.code()
}

// GetGLReg returns the G/L Register of the posting (No. 0 before the first line)
func (p *GenJnlPostLine) GetGLReg() *gtables.GLRegister {
	if p.glReg == nil {
		return gtables.NewGLRegister(p.src)
	}
	return p.glReg
}

func (p *GenJnlPostLine) code() error {
	genJnlLine := p.genJnlLine
	if tables.GenJnlLineEmptyLine(genJnlLine) {
		p.lastDocType = genJnlLine.Document_type
		p.lastDocNo = genJnlLine.Document_no
		p.lastLineNo = genJnlLine.Line_no
		p.lastDate = genJnlLine.Posting_date
		return nil
	}
	if p.checkLine {
		err := p.genJnlCheckLine.RunCheck(genJnlLine)
		if err != nil {
			return err
		}
	}
	p.currency = gtables.NewCurrency(p.src)
	if genJnlLine.Currency_code == "" {
		tables.InitRoundingPrecision(p.currency)
		genJnlLine.Amount_lcy = genJnlLine.Amount
		genJnlLine.Tax_amount_lcy = genJnlLine.Tax_amount
		genJnlLine.Tax_base_amount_lcy = genJnlLine.Tax_base_amount
	} else {
		err := p.currency.GetOrError(genJnlLine.Currency_code)
		if err != nil {
			return err
		}
		err = p.currency.TestField("amount_rounding_precision")
		if err != nil {
			return err
		}
		if !genJnlLine.System_created_entry {
			genJnlLine.Source_currency_code = genJnlLine.Currency_code
			genJnlLine.Source_currency_amount = genJnlLine.Amount
			genJnlLine.Source_curr_tax_base_amount = genJnlLine.Tax_base_amount
			genJnlLine.Source_curr_tax_amount = genJnlLine.Tax_amount
		}
	}
	if genJnlLine.Additional_currency_posting == gtables.GenJournalLine_Additional_currency_posting.None {
		if !genJnlLine.Amount.Equal(system.Round(genJnlLine.Amount, p.currency.Amount_rounding_precision)) {
			return apperrors.JnlNeedsRounding(jnlLine, "amount", genJnlLine.Amount.String())
		}
		if !genJnlLine.Amount_lcy.Equal(system.Round(genJnlLine.Amount_lcy)) {
			return apperrors.JnlNeedsRounding(jnlLine, "amount_lcy", genJnlLine.Amount_lcy.String())
		}
	}
	if genJnlLine.Sell_to_buy_from_no == "" {
		switch {
		case genJnlLine.Account_type == gtables.GenJournalLine_Account_type.Customer ||
			genJnlLine.Account_type == gtables.GenJournalLine_Account_type.Vendor:
			genJnlLine.Sell_to_buy_from_no = genJnlLine.Account_no
		case genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Customer ||
			genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Vendor:
			genJnlLine.Sell_to_buy_from_no = genJnlLine.Bal_account_no
		}
	}
	if genJnlLine.Document_date.IsZero() {
		genJnlLine.Document_date = genJnlLine.Posting_date
	}
	if genJnlLine.Due_date.IsZero() {
		genJnlLine.Due_date = genJnlLine.Posting_date
	}
	err := p.initCodeUnit()
	if err != nil {
		return err
	}
	if genJnlLine.Account_no != "" && genJnlLine.Bal_account_no != "" && !genJnlLine.System_created_entry &&
		isCustVendFA(int(genJnlLine.Account_type)) {
		ExchAccGLJnlLine(genJnlLine)
	}
	if genJnlLine.Account_no != "" {
		err = p.postAccount(genJnlLine)
		if err != nil {
			return err
		}
	}
	if genJnlLine.Bal_account_no != "" {
		ExchAccGLJnlLine(genJnlLine)
		if genJnlLine.Account_no != "" {
			err = p.postAccount(genJnlLine)
			if err != nil {
				return err
			}
		}
	}
	return p.finishCodeunit()
}

// postAccount posts the line's account side
func (p *GenJnlPostLine) postAccount(genJnlLine *gtables.GenJournalLine) error {
	switch genJnlLine.Account_type {
	case gtables.GenJournalLine_Account_type.GLAccount:
		return p.postGLAcc(genJnlLine)
	case gtables.GenJournalLine_Account_type.Customer:
		return p.postCust(genJnlLine)
	case gtables.GenJournalLine_Account_type.Vendor:
		return p.postVend(genJnlLine)
	case gtables.GenJournalLine_Account_type.BankAccount:
		return p.postBankAcc(genJnlLine)
	case gtables.GenJournalLine_Account_type.FixedAsset:
		return p.postFixedAsset(genJnlLine)
	}
	return nil
}

// ExchAccGLJnlLine is codeunit 366 Exchange Acc. G/L Journal Line: the line's account and
// balancing account (with their posting groups and tax) change places and the amounts their
// sign, so the balancing account is posted as the account
func ExchAccGLJnlLine(rec *gtables.GenJournalLine) {
	genJnlLine2 := gtables.NewGenJournalLine(rec)
	genJnlLine2.Copy(rec)
	rec.Account_type = gtables.GenJournalLineAccount_type(genJnlLine2.Bal_account_type)
	rec.Account_no = genJnlLine2.Bal_account_no
	rec.Tax_percent = genJnlLine2.Bal_tax_percent
	rec.Tax_amount = genJnlLine2.Bal_tax_amount
	rec.Tax_amount_lcy = genJnlLine2.Bal_tax_amount_lcy
	rec.Gen_posting_type = gtables.GenJournalLineGen_posting_type(genJnlLine2.Bal_gen_posting_type)
	rec.Gen_bus_posting_group = genJnlLine2.Bal_gen_bus_posting_group
	rec.Gen_prod_posting_group = genJnlLine2.Bal_gen_prod_posting_group
	rec.Tax_bus_posting_group = genJnlLine2.Bal_tax_bus_posting_group
	rec.Tax_prod_posting_group = genJnlLine2.Bal_tax_prod_posting_group
	rec.Tax_calculation_type = gtables.GenJournalLineTax_calculation_type(genJnlLine2.Bal_tax_calculation_type)
	rec.Tax_base_amount = genJnlLine2.Bal_tax_base_amount
	rec.Tax_base_amount_lcy = genJnlLine2.Bal_tax_base_amount_lcy
	rec.Tax_area_code = genJnlLine2.Bal_tax_area_code
	rec.Tax_liable = genJnlLine2.Bal_tax_liable
	rec.Tax_group_code = genJnlLine2.Bal_tax_group_code
	rec.Use_tax = genJnlLine2.Bal_use_tax
	rec.Bal_account_type = gtables.GenJournalLineBal_account_type(genJnlLine2.Account_type)
	rec.Bal_account_no = genJnlLine2.Account_no
	rec.Bal_tax_percent = genJnlLine2.Tax_percent
	rec.Bal_tax_amount = genJnlLine2.Tax_amount
	rec.Bal_tax_amount_lcy = genJnlLine2.Tax_amount_lcy
	rec.Bal_gen_posting_type = gtables.GenJournalLineBal_gen_posting_type(genJnlLine2.Gen_posting_type)
	rec.Bal_gen_bus_posting_group = genJnlLine2.Gen_bus_posting_group
	rec.Bal_gen_prod_posting_group = genJnlLine2.Gen_prod_posting_group
	rec.Bal_tax_bus_posting_group = genJnlLine2.Tax_bus_posting_group
	rec.Bal_tax_prod_posting_group = genJnlLine2.Tax_prod_posting_group
	rec.Bal_tax_calculation_type = gtables.GenJournalLineBal_tax_calculation_type(genJnlLine2.Tax_calculation_type)
	rec.Bal_tax_base_amount = genJnlLine2.Tax_base_amount
	rec.Bal_tax_base_amount_lcy = genJnlLine2.Tax_base_amount_lcy
	rec.Bal_tax_area_code = genJnlLine2.Tax_area_code
	rec.Bal_tax_liable = genJnlLine2.Tax_liable
	rec.Bal_tax_group_code = genJnlLine2.Tax_group_code
	rec.Bal_use_tax = genJnlLine2.Use_tax
	rec.Amount = genJnlLine2.Amount.Neg()
	rec.Debit_amount = genJnlLine2.Credit_amount
	rec.Credit_amount = genJnlLine2.Debit_amount
	rec.Amount_lcy = genJnlLine2.Amount_lcy.Neg()
	rec.Balance_lcy = genJnlLine2.Balance_lcy.Neg()
	if rec.Currency_code != "" && !rec.System_created_entry {
		rec.Source_currency_amount = rec.Amount
		rec.Source_curr_tax_base_amount = rec.Tax_base_amount
		rec.Source_curr_tax_amount = rec.Tax_amount
	}
}

// postGLAcc posts a G/L account line: its G/L entry (and its tax)
func (p *GenJnlPostLine) postGLAcc(genJnlLine *gtables.GenJournalLine) error {
	glAcc, err := p.initGLEntry(genJnlLine.Account_no, genJnlLine.Amount_lcy, genJnlLine.Source_currency_amount, true, genJnlLine.System_created_entry)
	if err != nil {
		return err
	}
	if !genJnlLine.System_created_entry {
		if !genJnlLine.Posting_date.IsClosing() {
			err = glAcc.TestField("direct_posting", true)
			if err != nil {
				return err
			}
		}
	}
	p.glEntry.Gen_posting_type = gtables.GLEntryGen_posting_type(genJnlLine.Gen_posting_type)
	p.glEntry.Bal_account_type = gtables.GLEntryBal_account_type(genJnlLine.Bal_account_type)
	p.glEntry.Bal_account_no = genJnlLine.Bal_account_no
	p.glEntry.No_series = genJnlLine.Posting_no_series
	if genJnlLine.Additional_currency_posting == gtables.GenJournalLine_Additional_currency_posting.AdditionalCurrencyAmountOnly {
		p.glEntry.Additional_currency_amount = genJnlLine.Amount
		p.glEntry.Amount = types.Decimal{}
	}
	err = p.initTax(genJnlLine)
	if err != nil {
		return err
	}
	err = p.insertGLEntry(true)
	if err != nil {
		return err
	}
	return p.postTax(genJnlLine)
}

// postBankAcc posts a bank account line: its bank account ledger entry, for a manual check its
// check ledger entry, and the G/L entry on the bank's posting group account
func (p *GenJnlPostLine) postBankAcc(genJnlLine *gtables.GenJournalLine) error {
	bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(p.src)
	bankAccLedgEntry.LockTable()
	bankAcc := gtables.NewBankAccount(p.src)
	err := bankAcc.GetOrError(genJnlLine.Account_no)
	if err != nil {
		return err
	}
	err = bankAcc.TestField("blocked", false)
	if err != nil {
		return err
	}
	if genJnlLine.Currency_code == "" {
		err = bankAcc.TestField("currency_code", types.Code(""))
		if err != nil {
			return err
		}
	} else {
		if bankAcc.Currency_code != "" {
			err = genJnlLine.TestField("currency_code", bankAcc.Currency_code)
			if err != nil {
				return err
			}
		}
	}
	err = bankAcc.TestField("bank_acc_posting_group")
	if err != nil {
		return err
	}
	bankAccPostingGr := gtables.NewBankAccountPostingGroup(p.src)
	err = bankAccPostingGr.GetOrError(bankAcc.Bank_acc_posting_group)
	if err != nil {
		return err
	}
	bankAccLedgEntry.Init()
	bankAccLedgEntry.Bank_account_no = genJnlLine.Account_no
	bankAccLedgEntry.Posting_date = genJnlLine.Posting_date
	bankAccLedgEntry.Document_date = genJnlLine.Document_date
	bankAccLedgEntry.Document_type = gtables.BankAccountLedgerEntryDocument_type(genJnlLine.Document_type)
	bankAccLedgEntry.Document_no = genJnlLine.Document_no
	bankAccLedgEntry.External_document_no = genJnlLine.External_document_no
	bankAccLedgEntry.Description = genJnlLine.Description
	bankAccLedgEntry.Bank_acc_posting_group = bankAcc.Bank_acc_posting_group
	bankAccLedgEntry.Dimension_1_code = genJnlLine.Dimension_1_code
	bankAccLedgEntry.Dimension_2_code = genJnlLine.Dimension_2_code
	bankAccLedgEntry.Our_contact_code = genJnlLine.Salespers_purch_code
	bankAccLedgEntry.Source_code = genJnlLine.Source_code
	bankAccLedgEntry.Journal_batch_name = genJnlLine.Journal_batch_name
	bankAccLedgEntry.Reason_code = genJnlLine.Reason_code
	bankAccLedgEntry.Entry_no = p.nextEntryNo
	bankAccLedgEntry.Transaction_no = p.nextTransactionNo
	bankAccLedgEntry.Currency_code = bankAcc.Currency_code
	if bankAcc.Currency_code != "" {
		bankAccLedgEntry.Amount = genJnlLine.Amount
	} else {
		bankAccLedgEntry.Amount = genJnlLine.Amount_lcy
	}
	bankAccLedgEntry.Amount_lcy = genJnlLine.Amount_lcy
	bankAccLedgEntry.User_id = types.Code(system.UserId())
	if !bankAccLedgEntry.Amount.IsZero() {
		bankAccLedgEntry.Open = true
		bankAccLedgEntry.Remaining_amount = bankAccLedgEntry.Amount
	}
	bankAccLedgEntry.Positive = bankAccLedgEntry.Amount.IsPositive()
	bankAccLedgEntry.Bal_account_type = gtables.BankAccountLedgerEntryBal_account_type(genJnlLine.Bal_account_type)
	bankAccLedgEntry.Bal_account_no = genJnlLine.Bal_account_no
	if genJnlLine.Amount.IsPositive() && !genJnlLine.Correction ||
		genJnlLine.Amount_lcy.IsPositive() && !genJnlLine.Correction ||
		genJnlLine.Amount.IsNegative() && genJnlLine.Correction ||
		genJnlLine.Amount_lcy.IsNegative() && genJnlLine.Correction {
		bankAccLedgEntry.Debit_amount = bankAccLedgEntry.Amount
		bankAccLedgEntry.Credit_amount = types.Decimal{}
		bankAccLedgEntry.Debit_amount_lcy = bankAccLedgEntry.Amount_lcy
		bankAccLedgEntry.Credit_amount_lcy = types.Decimal{}
	} else {
		bankAccLedgEntry.Debit_amount = types.Decimal{}
		bankAccLedgEntry.Credit_amount = bankAccLedgEntry.Amount.Neg()
		bankAccLedgEntry.Debit_amount_lcy = types.Decimal{}
		bankAccLedgEntry.Credit_amount_lcy = bankAccLedgEntry.Amount_lcy.Neg()
	}
	err = bankAccLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}

	if (genJnlLine.Amount.IsNegative() ||
		genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.Employee && genJnlLine.Amount.IsZero()) &&
		(genJnlLine.Bank_payment_type == gtables.GenJournalLine_Bank_payment_type.ComputerCheck && genJnlLine.Check_printed ||
			genJnlLine.Bank_payment_type == gtables.GenJournalLine_Bank_payment_type.ElectronicPayment ||
			genJnlLine.Bank_payment_type == gtables.GenJournalLine_Bank_payment_type.ManualCheck) {
		if bankAcc.Currency_code != genJnlLine.Currency_code {
			return apperrors.JnlBankPmtCurrency(jnlLine, "bank_payment_type", bankAcc.No.String())
		}
		switch genJnlLine.Bank_payment_type {
		case gtables.GenJournalLine_Bank_payment_type.ComputerCheck, gtables.GenJournalLine_Bank_payment_type.ElectronicPayment:
			return apperrors.JnlCheckPrintingNotYet(jnlLine, "bank_payment_type", genJnlLine.Bank_payment_type)
		case gtables.GenJournalLine_Bank_payment_type.ManualCheck:
			err = p.insertManualCheck(genJnlLine, bankAcc, bankAccLedgEntry)
			if err != nil {
				return err
			}
		}
	}

	err = bankAccPostingGr.TestField("g_l_bank_account_no")
	if err != nil {
		return err
	}
	_, err = p.initGLEntry(bankAccPostingGr.G_l_bank_account_no, genJnlLine.Amount_lcy, genJnlLine.Source_currency_amount, true, true)
	if err != nil {
		return err
	}
	p.glEntry.Bal_account_type = gtables.GLEntryBal_account_type(genJnlLine.Bal_account_type)
	p.glEntry.Bal_account_no = genJnlLine.Bal_account_no
	return p.insertGLEntry(true)
}

// insertManualCheck records a manual check in the check ledger (its check no. is the document
// no.; a check no. used before for the bank account is refused)
func (p *GenJnlPostLine) insertManualCheck(genJnlLine *gtables.GenJournalLine, bankAcc *gtables.BankAccount, bankAccLedgEntry *gtables.BankAccountLedgerEntry) error {
	if genJnlLine.Document_no == "" {
		return apperrors.JnlRequiredWhen(jnlLine, "bank_payment_type", "document_no", genJnlLine.Bank_payment_type)
	}
	checkLedgEntry := gtables.NewCheckLedgerEntry(p.src)
	if p.nextCheckEntryNo == 0 {
		checkLedgEntry.LockTable()
		if checkLedgEntry.FindLast() {
			p.nextCheckEntryNo = checkLedgEntry.Entry_no + 1
		} else {
			p.nextCheckEntryNo = 1
		}
	}
	checkLedgEntry.Reset()
	checkLedgEntry.SetCurrentKey("bank_account_no", "entry_status", "check_no")
	checkLedgEntry.SetRange("bank_account_no", genJnlLine.Account_no)
	checkLedgEntry.SetFilter("entry_status", fmt.Sprintf("%d|%d|%d", gtables.CheckLedgerEntry_Entry_status.Printed, gtables.CheckLedgerEntry_Entry_status.Posted, gtables.CheckLedgerEntry_Entry_status.FinanciallyVoided))
	checkLedgEntry.SetRange("check_no", genJnlLine.Document_no)
	if checkLedgEntry.FindFirst() {
		return apperrors.JnlCheckExists(genJnlLine.Document_no.String())
	}
	checkLedgEntry.Init()
	checkLedgEntry.Entry_no = p.nextCheckEntryNo
	checkLedgEntry.Bank_account_no = bankAccLedgEntry.Bank_account_no
	checkLedgEntry.Bank_account_ledger_entry_no = bankAccLedgEntry.Entry_no
	checkLedgEntry.Posting_date = bankAccLedgEntry.Posting_date
	checkLedgEntry.Document_type = gtables.CheckLedgerEntryDocument_type(bankAccLedgEntry.Document_type)
	checkLedgEntry.Document_no = bankAccLedgEntry.Document_no
	checkLedgEntry.External_document_no = bankAccLedgEntry.External_document_no
	checkLedgEntry.Description = bankAccLedgEntry.Description
	checkLedgEntry.Bank_payment_type = gtables.CheckLedgerEntryBank_payment_type(genJnlLine.Bank_payment_type)
	checkLedgEntry.Bal_account_type = gtables.CheckLedgerEntryBal_account_type(bankAccLedgEntry.Bal_account_type)
	checkLedgEntry.Bal_account_no = bankAccLedgEntry.Bal_account_no
	checkLedgEntry.Entry_status = gtables.CheckLedgerEntry_Entry_status.Posted
	checkLedgEntry.User_id = types.Code(system.UserId())
	checkLedgEntry.Check_date = bankAccLedgEntry.Posting_date
	checkLedgEntry.Check_no = bankAccLedgEntry.Document_no
	if bankAcc.Currency_code != "" {
		checkLedgEntry.Amount = genJnlLine.Amount.Neg()
	} else {
		checkLedgEntry.Amount = genJnlLine.Amount_lcy.Neg()
	}
	checkLedgEntry.Open = !checkLedgEntry.Amount.IsZero()
	err := checkLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	p.nextCheckEntryNo = p.nextCheckEntryNo + 1
	return nil
}

// initCodeUnit: the first line takes the next entry and transaction numbers (G/L Entry locked)
// and starts the G/L Register; a new document, date or line starts a new transaction
func (p *GenJnlPostLine) initCodeUnit() error {
	genJnlLine := p.genJnlLine
	if p.nextEntryNo == 0 {
		glEntry := gtables.NewGLEntry(p.src)
		glEntry.LockTable()
		if glEntry.FindLast() {
			p.nextEntryNo = glEntry.Entry_no + 1
			p.nextTransactionNo = glEntry.Transaction_no + 1
		} else {
			p.nextEntryNo = 1
			p.nextTransactionNo = 1
		}
		p.lastDocType = genJnlLine.Document_type
		p.lastDocNo = genJnlLine.Document_no
		p.lastLineNo = genJnlLine.Line_no
		p.lastDate = genJnlLine.Posting_date
		p.currentBalance = types.Decimal{}

		accountingPeriod := gtables.NewAccountingPeriod(p.src)
		accountingPeriod.SetCurrentKey("closed")
		accountingPeriod.SetRange("closed", false)
		if accountingPeriod.FindFirst() {
			p.fiscalYearStartDate = accountingPeriod.Starting_date
		}
		p.glSetup = gtables.GetGeneralLedgerSetup(p.src)
		p.genJnlTemplate = gtables.NewGenJournalTemplate(p.src)
		if !p.genJnlTemplate.Get(genJnlLine.Journal_template_name) {
			p.genJnlTemplate.InitRecord()
		}

		p.glReg = gtables.NewGLRegister(p.src)
		p.glReg.LockTable()
		no := 1
		if p.glReg.FindLast() {
			no = p.glReg.No + 1
		}
		p.glReg.Init()
		p.glReg.No = no
		p.glReg.From_entry_no = p.nextEntryNo
		p.glReg.Creation_date = system.Today()
		p.glReg.Source_code = genJnlLine.Source_code
		p.glReg.Journal_batch_name = genJnlLine.Journal_batch_name
		p.glReg.User_id = types.Code(system.UserId())
		err := p.initTaxRegister()
		if err != nil {
			return err
		}
	} else {
		if p.lastDocType != genJnlLine.Document_type || p.lastDocNo != genJnlLine.Document_no ||
			p.lastDate != genJnlLine.Posting_date || p.currentBalance.IsZero() && !genJnlLine.System_created_entry ||
			p.lastLineNo != genJnlLine.Line_no && !p.genJnlTemplate.Force_doc_balance {
			p.nextTransactionNo = p.nextTransactionNo + 1
			p.lastDocType = genJnlLine.Document_type
			p.lastDocNo = genJnlLine.Document_no
			p.lastLineNo = genJnlLine.Line_no
			p.lastDate = genJnlLine.Posting_date
			p.newTransaction()
		}
	}
	err := p.getCurrencyExchRate()
	if err != nil {
		return err
	}
	p.glEntryTmp = nil
	p.initGainsLosses()
	if genJnlLine.Account_no != "" && genJnlLine.Bal_account_no == "" {
		p.currentBalance = p.currentBalance.Add(genJnlLine.Amount_lcy)
	}
	if genJnlLine.Account_no == "" && genJnlLine.Bal_account_no != "" {
		p.currentBalance = p.currentBalance.Sub(genJnlLine.Amount_lcy)
	}
	return nil
}

// finishCodeunit writes the line's G/L entries (after the currency gains and losses of its
// applications) and the G/L Register; the G/L entries are consistent when they balance
func (p *GenJnlPostLine) finishCodeunit() error {
	err := p.postGainsLosses()
	if err != nil {
		return err
	}
	if len(p.glEntryTmp) > 0 {
		var glEntry *gtables.GLEntry
		for _, glEntryTmp := range p.glEntryTmp {
			glEntry = glEntryTmp
			if p.acyCode() == "" {
				glEntry.Additional_currency_amount = types.Decimal{}
			}
			err = glEntry.InsertOrError(false)
			if err != nil {
				return err
			}
		}
		p.finishTaxRegister()
		if p.glReg.To_entry_no == 0 {
			p.glReg.To_entry_no = glEntry.Entry_no
			err = p.glReg.InsertOrError(false)
		} else {
			p.glReg.To_entry_no = glEntry.Entry_no
			err = p.glReg.ModifyOrError(false)
		}
		if err != nil {
			return err
		}
	}
	glEntry := gtables.NewGLEntry(p.src)
	glEntry.Consistent(p.balance().balanced())
	return nil
}

// initGLEntry starts a G/L entry of the line on a G/L account (a posting account, not blocked,
// whose dimension 1 and dimension 2 posting rules the line follows); its additional-currency amount
// is amountAddCurr for a line in the additional currency when useAmountAddCurr, else the amount
// exchanged (glCalcAddCurrency)
func (p *GenJnlPostLine) initGLEntry(glAccNo types.Code, amount, amountAddCurr types.Decimal, useAmountAddCurr, systemCreatedEntry bool) (*gtables.GLAccount, error) {
	genJnlLine := p.genJnlLine
	glAcc := gtables.NewGLAccount(p.src)
	if glAccNo != "" {
		err := glAcc.GetOrError(glAccNo)
		if err != nil {
			return nil, err
		}
		err = glAcc.TestField("blocked", false)
		if err != nil {
			return nil, err
		}
		err = glAcc.TestField("account_type", gtables.GLAccount_Account_type.Posting)
		if err != nil {
			return nil, err
		}
		err = checkPostingCode(int(glAcc.Dimension_1_posting), "dimension_1_code", genJnlLine.Dimension_1_code, glAcc.Dimension_1_code, glAcc.No)
		if err != nil {
			return nil, err
		}
		err = checkPostingCode(int(glAcc.Dimension_2_posting), "dimension_2_code", genJnlLine.Dimension_2_code, glAcc.Dimension_2_code, glAcc.No)
		if err != nil {
			return nil, err
		}
	}
	p.glEntry = gtables.NewGLEntry(p.src)
	p.glEntry.Init()
	p.glEntry.Posting_date = genJnlLine.Posting_date
	p.glEntry.Document_date = genJnlLine.Document_date
	p.glEntry.Document_type = gtables.GLEntryDocument_type(genJnlLine.Document_type)
	p.glEntry.Document_no = genJnlLine.Document_no
	p.glEntry.External_document_no = genJnlLine.External_document_no
	p.glEntry.Description = genJnlLine.Description
	p.glEntry.Business_unit_code = genJnlLine.Business_unit_code
	p.glEntry.Dimension_1_code = genJnlLine.Dimension_1_code
	p.glEntry.Dimension_2_code = genJnlLine.Dimension_2_code
	p.glEntry.Source_code = genJnlLine.Source_code
	if genJnlLine.Account_type == gtables.GenJournalLine_Account_type.GLAccount {
		p.glEntry.Source_type = gtables.GLEntrySource_type(genJnlLine.Source_type)
		p.glEntry.Source_no = genJnlLine.Source_no
	} else {
		p.glEntry.Source_type = gtables.GLEntrySource_type(genJnlLine.Account_type)
		p.glEntry.Source_no = genJnlLine.Account_no
	}
	p.glEntry.Quantity = genJnlLine.Quantity
	p.glEntry.Journal_batch_name = genJnlLine.Journal_batch_name
	p.glEntry.Reason_code = genJnlLine.Reason_code
	p.glEntry.Entry_no = p.nextEntryNo
	p.glEntry.Transaction_no = p.nextTransactionNo
	p.glEntry.Gl_account_no = glAccNo
	p.glEntry.Amount = amount
	p.glEntry.User_id = types.Code(system.UserId())
	p.glEntry.No_series = genJnlLine.Posting_no_series
	p.glEntry.System_created_entry = systemCreatedEntry
	p.glEntry.Prior_year_entry = p.glEntry.Posting_date.Before(p.fiscalYearStartDate)
	err := p.glCalcAddCurrency(amountAddCurr, useAmountAddCurr)
	if err != nil {
		return nil, err
	}
	return glAcc, nil
}

// checkPostingCode: a G/L account's Dimension 1 / Dimension 2 Posting rule (Code Mandatory, Same
// Code, No Code)
func checkPostingCode(rule int, field string, lineCode, accCode types.Code, glAccNo types.Code) error {
	switch rule {
	case int(gtables.GLAccount_Dimension_1_posting.CodeMandatory):
		if lineCode == "" {
			return apperrors.JnlPostingCodeMandatory(jnlLine, field, glAccNo.String())
		}
	case int(gtables.GLAccount_Dimension_1_posting.SameCode):
		if lineCode != accCode {
			return apperrors.JnlPostingCodeSame(jnlLine, field, accCode.String(), glAccNo.String())
		}
	case int(gtables.GLAccount_Dimension_1_posting.NoCode):
		if lineCode != "" {
			return apperrors.JnlPostingCodeBlank(jnlLine, field, glAccNo.String())
		}
	}
	return nil
}

// insertGLEntry adds the G/L entry to the line's entries: rounded, debit/credit (a correction
// the other way round) in LCY and in the additional currency, counted in the balance check — the
// posting balances when the entries' amounts weighted by their dates add up to 0 (two
// weightings; the additional-currency amounts too); with calcAddCurrResiduals a residual of the
// additional-currency amounts follows (handleAddCurrResidualGLEntry)
func (p *GenJnlPostLine) insertGLEntry(calcAddCurrResiduals bool) error {
	glEntry := p.glEntry
	if !glEntry.Amount.Equal(system.Round(glEntry.Amount)) {
		return apperrors.JnlNeedsRounding(gtables.GLEntryTableName, "amount", glEntry.Amount.String())
	}
	p.balance().add(glEntry, p.acyCode() != "")
	correction := p.genJnlLine.Correction
	if glEntry.Amount.IsPositive() && !correction || glEntry.Amount.IsNegative() && correction {
		glEntry.Debit_amount = glEntry.Amount
		glEntry.Credit_amount = types.Decimal{}
	} else {
		glEntry.Debit_amount = types.Decimal{}
		glEntry.Credit_amount = glEntry.Amount.Neg()
	}
	acyAmount := glEntry.Additional_currency_amount
	if acyAmount.IsPositive() && !correction || acyAmount.IsNegative() && correction {
		glEntry.Add_currency_debit_amount = acyAmount
		glEntry.Add_currency_credit_amount = types.Decimal{}
	} else {
		glEntry.Add_currency_debit_amount = types.Decimal{}
		glEntry.Add_currency_credit_amount = acyAmount.Neg()
	}
	p.glEntryTmp = append(p.glEntryTmp, glEntry)
	p.nextEntryNo = p.nextEntryNo + 1
	if calcAddCurrResiduals {
		return p.handleAddCurrResidualGLEntry()
	}
	return nil
}

// postFixedAsset (PostFixedAsset) posts a fixed asset line: the line's G/L entry (with its tax)
// goes to codeunit 5632 as the FA or maintenance entry's amount; the G/L amounts the asset's
// posting group gives are the G/L entries (with the line's general and tax posting groups and the
// FA entry they belong to); the line's tax; the FA register points at the G/L register
func (p *GenJnlPostLine) postFixedAsset(genJnlLine *gtables.GenJournalLine) error {
	_, err := p.initGLEntry("", genJnlLine.Amount_lcy, genJnlLine.Source_currency_amount, true, genJnlLine.System_created_entry)
	if err != nil {
		return err
	}
	p.glEntry.Gen_posting_type = gtables.GLEntryGen_posting_type(genJnlLine.Gen_posting_type)
	p.glEntry.Bal_account_type = gtables.GLEntryBal_account_type(genJnlLine.Bal_account_type)
	p.glEntry.Bal_account_no = genJnlLine.Bal_account_no
	err = p.initTax(genJnlLine)
	if err != nil {
		return err
	}
	tempGLEntry := p.glEntry
	if p.faJnlPostLine == nil {
		p.faJnlPostLine = fajnl.NewFAJnlPostLine(p.src)
	}
	err = p.faJnlPostLine.GenJnlPostLine(genJnlLine, tempGLEntry.Amount, tempGLEntry.Tax_amount, p.nextTransactionNo, p.nextEntryNo)
	if err != nil {
		return err
	}
	dimension1Code := genJnlLine.Dimension_1_code
	dimension2Code := genJnlLine.Dimension_2_code
	correction := genJnlLine.Correction
	for _, b := range p.faJnlPostLine.GLBuffer() {
		genJnlLine.Dimension_1_code = b.Dimension1Code
		genJnlLine.Dimension_2_code = b.Dimension2Code
		genJnlLine.Correction = b.Correction
		if b.OriginalGeneralJnlLine {
			_, err = p.initGLEntry(b.AccountNo, b.Amount, tempGLEntry.Additional_currency_amount, true, true)
		} else {
			p.checkNonAddCurrCodeOccurred("")
			_, err = p.initGLEntry(b.AccountNo, b.Amount, types.Decimal{}, false, true)
		}
		if err != nil {
			return err
		}
		glEntry := p.glEntry
		glEntry.Gen_posting_type = tempGLEntry.Gen_posting_type
		glEntry.Gen_bus_posting_group = tempGLEntry.Gen_bus_posting_group
		glEntry.Gen_prod_posting_group = tempGLEntry.Gen_prod_posting_group
		glEntry.Tax_bus_posting_group = tempGLEntry.Tax_bus_posting_group
		glEntry.Tax_prod_posting_group = tempGLEntry.Tax_prod_posting_group
		glEntry.Tax_area_code = tempGLEntry.Tax_area_code
		glEntry.Tax_liable = tempGLEntry.Tax_liable
		glEntry.Tax_group_code = tempGLEntry.Tax_group_code
		glEntry.Use_tax = tempGLEntry.Use_tax
		glEntry.Tax_amount = tempGLEntry.Tax_amount
		glEntry.Bal_account_type = tempGLEntry.Bal_account_type
		glEntry.Bal_account_no = tempGLEntry.Bal_account_no
		glEntry.Fa_entry_type = b.FAEntryType
		glEntry.Fa_entry_no = b.FAEntryNo
		if b.AutomaticEntry {
			// codeunit 5607 AdjustGLEntry: an automatic amount has no quantity, balancing
			// account, tax or general posting groups
			glEntry.Quantity = types.Decimal{}
			glEntry.Bal_account_type = gtables.GLEntry_Bal_account_type.GLAccount
			glEntry.Bal_account_no = ""
			glEntry.Tax_amount = types.Decimal{}
			glEntry.Gen_posting_type = gtables.GLEntry_Gen_posting_type.Blank
			glEntry.Gen_bus_posting_group = ""
			glEntry.Gen_prod_posting_group = ""
		}
		err = p.insertGLEntry(true)
		if err != nil {
			return err
		}
	}
	genJnlLine.Dimension_1_code = dimension1Code
	genJnlLine.Dimension_2_code = dimension2Code
	genJnlLine.Correction = correction
	p.glEntry = tempGLEntry
	err = p.postTax(genJnlLine)
	if err != nil {
		return err
	}
	if faReg := p.faJnlPostLine.FARegister(); faReg != nil {
		faReg.G_l_register_no = p.glReg.No
		return faReg.ModifyOrError(false)
	}
	return nil
}
