package tables

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// depreciationBookLogic is the business logic of table 5611 Depreciation Book.
type depreciationBookLogic struct{}

func init() { gtables.RegisterDepreciationBookLogic(depreciationBookLogic{}) }

// OnInsert creates the book's FA Posting Type Setup: appreciation part of the book value and of
// the depreciable basis, included in the depreciation, an acquisition type posted as a debit;
// write-down, custom 1 and custom 2 part of the book value, included in the gain/loss, a
// depreciation type posted as a credit
func (depreciationBookLogic) OnInsert(rec *gtables.DepreciationBook) error {
	return InsertFAPostingTypeSetups(rec, rec.Code)
}

// EnsureFAPostingTypeSetups gives a depreciation book that has no FA Posting Type Setup lines
// its default lines: the books entered before the table existed get them on first use
func EnsureFAPostingTypeSetups(src ftables.Source, deprBookCode types.Code) error {
	setup := gtables.NewFAPostingTypeSetup(src)
	setup.SetRange("depreciation_book_code", deprBookCode)
	if !setup.IsEmpty() {
		return nil
	}
	return InsertFAPostingTypeSetups(src, deprBookCode)
}

// InsertFAPostingTypeSetups inserts a depreciation book's FA Posting Type Setup lines (the
// book's OnInsert)
func InsertFAPostingTypeSetups(src ftables.Source, deprBookCode types.Code) error {
	setup := gtables.NewFAPostingTypeSetup(src)
	setup.Depreciation_book_code = deprBookCode
	setup.Fa_posting_type = gtables.FAPostingTypeSetup_Fa_posting_type.Appreciation
	setup.Part_of_book_value = true
	setup.Part_of_depreciable_basis = true
	setup.Include_in_depr_calculation = true
	setup.Include_in_gain_loss_calc = false
	setup.Depreciation_type = false
	setup.Acquisition_type = true
	setup.Sign = gtables.FAPostingTypeSetup_Sign.Debit
	err := setup.InsertOrError(false)
	if err != nil {
		return err
	}
	setup.Part_of_depreciable_basis = false
	setup.Include_in_gain_loss_calc = true
	setup.Depreciation_type = true
	setup.Acquisition_type = false
	setup.Sign = gtables.FAPostingTypeSetup_Sign.Credit
	for _, postingType := range []gtables.FAPostingTypeSetupFa_posting_type{
		gtables.FAPostingTypeSetup_Fa_posting_type.WriteDown,
		gtables.FAPostingTypeSetup_Fa_posting_type.Custom1,
		gtables.FAPostingTypeSetup_Fa_posting_type.Custom2,
	} {
		setup.Fa_posting_type = postingType
		err = setup.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnModify sets Last Date Modified
func (depreciationBookLogic) OnModify(rec *gtables.DepreciationBook) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnRename sets Last Date Modified
func (depreciationBookLogic) OnRename(rec *gtables.DepreciationBook) error {
	rec.Last_date_modified = system.Today()
	return nil
}

// OnDelete: not while fixed assets use the book, nor while it is the insurance book with
// coverage entries
func (depreciationBookLogic) OnDelete(rec *gtables.DepreciationBook) error {
	faSetup := gtables.GetFASetup(rec)
	fADeprBook := gtables.NewFADepreciationBook(rec)
	fADeprBook.SetCurrentKey("depreciation_book_code")
	fADeprBook.SetRange("depreciation_book_code", rec.Code)
	if fADeprBook.FindFirst() {
		return apperrors.DeprBookInUse()
	}
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(rec)
	if insCoverageLedgEntry.FindFirst() && faSetup.Insurance_depr_book == rec.Code {
		return apperrors.DeprBookInsurance(gtables.FASetupTableName, "insurance_depr_book", rec.Code.String())
	}
	faPostingTypeSetup := gtables.NewFAPostingTypeSetup(rec)
	faPostingTypeSetup.SetRange("depreciation_book_code", rec.Code)
	faPostingTypeSetup.DeleteAll()
	faJnlSetup := gtables.NewFAJournalSetup(rec)
	faJnlSetup.SetRange("depreciation_book_code", rec.Code)
	faJnlSetup.DeleteAll()
	return nil
}

// OnValidate_Use_fa_exch_rate_in_duplic: without it there is no default exchange rate
func (depreciationBookLogic) OnValidate_Use_fa_exch_rate_in_duplic(rec *gtables.DepreciationBook) error {
	if !rec.Use_fa_exch_rate_in_duplic {
		rec.Default_exchange_rate = types.ZeroDecimal()
	}
	return nil
}

// OnValidate_Default_exchange_rate: only with Use FA Exch. Rate in Duplic.
func (depreciationBookLogic) OnValidate_Default_exchange_rate(rec *gtables.DepreciationBook) error {
	if rec.Default_exchange_rate.IsPositive() {
		return rec.TestField("use_fa_exch_rate_in_duplic", true)
	}
	return nil
}
