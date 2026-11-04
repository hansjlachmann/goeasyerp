package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// FA MoveEntries (codeunit 5623): what deleting a fixed asset's depreciation book, a fixed
// asset or an insurance does to the entries, and the Budgeted Asset check.

// MoveFAEntries: the entries of a fixed asset in a depreciation book lose the asset when the
// book is deleted — refused for an asset with entries that is not disposed, or with entries in
// a fiscal year that is not closed (a budgeted asset's entries simply lose it)
func MoveFAEntries(fADeprBook *gtables.FADepreciationBook) error {
	fa := gtables.NewFixedAsset(fADeprBook)
	faLedgEntry := gtables.NewFALedgerEntry(fADeprBook)
	maintenanceLedgEntry := gtables.NewMaintenanceLedgerEntry(fADeprBook)
	fa.LockTable()
	faLedgEntry.LockTable()
	maintenanceLedgEntry.LockTable()
	err := fa.GetOrError(fADeprBook.Fa_no)
	if err != nil {
		return err
	}

	accountingPeriod := gtables.NewAccountingPeriod(fADeprBook)
	accountingPeriod.SetCurrentKey("closed")
	accountingPeriod.SetRange("closed", false)
	var fiscalYearStartDate types.Date
	if accountingPeriod.FindFirst() {
		fiscalYearStartDate = accountingPeriod.Starting_date
	}

	faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	maintenanceLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	faLedgEntry.SetRange("fa_no", fADeprBook.Fa_no)
	maintenanceLedgEntry.SetRange("fa_no", fADeprBook.Fa_no)
	faLedgEntry.SetRange("depreciation_book_code", fADeprBook.Depreciation_book_code)
	maintenanceLedgEntry.SetRange("depreciation_book_code", fADeprBook.Depreciation_book_code)

	if !fa.Budgeted_asset {
		if faLedgEntry.FindFirst() {
			if fADeprBook.Disposal_date.IsZero() {
				return apperrors.FANotDisposed()
			}
		}
		faLedgEntry.SetFilter("fa_posting_date", ">="+fiscalYearStartDate.String())
		if !faLedgEntry.IsEmpty() {
			return apperrors.FADeleteEntries()
		}
		maintenanceLedgEntry.SetFilter("fa_posting_date", ">="+fiscalYearStartDate.String())
		if !maintenanceLedgEntry.IsEmpty() {
			return apperrors.FADeleteEntries()
		}
		faLedgEntry.SetRange("fa_posting_date")
		maintenanceLedgEntry.SetRange("fa_posting_date")

		faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_category", "fa_posting_type", "posting_date")
		maintenanceLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "posting_date")
		faLedgEntry.SetFilter("posting_date", ">="+fiscalYearStartDate.String())
		if !faLedgEntry.IsEmpty() {
			return apperrors.FADeleteEntries()
		}
		maintenanceLedgEntry.SetFilter("posting_date", ">="+fiscalYearStartDate.String())
		if !maintenanceLedgEntry.IsEmpty() {
			return apperrors.FADeleteEntries()
		}
		faLedgEntry.SetRange("posting_date")
		maintenanceLedgEntry.SetRange("posting_date")
	}
	// DeleteNo
	faLedgEntry.ModifyAll("fa_no", types.Code(""))
	faLedgEntry.SetRange("fa_no")
	faLedgEntry.SetCurrentKey("canceled_from_fa_no")
	faLedgEntry.SetRange("canceled_from_fa_no", fADeprBook.Fa_no)
	faLedgEntry.ModifyAll("canceled_from_fa_no", types.Code(""))
	maintenanceLedgEntry.ModifyAll("fa_no", types.Code(""))
	MoveFAInsuranceEntries(fADeprBook, fADeprBook.Fa_no)
	return nil
}

// MoveInsuranceEntries: the coverage entries of a deleted insurance lose the insurance and the
// fixed asset
func MoveInsuranceEntries(insurance *gtables.Insurance) {
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(insurance)
	insCoverageLedgEntry.LockTable()
	insCoverageLedgEntry.SetCurrentKey("insurance_no")
	insCoverageLedgEntry.SetRange("insurance_no", insurance.No)
	insCoverageLedgEntry.ModifyAll("fa_no", types.Code(""))
	insCoverageLedgEntry.ModifyAll("insurance_no", types.Code(""))
}

// MoveFAInsuranceEntries: the coverage entries of a deleted fixed asset lose the insurance and
// the fixed asset
func MoveFAInsuranceEntries(from ftables.Source, faNo types.Code) {
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(from)
	insCoverageLedgEntry.SetCurrentKey("fa_no")
	insCoverageLedgEntry.SetRange("fa_no", faNo)
	insCoverageLedgEntry.ModifyAll("insurance_no", types.Code(""))
	insCoverageLedgEntry.ModifyAll("fa_no", types.Code(""))
}

// ChangeBudget: Budgeted Asset cannot change for a fixed asset with FA, maintenance or insurance
// coverage entries
func ChangeBudget(fa *gtables.FixedAsset) error {
	faLedgEntry := gtables.NewFALedgerEntry(fa)
	maintenanceLedgEntry := gtables.NewMaintenanceLedgerEntry(fa)
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(fa)
	faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	maintenanceLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	insCoverageLedgEntry.SetCurrentKey("fa_no")
	faLedgEntry.SetRange("fa_no", fa.No)
	maintenanceLedgEntry.SetRange("fa_no", fa.No)
	insCoverageLedgEntry.SetRange("fa_no", fa.No)
	if !faLedgEntry.IsEmpty() {
		return apperrors.FABudgetEntries(gtables.FixedAssetTableName, "budgeted_asset")
	}
	if !maintenanceLedgEntry.IsEmpty() {
		return apperrors.FABudgetEntries(gtables.FixedAssetTableName, "budgeted_asset")
	}
	if !insCoverageLedgEntry.IsEmpty() {
		return apperrors.FABudgetInsurance(gtables.FixedAssetTableName, "budgeted_asset")
	}
	return nil
}
