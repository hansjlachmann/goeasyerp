package fadepr

import (
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Result is what a depreciation calculation gives: the depreciation and custom 1 amounts (zero or
// negative) and their numbers of days
type Result struct {
	DeprAmount          types.Decimal
	Custom1Amount       types.Decimal
	NumberOfDays        int
	Custom1NumberOfDays int
}

// Calculate is codeunit 5610 Calculate Depreciation: an asset's depreciation in a book up to a
// date — with the book's Use Custom 1 Depreciation and a Depr. Ending Date (Custom 1) by codeunit
// 5612 (depreciation and custom 1), else by codeunit 5611. Projections pass a date to start from
// and their amounts (EntryAmounts); daysInPeriod forces the number of days (Force No. of Days).
func Calculate(src ftables.Source, faNo, deprBookCode types.Code, untilDate types.Date, entryAmounts EntryAmounts, dateFromProjection types.Date, daysInPeriod int) (Result, error) {
	deprBook := gtables.NewDepreciationBook(src)
	if !deprBook.Get(deprBookCode) {
		return Result{}, nil
	}
	faDeprBook := gtables.NewFADepreciationBook(src)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode}) {
		return Result{}, nil
	}
	if deprBook.Use_custom_1_depreciation && !faDeprBook.Depr_ending_date_custom_1.IsZero() {
		deprAmount, custom1Amount, days, custom1Days, err := calculateCustom1(src, faNo, deprBookCode, untilDate, entryAmounts, dateFromProjection, daysInPeriod)
		return Result{DeprAmount: deprAmount, Custom1Amount: custom1Amount, NumberOfDays: days, Custom1NumberOfDays: custom1Days}, err
	}
	deprAmount, days, err := calculateNormal(src, faNo, deprBookCode, untilDate, entryAmounts, dateFromProjection, daysInPeriod)
	return Result{DeprAmount: deprAmount, NumberOfDays: days}, err
}

// AcqCostDepr is codeunit 5613 Calculate Acq. Cost Depr.: the depreciation an acquisition adds
// (Depr. Acquisition Cost) — the depreciation so far in proportion to the new depreciable basis
// (and custom 1 likewise on the acquisition cost)
func AcqCostDepr(src ftables.Source, faNo, deprBookCode types.Code, localDeprBasis, custom1LocalDeprBasis types.Decimal) (deprAmount, custom1Amount types.Decimal, err error) {
	deprBook := gtables.NewDepreciationBook(src)
	if err = deprBook.GetOrError(deprBookCode); err != nil {
		return
	}
	faDeprBook := gtables.NewFADepreciationBook(src)
	if !faDeprBook.Get(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode}) {
		return
	}
	faDeprBook.CalcFields("depreciation", "acquisition_cost", "depreciable_basis")
	deprBasis := faDeprBook.Depreciable_basis.Sub(localDeprBasis)
	if deprBasis.IsPositive() {
		deprAmount = CalcRounding(deprBook, faDeprBook.Depreciation.Mul(localDeprBasis).Div(deprBasis))
	}
	if deprBook.Use_custom_1_depreciation && !faDeprBook.Depr_ending_date_custom_1.IsZero() {
		deprBasis = faDeprBook.Acquisition_cost.Sub(custom1LocalDeprBasis)
		if deprBasis.IsPositive() {
			custom1Amount = CalcRounding(deprBook, faDeprBook.Depreciation.Mul(custom1LocalDeprBasis).Div(deprBasis))
		}
	}
	return
}
