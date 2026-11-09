package checkmgt

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Void types of a financial void (form 10113 Void Confirmation; electronic payments are not
// migrated, so there is no "Void direct deposits only")
const (
	VoidTypeReverseOriginalEntry = 0 // Unapply and void: the payment's applications undone, the void applied to it
	VoidTypeVoidCheckOnly        = 1 // the check's amount back into the bank account, the payment stays applied
)

// FinancialVoidCheck (FinancialVoidCheck): a posted check that has not been on a bank statement is
// voided after it was posted — a line on the void date puts the check's amount (every posted
// entry of the check no. and check date) back into the bank account against the check's balancing
// account (source code Financially Voided Check). Reverse Original Entry first unapplies the
// payment's applications to a customer or vendor (the invoices are open again, a payment
// discount taken goes back — the application register's Unapply Entries, which also replaces
// the classic BackOutDiscounts) and applies the void to the payment, so both are closed; the
// bank account must be in LCY. The check's entries become Financially Voided.
func (m *CheckManagement) FinancialVoidCheck(checkLedgEntry *gtables.CheckLedgerEntry, voidDate types.Date, voidType int) error {
	err := checkLedgEntry.TestField("entry_status", gtables.CheckLedgerEntry_Entry_status.Posted)
	if err == nil {
		err = checkLedgEntry.TestField("statement_status", gtables.CheckLedgerEntry_Statement_status.Open)
	}
	if err == nil {
		err = checkLedgEntry.TestField("bal_account_no")
	}
	if err != nil {
		return err
	}
	if voidDate.Before(checkLedgEntry.Check_date) {
		return apperrors.CheckVoidDateBeforeCheckDate(format.Date(checkLedgEntry.Check_date, fcodeunits.CurrentLanguage()))
	}

	bankAcc := gtables.NewBankAccount(m.src)
	err = bankAcc.GetOrError(checkLedgEntry.Bank_account_no)
	if err != nil {
		return err
	}
	bankAccLedgEntry2 := gtables.NewBankAccountLedgerEntry(m.src)
	err = bankAccLedgEntry2.GetOrError(checkLedgEntry.Bank_account_ledger_entry_no)
	if err != nil {
		return err
	}
	sourceCodeSetup := gtables.GetSourceCodeSetup(m.src)

	amountToVoid := types.Decimal{}
	checkLedgEntry2 := gtables.NewCheckLedgerEntry(m.src)
	setCheckFilters(checkLedgEntry2, checkLedgEntry, voidType)
	for found := checkLedgEntry2.FindSet(); found; found = checkLedgEntry2.Next() {
		amountToVoid = amountToVoid.Add(checkLedgEntry2.Amount)
	}

	genJnlLine2 := gtables.NewGenJournalLine(m.src)
	genJnlLine2.Init()
	genJnlLine2.System_created_entry = true
	genJnlLine2.Document_no = checkLedgEntry.Document_no
	genJnlLine2.Document_type = gtables.GenJournalLine_Document_type.Payment
	genJnlLine2.Account_type = gtables.GenJournalLine_Account_type.BankAccount
	genJnlLine2.Posting_date = voidDate
	err = genJnlLine2.Validate("account_no", checkLedgEntry.Bank_account_no)
	if err != nil {
		return err
	}
	genJnlLine2.Description = types.Text(system.StrSubstNo(i18n.GetInstance().Message("CHECK_VOIDING_DESCRIPTION", fcodeunits.CurrentLanguage()), checkLedgEntry.Check_no))
	err = genJnlLine2.Validate("amount", amountToVoid)
	if err != nil {
		return err
	}
	genJnlLine2.Bal_account_type = gtables.GenJournalLineBal_account_type(checkLedgEntry.Bal_account_type)
	err = genJnlLine2.Validate("bal_account_no", checkLedgEntry.Bal_account_no)
	if err == nil {
		err = genJnlLine2.Validate("dimension_1_code", bankAccLedgEntry2.Dimension_1_code)
	}
	if err == nil {
		err = genJnlLine2.Validate("dimension_2_code", bankAccLedgEntry2.Dimension_2_code)
	}
	if err != nil {
		return err
	}
	genJnlLine2.Source_code = sourceCodeSetup.Financially_voided_check
	if voidType == VoidTypeReverseOriginalEntry {
		err = bankAcc.TestField("currency_code", "")
		if err != nil {
			return err
		}
		unapplied, err := m.unApplyInvoices(checkLedgEntry)
		if err != nil {
			return err
		}
		if unapplied {
			genJnlLine2.Applies_to_id = checkLedgEntry.Document_no
		}
	}
	err = genjnl.NewGenJnlPostLine(m.src).Run(genJnlLine2)
	if err != nil {
		return err
	}

	return m.markCheckEntriesVoid(checkLedgEntry, voidType)
}

// setCheckFilters: the posted entries of the check — its bank account, check no. and check date,
// not on a statement — of the void type's bank payment types
func setCheckFilters(checkLedgEntry2 *gtables.CheckLedgerEntry, checkLedgEntry *gtables.CheckLedgerEntry, voidType int) {
	checkLedgEntry2.SetCurrentKey("bank_account_no", "entry_status", "check_no")
	checkLedgEntry2.SetRange("bank_account_no", checkLedgEntry.Bank_account_no)
	checkLedgEntry2.SetRange("entry_status", gtables.CheckLedgerEntry_Entry_status.Posted)
	checkLedgEntry2.SetRange("statement_status", gtables.CheckLedgerEntry_Statement_status.Open)
	checkLedgEntry2.SetRange("check_no", checkLedgEntry.Check_no)
	checkLedgEntry2.SetRange("check_date", checkLedgEntry.Check_date)
	if voidType == VoidTypeVoidCheckOnly {
		checkLedgEntry2.SetFilter("bank_payment_type", strconv.Itoa(int(gtables.CheckLedgerEntry_Bank_payment_type.ComputerCheck))+"|"+
			strconv.Itoa(int(gtables.CheckLedgerEntry_Bank_payment_type.ManualCheck)))
	}
}

// unApplyInvoices (UnApplyInvoices): the customer's or vendor's payment entries of the check's
// document lose every application (latest first, Unapply Entries) and get the document no. as
// Applies-to ID, so the void applies to them; false when the balancing account is no customer or
// vendor, or the check's payment entry is not found
func (m *CheckManagement) unApplyInvoices(checkLedgEntry *gtables.CheckLedgerEntry) (bool, error) {
	if checkLedgEntry.Bal_account_type == gtables.CheckLedgerEntry_Bal_account_type.Vendor {
		origPaymentVendLedgEntry := gtables.NewVendorLedgerEntry(m.src)
		origPaymentVendLedgEntry.SetCurrentKey("document_type", "document_no", "vendor_no")
		origPaymentVendLedgEntry.SetRange("vendor_no", checkLedgEntry.Bal_account_no)
		origPaymentVendLedgEntry.SetRange("document_type", int(checkLedgEntry.Document_type))
		origPaymentVendLedgEntry.SetRange("document_no", checkLedgEntry.Document_no)
		var entryNos []int
		for found := origPaymentVendLedgEntry.FindSetBuffered(); found; found = origPaymentVendLedgEntry.Next() {
			entryNos = append(entryNos, origPaymentVendLedgEntry.Entry_no)
		}
		if len(entryNos) == 0 {
			return false, nil
		}
		for _, entryNo := range entryNos {
			err := m.unapplyAll(gtables.EntryApplication_Account_type.Vendor, entryNo)
			if err != nil {
				return false, err
			}
			vendLedgEntry := gtables.NewVendorLedgerEntry(m.src)
			err = vendLedgEntry.GetOrError(entryNo)
			if err != nil {
				return false, err
			}
			vendLedgEntry.Applies_to_id = checkLedgEntry.Document_no
			err = vendLedgEntry.ModifyOrError(false)
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}
	if checkLedgEntry.Bal_account_type == gtables.CheckLedgerEntry_Bal_account_type.Customer {
		origPaymentCustLedgEntry := gtables.NewCustomerLedgerEntry(m.src)
		origPaymentCustLedgEntry.SetCurrentKey("document_type", "document_no", "customer_no")
		origPaymentCustLedgEntry.SetRange("customer_no", checkLedgEntry.Bal_account_no)
		origPaymentCustLedgEntry.SetRange("document_type", int(checkLedgEntry.Document_type))
		origPaymentCustLedgEntry.SetRange("document_no", checkLedgEntry.Document_no)
		var entryNos []int
		for found := origPaymentCustLedgEntry.FindSetBuffered(); found; found = origPaymentCustLedgEntry.Next() {
			entryNos = append(entryNos, origPaymentCustLedgEntry.Entry_no)
		}
		if len(entryNos) == 0 {
			return false, nil
		}
		for _, entryNo := range entryNos {
			err := m.unapplyAll(gtables.EntryApplication_Account_type.Customer, entryNo)
			if err != nil {
				return false, err
			}
			custLedgEntry := gtables.NewCustomerLedgerEntry(m.src)
			err = custLedgEntry.GetOrError(entryNo)
			if err != nil {
				return false, err
			}
			custLedgEntry.Applies_to_id = checkLedgEntry.Document_no
			err = custLedgEntry.ModifyOrError(false)
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}

// unapplyAll undoes every application of a ledger entry, the latest first
func (m *CheckManagement) unapplyAll(accountType gtables.EntryApplicationAccount_type, entryNo int) error {
	entryAppl := gtables.NewEntryApplication(m.src)
	entryAppl.SetCurrentKey("account_type", "ledger_entry_no", "application_no")
	entryAppl.SetRange("account_type", accountType)
	entryAppl.SetRange("ledger_entry_no", entryNo)
	entryAppl.SetRange("unapplied", false)
	for !entryAppl.IsEmpty() {
		var err error
		if accountType == gtables.EntryApplication_Account_type.Vendor {
			vendLedgEntry := gtables.NewVendorLedgerEntry(m.src)
			err = vendLedgEntry.GetOrError(entryNo)
			if err == nil {
				_, err = genjnl.UnapplyVendEntry(m.src, vendLedgEntry)
			}
		} else {
			custLedgEntry := gtables.NewCustomerLedgerEntry(m.src)
			err = custLedgEntry.GetOrError(entryNo)
			if err == nil {
				_, err = genjnl.UnapplyCustEntry(m.src, custLedgEntry)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// markCheckEntriesVoid (MarkCheckEntriesVoid): the check's other posted entries of the void type,
// and the check ledger entry itself, become Financially Voided (Original Entry Status: Posted)
func (m *CheckManagement) markCheckEntriesVoid(originalCheckEntry *gtables.CheckLedgerEntry, voidType int) error {
	relatedCheckEntry := gtables.NewCheckLedgerEntry(m.src)
	setCheckFilters(relatedCheckEntry, originalCheckEntry, voidType)
	relatedCheckEntry.SetFilter("entry_no", "<>"+strconv.Itoa(originalCheckEntry.Entry_no))
	voidOriginal := true
	if voidType == VoidTypeVoidCheckOnly {
		voidOriginal = originalCheckEntry.Bank_payment_type == gtables.CheckLedgerEntry_Bank_payment_type.ComputerCheck ||
			originalCheckEntry.Bank_payment_type == gtables.CheckLedgerEntry_Bank_payment_type.ManualCheck
	}
	for found := relatedCheckEntry.FindSetBuffered(); found; found = relatedCheckEntry.Next() {
		relatedCheckEntry2 := gtables.NewCheckLedgerEntry(m.src)
		relatedCheckEntry2.Copy(relatedCheckEntry)
		relatedCheckEntry2.Original_entry_status = gtables.CheckLedgerEntryOriginal_entry_status(relatedCheckEntry.Entry_status)
		relatedCheckEntry2.Entry_status = gtables.CheckLedgerEntry_Entry_status.FinanciallyVoided
		err := relatedCheckEntry2.ModifyOrError(false)
		if err != nil {
			return err
		}
	}

	if voidOriginal {
		checkLedgEntry := gtables.NewCheckLedgerEntry(m.src)
		err := checkLedgEntry.GetOrError(originalCheckEntry.Entry_no)
		if err != nil {
			return err
		}
		checkLedgEntry.Original_entry_status = gtables.CheckLedgerEntryOriginal_entry_status(checkLedgEntry.Entry_status)
		checkLedgEntry.Entry_status = gtables.CheckLedgerEntry_Entry_status.FinanciallyVoided
		err = checkLedgEntry.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReverseAllowed: Reverse Original Entry is offered only while none of the check's entries is
// financially voided or on a bank statement (form 10113's OnOpenForm)
func ReverseAllowed(checkLedgEntry *gtables.CheckLedgerEntry) bool {
	checkLedgEntry2 := gtables.NewCheckLedgerEntry(checkLedgEntry)
	checkLedgEntry2.SetCurrentKey("bank_account_no", "entry_status", "check_no")
	checkLedgEntry2.SetRange("bank_account_no", checkLedgEntry.Bank_account_no)
	checkLedgEntry2.SetRange("check_date", checkLedgEntry.Check_date)
	checkLedgEntry2.SetRange("check_no", checkLedgEntry.Check_no)
	checkLedgEntry2.SetRange("entry_status", gtables.CheckLedgerEntry_Entry_status.FinanciallyVoided)
	numAlreadyVoided := checkLedgEntry2.Count()
	checkLedgEntry2.SetRange("entry_status", gtables.CheckLedgerEntry_Entry_status.Posted)
	checkLedgEntry2.SetFilter("statement_status", "<>"+strconv.Itoa(int(gtables.CheckLedgerEntry_Statement_status.Open)))
	numAlreadyCleared := checkLedgEntry2.Count()
	return numAlreadyVoided == 0 && numAlreadyCleared == 0
}
