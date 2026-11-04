package fajnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fadepr"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunit 5605 Calculate Disposal: the entries a disposal makes. The 14 amounts (classic ARRAY
// [14], index 1..14 here 0..13): 1 gain, 2 loss, 3 acquisition cost, 4 depreciation, 5–8 write-
// down, appreciation, custom 1, custom 2 (category Disposal), 9 salvage value, 10 book value on
// disposal (method Gross), 11–14 the balancing write-down … custom 2 (category Bal. Disposal,
// method Net).
type disposalAmounts [14]types.Decimal

// disposalType is GetDisposalType's: the first disposal, a second one (more proceeds), or the
// cancelling of the last (or an earlier) disposal
type disposalType int

const (
	firstDisposal disposalType = iota
	secondDisposal
	errorDisposal
	lastErrorDisposal
)

// disposalPostingType (SetFAPostingType): the posting type of amount i (0-based)
func disposalPostingType(i int) gtables.FALedgerEntryFa_posting_type {
	switch i + 1 {
	case 1, 2:
		return gtables.FALedgerEntry_Fa_posting_type.GainLoss
	case 3:
		return gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost
	case 4:
		return gtables.FALedgerEntry_Fa_posting_type.Depreciation
	case 5, 11:
		return gtables.FALedgerEntry_Fa_posting_type.WriteDown
	case 6, 12:
		return gtables.FALedgerEntry_Fa_posting_type.Appreciation
	case 7, 13:
		return gtables.FALedgerEntry_Fa_posting_type.Custom1
	case 8, 14:
		return gtables.FALedgerEntry_Fa_posting_type.Custom2
	case 9:
		return gtables.FALedgerEntry_Fa_posting_type.SalvageValue
	}
	return gtables.FALedgerEntry_Fa_posting_type.BookValueOnDisposal
}

// disposalPostingCategory (SetFAPostingCategory): the category of amount i (0-based)
func disposalPostingCategory(i int) gtables.FALedgerEntryFa_posting_category {
	switch {
	case i < 2:
		return gtables.FALedgerEntry_Fa_posting_category.Blank
	case i < 10:
		return gtables.FALedgerEntry_Fa_posting_category.Disposal
	}
	return gtables.FALedgerEntry_Fa_posting_category.BalDisposal
}

// reverseType (SetReverseType): the posting type of reverse amount i (0..3)
func reverseType(i int) gtables.FALedgerEntryFa_posting_type {
	return [4]gtables.FALedgerEntryFa_posting_type{
		gtables.FALedgerEntry_Fa_posting_type.WriteDown,
		gtables.FALedgerEntry_Fa_posting_type.Appreciation,
		gtables.FALedgerEntry_Fa_posting_type.Custom1,
		gtables.FALedgerEntry_Fa_posting_type.Custom2,
	}[i]
}

// calcGainLoss (CalcGainLoss): the first disposal's entries — the asset's acquisition cost,
// depreciation, salvage value and the posting types in the book value taken out (category
// Disposal), the book value on disposal (Gross), the balancing of the types not in the gain/loss
// (Net), and the gain or loss: book value + proceeds (+ the types not in the gain/loss)
func calcGainLoss(src ftables.Source, faNo, deprBookCode types.Code) (disposalAmounts, error) {
	var a disposalAmounts
	deprBook := gtables.NewDepreciationBook(src)
	err := deprBook.GetOrError(deprBookCode)
	if err != nil {
		return a, err
	}
	entryAmounts2, err := fadepr.CalcEntryAmounts(src, faNo, deprBookCode, types.Date{}, types.Date{})
	if err != nil {
		return a, err
	}
	for i := 0; i < 4; i++ {
		a[i+4] = entryAmounts2[i].Neg()
	}
	faDeprBook := gtables.NewFADepreciationBook(src)
	err = faDeprBook.GetOrError(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode})
	if err != nil {
		return a, err
	}
	faDeprBook.CalcFields("book_value", "proceeds_on_disposal", "acquisition_cost", "salvage_value", "depreciation")
	a[2] = faDeprBook.Acquisition_cost.Neg()
	a[3] = faDeprBook.Depreciation.Neg()
	a[8] = faDeprBook.Salvage_value.Neg()
	if deprBook.Disposal_calculation_method == gtables.DepreciationBook_Disposal_calculation_method.Gross {
		a[9] = faDeprBook.Book_value
	}
	gainLoss := faDeprBook.Book_value.Add(faDeprBook.Proceeds_on_disposal)
	for i := 0; i < 4; i++ {
		included, err := fadepr.GetPartOfCalculation(src, fadepr.IncludeInGainLoss, i, deprBookCode)
		if err != nil {
			return a, err
		}
		if !included {
			gainLoss = gainLoss.Add(a[i+4])
			if deprBook.Disposal_calculation_method == gtables.DepreciationBook_Disposal_calculation_method.Net {
				a[i+10] = a[i+4].Neg()
			}
		}
	}
	if !gainLoss.IsPositive() {
		a[0] = gainLoss
	} else {
		a[1] = gainLoss
	}
	return a, nil
}

// calcSecondGainLoss (CalcSecondGainLoss): a second disposal's proceeds go to the gain or loss
// (both when the result changes sign)
func calcSecondGainLoss(src ftables.Source, faNo, deprBookCode types.Code, lastDisposalPrice types.Decimal) (disposalAmounts, error) {
	var a disposalAmounts
	faDeprBook := gtables.NewFADepreciationBook(src)
	err := faDeprBook.GetOrError(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode})
	if err != nil {
		return a, err
	}
	faDeprBook.CalcFields("gain_loss")
	gainLoss := faDeprBook.Gain_loss
	newGainLoss := lastDisposalPrice.Add(gainLoss)
	identicalSign := !newGainLoss.IsPositive() && !gainLoss.IsPositive() || !newGainLoss.IsNegative() && !gainLoss.IsNegative()
	switch {
	case identicalSign && !gainLoss.IsPositive():
		a[0] = lastDisposalPrice
	case identicalSign:
		a[1] = lastDisposalPrice
	case !gainLoss.IsPositive():
		a[0] = gainLoss.Neg()
		a[1] = newGainLoss
	default:
		a[1] = gainLoss.Neg()
		a[0] = newGainLoss
	}
	return a, nil
}

// calcReverseAmounts (CalcReverseAmounts): the posting types in the book value set to Reverse
// before Disposal, reversed (all of them: one only partly reversed is an error)
func calcReverseAmounts(src ftables.Source, faNo, deprBookCode types.Code) ([4]types.Decimal, error) {
	var a [4]types.Decimal
	faDeprBook := gtables.NewFADepreciationBook(src)
	err := faDeprBook.GetOrError(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode})
	if err != nil {
		return a, err
	}
	bookValueAmounts, err := fadepr.CalcEntryAmounts(src, faNo, deprBookCode, types.Date{}, types.Date{})
	if err != nil {
		return a, err
	}
	err = tables.EnsureFAPostingTypeSetups(src, deprBookCode)
	if err != nil {
		return a, err
	}
	setupTypes := [4]gtables.FAPostingTypeSetupFa_posting_type{
		gtables.FAPostingTypeSetup_Fa_posting_type.WriteDown,
		gtables.FAPostingTypeSetup_Fa_posting_type.Appreciation,
		gtables.FAPostingTypeSetup_Fa_posting_type.Custom1,
		gtables.FAPostingTypeSetup_Fa_posting_type.Custom2,
	}
	faDeprBook.CalcFields("write_down", "appreciation", "custom_1", "custom_2")
	totals := [4]types.Decimal{faDeprBook.Write_down, faDeprBook.Appreciation, faDeprBook.Custom_1, faDeprBook.Custom_2}
	for i := 0; i < 4; i++ {
		setup := gtables.NewFAPostingTypeSetup(src)
		err = setup.GetOrError(map[string]interface{}{"depreciation_book_code": deprBookCode, "fa_posting_type": int(setupTypes[i])})
		if err != nil {
			return a, err
		}
		if setup.Part_of_book_value && setup.Reverse_before_disposal {
			a[i] = totals[i].Neg()
			if !a[i].Add(bookValueAmounts[i]).IsZero() {
				return a, setup.TestField("reverse_before_disposal", false)
			}
		}
	}
	return a, nil
}

// getDisposalType (GetDisposalType): a first disposal, or — with proceeds posted — a second one
// or (a cancelling line) the cancelling of the last or an earlier disposal; the method and number
// of the last disposal and its proceeds entry
func getDisposalType(src ftables.Source, faNo, deprBookCode types.Code, errorNo int) (dt disposalType, method gtables.FALedgerEntryDisposal_calculation_method, maxDisposalNo, salesEntryNo int) {
	faLedgEntry := gtables.NewFALedgerEntry(src)
	faLedgEntry.SetRange("fa_no", faNo)
	faLedgEntry.SetRange("depreciation_book_code", deprBookCode)
	faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
	faLedgEntry.SetRange("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal)
	found := false
	for ok := faLedgEntry.FindSet(); ok; ok = faLedgEntry.Next() {
		found = true
		method = faLedgEntry.Disposal_calculation_method
		if faLedgEntry.Disposal_entry_no > maxDisposalNo {
			maxDisposalNo = faLedgEntry.Disposal_entry_no
			salesEntryNo = faLedgEntry.Entry_no
		}
	}
	switch {
	case !found:
		dt = firstDisposal
	case errorNo == 0:
		dt = secondDisposal
	case maxDisposalNo == 1:
		dt = lastErrorDisposal
	default:
		dt = errorDisposal
	}
	return
}

// getErrorDisposal (GetErrorDisposal): the entries of the last disposal to cancel — its gain and
// loss, and (the first disposal) its other entries
func getErrorDisposal(src ftables.Source, faNo, deprBookCode types.Code, onlyGainLoss bool) (disposalAmounts, [14]int) {
	var a disposalAmounts
	var numbers [14]int
	faLedgEntry := gtables.NewFALedgerEntry(src)
	faLedgEntry.SetRange("fa_no", faNo)
	faLedgEntry.SetRange("depreciation_book_code", deprBookCode)
	faLedgEntry.SetRange("fa_posting_category", gtables.FALedgerEntry_Fa_posting_category.Blank)
	faLedgEntry.SetRange("fa_posting_type", gtables.FALedgerEntry_Fa_posting_type.GainLoss)
	maxDisposalNo := 0
	for ok := faLedgEntry.FindSet(); ok; ok = faLedgEntry.Next() {
		if faLedgEntry.Disposal_entry_no > maxDisposalNo {
			maxDisposalNo = faLedgEntry.Disposal_entry_no
		}
	}
	for ok := faLedgEntry.FindSet(); ok; ok = faLedgEntry.Next() {
		if faLedgEntry.Disposal_entry_no != maxDisposalNo {
			continue
		}
		switch faLedgEntry.Result_on_disposal {
		case gtables.FALedgerEntry_Result_on_disposal.Gain:
			a[0] = faLedgEntry.Amount.Neg()
			numbers[0] = faLedgEntry.Entry_no
		case gtables.FALedgerEntry_Result_on_disposal.Loss:
			a[1] = faLedgEntry.Amount.Neg()
			numbers[1] = faLedgEntry.Entry_no
		}
	}
	if !onlyGainLoss {
		for i := 2; i < 14; i++ {
			entry := gtables.NewFALedgerEntry(src)
			entry.SetRange("fa_no", faNo)
			entry.SetRange("depreciation_book_code", deprBookCode)
			entry.SetRange("fa_posting_category", disposalPostingCategory(i))
			entry.SetRange("fa_posting_type", disposalPostingType(i))
			if entry.FindFirst() {
				numbers[i] = entry.Entry_no
				a[i] = entry.Amount.Neg()
			}
		}
	}
	return a, numbers
}
