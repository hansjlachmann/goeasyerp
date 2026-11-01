package reminderissue

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/noseries"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// FinChrgMemoIssue is codeunit 395 FinChrgMemo-Issue (one per memo: Set, then Run)
type FinChrgMemoIssue struct {
	src                     ftables.Source
	finChrgMemoHeader       *gtables.FinanceChargeMemoHeader
	issuedFinChrgMemoHeader *gtables.IssuedFinChargeMemoHeader
	replacePostingDate      bool
	postingDate             types.Date
	docNo                   types.Code
	sourceCode              types.Code
	currencyFactor          types.Decimal
	genJnlLines             []*gtables.GenJournalLine
	totalAmount             types.Decimal
	totalAmountLCY          types.Decimal
}

// NewFinChrgMemoIssue opens the codeunit on the caller's transaction
func NewFinChrgMemoIssue(src ftables.Source) *FinChrgMemoIssue {
	return &FinChrgMemoIssue{src: src}
}

// Set (Set): the memo and the posting date that replaces its own
func (c *FinChrgMemoIssue) Set(finChrgMemoHeader *gtables.FinanceChargeMemoHeader, replacePostingDate bool, postingDate types.Date) {
	c.finChrgMemoHeader = finChrgMemoHeader
	c.replacePostingDate = replacePostingDate
	c.postingDate = postingDate
}

// GetIssuedFinChrgMemo (GetIssuedFinChrgMemo): the issued memo Run made
func (c *FinChrgMemoIssue) GetIssuedFinChrgMemo() *gtables.IssuedFinChargeMemoHeader {
	return c.issuedFinChrgMemoHeader
}

// Run (OnRun)
func (c *FinChrgMemoIssue) Run() error {
	finChrgMemoHeader := c.finChrgMemoHeader
	if !c.postingDate.IsZero() && (c.replacePostingDate || finChrgMemoHeader.Posting_date.IsZero()) {
		err := finChrgMemoHeader.Validate("posting_date", c.postingDate)
		if err != nil {
			return err
		}
	}
	for _, field := range []string{"customer_no", "posting_date", "document_date", "due_date", "customer_posting_group"} {
		err := finChrgMemoHeader.TestField(field)
		if err != nil {
			return err
		}
	}
	custPostingGroup := gtables.NewCustomerPostingGroup(c.src)
	err := custPostingGroup.GetOrError(finChrgMemoHeader.Customer_posting_group)
	if err != nil {
		return err
	}
	finChrgMemoHeader.CalcFields("interest_amount", "additional_fee", "remaining_amount")
	if finChrgMemoHeader.Interest_amount.IsZero() && finChrgMemoHeader.Additional_fee.IsZero() && finChrgMemoHeader.Remaining_amount.IsZero() {
		return apperrors.ReminderNothingToIssue()
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(c.src)
	err = sourceCodeSetup.TestField("finance_charge_memo")
	if err != nil {
		return err
	}
	c.sourceCode = sourceCodeSetup.Finance_charge_memo
	if finChrgMemoHeader.Issuing_no == "" && finChrgMemoHeader.No_series != finChrgMemoHeader.Issuing_no_series {
		err = finChrgMemoHeader.TestField("issuing_no_series")
		if err != nil {
			return err
		}
		finChrgMemoHeader.Issuing_no, err = noseries.New(c.src).GetNextNo(finChrgMemoHeader.Issuing_no_series, finChrgMemoHeader.Posting_date, true)
		if err == nil {
			err = finChrgMemoHeader.ModifyOrError(false)
		}
		if err != nil {
			return err
		}
	}
	if finChrgMemoHeader.Issuing_no == "" {
		c.docNo = finChrgMemoHeader.No
	} else {
		c.docNo = finChrgMemoHeader.Issuing_no
	}
	c.currencyFactor, err = currencyFactor(c.src, finChrgMemoHeader.Currency_code, finChrgMemoHeader.Posting_date)
	if err != nil {
		return err
	}

	var interestAmount, interestTaxAmount types.Decimal
	finChrgMemoLine := gtables.NewFinanceChargeMemoLine(c.src)
	finChrgMemoLine.SetRange("finance_charge_memo_no", finChrgMemoHeader.No)
	for found := finChrgMemoLine.FindSet(); found; found = finChrgMemoLine.Next() {
		switch finChrgMemoLine.Type {
		case gtables.FinanceChargeMemoLine_Type.Blank:
			err = finChrgMemoLine.TestField("amount", types.Decimal{})
		case gtables.FinanceChargeMemoLine_Type.GLAccount:
			if !finChrgMemoLine.Amount.IsZero() && finChrgMemoHeader.Post_additional_fee {
				err = finChrgMemoLine.TestField("no")
				if err != nil {
					return err
				}
				genJnlLine := c.initGenJnlLine(gtables.GenJournalLine_Account_type.GLAccount, finChrgMemoLine.No, false)
				genJnlLine.Gen_prod_posting_group = finChrgMemoLine.Gen_prod_posting_group
				genJnlLine.Tax_prod_posting_group = finChrgMemoLine.Tax_prod_posting_group
				genJnlLine.Tax_calculation_type = gtables.GenJournalLineTax_calculation_type(finChrgMemoLine.Tax_calculation_type)
				if finChrgMemoLine.Tax_calculation_type == gtables.FinanceChargeMemoLine_Tax_calculation_type.SalesTax {
					genJnlLine.Tax_area_code = finChrgMemoHeader.Tax_area_code
					genJnlLine.Tax_liable = finChrgMemoHeader.Tax_liable
					genJnlLine.Tax_group_code = finChrgMemoLine.Tax_group_code
				}
				genJnlLine.Tax_percent = finChrgMemoLine.Tax_percent
				c.setAmounts(genJnlLine, finChrgMemoLine.Amount.Neg(), finChrgMemoLine.Tax_amount.Neg())
			}
		case gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry:
			err = finChrgMemoLine.TestField("entry_no")
			if err != nil {
				return err
			}
			custLedgEntry := gtables.NewCustomerLedgerEntry(c.src)
			err = custLedgEntry.GetOrError(finChrgMemoLine.Entry_no)
			if err == nil {
				err = custLedgEntry.TestField("currency_code", finChrgMemoHeader.Currency_code)
			}
			if err == nil && finChrgMemoLine.Amount.IsNegative() {
				err = apperrors.ReminderAmountNegative(gtables.FinanceChargeMemoLineTableName)
			}
			interestAmount = interestAmount.Add(finChrgMemoLine.Amount)
			interestTaxAmount = interestTaxAmount.Add(finChrgMemoLine.Tax_amount)
		}
		if err != nil {
			return err
		}
	}
	if !interestAmount.IsZero() && finChrgMemoHeader.Post_interest {
		err = custPostingGroup.TestField("interest_account")
		if err != nil {
			return err
		}
		genJnlLine := c.initGenJnlLine(gtables.GenJournalLine_Account_type.GLAccount, custPostingGroup.Interest_account, true)
		err = setInterestTax(c.src, genJnlLine, finChrgMemoHeader.Tax_bus_posting_group, finChrgMemoHeader.Tax_area_code, finChrgMemoHeader.Tax_liable, custPostingGroup.Interest_account)
		if err != nil {
			return err
		}
		c.setAmounts(genJnlLine, interestAmount.Neg(), interestTaxAmount.Neg())
	}
	if !c.totalAmount.IsZero() || !c.totalAmountLCY.IsZero() {
		genJnlLine := c.initGenJnlLine(gtables.GenJournalLine_Account_type.Customer, finChrgMemoHeader.Customer_no, true)
		genJnlLine.Amount = c.totalAmount
		genJnlLine.Amount_lcy = c.totalAmountLCY
		genJnlLine.Source_currency_amount = c.totalAmount
		genJnlLine.Due_date = finChrgMemoHeader.Due_date
		c.genJnlLines = append(c.genJnlLines, genJnlLine)
	}
	genJnlPostLine := genjnl.NewGenJnlPostLine(c.src)
	for _, genJnlLine := range c.genJnlLines {
		err = genJnlPostLine.RunWithoutCheck(genJnlLine)
		if err != nil {
			return err
		}
	}
	if !interestAmount.IsZero() {
		err = finChrgMemoHeader.TestField("fin_charge_terms_code")
		if err == nil {
			err = markInterestCalculated(c.src, finChrgMemoHeader.Fin_charge_terms_code, finChrgMemoHeader.Currency_code, finChrgMemoHeader.No, true)
		}
		if err != nil {
			return err
		}
	}

	interestPosted := !interestAmount.IsZero() && finChrgMemoHeader.Post_interest
	issued := gtables.NewIssuedFinChargeMemoHeader(c.src)
	issued.TransferFields(finChrgMemoHeader)
	issued.No = c.docNo
	issued.Pre_assigned_no = finChrgMemoHeader.No
	issued.Pre_assigned_no_series = finChrgMemoHeader.No_series
	issued.No_series = finChrgMemoHeader.Issuing_no_series
	issued.Interest_posted = interestPosted
	issued.Additional_fee_posted = !finChrgMemoHeader.Additional_fee.IsZero() && finChrgMemoHeader.Post_additional_fee
	issued.Source_code = c.sourceCode
	issued.User_id = types.Code(system.UserId())
	err = issued.InsertOrError(false)
	if err != nil {
		return err
	}
	c.issuedFinChrgMemoHeader = issued

	reminderEntry := gtables.NewReminderFinChargeEntry(c.src)
	reminderEntry.LockTable()
	nextEntryNo := 1
	if reminderEntry.FindLast() {
		nextEntryNo = reminderEntry.Entry_no + 1
	}
	commentLine := gtables.NewFinChargeCommentLine(c.src)
	commentLine.SetRange("type", gtables.FinChargeCommentLine_Type.FinanceChargeMemo)
	commentLine.SetRange("no", finChrgMemoHeader.No)
	for found := commentLine.FindSet(); found; found = commentLine.Next() {
		commentLine2 := gtables.NewFinChargeCommentLine(c.src)
		commentLine2.TransferFields(commentLine)
		commentLine2.Type = gtables.FinChargeCommentLine_Type.IssuedFinanceChargeMemo
		commentLine2.No = issued.No
		err = commentLine2.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	commentLine.DeleteAll()
	for found := finChrgMemoLine.FindSet(); found; found = finChrgMemoLine.Next() {
		if finChrgMemoLine.Type == gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry {
			entry := gtables.NewReminderFinChargeEntry(c.src)
			entry.Init()
			entry.Entry_no = nextEntryNo
			entry.Type = gtables.ReminderFinChargeEntry_Type.FinanceChargeMemo
			entry.No = issued.No
			entry.Posting_date = finChrgMemoHeader.Posting_date
			entry.Document_date = finChrgMemoHeader.Document_date
			entry.Customer_no = finChrgMemoHeader.Customer_no
			entry.Customer_entry_no = finChrgMemoLine.Entry_no
			entry.Document_type = gtables.ReminderFinChargeEntryDocument_type(finChrgMemoLine.Document_type)
			entry.Document_no = finChrgMemoLine.Document_no
			entry.Remaining_amount = finChrgMemoLine.Remaining_amount
			entry.Interest_amount = finChrgMemoLine.Amount
			entry.Interest_posted = interestPosted
			entry.User_id = types.Code(system.UserId())
			err = entry.InsertOrError(false)
			if err != nil {
				return err
			}
			nextEntryNo++
		}
		issuedLine := gtables.NewIssuedFinChargeMemoLine(c.src)
		issuedLine.TransferFields(finChrgMemoLine)
		issuedLine.Finance_charge_memo_no = issued.No
		err = issuedLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	finChrgMemoLine.DeleteAll()
	return finChrgMemoHeader.DeleteOrError(false)
}

// initGenJnlLine (InitGenJnlLine): a line of the memo's posting
func (c *FinChrgMemoIssue) initGenJnlLine(accountType gtables.GenJournalLineAccount_type, accountNo types.Code, systemCreatedEntry bool) *gtables.GenJournalLine {
	h := c.finChrgMemoHeader
	genJnlLine := gtables.NewGenJournalLine(c.src)
	genJnlLine.Init()
	genJnlLine.Line_no = len(c.genJnlLines) + 1
	genJnlLine.Document_type = gtables.GenJournalLine_Document_type.FinanceChargeMemo
	genJnlLine.Document_no = c.docNo
	genJnlLine.Posting_date = h.Posting_date
	genJnlLine.Document_date = h.Document_date
	genJnlLine.Account_type = accountType
	genJnlLine.Account_no = accountNo
	if accountType == gtables.GenJournalLine_Account_type.GLAccount {
		genJnlLine.Gen_posting_type = gtables.GenJournalLine_Gen_posting_type.Sale
		genJnlLine.Gen_bus_posting_group = h.Gen_bus_posting_group
		genJnlLine.Tax_bus_posting_group = h.Tax_bus_posting_group
	}
	if accountType == gtables.GenJournalLine_Account_type.Customer {
		genJnlLine.Posting_group = h.Customer_posting_group
	}
	genJnlLine.Currency_code = h.Currency_code
	genJnlLine.Currency_factor = c.currencyFactor
	genJnlLine.Source_currency_code = h.Currency_code
	genJnlLine.Description = h.Posting_description
	genJnlLine.Department_code = h.Department_code
	genJnlLine.Project_code = h.Project_code
	genJnlLine.Source_type = gtables.GenJournalLine_Source_type.Customer
	genJnlLine.Source_no = h.Customer_no
	genJnlLine.Source_code = c.sourceCode
	genJnlLine.Reason_code = h.Reason_code
	genJnlLine.System_created_entry = systemCreatedEntry
	genJnlLine.Posting_no_series = h.Issuing_no_series
	return genJnlLine
}

// setAmounts: a G/L line's amount including its tax; the customer line takes the opposite
func (c *FinChrgMemoIssue) setAmounts(genJnlLine *gtables.GenJournalLine, amount, taxAmount types.Decimal) {
	setLineAmounts(c.src, genJnlLine, c.finChrgMemoHeader.Currency_code, c.currencyFactor, amount, taxAmount)
	c.totalAmount = c.totalAmount.Sub(genJnlLine.Amount).Sub(genJnlLine.Tax_amount)
	c.totalAmountLCY = c.totalAmountLCY.Sub(genJnlLine.Amount_lcy).Sub(genJnlLine.Tax_amount_lcy)
	c.genJnlLines = append(c.genJnlLines, genJnlLine)
}
