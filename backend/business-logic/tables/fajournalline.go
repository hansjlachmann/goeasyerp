package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fAJournalLineLogic is the business logic of table 5621 FA Journal Line: a posting to a fixed
// asset's depreciation book without G/L (a book whose G/L integration is off for the posting
// type); posted by codeunit 5632 (package fajnl).
//
// Not migrated yet: budgeted assets (Budgeted FA No.), recurring FA journals.
type fAJournalLineLogic struct{}

func init() { gtables.RegisterFAJournalLineLogic(fAJournalLineLogic{}) }

// OnInsert: the template's source code and the batch's reason code
func (fAJournalLineLogic) OnInsert(rec *gtables.FAJournalLine) error {
	rec.LockTable()
	faJnlTemplate := gtables.NewFAJournalTemplate(rec)
	err := faJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	rec.Source_code = faJnlTemplate.Source_code
	faJnlBatch := gtables.NewFAJournalBatch(rec)
	err = faJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return err
	}
	rec.Reason_code = faJnlBatch.Reason_code
	return nil
}

// OnValidate_Depreciation_book_code: the asset's posting group in the book
func (fAJournalLineLogic) OnValidate_Depreciation_book_code(rec *gtables.FAJournalLine) error {
	if rec.Fa_no == "" || rec.Depreciation_book_code == "" {
		return nil
	}
	faDeprBook := gtables.NewFADepreciationBook(rec)
	err := faDeprBook.GetOrError(map[string]interface{}{"fa_no": rec.Fa_no, "depreciation_book_code": rec.Depreciation_book_code})
	if err != nil {
		return err
	}
	rec.Fa_posting_group = faDeprBook.Fa_posting_group
	return nil
}

// OnValidate_Fa_posting_type: an insurance only on an acquisition, a maintenance code only on a
// maintenance
func (fAJournalLineLogic) OnValidate_Fa_posting_type(rec *gtables.FAJournalLine) error {
	if rec.Fa_posting_type != gtables.FAJournalLine_Fa_posting_type.AcquisitionCost {
		err := rec.TestField("insurance_no", "")
		if err != nil {
			return err
		}
	}
	if rec.Fa_posting_type != gtables.FAJournalLine_Fa_posting_type.Maintenance {
		return rec.TestField("maintenance_code", "")
	}
	return nil
}

// OnValidate_Fa_no: an asset that is not blocked or inactive; its description, dimension 1 and
// dimension 2; the FA Setup's default depreciation book when the asset has it, and the asset's
// posting group in the book
func (fAJournalLineLogic) OnValidate_Fa_no(rec *gtables.FAJournalLine) error {
	if rec.Fa_no == "" {
		return nil
	}
	fa := gtables.NewFixedAsset(rec)
	err := fa.GetOrError(rec.Fa_no)
	if err != nil {
		return err
	}
	err = fa.TestField("blocked", false)
	if err != nil {
		return err
	}
	err = fa.TestField("inactive", false)
	if err != nil {
		return err
	}
	rec.Description = fa.Description
	rec.Dimension_1_code = fa.Dimension_1_code
	rec.Dimension_2_code = fa.Dimension_2_code
	faDeprBook := gtables.NewFADepreciationBook(rec)
	if rec.Depreciation_book_code == "" {
		faSetup := gtables.GetFASetup(rec)
		rec.Depreciation_book_code = faSetup.Default_depr_book
		if !faDeprBook.Get(map[string]interface{}{"fa_no": rec.Fa_no, "depreciation_book_code": rec.Depreciation_book_code}) {
			rec.Depreciation_book_code = ""
		}
	}
	if rec.Depreciation_book_code != "" {
		err = faDeprBook.GetOrError(map[string]interface{}{"fa_no": rec.Fa_no, "depreciation_book_code": rec.Depreciation_book_code})
		if err != nil {
			return err
		}
		rec.Fa_posting_group = faDeprBook.Fa_posting_group
	}
	return nil
}

// OnValidate_Amount: Debit/Credit Amount (a correction the other way round)
func (fAJournalLineLogic) OnValidate_Amount(rec *gtables.FAJournalLine) error {
	if rec.Amount.IsPositive() && !rec.Correction || rec.Amount.IsNegative() && rec.Correction {
		rec.Debit_amount = rec.Amount
		rec.Credit_amount = types.Decimal{}
	} else {
		rec.Debit_amount = types.Decimal{}
		rec.Credit_amount = rec.Amount.Neg()
	}
	return nil
}

// OnValidate_Debit_amount: the amount (a negative debit is a correction)
func (fAJournalLineLogic) OnValidate_Debit_amount(rec *gtables.FAJournalLine) error {
	rec.Correction = rec.Debit_amount.IsNegative()
	rec.Amount = rec.Debit_amount
	return rec.Validate("amount", rec.Amount)
}

// OnValidate_Credit_amount: the amount (a negative credit is a correction)
func (fAJournalLineLogic) OnValidate_Credit_amount(rec *gtables.FAJournalLine) error {
	rec.Correction = rec.Credit_amount.IsNegative()
	rec.Amount = rec.Credit_amount.Neg()
	return rec.Validate("amount", rec.Amount)
}

// OnValidate_Correction: Debit/Credit Amount again
func (fAJournalLineLogic) OnValidate_Correction(rec *gtables.FAJournalLine) error {
	return rec.Validate("amount", rec.Amount)
}

// OnValidate_Maintenance_code: only on a maintenance
func (fAJournalLineLogic) OnValidate_Maintenance_code(rec *gtables.FAJournalLine) error {
	if rec.Maintenance_code != "" {
		return rec.TestField("fa_posting_type", gtables.FAJournalLine_Fa_posting_type.Maintenance)
	}
	return nil
}

// OnValidate_Insurance_no: only on an acquisition
func (fAJournalLineLogic) OnValidate_Insurance_no(rec *gtables.FAJournalLine) error {
	if rec.Insurance_no != "" {
		return rec.TestField("fa_posting_type", gtables.FAJournalLine_Fa_posting_type.AcquisitionCost)
	}
	return nil
}

// OnValidate_Use_duplication_list: not with a book to duplicate in
func (fAJournalLineLogic) OnValidate_Use_duplication_list(rec *gtables.FAJournalLine) error {
	rec.Duplicate_in_depreciation_book = ""
	return nil
}

// OnValidate_Duplicate_in_depreciation_book: not with the duplication list
func (fAJournalLineLogic) OnValidate_Duplicate_in_depreciation_book(rec *gtables.FAJournalLine) error {
	rec.Use_duplication_list = false
	return nil
}

// FAJnlLineConvertToLedgEntry (ConvertToLedgEntry): the FA ledger entry's posting type of a
// journal line's (a disposal is Proceeds on Disposal; a maintenance has none: it writes a
// maintenance ledger entry)
func FAJnlLineConvertToLedgEntry(postingType gtables.FAJournalLineFa_posting_type) gtables.FALedgerEntryFa_posting_type {
	switch postingType {
	case gtables.FAJournalLine_Fa_posting_type.Depreciation:
		return gtables.FALedgerEntry_Fa_posting_type.Depreciation
	case gtables.FAJournalLine_Fa_posting_type.WriteDown:
		return gtables.FALedgerEntry_Fa_posting_type.WriteDown
	case gtables.FAJournalLine_Fa_posting_type.Appreciation:
		return gtables.FALedgerEntry_Fa_posting_type.Appreciation
	case gtables.FAJournalLine_Fa_posting_type.Custom1:
		return gtables.FALedgerEntry_Fa_posting_type.Custom1
	case gtables.FAJournalLine_Fa_posting_type.Custom2:
		return gtables.FALedgerEntry_Fa_posting_type.Custom2
	case gtables.FAJournalLine_Fa_posting_type.Disposal:
		return gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal
	case gtables.FAJournalLine_Fa_posting_type.SalvageValue:
		return gtables.FALedgerEntry_Fa_posting_type.SalvageValue
	}
	return gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost
}

// SetUpNewFAJnlLine (SetUpNewLine): a new line takes the FA posting date and document no. of
// the last line — the first line of a batch the work date and the next number of the batch's
// series —, the template's source code and the batch's reason code and posting no. series
func SetUpNewFAJnlLine(rec, lastFAJnlLine *gtables.FAJournalLine) error {
	faJnlTemplate := gtables.NewFAJournalTemplate(rec)
	err := faJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return err
	}
	faJnlBatch := gtables.NewFAJournalBatch(rec)
	err = faJnlBatch.GetOrError(map[string]interface{}{"journal_template_name": rec.Journal_template_name, "name": rec.Journal_batch_name})
	if err != nil {
		return err
	}
	faJnlLine := gtables.NewFAJournalLine(rec)
	faJnlLine.SetRange("journal_template_name", rec.Journal_template_name)
	faJnlLine.SetRange("journal_batch_name", rec.Journal_batch_name)
	if faJnlLine.FindFirst() {
		rec.Fa_posting_date = lastFAJnlLine.Fa_posting_date
		rec.Document_no = lastFAJnlLine.Document_no
	} else {
		rec.Fa_posting_date = system.WorkDate()
		if faJnlBatch.No_series != "" {
			noSeriesMgt := noseries.New(rec)
			rec.Document_no = noSeriesMgt.TryGetNextNo(faJnlBatch.No_series, rec.Fa_posting_date)
		}
	}
	rec.Recurring_method = lastFAJnlLine.Recurring_method
	rec.Source_code = faJnlTemplate.Source_code
	rec.Reason_code = faJnlBatch.Reason_code
	rec.Posting_no_series = faJnlBatch.Posting_no_series
	return nil
}
