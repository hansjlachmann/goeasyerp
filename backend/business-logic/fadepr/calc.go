// Package fadepr is the fixed asset depreciation calculation: codeunits 5610 Calculate
// Depreciation, 5611 Calculate Normal Depreciation, 5612 Calculate Custom 1 Depr., 5613 Calculate
// Acq. Cost Depr., 5616 Depreciation Calculation, 5617 FA Date Calculation and 5618 Table Depr.
// Calculation. It computes amounts; posting them is the FA journals' (package fajnl) and report
// 5692 Calculate Depreciation's.
package fadepr

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// EntryAmounts are the write-down, appreciation, custom 1 and custom 2 amounts of an asset
// (classic ARRAY [4]; the projection's book value, custom 1, depreciation in the fiscal year and
// custom 1 basis when passed to Calculate)
type EntryAmounts [4]types.Decimal

// the posting types EntryAmounts and GetPartOfCalculation count in order
var entryPostingTypes = [4]gtables.FALedgerEntryFa_posting_type{
	gtables.FALedgerEntry_Fa_posting_type.WriteDown,
	gtables.FALedgerEntry_Fa_posting_type.Appreciation,
	gtables.FALedgerEntry_Fa_posting_type.Custom1,
	gtables.FALedgerEntry_Fa_posting_type.Custom2,
}

var setupPostingTypes = [4]gtables.FAPostingTypeSetupFa_posting_type{
	gtables.FAPostingTypeSetup_Fa_posting_type.WriteDown,
	gtables.FAPostingTypeSetup_Fa_posting_type.Appreciation,
	gtables.FAPostingTypeSetup_Fa_posting_type.Custom1,
	gtables.FAPostingTypeSetup_Fa_posting_type.Custom2,
}

// DeprDays: the depreciation days from a date to a date (360-day year, both included)
func DeprDays(startingDate, endingDate types.Date) int {
	return tables.DeprDays(startingDate, endingDate)
}

// Yesterday: the day before (the 31st counts as the 30th)
func Yesterday(thisDate types.Date) types.Date {
	return tables.Yesterday(thisDate)
}

// ToMorrow: the day after (the 31st is skipped)
func ToMorrow(thisDate types.Date) types.Date {
	thisDate = thisDate.AddDays(1)
	if thisDate.Day() == 31 {
		thisDate = thisDate.AddDays(1)
	}
	return thisDate
}

// SetFAFilter (SetFAFilter): an asset's entries in a book — in posting type order (category blank)
// or in date order
func SetFAFilter(src ftables.Source, faNo, deprBookCode types.Code, faPostingTypeOrder bool) *gtables.FALedgerEntry {
	faLedgEntry := gtables.NewFALedgerEntry(src)
	if faPostingTypeOrder {
		faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_category", "fa_posting_type", "fa_posting_date")
		faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
	} else {
		faLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	}
	faLedgEntry.SetRange("fa_no", faNo)
	faLedgEntry.SetRange("depreciation_book_code", deprBookCode)
	return faLedgEntry
}

// CalcEntryAmounts (CalcEntryAmounts): the write-down, appreciation, custom 1 and custom 2 in the
// book value between two dates (a blank ending date: no end)
func CalcEntryAmounts(src ftables.Source, faNo, deprBookCode types.Code, startingDate, endingDate types.Date) (EntryAmounts, error) {
	var amounts EntryAmounts
	for i, postingType := range entryPostingTypes {
		faLedgEntry := SetFAFilter(src, faNo, deprBookCode, true)
		if endingDate.IsZero() {
			if !startingDate.IsZero() {
				faLedgEntry.SetFilter("fa_posting_date", startingDate.String()+"..")
			}
		} else {
			faLedgEntry.SetRange("fa_posting_date", startingDate, endingDate)
		}
		faLedgEntry.SetRange("part_of_book_value", true)
		faLedgEntry.SetRange("fa_posting_type", postingType)
		err := faLedgEntry.CalcSums("amount")
		if err != nil {
			return amounts, err
		}
		amounts[i] = faLedgEntry.Amount
	}
	return amounts, nil
}

// GetPartOfCalculation types (classic option IncludeInDeprCalc, IncludeInGainLoss,
// DepreciationType, ReverseType)
const (
	IncludeInDeprCalc = iota
	IncludeInGainLoss
	DepreciationType
	ReverseType
)

// GetPartOfCalculation (GetPartOfCalculation): a setting of a posting type's FA Posting Type
// Setup line (postingType 0..3: write-down, appreciation, custom 1, custom 2)
func GetPartOfCalculation(src ftables.Source, calcType, postingType int, deprBookCode types.Code) (bool, error) {
	err := tables.EnsureFAPostingTypeSetups(src, deprBookCode)
	if err != nil {
		return false, err
	}
	setup := gtables.NewFAPostingTypeSetup(src)
	err = setup.GetOrError(map[string]interface{}{"depreciation_book_code": deprBookCode, "fa_posting_type": int(setupPostingTypes[postingType])})
	if err != nil {
		return false, err
	}
	switch calcType {
	case IncludeInDeprCalc:
		return setup.Include_in_depr_calculation, nil
	case IncludeInGainLoss:
		return setup.Include_in_gain_loss_calc, nil
	case DepreciationType:
		return setup.Depreciation_type, nil
	}
	return setup.Reverse_before_disposal, nil
}

// getPartOfDeprCalculation (GetPartOfDeprCalculation): an entry of write-down, appreciation or
// custom 1/2 counted in the depreciation calculation
func getPartOfDeprCalculation(src ftables.Source, entry *gtables.FALedgerEntry) (bool, error) {
	for i, postingType := range entryPostingTypes {
		if entry.Fa_posting_type == postingType {
			return GetPartOfCalculation(src, IncludeInDeprCalc, i, entry.Depreciation_book_code)
		}
	}
	return false, nil
}

// getLastEntryDates (GetLastEntryDates): the last date of each of the four posting types counted
// in the depreciation calculation
func getLastEntryDates(src ftables.Source, faNo, deprBookCode types.Code) ([4]types.Date, error) {
	var entryDates [4]types.Date
	for i, postingType := range entryPostingTypes {
		included, err := GetPartOfCalculation(src, IncludeInDeprCalc, i, deprBookCode)
		if err != nil {
			return entryDates, err
		}
		if !included {
			continue
		}
		faLedgEntry := SetFAFilter(src, faNo, deprBookCode, true)
		faLedgEntry.SetRange("fa_posting_type", postingType)
		for found := faLedgEntry.FindSet(); found; found = faLedgEntry.Next() {
			if (faLedgEntry.Part_of_book_value || faLedgEntry.Part_of_depreciable_basis) && faLedgEntry.Fa_posting_date.After(entryDates[i]) {
				entryDates[i] = faLedgEntry.Fa_posting_date
			}
		}
	}
	return entryDates, nil
}

// UseDeprStartingDate (UseDeprStartingDate): no depreciation and none of the four posting types
// posted yet — the depreciation starts at the Depreciation Starting Date
func UseDeprStartingDate(src ftables.Source, faNo, deprBookCode types.Code) (bool, error) {
	faLedgEntry := SetFAFilter(src, faNo, deprBookCode, true)
	faLedgEntry.SetRange("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.Depreciation)
	if faLedgEntry.FindFirst() {
		return false, nil
	}
	entryDates, err := getLastEntryDates(src, faNo, deprBookCode)
	if err != nil {
		return false, err
	}
	for _, d := range entryDates {
		if !d.IsZero() {
			return false, nil
		}
	}
	return true, nil
}

// GetFirstDeprDate (GetFirstDeprDate): the first day not depreciated — the last acquisition or
// salvage value, the day after the last depreciation, the last of the posting types counted in
// the depreciation
func GetFirstDeprDate(src ftables.Source, faNo, deprBookCode types.Code) (types.Date, error) {
	var localDate types.Date
	faLedgEntry := SetFAFilter(src, faNo, deprBookCode, true)
	for _, postingType := range []gtables.FALedgerEntryFa_posting_type{gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost, gtables.FALedgerEntry_Fa_posting_type.SalvageValue} {
		faLedgEntry.SetRange("fa_posting_type", postingType)
		if faLedgEntry.FindLast() && faLedgEntry.Fa_posting_date.After(localDate) {
			localDate = faLedgEntry.Fa_posting_date
		}
	}
	faLedgEntry.SetRange("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.Depreciation)
	if faLedgEntry.FindLast() && ToMorrow(faLedgEntry.Fa_posting_date).After(localDate) {
		localDate = ToMorrow(faLedgEntry.Fa_posting_date)
	}
	entryDates, err := getLastEntryDates(src, faNo, deprBookCode)
	if err != nil {
		return types.Date{}, err
	}
	for _, d := range entryDates {
		if d.After(localDate) {
			localDate = d
		}
	}
	return localDate, nil
}

// GetMinusBookValue (GetMinusBookValue): the write-down, appreciation and custom 1/2 in the book
// value that are not counted in the depreciation
func GetMinusBookValue(src ftables.Source, faNo, deprBookCode types.Code, startingDate, endingDate types.Date) (types.Decimal, error) {
	entryAmounts, err := CalcEntryAmounts(src, faNo, deprBookCode, startingDate, endingDate)
	if err != nil {
		return types.Decimal{}, err
	}
	amount := types.Decimal{}
	for i := 0; i < 4; i++ {
		included, err := GetPartOfCalculation(src, IncludeInDeprCalc, i, deprBookCode)
		if err != nil {
			return types.Decimal{}, err
		}
		if !included {
			amount = amount.Add(entryAmounts[i])
		}
	}
	return amount, nil
}

// CalcMaxDepr (CalcMaxDepr): the most the book value may be depreciated (to the ending book
// value; with a salvage value to the salvage value)
func CalcMaxDepr(deprBook *gtables.DepreciationBook, bookValue, salvageValue, endingBookValue types.Decimal) types.Decimal {
	if endingBookValue.IsZero() {
		endingBookValue = deprBook.Default_ending_book_value
	}
	if !salvageValue.IsZero() {
		endingBookValue = types.Decimal{}
	}
	maxDepr := bookValue.Add(salvageValue).Sub(endingBookValue).Neg()
	if maxDepr.IsPositive() {
		maxDepr = types.Decimal{}
	}
	return maxDepr
}

// AdjustDepr (AdjustDepr): a depreciation rounded per the book; the rest of the book value (down
// to the ending book value or the salvage value) when less than the final rounding amount would
// be left
func AdjustDepr(deprBook *gtables.DepreciationBook, depreciation, bookValue, salvageValue, endingBookValue, finalRoundingAmount types.Decimal) types.Decimal {
	if finalRoundingAmount.IsZero() {
		finalRoundingAmount = deprBook.Default_final_rounding_amount
	}
	if endingBookValue.IsZero() {
		endingBookValue = deprBook.Default_ending_book_value
	}
	depreciation = CalcRounding(deprBook, depreciation)
	if !depreciation.IsNegative() {
		return types.Decimal{}
	}
	if !salvageValue.IsZero() {
		endingBookValue = types.Decimal{}
	}
	maxDepr := bookValue.Add(salvageValue).Sub(endingBookValue)
	if maxDepr.Add(depreciation).LessThan(finalRoundingAmount) {
		depreciation = maxDepr.Neg()
	}
	if depreciation.IsPositive() {
		depreciation = types.Decimal{}
	}
	return depreciation
}

// AdjustCustom1 (AdjustCustom1): the depreciation and custom 1 amounts rounded and limited to
// what the book value allows (custom 1 first)
func AdjustCustom1(deprBook *gtables.DepreciationBook, deprAmount, custom1Amount, bookValue, salvageValue, endingBookValue, finalRoundingAmount types.Decimal) (types.Decimal, types.Decimal) {
	if deprAmount.IsPositive() {
		deprAmount = types.Decimal{}
	}
	if custom1Amount.IsPositive() {
		custom1Amount = types.Decimal{}
	}
	deprAmount = CalcRounding(deprBook, deprAmount)
	custom1Amount = CalcRounding(deprBook, custom1Amount)
	if finalRoundingAmount.IsZero() {
		finalRoundingAmount = deprBook.Default_final_rounding_amount
	}
	if endingBookValue.IsZero() {
		endingBookValue = deprBook.Default_ending_book_value
	}
	if custom1Amount.IsNegative() {
		maxDepr := CalcMaxDepr(deprBook, bookValue, salvageValue, endingBookValue)
		if !custom1Amount.GreaterThan(maxDepr) {
			custom1Amount = maxDepr
			deprAmount = types.Decimal{}
		}
		if !deprAmount.IsNegative() {
			custom1Amount = AdjustDepr(deprBook, custom1Amount, bookValue, salvageValue, endingBookValue, finalRoundingAmount)
		}
		bookValue = bookValue.Add(custom1Amount)
	}
	if deprAmount.IsNegative() {
		maxDepr := CalcMaxDepr(deprBook, bookValue, salvageValue, endingBookValue)
		if !deprAmount.GreaterThan(maxDepr) {
			deprAmount = maxDepr
		}
		if deprAmount.IsNegative() {
			deprAmount = AdjustDepr(deprBook, deprAmount, bookValue, salvageValue, endingBookValue, finalRoundingAmount)
		}
	}
	if deprAmount.IsPositive() {
		deprAmount = types.Decimal{}
	}
	if custom1Amount.IsPositive() {
		custom1Amount = types.Decimal{}
	}
	return deprAmount, custom1Amount
}

// GetSign (GetSign): 1 for an asset with positive values (a normal asset), -1 for one with
// negative values, 0 when they are mixed (nothing to depreciate)
func GetSign(bookValue, deprBasis, salvageValue, minusBookValue types.Decimal) int {
	if !salvageValue.IsPositive() && !deprBasis.IsNegative() && !bookValue.IsNegative() && !minusBookValue.IsPositive() {
		return 1
	}
	if !salvageValue.IsNegative() && !deprBasis.IsPositive() && !bookValue.IsPositive() && !minusBookValue.IsNegative() {
		return -1
	}
	return 0
}

// getCustom1Sign (GetCustom1Sign): GetSign with the acquisition cost and custom 1
func getCustom1Sign(bookValue, acquisitionCost, custom1, salvageValue, minusBookValue types.Decimal) int {
	if !salvageValue.IsPositive() && !acquisitionCost.IsNegative() && !bookValue.IsNegative() && !custom1.IsPositive() && !minusBookValue.IsPositive() {
		return 1
	}
	if !salvageValue.IsNegative() && !acquisitionCost.IsPositive() && !bookValue.IsPositive() && !custom1.IsNegative() && !minusBookValue.IsNegative() {
		return -1
	}
	return 0
}

// CalcRounding (CalcRounding): an amount rounded to whole units with the book's Use Rounding in
// Periodic Depr., else to the currency's precision
func CalcRounding(deprBook *gtables.DepreciationBook, deprAmount types.Decimal) types.Decimal {
	if deprBook.Use_rounding_in_periodic_depr {
		return system.Round(deprAmount, types.NewDecimalFromInt(1))
	}
	return system.Round(deprAmount)
}

// calculateDeprInPeriod (CalculateDeprInPeriod): the book value (with the depreciation
// calculated so far), depreciable basis, salvage value and minus book value up to a date
func calculateDeprInPeriod(src ftables.Source, faNo, deprBookCode types.Code, endingDate types.Date, calculatedDepr types.Decimal, sign int) (newBookValue, deprBasis, salvageValue, minusBookValue types.Decimal, err error) {
	s := types.NewDecimalFromInt(int64(sign))
	sum := func(field string, value interface{}) (types.Decimal, error) {
		faLedgEntry := gtables.NewFALedgerEntry(src)
		faLedgEntry.SetRange("depreciation_book_code", deprBookCode)
		faLedgEntry.SetRange("fa_no", faNo)
		faLedgEntry.SetRange("fa_posting_date", types.Date{}, endingDate)
		if field == "fa_posting_type" {
			faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
		}
		faLedgEntry.SetRange(field, value)
		err := faLedgEntry.CalcSums("amount")
		return faLedgEntry.Amount, err
	}
	amount, err := sum("part_of_book_value", true)
	if err != nil {
		return
	}
	newBookValue = s.Mul(amount).Add(calculatedDepr)
	amount, err = sum("part_of_depreciable_basis", true)
	if err != nil {
		return
	}
	deprBasis = s.Mul(amount)
	amount, err = sum("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.SalvageValue)
	if err != nil {
		return
	}
	salvageValue = s.Mul(amount)
	amount, err = GetMinusBookValue(src, faNo, deprBookCode, types.Date{}, endingDate)
	minusBookValue = s.Mul(amount)
	return
}

// getDeprPeriod (GetDeprPeriod): the next period to depreciate (method Last Depr. Entry): from
// the day after the last depreciation (or the Depreciation Starting Date) to the day before the
// next entry that changes the depreciation (acquisition, salvage value, a posting type counted in
// the depreciation), else to the until date; the period's depreciation days
func getDeprPeriod(src ftables.Source, faNo, deprBookCode types.Code, untilDate types.Date, startingDate, endingDate *types.Date) (int, error) {
	faDeprBook := gtables.NewFADepreciationBook(src)
	err := faDeprBook.GetOrError(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode})
	if err != nil {
		return 0, err
	}
	usedDeprStartingDate := false
	if startingDate.IsZero() {
		faLedgEntry := SetFAFilter(src, faNo, deprBookCode, true)
		faLedgEntry.SetRange("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.Depreciation)
		if faLedgEntry.FindLast() {
			*startingDate = ToMorrow(faLedgEntry.Fa_posting_date)
		} else {
			*startingDate = faDeprBook.Depreciation_starting_date
			usedDeprStartingDate = true
		}
	} else {
		*startingDate = ToMorrow(*endingDate)
	}
	*endingDate = types.Date{}
	faLedgEntry := SetFAFilter(src, faNo, deprBookCode, false)
	if !usedDeprStartingDate {
		faLedgEntry.SetFilter("fa_posting_date", startingDate.AddDays(1).String()+"..")
	}
	for found := faLedgEntry.FindSet(); found; found = faLedgEntry.Next() {
		if faLedgEntry.Part_of_book_value || faLedgEntry.Part_of_depreciable_basis {
			if faLedgEntry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost ||
				faLedgEntry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.SalvageValue {
				if !usedDeprStartingDate {
					*endingDate = faLedgEntry.Fa_posting_date
				}
			} else {
				part, err := getPartOfDeprCalculation(src, faLedgEntry)
				if err != nil {
					return 0, err
				}
				if part {
					*endingDate = faLedgEntry.Fa_posting_date
				}
			}
			*endingDate = Yesterday(*endingDate)
			if endingDate.Before(*startingDate) {
				*endingDate = types.Date{}
			}
		}
		if !endingDate.IsZero() {
			break
		}
	}
	if endingDate.IsZero() {
		*endingDate = untilDate
	}
	return DeprDays(*startingDate, *endingDate), nil
}

// DeprInFiscalYear (DeprInFiscalYear): the depreciation (and the posting types of depreciation
// type) posted in the fiscal year of a date
func DeprInFiscalYear(src ftables.Source, faNo, deprBookCode types.Code, startingDate types.Date) (types.Decimal, error) {
	fiscalYearBegin, err := GetFiscalYear(src, deprBookCode, startingDate)
	if err != nil {
		return types.Decimal{}, err
	}
	faLedgEntry := SetFAFilter(src, faNo, deprBookCode, true)
	faLedgEntry.SetFilter("fa_posting_date", fiscalYearBegin.String()+"..")
	faLedgEntry.SetRange("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.Depreciation)
	faLedgEntry.SetRange("part_of_book_value", true)
	err = faLedgEntry.CalcSums("amount")
	if err != nil {
		return types.Decimal{}, err
	}
	localAmount := faLedgEntry.Amount
	entryAmounts, err := CalcEntryAmounts(src, faNo, deprBookCode, fiscalYearBegin, types.Date{})
	if err != nil {
		return types.Decimal{}, err
	}
	for i := 0; i < 4; i++ {
		deprType, err := GetPartOfCalculation(src, DepreciationType, i, deprBookCode)
		if err != nil {
			return types.Decimal{}, err
		}
		if deprType {
			localAmount = localAmount.Add(entryAmounts[i])
		}
	}
	return localAmount, nil
}

// GetFiscalYear is codeunit 5617 FA Date Calculation's GetFiscalYear: the start of the fiscal
// year of a date — the book's New Fiscal Year Starting Date, else the accounting period that
// starts a new fiscal year
func GetFiscalYear(src ftables.Source, deprBookCode types.Code, endingDate types.Date) (types.Date, error) {
	deprBook := gtables.NewDepreciationBook(src)
	err := deprBook.GetOrError(deprBookCode)
	if err != nil {
		return types.Date{}, err
	}
	if !deprBook.New_fiscal_year_starting_date.IsZero() {
		if deprBook.New_fiscal_year_starting_date.After(endingDate) {
			return types.Date{}, apperrors.FANewFiscalYearLater(gtables.DepreciationBookTableName, "new_fiscal_year_starting_date")
		}
		return deprBook.New_fiscal_year_starting_date, nil
	}
	accountingPeriod := gtables.NewAccountingPeriod(src)
	accountingPeriod.SetRange("new_fiscal_year", true)
	accountingPeriod.SetRange("starting_date", types.Date{}, endingDate)
	if accountingPeriod.FindLast() {
		return accountingPeriod.Starting_date, nil
	}
	return types.Date{}, apperrors.FANoFiscalYear()
}
