package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fASetupLogic is the business logic of table 5603 FA Setup
type fASetupLogic struct{}

func init() { gtables.RegisterFASetupLogic(fASetupLogic{}) }

// OnValidate_Default_depr_book: the default book is also the insurance book, while there is none
func (fASetupLogic) OnValidate_Default_depr_book(rec *gtables.FASetup) error {
	if rec.Insurance_depr_book == "" {
		return rec.Validate("insurance_depr_book", rec.Default_depr_book)
	}
	return nil
}

// OnValidate_Insurance_depr_book: the coverage entries of the assets disposed in the insurance
// book are marked Disposed FA (the others not)
func (fASetupLogic) OnValidate_Insurance_depr_book(rec *gtables.FASetup) error {
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(rec)
	if !insCoverageLedgEntry.FindFirst() {
		return nil
	}
	if rec.Insurance_depr_book != "" {
		deprBook := gtables.NewDepreciationBook(rec)
		err := deprBook.GetOrError(rec.Insurance_depr_book)
		if err != nil {
			return err
		}
	}
	insCoverageLedgEntry.SetCurrentKey("insurance_no", "disposed_fa")
	insCoverageLedgEntry.SetRange("disposed_fa", true)
	insCoverageLedgEntry.ModifyAll("disposed_fa", false)
	if rec.Insurance_depr_book != "" {
		insCoverageLedgEntry.Reset()
		insCoverageLedgEntry.SetCurrentKey("fa_no")
		fADeprBook := gtables.NewFADepreciationBook(rec)
		fADeprBook.SetCurrentKey("depreciation_book_code")
		fADeprBook.SetRange("depreciation_book_code", rec.Insurance_depr_book)
		for found := fADeprBook.FindSetBuffered(); found; found = fADeprBook.Next() {
			insCoverageLedgEntry.SetRange("fa_no", fADeprBook.Fa_no)
			if !fADeprBook.Disposal_date.IsZero() {
				insCoverageLedgEntry.ModifyAll("disposed_fa", true)
			}
		}
	}
	return nil
}
