package tables

import (
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// fAReclassJournalLineLogic is the business logic of table 5624 FA Reclass. Journal Line: a share
// of a fixed asset's amounts in a depreciation book (an acquisition cost amount or %) moved to
// another asset — codeunit 5642 turns it into FA or general journal lines.
type fAReclassJournalLineLogic struct{}

func init() { gtables.RegisterFAReclassJournalLineLogic(fAReclassJournalLineLogic{}) }

// OnValidate_Fa_no: an asset that is not blocked or inactive; its description and the FA Setup's
// default depreciation book
func (fAReclassJournalLineLogic) OnValidate_Fa_no(rec *gtables.FAReclassJournalLine) error {
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
	if rec.Depreciation_book_code == "" {
		faSetup := gtables.GetFASetup(rec)
		rec.Depreciation_book_code = faSetup.Default_depr_book
	}
	return nil
}

// OnValidate_New_fa_no: an asset that is not blocked or inactive
func (fAReclassJournalLineLogic) OnValidate_New_fa_no(rec *gtables.FAReclassJournalLine) error {
	if rec.New_fa_no == "" {
		return nil
	}
	fa := gtables.NewFixedAsset(rec)
	err := fa.GetOrError(rec.New_fa_no)
	if err != nil {
		return err
	}
	err = fa.TestField("blocked", false)
	if err != nil {
		return err
	}
	return fa.TestField("inactive", false)
}

// OnValidate_Reclassify_acq_cost_amount: an amount, not a %
func (fAReclassJournalLineLogic) OnValidate_Reclassify_acq_cost_amount(rec *gtables.FAReclassJournalLine) error {
	rec.Reclassify_acq_cost_percent = types.Decimal{}
	return nil
}

// OnValidate_Reclassify_acq_cost_percent: a %, not an amount
func (fAReclassJournalLineLogic) OnValidate_Reclassify_acq_cost_percent(rec *gtables.FAReclassJournalLine) error {
	rec.Reclassify_acq_cost_amount = types.Decimal{}
	return nil
}

// SetUpNewFAReclassJnlLine (SetUpNewLine): a new line takes the FA posting date of the last line
func SetUpNewFAReclassJnlLine(rec, lastFAReclassJnlLine *gtables.FAReclassJournalLine) {
	rec.Fa_posting_date = lastFAReclassJnlLine.Fa_posting_date
}
