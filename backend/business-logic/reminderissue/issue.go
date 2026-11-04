// Package reminderissue issues reminders and finance charge memos: codeunits 393 Reminder-Issue
// and 395 FinChrgMemo-Issue — the document becomes an issued one, its additional fees and interest
// posted to the G/L and the customer (codeunit 12), a Reminder/Fin. Charge Entry per customer
// entry, the document deleted.
package reminderissue

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ReminderIssue is codeunit 393 Reminder-Issue (one per reminder: Set, then Run)
type ReminderIssue struct {
	src                  ftables.Source
	reminderHeader       *gtables.ReminderHeader
	issuedReminderHeader *gtables.IssuedReminderHeader
	replacePostingDate   bool
	postingDate          types.Date
	docNo                types.Code
	sourceCode           types.Code
	currencyFactor       types.Decimal
	genJnlLines          []*gtables.GenJournalLine
	totalAmount          types.Decimal
	totalAmountLCY       types.Decimal
}

// New opens the codeunit on the caller's transaction
func New(src ftables.Source) *ReminderIssue {
	return &ReminderIssue{src: src}
}

// Set (Set): the reminder, and the posting date that replaces its own (ReplacePostingDate, or
// when it has none)
func (c *ReminderIssue) Set(reminderHeader *gtables.ReminderHeader, replacePostingDate bool, postingDate types.Date) {
	c.reminderHeader = reminderHeader
	c.replacePostingDate = replacePostingDate
	c.postingDate = postingDate
}

// GetIssuedReminder (GetIssuedReminder): the issued reminder Run made
func (c *ReminderIssue) GetIssuedReminder() *gtables.IssuedReminderHeader {
	return c.issuedReminderHeader
}

// Run (OnRun)
func (c *ReminderIssue) Run() error {
	reminderHeader := c.reminderHeader
	if !c.postingDate.IsZero() && (c.replacePostingDate || reminderHeader.Posting_date.IsZero()) {
		err := reminderHeader.Validate("posting_date", c.postingDate)
		if err != nil {
			return err
		}
	}
	for _, field := range []string{"customer_no", "posting_date", "document_date", "due_date", "customer_posting_group"} {
		err := reminderHeader.TestField(field)
		if err != nil {
			return err
		}
	}
	custPostingGroup := gtables.NewCustomerPostingGroup(c.src)
	err := custPostingGroup.GetOrError(reminderHeader.Customer_posting_group)
	if err != nil {
		return err
	}
	reminderHeader.CalcFields("interest_amount", "additional_fee", "remaining_amount")
	if reminderHeader.Interest_amount.IsZero() && reminderHeader.Additional_fee.IsZero() && reminderHeader.Remaining_amount.IsZero() {
		return apperrors.ReminderNothingToIssue()
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(c.src)
	err = sourceCodeSetup.TestField("reminder")
	if err != nil {
		return err
	}
	c.sourceCode = sourceCodeSetup.Reminder
	if reminderHeader.Issuing_no == "" && reminderHeader.No_series != reminderHeader.Issuing_no_series {
		err = reminderHeader.TestField("issuing_no_series")
		if err != nil {
			return err
		}
		reminderHeader.Issuing_no, err = noseries.New(c.src).GetNextNo(reminderHeader.Issuing_no_series, reminderHeader.Posting_date, true)
		if err == nil {
			err = reminderHeader.ModifyOrError(false)
		}
		if err != nil {
			return err
		}
	}
	if reminderHeader.Issuing_no != "" {
		c.docNo = reminderHeader.Issuing_no
	} else {
		c.docNo = reminderHeader.No
	}
	c.currencyFactor, err = currencyFactor(c.src, reminderHeader.Currency_code, reminderHeader.Posting_date)
	if err != nil {
		return err
	}

	var reminderInterestAmount, reminderInterestTaxAmount types.Decimal
	reminderLine := gtables.NewReminderLine(c.src)
	reminderLine.SetRange("reminder_no", reminderHeader.No)
	for found := reminderLine.FindSet(); found; found = reminderLine.Next() {
		switch reminderLine.Type {
		case gtables.ReminderLine_Type.Blank:
			err = reminderLine.TestField("amount", types.Decimal{})
		case gtables.ReminderLine_Type.GLAccount:
			if !reminderLine.Amount.IsZero() && reminderHeader.Post_additional_fee {
				err = reminderLine.TestField("no")
				if err != nil {
					return err
				}
				genJnlLine := c.initGenJnlLine(gtables.GenJournalLine_Account_type.GLAccount, reminderLine.No, false)
				genJnlLine.Gen_prod_posting_group = reminderLine.Gen_prod_posting_group
				genJnlLine.Tax_prod_posting_group = reminderLine.Tax_prod_posting_group
				genJnlLine.Tax_calculation_type = gtables.GenJournalLineTax_calculation_type(reminderLine.Tax_calculation_type)
				if reminderLine.Tax_calculation_type == gtables.ReminderLine_Tax_calculation_type.SalesTax {
					genJnlLine.Tax_area_code = reminderHeader.Tax_area_code
					genJnlLine.Tax_liable = reminderHeader.Tax_liable
					genJnlLine.Tax_group_code = reminderLine.Tax_group_code
				}
				genJnlLine.Tax_percent = reminderLine.Tax_percent
				c.setAmounts(genJnlLine, reminderLine.Amount.Neg(), reminderLine.Tax_amount.Neg())
			}
		case gtables.ReminderLine_Type.CustomerLedgerEntry:
			err = reminderLine.TestField("entry_no")
			if err == nil && reminderLine.Amount.IsNegative() {
				err = apperrors.ReminderAmountNegative(gtables.ReminderLineTableName)
			}
			reminderInterestAmount = reminderInterestAmount.Add(reminderLine.Amount)
			reminderInterestTaxAmount = reminderInterestTaxAmount.Add(reminderLine.Tax_amount)
		}
		if err != nil {
			return err
		}
	}
	if !reminderInterestAmount.IsZero() && reminderHeader.Post_interest {
		err = custPostingGroup.TestField("interest_account")
		if err != nil {
			return err
		}
		genJnlLine := c.initGenJnlLine(gtables.GenJournalLine_Account_type.GLAccount, custPostingGroup.Interest_account, true)
		err = setInterestTax(c.src, genJnlLine, reminderHeader.Tax_bus_posting_group, reminderHeader.Tax_area_code, reminderHeader.Tax_liable, custPostingGroup.Interest_account)
		if err != nil {
			return err
		}
		c.setAmounts(genJnlLine, reminderInterestAmount.Neg(), reminderInterestTaxAmount.Neg())
	}
	if !c.totalAmount.IsZero() || !c.totalAmountLCY.IsZero() {
		genJnlLine := c.initGenJnlLine(gtables.GenJournalLine_Account_type.Customer, reminderHeader.Customer_no, true)
		genJnlLine.Amount = c.totalAmount
		genJnlLine.Amount_lcy = c.totalAmountLCY
		genJnlLine.Source_currency_amount = c.totalAmount
		genJnlLine.Due_date = reminderHeader.Due_date
		c.genJnlLines = append(c.genJnlLines, genJnlLine)
	}
	genJnlPostLine := genjnl.NewGenJnlPostLine(c.src)
	for _, genJnlLine := range c.genJnlLines {
		err = genJnlPostLine.RunWithoutCheck(genJnlLine)
		if err != nil {
			return err
		}
	}

	interestPosted := !reminderInterestAmount.IsZero() && reminderHeader.Post_interest
	if interestPosted {
		err = reminderHeader.TestField("fin_charge_terms_code")
		if err != nil {
			return err
		}
		err = markInterestCalculated(c.src, reminderHeader.Fin_charge_terms_code, reminderHeader.Currency_code, reminderHeader.No, false)
		if err != nil {
			return err
		}
	}

	issuedReminderHeader := gtables.NewIssuedReminderHeader(c.src)
	issuedReminderHeader.TransferFields(reminderHeader)
	issuedReminderHeader.No = c.docNo
	issuedReminderHeader.Pre_assigned_no = reminderHeader.No
	issuedReminderHeader.Pre_assigned_no_series = reminderHeader.No_series
	issuedReminderHeader.No_series = reminderHeader.Issuing_no_series
	issuedReminderHeader.Interest_posted = interestPosted
	issuedReminderHeader.Additional_fee_posted = !reminderHeader.Additional_fee.IsZero() && reminderHeader.Post_additional_fee
	issuedReminderHeader.Source_code = c.sourceCode
	issuedReminderHeader.User_id = types.Code(system.UserId())
	err = issuedReminderHeader.InsertOrError(false)
	if err != nil {
		return err
	}
	c.issuedReminderHeader = issuedReminderHeader

	reminderEntry := gtables.NewReminderFinChargeEntry(c.src)
	reminderEntry.LockTable()
	nextEntryNo := 1
	if reminderEntry.FindLast() {
		nextEntryNo = reminderEntry.Entry_no + 1
	}
	reminderCommentLine := gtables.NewReminderCommentLine(c.src)
	reminderCommentLine.SetRange("type", gtables.ReminderCommentLine_Type.Reminder)
	reminderCommentLine.SetRange("no", reminderHeader.No)
	for found := reminderCommentLine.FindSet(); found; found = reminderCommentLine.Next() {
		reminderCommentLine2 := gtables.NewReminderCommentLine(c.src)
		reminderCommentLine2.TransferFields(reminderCommentLine)
		reminderCommentLine2.Type = gtables.ReminderCommentLine_Type.IssuedReminder
		reminderCommentLine2.No = issuedReminderHeader.No
		err = reminderCommentLine2.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	reminderCommentLine.DeleteAll()
	for found := reminderLine.FindSet(); found; found = reminderLine.Next() {
		if reminderLine.Type == gtables.ReminderLine_Type.CustomerLedgerEntry && reminderLine.Entry_no != 0 {
			reminderEntry := gtables.NewReminderFinChargeEntry(c.src)
			reminderEntry.Init()
			reminderEntry.Entry_no = nextEntryNo
			reminderEntry.Type = gtables.ReminderFinChargeEntry_Type.Reminder
			reminderEntry.No = issuedReminderHeader.No
			reminderEntry.Posting_date = reminderHeader.Posting_date
			reminderEntry.Document_date = reminderHeader.Document_date
			reminderEntry.Customer_no = reminderHeader.Customer_no
			reminderEntry.Customer_entry_no = reminderLine.Entry_no
			reminderEntry.Document_type = gtables.ReminderFinChargeEntryDocument_type(reminderLine.Document_type)
			reminderEntry.Document_no = reminderLine.Document_no
			reminderEntry.Reminder_level = reminderLine.Reminder_level
			reminderEntry.Remaining_amount = reminderLine.Remaining_amount
			reminderEntry.Interest_amount = reminderLine.Amount
			reminderEntry.Interest_posted = interestPosted
			reminderEntry.User_id = types.Code(system.UserId())
			err = reminderEntry.InsertOrError(false)
			if err != nil {
				return err
			}
			nextEntryNo++
		}
		issuedReminderLine := gtables.NewIssuedReminderLine(c.src)
		issuedReminderLine.TransferFields(reminderLine)
		issuedReminderLine.Reminder_no = issuedReminderHeader.No
		err = issuedReminderLine.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	reminderLine.DeleteAll()
	return reminderHeader.DeleteOrError(false)
}

// initGenJnlLine (InitGenJnlLine): a line of the reminder's posting — G/L lines are sales of the
// customer's general and tax business posting groups
func (c *ReminderIssue) initGenJnlLine(accountType gtables.GenJournalLineAccount_type, accountNo types.Code, systemCreatedEntry bool) *gtables.GenJournalLine {
	reminderHeader := c.reminderHeader
	genJnlLine := gtables.NewGenJournalLine(c.src)
	genJnlLine.Init()
	genJnlLine.Line_no = len(c.genJnlLines) + 1
	genJnlLine.Document_type = gtables.GenJournalLine_Document_type.Reminder
	genJnlLine.Document_no = c.docNo
	genJnlLine.Posting_date = reminderHeader.Posting_date
	genJnlLine.Document_date = reminderHeader.Document_date
	genJnlLine.Account_type = accountType
	genJnlLine.Account_no = accountNo
	if accountType == gtables.GenJournalLine_Account_type.GLAccount {
		genJnlLine.Gen_posting_type = gtables.GenJournalLine_Gen_posting_type.Sale
		genJnlLine.Gen_bus_posting_group = reminderHeader.Gen_bus_posting_group
		genJnlLine.Tax_bus_posting_group = reminderHeader.Tax_bus_posting_group
	}
	if accountType == gtables.GenJournalLine_Account_type.Customer {
		genJnlLine.Posting_group = reminderHeader.Customer_posting_group
	}
	genJnlLine.Currency_code = reminderHeader.Currency_code
	genJnlLine.Currency_factor = c.currencyFactor
	genJnlLine.Source_currency_code = reminderHeader.Currency_code
	genJnlLine.Description = reminderHeader.Posting_description
	genJnlLine.Dimension_1_code = reminderHeader.Dimension_1_code
	genJnlLine.Dimension_2_code = reminderHeader.Dimension_2_code
	genJnlLine.Source_type = gtables.GenJournalLine_Source_type.Customer
	genJnlLine.Source_no = reminderHeader.Customer_no
	genJnlLine.Source_code = c.sourceCode
	genJnlLine.Reason_code = reminderHeader.Reason_code
	genJnlLine.System_created_entry = systemCreatedEntry
	genJnlLine.Posting_no_series = reminderHeader.Issuing_no_series
	return genJnlLine
}

// setAmounts: a G/L line's amount and its tax (Manual VAT Entry: codeunit 12 posts the tax
// on top), in the document's currency and LCY; the customer line takes the opposite of both
func (c *ReminderIssue) setAmounts(genJnlLine *gtables.GenJournalLine, amount, taxAmount types.Decimal) {
	setLineAmounts(c.src, genJnlLine, c.reminderHeader.Currency_code, c.currencyFactor, amount, taxAmount)
	c.totalAmount = c.totalAmount.Sub(genJnlLine.Amount).Sub(genJnlLine.Tax_amount)
	c.totalAmountLCY = c.totalAmountLCY.Sub(genJnlLine.Amount_lcy).Sub(genJnlLine.Tax_amount_lcy)
	c.genJnlLines = append(c.genJnlLines, genJnlLine)
}

// setLineAmounts: amount (without tax) and tax of a reminder or finance charge memo G/L line, in
// FCY and LCY
func setLineAmounts(src ftables.Source, genJnlLine *gtables.GenJournalLine, currencyCode types.Code, currencyFactor, amount, taxAmount types.Decimal) {
	toLCY := func(a types.Decimal) types.Decimal {
		if currencyCode == "" {
			return a
		}
		return system.Round(a.Div(currencyFactor))
	}
	genJnlLine.Amount = amount
	genJnlLine.Amount_lcy = toLCY(amount)
	genJnlLine.Source_currency_amount = amount
	if genJnlLine.Gen_posting_type != 0 {
		genJnlLine.Vat_posting = gtables.GenJournalLine_Vat_posting.ManualVATEntry
		genJnlLine.Tax_amount = taxAmount
		genJnlLine.Tax_amount_lcy = toLCY(taxAmount)
		genJnlLine.Tax_base_amount = amount
		genJnlLine.Tax_base_amount_lcy = genJnlLine.Amount_lcy
		genJnlLine.Source_curr_tax_amount = taxAmount
		genJnlLine.Source_curr_tax_base_amount = genJnlLine.Tax_base_amount
	}
}

// setInterestTax: the interest account's product posting groups and the tax of the document's
// business group on the interest line
func setInterestTax(src ftables.Source, genJnlLine *gtables.GenJournalLine, taxBusPostingGroup, taxAreaCode types.Code, taxLiable bool, interestAccount types.Code) error {
	glAcc := gtables.NewGLAccount(src)
	err := glAcc.GetOrError(interestAccount)
	if err != nil {
		return err
	}
	genJnlLine.Gen_prod_posting_group = glAcc.Gen_prod_posting_group
	genJnlLine.Tax_prod_posting_group = glAcc.Tax_prod_posting_group
	taxPostingSetup := gtables.NewTaxPostingSetup(src)
	if taxPostingSetup.Get(map[string]interface{}{"tax_bus_posting_group": taxBusPostingGroup, "tax_prod_posting_group": glAcc.Tax_prod_posting_group}) {
		genJnlLine.Tax_calculation_type = gtables.GenJournalLineTax_calculation_type(taxPostingSetup.Tax_calculation_type)
		genJnlLine.Tax_percent = taxPostingSetup.Vat_percent
	}
	if genJnlLine.Tax_calculation_type == gtables.GenJournalLine_Tax_calculation_type.SalesTax {
		genJnlLine.Tax_area_code = taxAreaCode
		genJnlLine.Tax_liable = taxLiable
		genJnlLine.Tax_group_code = glAcc.Tax_group_code
	}
	return nil
}

// currencyFactor: the document's currency factor on the posting date (1: no currency)
func currencyFactor(src ftables.Source, currencyCode types.Code, postingDate types.Date) (types.Decimal, error) {
	if currencyCode == "" {
		return types.NewDecimalFromInt(1), nil
	}
	return tables.ExchangeRates(src).ExchangeRate(postingDate, currencyCode)
}

// markInterestCalculated: with finance charge terms charging closed entries, the document's
// entries that are paid no longer earn interest and the entries that closed them are charged
// (Calculate Interest, Closing Interest Calculated)
func markInterestCalculated(src ftables.Source, finChrgTermsCode, currencyCode, documentNo types.Code, financeChargeMemo bool) error {
	finChrgTerms := gtables.NewFinanceChargeTerms(src)
	err := finChrgTerms.GetOrError(finChrgTermsCode)
	if err != nil {
		return err
	}
	if finChrgTerms.Interest_calculation != gtables.FinanceChargeTerms_Interest_calculation.ClosedEntries &&
		finChrgTerms.Interest_calculation != gtables.FinanceChargeTerms_Interest_calculation.AllEntries {
		return nil
	}
	var entryNos []int
	if financeChargeMemo {
		line := gtables.NewFinanceChargeMemoLine(src)
		line.SetRange("finance_charge_memo_no", documentNo)
		line.SetRange("type", gtables.FinanceChargeMemoLine_Type.CustomerLedgerEntry)
		for found := line.FindSet(); found; found = line.Next() {
			entryNos = append(entryNos, line.Entry_no)
		}
	} else {
		line := gtables.NewReminderLine(src)
		line.SetRange("reminder_no", documentNo)
		line.SetRange("type", gtables.ReminderLine_Type.CustomerLedgerEntry)
		for found := line.FindSet(); found; found = line.Next() {
			entryNos = append(entryNos, line.Entry_no)
		}
	}
	for _, entryNo := range entryNos {
		custLedgEntry := gtables.NewCustomerLedgerEntry(src)
		err = custLedgEntry.GetOrError(entryNo)
		if err == nil {
			err = custLedgEntry.TestField("currency_code", currencyCode)
		}
		if err != nil {
			return err
		}
		if custLedgEntry.Remaining_amount.IsZero() {
			custLedgEntry.Calculate_interest = false
			err = custLedgEntry.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
		custLedgEntry2 := gtables.NewCustomerLedgerEntry(src)
		custLedgEntry2.SetRange("closed_by_entry_no", entryNo)
		custLedgEntry2.SetRange("closing_interest_calculated", false)
		custLedgEntry2.ModifyAll("closing_interest_calculated", true)
	}
	return nil
}
