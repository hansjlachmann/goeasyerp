package genjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The application register (tables 905 Entry Application, 906 Entry Application G/L): every
// application of customer or vendor entries the posting makes — a journal line applied when it
// is posted, or posted entries applied with Post Application (codeunits 226/227) — is recorded
// with the entries' application fields as they were before it and the G/L postings it made, so
// Unapply Entries can undo it.

// entryApplication collects one application while the line is posted
type entryApplication struct {
	accountType gtables.EntryApplicationAccount_type
	accountNo   types.Code
	entries     []*gtables.EntryApplication // the entries' fields before, in the order they were applied
	recorded    map[int]bool
	glLines     []entryApplicationGL
	taxEntries  []int // the tax entries it wrote: realized unrealized tax, payment discount tax
}

// entryApplicationGL is one G/L posting of an application
type entryApplicationGL struct {
	glAccNo       types.Code
	amount        types.Decimal
	amountAddCurr types.Decimal
	useAddCurr    bool
}

// startEntryApplication: a customer or vendor line starts recording its application
func (p *GenJnlPostLine) startEntryApplication(accountType gtables.EntryApplicationAccount_type, accountNo types.Code) {
	p.entryAppl = &entryApplication{accountType: accountType, accountNo: accountNo, recorded: map[int]bool{}}
	p.entryAppls = append(p.entryAppls, p.entryAppl)
}

// recordEntry keeps a ledger entry's application fields before the application changes them
// (once per entry and application)
func (p *GenJnlPostLine) recordEntry(entryNo int, applying bool, fill func(before *gtables.EntryApplication)) {
	if p.entryAppl == nil || p.entryAppl.recorded[entryNo] {
		return
	}
	before := gtables.NewEntryApplication(p.src)
	before.Init()
	before.Account_type = p.entryAppl.accountType
	before.Account_no = p.entryAppl.accountNo
	before.Ledger_entry_no = entryNo
	before.Applying_entry = applying
	fill(before)
	p.entryAppl.recorded[entryNo] = true
	p.entryAppl.entries = append(p.entryAppl.entries, before)
}

// recordCustEntry keeps a customer entry's application fields before the application
func (p *GenJnlPostLine) recordCustEntry(custLedgEntry *gtables.CustomerLedgerEntry, applying bool) {
	p.recordEntry(custLedgEntry.Entry_no, applying, func(before *gtables.EntryApplication) {
		before.Document_no = custLedgEntry.Document_no
		before.Amount = custLedgEntry.Amount
		before.Amount_lcy = custLedgEntry.Amount_lcy
		before.Original_amount_lcy = custLedgEntry.Original_amount_lcy
		before.Remaining_amount = custLedgEntry.Remaining_amount
		before.Remaining_amt_lcy = custLedgEntry.Remaining_amt_lcy
		before.Open = custLedgEntry.Open
		before.Debit_amount = custLedgEntry.Debit_amount
		before.Credit_amount = custLedgEntry.Credit_amount
		before.Debit_amount_lcy = custLedgEntry.Debit_amount_lcy
		before.Credit_amount_lcy = custLedgEntry.Credit_amount_lcy
		before.Closed_by_entry_no = custLedgEntry.Closed_by_entry_no
		before.Closed_at_date = custLedgEntry.Closed_at_date
		before.Closed_by_amount = custLedgEntry.Closed_by_amount
		before.Closed_by_amount_lcy = custLedgEntry.Closed_by_amount_lcy
		before.Closed_by_currency_code = custLedgEntry.Closed_by_currency_code
		before.Closed_by_currency_amount = custLedgEntry.Closed_by_currency_amount
		before.Pmt_disc_lcy = custLedgEntry.Pmt_disc_given_lcy
		before.Rounding_amount = custLedgEntry.Rounding_amount
		before.Rounding_amount_lcy = custLedgEntry.Rounding_amount_lcy
		before.Rounding_currency = custLedgEntry.Rounding_currency
		before.Applies_to_id = custLedgEntry.Applies_to_id
		before.Calculate_interest = custLedgEntry.Calculate_interest
	})
}

// recordVendEntry keeps a vendor entry's application fields before the application
func (p *GenJnlPostLine) recordVendEntry(vendLedgEntry *gtables.VendorLedgerEntry, applying bool) {
	p.recordEntry(vendLedgEntry.Entry_no, applying, func(before *gtables.EntryApplication) {
		before.Document_no = vendLedgEntry.Document_no
		before.Amount = vendLedgEntry.Amount
		before.Amount_lcy = vendLedgEntry.Amount_lcy
		before.Original_amount_lcy = vendLedgEntry.Original_amount_lcy
		before.Remaining_amount = vendLedgEntry.Remaining_amount
		before.Remaining_amt_lcy = vendLedgEntry.Remaining_amt_lcy
		before.Open = vendLedgEntry.Open
		before.Debit_amount = vendLedgEntry.Debit_amount
		before.Credit_amount = vendLedgEntry.Credit_amount
		before.Debit_amount_lcy = vendLedgEntry.Debit_amount_lcy
		before.Credit_amount_lcy = vendLedgEntry.Credit_amount_lcy
		before.Closed_by_entry_no = vendLedgEntry.Closed_by_entry_no
		before.Closed_at_date = vendLedgEntry.Closed_at_date
		before.Closed_by_amount = vendLedgEntry.Closed_by_amount
		before.Closed_by_amount_lcy = vendLedgEntry.Closed_by_amount_lcy
		before.Closed_by_currency_code = vendLedgEntry.Closed_by_currency_code
		before.Closed_by_currency_amount = vendLedgEntry.Closed_by_currency_amount
		before.Pmt_disc_lcy = vendLedgEntry.Pmt_disc_rcd_lcy
		before.Rounding_amount = vendLedgEntry.Rounding_amount
		before.Rounding_amount_lcy = vendLedgEntry.Rounding_amount_lcy
		before.Rounding_currency = vendLedgEntry.Rounding_currency
		before.Applies_to_id = vendLedgEntry.Applies_to_id
	})
}

// recordApplicationGL keeps a G/L posting of the application (one with an amount)
func (p *GenJnlPostLine) recordApplicationGL(glAccNo types.Code, amount, amountAddCurr types.Decimal, useAddCurr bool) {
	appl := p.entryAppl
	if appl == nil || len(appl.entries) == 0 {
		// the gains and losses are posted after the line's accounts: the first application
		for _, a := range p.entryAppls {
			if len(a.entries) > 0 {
				appl = a
				break
			}
		}
	}
	if appl == nil || len(appl.entries) == 0 || amount.IsZero() && amountAddCurr.IsZero() {
		return
	}
	appl.glLines = append(appl.glLines, entryApplicationGL{glAccNo, amount, amountAddCurr, useAddCurr})
}

// recordApplicationTaxEntry keeps a tax entry the application wrote (Unapply Entries reverses it)
func (p *GenJnlPostLine) recordApplicationTaxEntry(taxEntryNo int) {
	appl := p.entryAppl
	if appl == nil || len(appl.entries) == 0 {
		return
	}
	appl.taxEntries = append(appl.taxEntries, taxEntryNo)
}

// writeEntryApplications writes the line's applications into the register (FinishCodeunit): the
// entries with the amount applied (their remaining amount before less now) and the G/L
// postings, under the line's transaction
func (p *GenJnlPostLine) writeEntryApplications() error {
	appls := p.entryAppls
	p.entryAppls = nil
	p.entryAppl = nil
	for _, appl := range appls {
		if len(appl.entries) < 2 {
			continue
		}
		last := gtables.NewEntryApplication(p.src)
		last.LockTable()
		entryNo, applicationNo := 0, 1
		if last.FindLast() {
			entryNo = last.Entry_no
		}
		last.SetCurrentKey("application_no")
		if last.FindLast() {
			applicationNo = last.Application_no + 1
		}
		for _, before := range appl.entries {
			remaining, err := p.remainingAmount(appl.accountType, before.Ledger_entry_no)
			if err != nil {
				return err
			}
			entryNo++
			before.Entry_no = entryNo
			before.Application_no = applicationNo
			before.Posting_date = p.genJnlLine.Posting_date
			before.Transaction_no = p.nextTransactionNo
			before.User_id = types.Code(system.UserId())
			before.Amount_applied = before.Remaining_amount.Sub(remaining)
			err = before.InsertOrError(false)
			if err != nil {
				return err
			}
		}
		for i, line := range appl.glLines {
			glLine := gtables.NewEntryApplicationGL(p.src)
			glLine.Init()
			glLine.Application_no = applicationNo
			glLine.Line_no = (i + 1) * 10000
			glLine.Gl_account_no = line.glAccNo
			glLine.Amount = line.amount
			glLine.Additional_currency_amount = line.amountAddCurr
			glLine.Use_add_curr_amount = line.useAddCurr
			err := glLine.InsertOrError(false)
			if err != nil {
				return err
			}
		}
		for i, taxEntryNo := range appl.taxEntries {
			glLine := gtables.NewEntryApplicationGL(p.src)
			glLine.Init()
			glLine.Application_no = applicationNo
			glLine.Line_no = (len(appl.glLines) + i + 1) * 10000
			glLine.Tax_entry_no = taxEntryNo
			err := glLine.InsertOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// remainingAmount is a customer or vendor entry's remaining amount now
func (p *GenJnlPostLine) remainingAmount(accountType gtables.EntryApplicationAccount_type, entryNo int) (types.Decimal, error) {
	if accountType == gtables.EntryApplication_Account_type.Customer {
		custLedgEntry := gtables.NewCustomerLedgerEntry(p.src)
		err := custLedgEntry.GetOrError(entryNo)
		return custLedgEntry.Remaining_amount, err
	}
	vendLedgEntry := gtables.NewVendorLedgerEntry(p.src)
	err := vendLedgEntry.GetOrError(entryNo)
	return vendLedgEntry.Remaining_amount, err
}
