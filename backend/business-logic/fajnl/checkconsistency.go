package fajnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// checkConsistencyOfEntry is codeunit 5606 FA Check Consistency, run on each FA ledger entry the
// posting inserts (not on the disposal's gain/loss and book value entries): an asset's first
// entry in a book is an acquisition cost; a disposed asset takes no entries; with the asset's
// Use FA Ledger Check, from the entry's date on the posting type's total keeps its sign (an
// acquisition cost not negative, depreciation and salvage value not positive, write-down,
// appreciation and custom 1/2 as their FA Posting Type Setup's Sign), the book value plus salvage
// value is not negative (depreciation may, with the book's Allow Depr. below Zero) and the
// depreciable basis is not negative. Then the FA depreciation book's Acquisition Date, Last …
// Date of the posting type (SetFAPostingDate).
//
// A disposal (proceeds) is checked by CheckSalesPosting; it sets the Disposal Date (and the
// insurance coverage's Disposed FA in the insurance book).
func checkConsistencyOfEntry(entry *gtables.FALedgerEntry) error {
	if entry.Fa_posting_category != gtables.FALedgerEntry_Fa_posting_category.Blank ||
		entry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.GainLoss ||
		entry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.BookValueOnDisposal {
		return nil
	}
	faNo := entry.Fa_no
	deprBookCode := entry.Depreciation_book_code
	deprBook := gtables.NewDepreciationBook(entry)
	err := deprBook.GetOrError(deprBookCode)
	if err != nil {
		return err
	}
	faDeprBook := gtables.NewFADepreciationBook(entry)
	err = faDeprBook.GetOrError(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode})
	if err != nil {
		return err
	}
	var sign gtables.FAPostingTypeSetupSign
	if setupType, ok := postingTypeSetupType(entry.Fa_posting_type); ok {
		err := tables.EnsureFAPostingTypeSetups(entry, deprBookCode)
		if err != nil {
			return err
		}
		faPostingTypeSetup := gtables.NewFAPostingTypeSetup(entry)
		err = faPostingTypeSetup.GetOrError(map[string]interface{}{"depreciation_book_code": deprBookCode, "fa_posting_type": int(setupType)})
		if err != nil {
			return err
		}
		sign = faPostingTypeSetup.Sign
	}
	if entry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal {
		err = checkSalesPosting(entry, deprBook, faDeprBook)
	} else {
		err = checkNormalPosting(entry, deprBook, faDeprBook, sign)
	}
	if err != nil {
		return err
	}
	err = setFAPostingDate(entry, faDeprBook)
	if err != nil {
		return err
	}
	return checkInsuranceIntegration(entry, deprBook, faDeprBook)
}

// checkSalesPosting (CheckSalesPosting): a disposal of an acquired asset, on its last date (no
// entry in the book value or depreciable basis after it); with Use FA Ledger Check the proceeds
// are not positive in total
func checkSalesPosting(entry *gtables.FALedgerEntry, deprBook *gtables.DepreciationBook, faDeprBook *gtables.FADepreciationBook) error {
	faNo, deprBookCode := entry.Fa_no.String(), entry.Depreciation_book_code.String()
	if faDeprBook.Acquisition_date.IsZero() {
		return apperrors.FAFirstAcquisition(faNo, deprBookCode)
	}
	after := entry.Fa_posting_date.AddDays(1).String() + ".."
	for _, field := range []string{"part_of_book_value", "part_of_depreciable_basis"} {
		later := gtables.NewFALedgerEntry(entry)
		later.SetRange("fa_no", entry.Fa_no)
		later.SetRange("depreciation_book_code", entry.Depreciation_book_code)
		later.SetRange(field, true)
		later.SetFilter("fa_posting_date", after)
		if later.FindFirst() {
			return apperrors.FADisposalLastDate(faNo, deprBookCode)
		}
	}
	if !faDeprBook.Use_fa_ledger_check {
		return deprBook.TestField("use_fa_ledger_check", false)
	}
	proceeds := gtables.NewFALedgerEntry(entry)
	proceeds.SetRange("fa_no", entry.Fa_no)
	proceeds.SetRange("depreciation_book_code", entry.Depreciation_book_code)
	proceeds.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
	proceeds.SetRange("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal)
	newAmount := types.Decimal{}
	for found := proceeds.FindSet(); found; found = proceeds.Next() {
		newAmount = newAmount.Add(proceeds.Amount)
		if newAmount.IsPositive() {
			return apperrors.FAMustNotBeSigned(true, faNo, deprBookCode, proceeds.Fa_posting_date.String(),
				gtables.FALedgerEntryTableName, "fa_posting_type", entry.Fa_posting_type)
		}
	}
	return nil
}

// checkInsuranceIntegration (CheckInsuranceIntegration): a disposal in the FA Setup's insurance
// book marks the asset's insurance coverage entries Disposed FA
func checkInsuranceIntegration(entry *gtables.FALedgerEntry, deprBook *gtables.DepreciationBook, faDeprBook *gtables.FADepreciationBook) error {
	if entry.Fa_posting_type != gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal {
		return nil
	}
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(entry)
	if !insCoverageLedgEntry.FindFirst() {
		return nil
	}
	faSetup := gtables.GetFASetup(entry)
	err := faSetup.TestField("insurance_depr_book")
	if err != nil {
		return err
	}
	if deprBook.Code != faSetup.Insurance_depr_book {
		return nil
	}
	insCoverageLedgEntry = gtables.NewInsCoverageLedgerEntry(entry)
	insCoverageLedgEntry.SetRange("fa_no", entry.Fa_no)
	insCoverageLedgEntry.ModifyAll("disposed_fa", !faDeprBook.Disposal_date.IsZero())
	return nil
}

// postingTypeSetupType: the FA Posting Type Setup line of a posting type (write-down,
// appreciation, custom 1/2 have one)
func postingTypeSetupType(postingType gtables.FALedgerEntryFa_posting_type) (gtables.FAPostingTypeSetupFa_posting_type, bool) {
	switch postingType {
	case gtables.FALedgerEntry_Fa_posting_type.WriteDown:
		return gtables.FAPostingTypeSetup_Fa_posting_type.WriteDown, true
	case gtables.FALedgerEntry_Fa_posting_type.Appreciation:
		return gtables.FAPostingTypeSetup_Fa_posting_type.Appreciation, true
	case gtables.FALedgerEntry_Fa_posting_type.Custom1:
		return gtables.FAPostingTypeSetup_Fa_posting_type.Custom1, true
	case gtables.FALedgerEntry_Fa_posting_type.Custom2:
		return gtables.FAPostingTypeSetup_Fa_posting_type.Custom2, true
	}
	return 0, false
}

// checkNormalPosting (CheckNormalPosting): the running totals from the entry's FA posting date
// on, entry by entry
func checkNormalPosting(entry *gtables.FALedgerEntry, deprBook *gtables.DepreciationBook, faDeprBook *gtables.FADepreciationBook, sign gtables.FAPostingTypeSetupSign) error {
	faNo := entry.Fa_no.String()
	deprBookCode := entry.Depreciation_book_code.String()
	if !faDeprBook.Disposal_date.IsZero() {
		return apperrors.FADisposed(faNo, deprBookCode)
	}
	faLedgEntry := gtables.NewFALedgerEntry(entry)
	faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	faLedgEntry.SetRange("fa_no", entry.Fa_no)
	faLedgEntry.SetRange("depreciation_book_code", entry.Depreciation_book_code)
	if !faLedgEntry.FindFirst() {
		return nil
	}
	if faLedgEntry.Fa_posting_type != gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost {
		return apperrors.FAFirstAcquisition(faNo, deprBookCode)
	}
	if !faDeprBook.Use_fa_ledger_check {
		return deprBook.TestField("use_fa_ledger_check", false)
	}
	before := entry.Fa_posting_date.AddDays(-1)
	sum := func(field string, value interface{}) (types.Decimal, error) {
		entries := gtables.NewFALedgerEntry(entry)
		entries.SetRange("fa_no", entry.Fa_no)
		entries.SetRange("depreciation_book_code", entry.Depreciation_book_code)
		if field == "fa_posting_type" {
			entries.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
		}
		entries.SetRange(field, value)
		entries.SetRange("fa_posting_date", types.Date{}, before)
		err := entries.CalcSums("amount")
		return entries.Amount, err
	}
	bookValue, err := sum("part_of_book_value", true)
	if err != nil {
		return err
	}
	deprBasis, err := sum("part_of_depreciable_basis", true)
	if err != nil {
		return err
	}
	salvageValue, err := sum("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.SalvageValue)
	if err != nil {
		return err
	}
	newAmount, err := sum("fa_posting_type", entry.Fa_posting_type)
	if err != nil {
		return err
	}
	faLedgEntry.SetFilter("fa_posting_date", entry.Fa_posting_date.String()+"..")
	for found := faLedgEntry.FindSet(); found; found = faLedgEntry.Next() {
		if faLedgEntry.Part_of_book_value {
			bookValue = bookValue.Add(faLedgEntry.Amount)
		}
		if faLedgEntry.Part_of_depreciable_basis {
			deprBasis = deprBasis.Add(faLedgEntry.Amount)
		}
		if faLedgEntry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.SalvageValue {
			salvageValue = salvageValue.Add(faLedgEntry.Amount)
		}
		if faLedgEntry.Fa_posting_type == entry.Fa_posting_type {
			newAmount = newAmount.Add(faLedgEntry.Amount)
		}
		err = checkForError(entry, faLedgEntry.Fa_posting_date, deprBook, sign, bookValue, deprBasis, salvageValue, newAmount)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkForError (CheckForError): the totals up to an entry
func checkForError(entry *gtables.FALedgerEntry, date types.Date, deprBook *gtables.DepreciationBook, sign gtables.FAPostingTypeSetupSign,
	bookValue, deprBasis, salvageValue, newAmount types.Decimal) error {
	faNo := entry.Fa_no.String()
	deprBookCode := entry.Depreciation_book_code.String()
	postingTypeError := func() error {
		return apperrors.FAMustNotBeSigned(newAmount.IsPositive(), faNo, deprBookCode, date.String(),
			gtables.FALedgerEntryTableName, "fa_posting_type", entry.Fa_posting_type)
	}
	switch entry.Fa_posting_type {
	case gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost:
		if newAmount.IsNegative() {
			return postingTypeError()
		}
	case gtables.FALedgerEntry_Fa_posting_type.Depreciation, gtables.FALedgerEntry_Fa_posting_type.SalvageValue:
		if newAmount.IsPositive() {
			return postingTypeError()
		}
	case gtables.FALedgerEntry_Fa_posting_type.WriteDown, gtables.FALedgerEntry_Fa_posting_type.Appreciation,
		gtables.FALedgerEntry_Fa_posting_type.Custom1, gtables.FALedgerEntry_Fa_posting_type.Custom2:
		if newAmount.IsPositive() && sign == gtables.FAPostingTypeSetup_Sign.Credit ||
			newAmount.IsNegative() && sign == gtables.FAPostingTypeSetup_Sign.Debit {
			return postingTypeError()
		}
	}
	if bookValue.Add(salvageValue).IsNegative() {
		if !deprBook.Allow_depr_below_zero || entry.Fa_posting_type != gtables.FALedgerEntry_Fa_posting_type.Depreciation {
			return apperrors.FABookValueNegative(faNo, deprBookCode, date.String())
		}
	}
	if deprBasis.IsNegative() {
		return apperrors.FADeprBasisNegative(faNo, deprBookCode, date.String())
	}
	return nil
}

// setFAPostingDate (SetFAPostingDate): the FA depreciation book's dates of the posting type —
// the last one posted; for an acquisition cost also the first (Acquisition Date) and the first
// posting date (G/L Acquisition Date)
func setFAPostingDate(entry *gtables.FALedgerEntry, faDeprBook *gtables.FADepreciationBook) error {
	faLedgEntry := gtables.NewFALedgerEntry(entry)
	faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_category", "fa_posting_type", "fa_posting_date")
	faLedgEntry.SetRange("depreciation_book_code", entry.Depreciation_book_code)
	faLedgEntry.SetRange("fa_no", entry.Fa_no)
	faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
	faLedgEntry.SetRange("fa_posting_type", entry.Fa_posting_type)
	var maxDate, minDate, glDate types.Date
	if faLedgEntry.FindLast() {
		maxDate = faLedgEntry.Fa_posting_date
	}
	if entry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost ||
		entry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal {
		if faLedgEntry.FindFirst() {
			minDate = faLedgEntry.Fa_posting_date
		}
	}
	if entry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost {
		faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_category", "fa_posting_type", "posting_date")
		if faLedgEntry.FindFirst() {
			glDate = faLedgEntry.Posting_date
		}
	}
	switch entry.Fa_posting_type {
	case gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost:
		faDeprBook.Last_acquisition_cost_date = maxDate
		faDeprBook.Acquisition_date = minDate
		faDeprBook.G_l_acquisition_date = glDate
	case gtables.FALedgerEntry_Fa_posting_type.SalvageValue:
		faDeprBook.Last_salvage_value_date = maxDate
	case gtables.FALedgerEntry_Fa_posting_type.Depreciation:
		faDeprBook.Last_depreciation_date = maxDate
	case gtables.FALedgerEntry_Fa_posting_type.WriteDown:
		faDeprBook.Last_write_down_date = maxDate
	case gtables.FALedgerEntry_Fa_posting_type.Appreciation:
		faDeprBook.Last_appreciation_date = maxDate
	case gtables.FALedgerEntry_Fa_posting_type.Custom1:
		faDeprBook.Last_custom_1_date = maxDate
	case gtables.FALedgerEntry_Fa_posting_type.Custom2:
		faDeprBook.Last_custom_2_date = maxDate
	case gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal:
		faDeprBook.Disposal_date = minDate
	}
	return faDeprBook.ModifyOrError(false)
}

// SetFAPostingDate (FA Check Consistency SetFAPostingDate, from outside the posting): the FA
// depreciation book's dates of the entry's posting type again — after a date compression made
// the entry
func SetFAPostingDate(entry *gtables.FALedgerEntry) error {
	faDeprBook := gtables.NewFADepreciationBook(entry)
	err := faDeprBook.GetOrError(map[string]interface{}{"fa_no": entry.Fa_no, "depreciation_book_code": entry.Depreciation_book_code})
	if err != nil {
		return err
	}
	return setFAPostingDate(entry, faDeprBook)
}
