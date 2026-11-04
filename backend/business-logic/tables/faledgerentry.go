package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fALedgerEntryLogic is the business logic of table 5601 FA_Ledger_Entry.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/faledgerentry_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type fALedgerEntryLogic struct{}

func init() { gtables.RegisterFALedgerEntryLogic(fALedgerEntryLogic{}) }

// OnInsert runs before a new record is inserted
func (fALedgerEntryLogic) OnInsert(rec *gtables.FALedgerEntry) error {
	return nil
}

// OnModify runs before a record is modified
func (fALedgerEntryLogic) OnModify(rec *gtables.FALedgerEntry) error {
	return nil
}

// OnDelete runs before a record is deleted
func (fALedgerEntryLogic) OnDelete(rec *gtables.FALedgerEntry) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (fALedgerEntryLogic) OnRename(rec *gtables.FALedgerEntry) error {
	return nil
}

// FALedgEntryConvertPostingType (ConvertPostingType): the FA journal's posting type of an FA
// ledger entry's (Proceeds on Disposal is a disposal)
func FALedgEntryConvertPostingType(rec *gtables.FALedgerEntry) gtables.FAJournalLineFa_posting_type {
	switch rec.Fa_posting_type {
	case gtables.FALedgerEntry_Fa_posting_type.Depreciation:
		return gtables.FAJournalLine_Fa_posting_type.Depreciation
	case gtables.FALedgerEntry_Fa_posting_type.WriteDown:
		return gtables.FAJournalLine_Fa_posting_type.WriteDown
	case gtables.FALedgerEntry_Fa_posting_type.Appreciation:
		return gtables.FAJournalLine_Fa_posting_type.Appreciation
	case gtables.FALedgerEntry_Fa_posting_type.Custom1:
		return gtables.FAJournalLine_Fa_posting_type.Custom1
	case gtables.FALedgerEntry_Fa_posting_type.Custom2:
		return gtables.FAJournalLine_Fa_posting_type.Custom2
	case gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal:
		return gtables.FAJournalLine_Fa_posting_type.Disposal
	case gtables.FALedgerEntry_Fa_posting_type.SalvageValue:
		return gtables.FAJournalLine_Fa_posting_type.SalvageValue
	}
	return gtables.FAJournalLine_Fa_posting_type.AcquisitionCost
}

// FALedgEntryMoveToGenJnl (MoveToGenJnl): a general journal line (Init) of the entry — fixed
// asset, book, amount, correction, document and depreciation fields
func FALedgEntryMoveToGenJnl(rec *gtables.FALedgerEntry, genJnlLine *gtables.GenJournalLine) error {
	genJnlLine.Init()
	genJnlLine.Fa_posting_type = gtables.GenJournalLineFa_posting_type(FALedgEntryConvertPostingType(rec) + 1)
	genJnlLine.Posting_date = rec.Posting_date
	genJnlLine.Fa_posting_date = rec.Fa_posting_date
	if genJnlLine.Posting_date.Equal(genJnlLine.Fa_posting_date) {
		genJnlLine.Fa_posting_date = types.Date{}
	}
	err := genJnlLine.Validate("account_type", gtables.GenJournalLine_Account_type.FixedAsset)
	if err != nil {
		return err
	}
	err = genJnlLine.Validate("account_no", rec.Fa_no)
	if err != nil {
		return err
	}
	err = genJnlLine.Validate("depreciation_book_code", rec.Depreciation_book_code)
	if err != nil {
		return err
	}
	err = genJnlLine.Validate("amount", rec.Amount)
	if err != nil {
		return err
	}
	err = genJnlLine.Validate("correction", rec.Correction)
	if err != nil {
		return err
	}
	genJnlLine.Document_type = gtables.GenJournalLineDocument_type(rec.Document_type)
	genJnlLine.Document_no = rec.Document_no
	genJnlLine.Document_date = rec.Document_date
	genJnlLine.External_document_no = rec.External_document_no
	genJnlLine.Quantity = rec.Quantity
	genJnlLine.No_of_depreciation_days = rec.No_of_depreciation_days
	genJnlLine.Fa_reclassification_entry = rec.Reclassification_entry
	genJnlLine.Index_entry = rec.Index_entry
	return nil
}

// FALedgEntryMoveToFAJnl (MoveToFAJnl): an FA journal line (Init) of the entry
func FALedgEntryMoveToFAJnl(rec *gtables.FALedgerEntry, faJnlLine *gtables.FAJournalLine) error {
	faJnlLine.Init()
	faJnlLine.Fa_posting_type = FALedgEntryConvertPostingType(rec)
	faJnlLine.Posting_date = rec.Posting_date
	faJnlLine.Fa_posting_date = rec.Fa_posting_date
	if faJnlLine.Posting_date.Equal(faJnlLine.Fa_posting_date) {
		faJnlLine.Posting_date = types.Date{}
	}
	err := faJnlLine.Validate("fa_no", rec.Fa_no)
	if err != nil {
		return err
	}
	err = faJnlLine.Validate("depreciation_book_code", rec.Depreciation_book_code)
	if err != nil {
		return err
	}
	err = faJnlLine.Validate("amount", rec.Amount)
	if err != nil {
		return err
	}
	err = faJnlLine.Validate("correction", rec.Correction)
	if err != nil {
		return err
	}
	faJnlLine.Quantity = rec.Quantity
	faJnlLine.Document_type = gtables.FAJournalLineDocument_type(rec.Document_type)
	faJnlLine.Document_no = rec.Document_no
	faJnlLine.Document_date = rec.Document_date
	faJnlLine.External_document_no = rec.External_document_no
	faJnlLine.No_of_depreciation_days = rec.No_of_depreciation_days
	faJnlLine.Fa_reclassification_entry = rec.Reclassification_entry
	faJnlLine.Index_entry = rec.Index_entry
	return nil
}
