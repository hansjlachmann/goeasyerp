package genjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// CustPostApplyCustLedgEntry (codeunit 12 CustPostApplyCustLedgEntry) posts the application of a
// posted customer entry to the customer's open entries marked with its Applies-to ID (Post
// Application, codeunit 226): the entries are applied as a payment line applies when posted, and
// what the application gives — payment discount, application rounding, exchange-rate gains and
// losses — is posted against the receivables account
func (p *GenJnlPostLine) CustPostApplyCustLedgEntry(genJnlLinePostApply *gtables.GenJournalLine, custLedgEntryPostApply *gtables.CustomerLedgerEntry) error {
	p.genJnlLine = gtables.NewGenJournalLine(p.src)
	p.genJnlLine.Copy(genJnlLinePostApply)
	genJnlLine := p.genJnlLine
	genJnlLine.Source_currency_code = custLedgEntryPostApply.Currency_code
	custLedgEntry := gtables.NewCustomerLedgerEntry(p.src)
	custLedgEntry.Copy(custLedgEntryPostApply)
	err := p.genJnlCheckLine.RunCheck(genJnlLine)
	if err != nil {
		return err
	}
	err = p.initPostApply(genJnlLine)
	if err != nil {
		return err
	}
	cust := gtables.NewCustomer(p.src)
	err = cust.GetOrError(custLedgEntry.Customer_no)
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
	custLedgEntry.LockTable()
	// Post customer entry
	p.resetApplicationTotals()
	p.startEntryApplication(gtables.EntryApplication_Account_type.Customer, custLedgEntry.Customer_no)
	storeAppliesToDocNo := custLedgEntry.Applies_to_doc_no
	custLedgEntry.Applies_to_doc_no = ""
	err = p.applyCustLedgEntry(custLedgEntry, cust, genJnlLine)
	if err != nil {
		return err
	}
	custLedgEntry.Applies_to_id = ""
	custLedgEntry.Applies_to_doc_no = storeAppliesToDocNo
	custUpdateDebitCredit(genJnlLine.Correction, custLedgEntry)
	err = custLedgEntry.ModifyOrError(false)
	if err != nil {
		return err
	}
	if len(p.entryAppl.entries) < 2 {
		return apperrors.ApplyNothingToApply(custLedgEntry.Entry_no)
	}
	err = p.postApplicationOnly(custPostingGr, custPostingGr.Receivables_account, custPostingGr.Payment_disc_acc, custPostingGr.Curr_appln_rounding_account)
	if err != nil {
		return err
	}
	return p.finishCodeunit()
}

// VendPostApplyVendLedgEntry (codeunit 12 VendPostApplyVendLedgEntry) posts the application of a
// posted vendor entry (Post Application, codeunit 227), against the payables account
func (p *GenJnlPostLine) VendPostApplyVendLedgEntry(genJnlLinePostApply *gtables.GenJournalLine, vendLedgEntryPostApply *gtables.VendorLedgerEntry) error {
	p.genJnlLine = gtables.NewGenJournalLine(p.src)
	p.genJnlLine.Copy(genJnlLinePostApply)
	genJnlLine := p.genJnlLine
	genJnlLine.Source_currency_code = vendLedgEntryPostApply.Currency_code
	vendLedgEntry := gtables.NewVendorLedgerEntry(p.src)
	vendLedgEntry.Copy(vendLedgEntryPostApply)
	err := p.genJnlCheckLine.RunCheck(genJnlLine)
	if err != nil {
		return err
	}
	err = p.initPostApply(genJnlLine)
	if err != nil {
		return err
	}
	vend := gtables.NewVendor(p.src)
	err = vend.GetOrError(vendLedgEntry.Vendor_no)
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
	vendLedgEntry.LockTable()
	// Post vendor entry
	p.resetApplicationTotals()
	p.startEntryApplication(gtables.EntryApplication_Account_type.Vendor, vendLedgEntry.Vendor_no)
	storeAppliesToDocNo := vendLedgEntry.Applies_to_doc_no
	vendLedgEntry.Applies_to_doc_no = ""
	err = p.applyVendLedgEntry(vendLedgEntry, vend, genJnlLine)
	if err != nil {
		return err
	}
	vendLedgEntry.Applies_to_id = ""
	vendLedgEntry.Applies_to_doc_no = storeAppliesToDocNo
	vendUpdateDebitCredit(genJnlLine.Correction, vendLedgEntry)
	err = vendLedgEntry.ModifyOrError(false)
	if err != nil {
		return err
	}
	if len(p.entryAppl.entries) < 2 {
		return apperrors.ApplyNothingToApply(vendLedgEntry.Entry_no)
	}
	err = p.postApplicationOnly(vendPostingGr, vendPostingGr.Payables_account, vendPostingGr.Payment_disc_acc, vendPostingGr.Curr_appln_rounding_account)
	if err != nil {
		return err
	}
	return p.finishCodeunit()
}

// initPostApply starts the codeunit for a posting without a journal line's amount (InitCodeUnit)
func (p *GenJnlPostLine) initPostApply(genJnlLine *gtables.GenJournalLine) error {
	p.currency = gtables.NewCurrency(p.src)
	if genJnlLine.Source_currency_code == "" {
		tables.InitRoundingPrecision(p.currency)
	} else {
		err := p.currency.GetOrError(genJnlLine.Source_currency_code)
		if err != nil {
			return err
		}
	}
	return p.initCodeUnit()
}

// postApplicationOnly posts what an application of posted entries gives (CustPostApplyCustLedgEntry
// after the entry): the receivables or payables account the other way, the payment discount and the
// application rounding — the gains and losses follow in FinishCodeunit
func (p *GenJnlPostLine) postApplicationOnly(postingGroup interface {
	TestField(field string, value ...any) error
}, account, pmtDiscAccount, roundingAccount types.Code) error {
	addCurr, err := p.applicationAddCurr()
	if err != nil {
		return err
	}
	acy := p.acyCode() != ""
	// Post to Receivables Account
	if !p.applicationLCY().IsZero() || !addCurr.IsZero() && acy {
		p.recordApplicationGL(account, p.applicationLCY().Neg(), addCurr.Neg(), true)
		_, err = p.initGLEntry(account, p.applicationLCY().Neg(), addCurr.Neg(), true, true)
		if err != nil {
			return err
		}
		err = p.insertGLEntry(true)
		if err != nil {
			return err
		}
	}
	// Post payment discount
	if !p.totalPmtDiscLCY.IsZero() || !p.totalPmtDiscAddCurr.IsZero() && acy {
		err = postingGroup.TestField("payment_disc_acc")
		if err != nil {
			return err
		}
		p.recordApplicationGL(pmtDiscAccount, p.totalPmtDiscLCY, p.totalPmtDiscAddCurr, true)
		_, err = p.initGLEntry(pmtDiscAccount, p.totalPmtDiscLCY, p.totalPmtDiscAddCurr, true, true)
		if err != nil {
			return err
		}
		err = p.insertGLEntry(true)
		if err != nil {
			return err
		}
	}
	// Post to Curr. Appln. Rounding Account
	if !p.totalApplnRoundingLCY.IsZero() || !p.totalApplnRoundingAddCurr.IsZero() && acy {
		err = postingGroup.TestField("curr_appln_rounding_account")
		if err != nil {
			return err
		}
		p.recordApplicationGL(roundingAccount, p.totalApplnRoundingLCY, p.totalApplnRoundingAddCurr, true)
		_, err = p.initGLEntry(roundingAccount, p.totalApplnRoundingLCY, p.totalApplnRoundingAddCurr, true, true)
		if err != nil {
			return err
		}
		return p.insertGLEntry(true)
	}
	return nil
}

// UnapplyEntries undoes the latest application of a customer or vendor ledger entry (Unapply
// Entries, codeunits 226/227): every entry of that application gets its application fields back
// as they were before it, and the G/L postings of the application are posted again with the
// opposite amounts, on the application's posting date. An entry applied again later must have
// that application unapplied first. newLine gives the journal line of the G/L postings for the
// application's applying entry (its document); the application number is returned.
func UnapplyEntries(src ftables.Source, accountType gtables.EntryApplicationAccount_type, ledgerEntryNo int, newLine func(applyingEntryNo int) (*gtables.GenJournalLine, error)) (int, error) {
	entryAppl := gtables.NewEntryApplication(src)
	entryAppl.LockTable()
	entryAppl.SetCurrentKey("account_type", "ledger_entry_no", "application_no")
	entryAppl.SetRange("account_type", accountType)
	entryAppl.SetRange("ledger_entry_no", ledgerEntryNo)
	entryAppl.SetRange("unapplied", false)
	if !entryAppl.FindLast() {
		return 0, apperrors.UnapplyNoApplication(ledgerEntryNo)
	}
	applicationNo := entryAppl.Application_no
	postingDate := entryAppl.Posting_date

	// the entries of the application, none applied again later
	applEntry := gtables.NewEntryApplication(src)
	applEntry.SetCurrentKey("application_no")
	applEntry.SetRange("application_no", applicationNo)
	var entries []*gtables.EntryApplication
	applyingEntryNo := ledgerEntryNo
	for found := applEntry.FindSet(); found; found = applEntry.Next() {
		e := gtables.NewEntryApplication(src)
		e.Copy(applEntry)
		entries = append(entries, e)
		if applEntry.Applying_entry {
			applyingEntryNo = applEntry.Ledger_entry_no
		}
		later := gtables.NewEntryApplication(src)
		later.SetCurrentKey("account_type", "ledger_entry_no", "application_no")
		later.SetRange("account_type", applEntry.Account_type)
		later.SetRange("ledger_entry_no", applEntry.Ledger_entry_no)
		later.SetRange("unapplied", false)
		later.SetFilter("application_no", ">"+itoa(applicationNo))
		if later.FindFirst() {
			return 0, apperrors.UnapplyLaterApplication(applEntry.Ledger_entry_no, later.Application_no)
		}
	}

	// the entries as they were before the application
	for _, e := range entries {
		err := restoreEntry(src, e)
		if err != nil {
			return 0, err
		}
	}

	// the application's G/L postings, the other way
	glLine := gtables.NewEntryApplicationGL(src)
	glLine.SetRange("application_no", applicationNo)
	transactionNo := 0
	if !glLine.IsEmpty() {
		genJnlLine, err := newLine(applyingEntryNo)
		if err != nil {
			return 0, err
		}
		genJnlLine.Posting_date = postingDate
		p := NewGenJnlPostLine(src)
		err = p.postUnapply(genJnlLine, glLine)
		if err != nil {
			return 0, err
		}
		transactionNo = p.nextTransactionNo
	}

	for _, e := range entries {
		e.Unapplied = true
		e.Unapplied_transaction_no = transactionNo
		e.Unapplied_date = system.Today()
		e.Unapplied_by_user_id = types.Code(system.UserId())
		err := e.ModifyOrError(false)
		if err != nil {
			return 0, err
		}
	}
	return applicationNo, nil
}

// LastApplication is the number and posting date of a ledger entry's latest application not
// unapplied (0 when it has none) — what Unapply Entries undoes
func LastApplication(src ftables.Source, accountType gtables.EntryApplicationAccount_type, ledgerEntryNo int) (int, types.Date) {
	entryAppl := gtables.NewEntryApplication(src)
	entryAppl.SetCurrentKey("account_type", "ledger_entry_no", "application_no")
	entryAppl.SetRange("account_type", accountType)
	entryAppl.SetRange("ledger_entry_no", ledgerEntryNo)
	entryAppl.SetRange("unapplied", false)
	if !entryAppl.FindLast() {
		return 0, types.Date{}
	}
	return entryAppl.Application_no, entryAppl.Posting_date
}

// restoreEntry gives a customer or vendor entry its application fields back (Applies-to ID blank)
func restoreEntry(src ftables.Source, before *gtables.EntryApplication) error {
	if before.Account_type == gtables.EntryApplication_Account_type.Customer {
		custLedgEntry := gtables.NewCustomerLedgerEntry(src)
		err := custLedgEntry.GetOrError(before.Ledger_entry_no)
		if err != nil {
			return err
		}
		custLedgEntry.Amount = before.Amount
		custLedgEntry.Amount_lcy = before.Amount_lcy
		custLedgEntry.Original_amount_lcy = before.Original_amount_lcy
		custLedgEntry.Remaining_amount = before.Remaining_amount
		custLedgEntry.Remaining_amt_lcy = before.Remaining_amt_lcy
		custLedgEntry.Open = before.Open
		custLedgEntry.Debit_amount = before.Debit_amount
		custLedgEntry.Credit_amount = before.Credit_amount
		custLedgEntry.Debit_amount_lcy = before.Debit_amount_lcy
		custLedgEntry.Credit_amount_lcy = before.Credit_amount_lcy
		custLedgEntry.Closed_by_entry_no = before.Closed_by_entry_no
		custLedgEntry.Closed_at_date = before.Closed_at_date
		custLedgEntry.Closed_by_amount = before.Closed_by_amount
		custLedgEntry.Closed_by_amount_lcy = before.Closed_by_amount_lcy
		custLedgEntry.Closed_by_currency_code = before.Closed_by_currency_code
		custLedgEntry.Closed_by_currency_amount = before.Closed_by_currency_amount
		custLedgEntry.Pmt_disc_given_lcy = before.Pmt_disc_lcy
		custLedgEntry.Rounding_amount = before.Rounding_amount
		custLedgEntry.Rounding_amount_lcy = before.Rounding_amount_lcy
		custLedgEntry.Rounding_currency = before.Rounding_currency
		custLedgEntry.Calculate_interest = before.Calculate_interest
		custLedgEntry.Applies_to_id = ""
		return custLedgEntry.ModifyOrError(false)
	}
	vendLedgEntry := gtables.NewVendorLedgerEntry(src)
	err := vendLedgEntry.GetOrError(before.Ledger_entry_no)
	if err != nil {
		return err
	}
	vendLedgEntry.Amount = before.Amount
	vendLedgEntry.Amount_lcy = before.Amount_lcy
	vendLedgEntry.Original_amount_lcy = before.Original_amount_lcy
	vendLedgEntry.Remaining_amount = before.Remaining_amount
	vendLedgEntry.Remaining_amt_lcy = before.Remaining_amt_lcy
	vendLedgEntry.Open = before.Open
	vendLedgEntry.Debit_amount = before.Debit_amount
	vendLedgEntry.Credit_amount = before.Credit_amount
	vendLedgEntry.Debit_amount_lcy = before.Debit_amount_lcy
	vendLedgEntry.Credit_amount_lcy = before.Credit_amount_lcy
	vendLedgEntry.Closed_by_entry_no = before.Closed_by_entry_no
	vendLedgEntry.Closed_at_date = before.Closed_at_date
	vendLedgEntry.Closed_by_amount = before.Closed_by_amount
	vendLedgEntry.Closed_by_amount_lcy = before.Closed_by_amount_lcy
	vendLedgEntry.Closed_by_currency_code = before.Closed_by_currency_code
	vendLedgEntry.Closed_by_currency_amount = before.Closed_by_currency_amount
	vendLedgEntry.Pmt_disc_rcd_lcy = before.Pmt_disc_lcy
	vendLedgEntry.Rounding_amount = before.Rounding_amount
	vendLedgEntry.Rounding_amount_lcy = before.Rounding_amount_lcy
	vendLedgEntry.Rounding_currency = before.Rounding_currency
	vendLedgEntry.Applies_to_id = ""
	return vendLedgEntry.ModifyOrError(false)
}

// postUnapply posts an application's G/L postings with the opposite amounts, as one transaction
// of a G/L register (checked like a journal line: allowed posting dates)
func (p *GenJnlPostLine) postUnapply(genJnlLinePost *gtables.GenJournalLine, glLine *gtables.EntryApplicationGL) error {
	p.genJnlLine = gtables.NewGenJournalLine(p.src)
	p.genJnlLine.Copy(genJnlLinePost)
	err := p.genJnlCheckLine.RunCheck(p.genJnlLine)
	if err != nil {
		return err
	}
	err = p.initPostApply(p.genJnlLine)
	if err != nil {
		return err
	}
	for found := glLine.FindSet(); found; found = glLine.Next() {
		_, err = p.initGLEntry(glLine.Gl_account_no, glLine.Amount.Neg(), glLine.Additional_currency_amount.Neg(), glLine.Use_add_curr_amount, true)
		if err != nil {
			return err
		}
		err = p.insertGLEntry(true)
		if err != nil {
			return err
		}
	}
	return p.finishCodeunit()
}

// PostCustApplication is codeunit 226 CustEntry-Apply Posted Entries without its question: the
// application of a posted customer entry to the open entries marked with its Applies-to ID, posted
// with the entry's posting date, document and dimensions (source code Sales Entry Application)
func PostCustApplication(src ftables.Source, custLedgEntry *gtables.CustomerLedgerEntry) error {
	err := custLedgEntry.TestField("applies_to_id")
	if err != nil {
		return err
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(src)
	genJnlLine := gtables.NewGenJournalLine(src)
	genJnlLine.Init()
	genJnlLine.Posting_date = custLedgEntry.Posting_date
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.Customer
	genJnlLine.Account_no = custLedgEntry.Customer_no
	genJnlLine.Correction = custLedgEntry.Debit_amount.IsNegative() || custLedgEntry.Credit_amount.IsNegative() ||
		custLedgEntry.Debit_amount_lcy.IsNegative() || custLedgEntry.Credit_amount_lcy.IsNegative()
	genJnlLine.Document_type = gtables.GenJournalLineDocument_type(custLedgEntry.Document_type)
	genJnlLine.Document_no = custLedgEntry.Document_no
	genJnlLine.Description = custLedgEntry.Description
	genJnlLine.Dimension_1_code = custLedgEntry.Dimension_1_code
	genJnlLine.Dimension_2_code = custLedgEntry.Dimension_2_code
	genJnlLine.Posting_group = custLedgEntry.Customer_posting_group
	genJnlLine.Source_type = gtables.GenJournalLine_Source_type.Customer
	genJnlLine.Source_no = custLedgEntry.Customer_no
	genJnlLine.Source_code = sourceCodeSetup.Sales_entry_application
	genJnlLine.System_created_entry = true
	return NewGenJnlPostLine(src).CustPostApplyCustLedgEntry(genJnlLine, custLedgEntry)
}

// PostVendApplication is codeunit 227 VendEntry-Apply Posted Entries without its question
// (source code Purchase Entry Application)
func PostVendApplication(src ftables.Source, vendLedgEntry *gtables.VendorLedgerEntry) error {
	err := vendLedgEntry.TestField("applies_to_id")
	if err != nil {
		return err
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(src)
	genJnlLine := gtables.NewGenJournalLine(src)
	genJnlLine.Init()
	genJnlLine.Posting_date = vendLedgEntry.Posting_date
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.Vendor
	genJnlLine.Account_no = vendLedgEntry.Vendor_no
	genJnlLine.Correction = vendLedgEntry.Debit_amount.IsNegative() || vendLedgEntry.Credit_amount.IsNegative() ||
		vendLedgEntry.Debit_amount_lcy.IsNegative() || vendLedgEntry.Credit_amount_lcy.IsNegative()
	genJnlLine.Document_type = gtables.GenJournalLineDocument_type(vendLedgEntry.Document_type)
	genJnlLine.Document_no = vendLedgEntry.Document_no
	genJnlLine.Description = vendLedgEntry.Description
	genJnlLine.Dimension_1_code = vendLedgEntry.Dimension_1_code
	genJnlLine.Dimension_2_code = vendLedgEntry.Dimension_2_code
	genJnlLine.Posting_group = vendLedgEntry.Vendor_posting_group
	genJnlLine.Source_type = gtables.GenJournalLine_Source_type.Vendor
	genJnlLine.Source_no = vendLedgEntry.Vendor_no
	genJnlLine.Source_code = sourceCodeSetup.Purchase_entry_application
	genJnlLine.System_created_entry = true
	return NewGenJnlPostLine(src).VendPostApplyVendLedgEntry(genJnlLine, vendLedgEntry)
}

// UnapplyCustEntry unapplies the latest application of a customer entry (Unapply Entries,
// codeunit 226): the G/L postings go back under the entry's document no. (source code Sales Entry
// Application). It returns the application's number.
func UnapplyCustEntry(src ftables.Source, rec *gtables.CustomerLedgerEntry) (int, error) {
	return UnapplyEntries(src, gtables.EntryApplication_Account_type.Customer, rec.Entry_no, func(applyingEntryNo int) (*gtables.GenJournalLine, error) {
		custLedgEntry := gtables.NewCustomerLedgerEntry(src)
		err := custLedgEntry.GetOrError(applyingEntryNo)
		if err != nil {
			return nil, err
		}
		return custUnapplyLine(src, custLedgEntry), nil
	})
}

// custUnapplyLine is the journal line of an unapplication's G/L postings: the applying entry's
// customer, document and dimensions
func custUnapplyLine(src ftables.Source, custLedgEntry *gtables.CustomerLedgerEntry) *gtables.GenJournalLine {
	genJnlLine := gtables.NewGenJournalLine(src)
	genJnlLine.Init()
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.Customer
	genJnlLine.Account_no = custLedgEntry.Customer_no
	genJnlLine.Document_type = gtables.GenJournalLineDocument_type(custLedgEntry.Document_type)
	genJnlLine.Document_no = custLedgEntry.Document_no
	genJnlLine.Description = custLedgEntry.Description
	genJnlLine.Dimension_1_code = custLedgEntry.Dimension_1_code
	genJnlLine.Dimension_2_code = custLedgEntry.Dimension_2_code
	genJnlLine.Source_type = gtables.GenJournalLine_Source_type.Customer
	genJnlLine.Source_no = custLedgEntry.Customer_no
	genJnlLine.Source_code = gtables.GetSourceCodeSetup(src).Sales_entry_application
	genJnlLine.System_created_entry = true
	return genJnlLine
}

// UnapplyVendEntry unapplies the latest application of a vendor entry (codeunit 227)
func UnapplyVendEntry(src ftables.Source, rec *gtables.VendorLedgerEntry) (int, error) {
	return UnapplyEntries(src, gtables.EntryApplication_Account_type.Vendor, rec.Entry_no, func(applyingEntryNo int) (*gtables.GenJournalLine, error) {
		vendLedgEntry := gtables.NewVendorLedgerEntry(src)
		err := vendLedgEntry.GetOrError(applyingEntryNo)
		if err != nil {
			return nil, err
		}
		return vendUnapplyLine(src, vendLedgEntry), nil
	})
}

// vendUnapplyLine is the journal line of an unapplication's G/L postings (the applying vendor entry)
func vendUnapplyLine(src ftables.Source, vendLedgEntry *gtables.VendorLedgerEntry) *gtables.GenJournalLine {
	genJnlLine := gtables.NewGenJournalLine(src)
	genJnlLine.Init()
	genJnlLine.Account_type = gtables.GenJournalLine_Account_type.Vendor
	genJnlLine.Account_no = vendLedgEntry.Vendor_no
	genJnlLine.Document_type = gtables.GenJournalLineDocument_type(vendLedgEntry.Document_type)
	genJnlLine.Document_no = vendLedgEntry.Document_no
	genJnlLine.Description = vendLedgEntry.Description
	genJnlLine.Dimension_1_code = vendLedgEntry.Dimension_1_code
	genJnlLine.Dimension_2_code = vendLedgEntry.Dimension_2_code
	genJnlLine.Source_type = gtables.GenJournalLine_Source_type.Vendor
	genJnlLine.Source_no = vendLedgEntry.Vendor_no
	genJnlLine.Source_code = gtables.GetSourceCodeSetup(src).Purchase_entry_application
	genJnlLine.System_created_entry = true
	return genJnlLine
}
