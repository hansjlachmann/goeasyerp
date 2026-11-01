package fajnl

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// FAJnlCheckLine is codeunit 5631 FA Jnl.-Check Line for FA journal lines (CheckFAJnlLine): the
// line has an FA posting date, a book and a document no.; the FA posting date is allowed (the
// user's or the FA Setup's Allow FA Posting From/To) and the same as the posting date when the
// book wants it; the posting type is not G/L-integrated in the book (those post in the general
// journal); the fields the posting type does not use are blank; a cancelling line (FA Error
// Entry No.) uses none of the extras.
//
// Not migrated yet: the general journal's fixed asset lines (step FA2), disposal (FA4),
// Depr. until FA Posting Date / Depr. Acquisition Cost (depreciation, FA3), duplication in other
// books (FA5), budgeted assets.
type FAJnlCheckLine struct {
	allowPostingFrom types.Date
	allowPostingTo   types.Date
	datesRead        bool
}

// CheckFAJnlLine checks an FA journal line before it is posted (a line without asset is not
// checked)
func (c *FAJnlCheckLine) CheckFAJnlLine(faJnlLine *gtables.FAJournalLine) error {
	if faJnlLine.Fa_no == "" {
		return nil
	}
	for _, field := range []string{"fa_posting_date", "depreciation_book_code", "document_no"} {
		err := faJnlLine.TestField(field)
		if err != nil {
			return err
		}
	}
	if faJnlLine.Duplicate_in_depreciation_book == faJnlLine.Depreciation_book_code {
		return apperrors.FANotDifferentThan(gtables.FAJournalLineTableName, "duplicate_in_depreciation_book", "depreciation_book_code")
	}
	err := c.checkNotYet(faJnlLine)
	if err != nil {
		return err
	}
	postingDate := faJnlLine.Posting_date
	if postingDate.IsZero() {
		postingDate = faJnlLine.Fa_posting_date
	}
	fa := gtables.NewFixedAsset(faJnlLine)
	err = fa.GetOrError(faJnlLine.Fa_no)
	if err != nil {
		return err
	}
	deprBook := gtables.NewDepreciationBook(faJnlLine)
	err = deprBook.GetOrError(faJnlLine.Depreciation_book_code)
	if err != nil {
		return err
	}
	faDeprBook := gtables.NewFADepreciationBook(faJnlLine)
	err = faDeprBook.GetOrError(map[string]interface{}{"fa_no": faJnlLine.Fa_no, "depreciation_book_code": faJnlLine.Depreciation_book_code})
	if err != nil {
		return err
	}
	err = c.checkFAPostingDate(faJnlLine, deprBook, postingDate)
	if err != nil {
		return err
	}
	err = checkFAIntegration(faJnlLine, deprBook)
	if err != nil {
		return err
	}
	err = checkConsistency(faJnlLine, deprBook)
	if err != nil {
		return err
	}
	return checkErrorNo(faJnlLine)
}

// checkNotYet: what later fixed asset steps bring
func (c *FAJnlCheckLine) checkNotYet(faJnlLine *gtables.FAJournalLine) error {
	table := gtables.FAJournalLineTableName
	switch {
	case faJnlLine.Fa_posting_type == gtables.FAJournalLine_Fa_posting_type.Disposal:
		return apperrors.FANotYet(table, "fa_posting_type", faJnlLine.Fa_posting_type)
	case faJnlLine.Depr_until_fa_posting_date:
		return apperrors.FANotYet(table, "depr_until_fa_posting_date", true)
	case faJnlLine.Depr_acquisition_cost:
		return apperrors.FANotYet(table, "depr_acquisition_cost", true)
	case faJnlLine.Use_duplication_list:
		return apperrors.FANotYet(table, "use_duplication_list", true)
	case faJnlLine.Duplicate_in_depreciation_book != "":
		return apperrors.FANotYet(table, "duplicate_in_depreciation_book", faJnlLine.Duplicate_in_depreciation_book)
	}
	return nil
}

// checkFAPostingDate (CheckFAPostingDate): not a closing date, within the allowed FA posting
// dates, the same as the posting date when the book uses the same FA and G/L posting dates
func (c *FAJnlCheckLine) checkFAPostingDate(faJnlLine *gtables.FAJournalLine, deprBook *gtables.DepreciationBook, postingDate types.Date) error {
	table := gtables.FAJournalLineTableName
	faPostingDate := faJnlLine.Fa_posting_date
	if !faPostingDate.Equal(system.NormalDate(faPostingDate)) {
		return apperrors.JnlClosingDateGLOnly(table, "fa_posting_date")
	}
	if !c.datesRead {
		c.datesRead = true
		userSetup := gtables.NewUserSetup(faJnlLine)
		if system.UserId() != "" && userSetup.Get(types.Code(system.UserId())) {
			c.allowPostingFrom = userSetup.Allow_fa_posting_from
			c.allowPostingTo = userSetup.Allow_fa_posting_to
		}
		if c.allowPostingFrom.IsZero() && c.allowPostingTo.IsZero() {
			faSetup := gtables.GetFASetup(faJnlLine)
			c.allowPostingFrom = faSetup.Allow_fa_posting_from
			c.allowPostingTo = faSetup.Allow_fa_posting_to
		}
	}
	if faPostingDate.Before(c.allowPostingFrom) || !c.allowPostingTo.IsZero() && faPostingDate.After(c.allowPostingTo) {
		return apperrors.JnlDateNotAllowed(table, "fa_posting_date")
	}
	if deprBook.Use_same_fa_g_l_posting_dates && !postingDate.Equal(faPostingDate) {
		return apperrors.FAMustBeIdentical(table, "posting_date", "fa_posting_date")
	}
	return nil
}

// checkFAIntegration (CheckFAIntegration): a posting type that is G/L-integrated in the book
// must be posted in the general journal (a salvage value never is)
func checkFAIntegration(faJnlLine *gtables.FAJournalLine, deprBook *gtables.DepreciationBook) error {
	if glIntegration(deprBook, faJnlLine.Fa_posting_type) {
		return apperrors.FAPostInGenJnl(gtables.FAJournalLineTableName, "fa_posting_type", faJnlLine.Fa_posting_type)
	}
	return nil
}

// glIntegration: whether the book posts a posting type to the G/L
func glIntegration(deprBook *gtables.DepreciationBook, postingType gtables.FAJournalLineFa_posting_type) bool {
	switch postingType {
	case gtables.FAJournalLine_Fa_posting_type.AcquisitionCost:
		return deprBook.G_l_integration_acq_cost
	case gtables.FAJournalLine_Fa_posting_type.Depreciation:
		return deprBook.G_l_integration_depreciation
	case gtables.FAJournalLine_Fa_posting_type.WriteDown:
		return deprBook.G_l_integration_write_down
	case gtables.FAJournalLine_Fa_posting_type.Appreciation:
		return deprBook.G_l_integration_appreciation
	case gtables.FAJournalLine_Fa_posting_type.Custom1:
		return deprBook.G_l_integration_custom_1
	case gtables.FAJournalLine_Fa_posting_type.Custom2:
		return deprBook.G_l_integration_custom_2
	case gtables.FAJournalLine_Fa_posting_type.Disposal:
		return deprBook.G_l_integration_disposal
	case gtables.FAJournalLine_Fa_posting_type.Maintenance:
		return deprBook.G_l_integration_maintenance
	}
	return false
}

// checkConsistency (CheckConsistency): the fields the posting type does not use are blank
func checkConsistency(faJnlLine *gtables.FAJournalLine, deprBook *gtables.DepreciationBook) error {
	table := gtables.FAJournalLineTableName
	postingType := faJnlLine.Fa_posting_type
	notTogether := func(field string) error {
		return apperrors.FANotTogetherWith(table, field, "fa_posting_type", postingType)
	}
	if faJnlLine.Fa_error_entry_no > 0 && postingType == gtables.FAJournalLine_Fa_posting_type.Maintenance {
		return notTogether("fa_error_entry_no")
	}
	if postingType != gtables.FAJournalLine_Fa_posting_type.AcquisitionCost {
		switch {
		case faJnlLine.Depr_acquisition_cost:
			return notTogether("depr_acquisition_cost")
		case !faJnlLine.Salvage_value.IsZero():
			return notTogether("salvage_value")
		case !faJnlLine.Quantity.IsZero() && postingType != gtables.FAJournalLine_Fa_posting_type.Maintenance:
			return notTogether("quantity")
		case faJnlLine.Insurance_no != "":
			return notTogether("insurance_no")
		}
	}
	if postingType == gtables.FAJournalLine_Fa_posting_type.Maintenance && faJnlLine.Depr_until_fa_posting_date {
		return notTogether("depr_until_fa_posting_date")
	}
	if postingType != gtables.FAJournalLine_Fa_posting_type.Maintenance && faJnlLine.Maintenance_code != "" {
		return notTogether("maintenance_code")
	}
	if postingType != gtables.FAJournalLine_Fa_posting_type.Depreciation && postingType != gtables.FAJournalLine_Fa_posting_type.Custom1 &&
		faJnlLine.No_of_depreciation_days != 0 {
		return notTogether("no_of_depreciation_days")
	}
	if postingType == gtables.FAJournalLine_Fa_posting_type.Disposal && faJnlLine.Fa_reclassification_entry {
		return notTogether("fa_reclassification_entry")
	}
	if postingType == gtables.FAJournalLine_Fa_posting_type.AcquisitionCost && faJnlLine.Insurance_no != "" {
		faSetup := gtables.GetFASetup(faJnlLine)
		if deprBook.Code != faSetup.Insurance_depr_book {
			return faJnlLine.TestField("insurance_no", "")
		}
	}
	return nil
}

// checkErrorNo (CheckErrorNo): a line that cancels an entry uses none of the extras
func checkErrorNo(faJnlLine *gtables.FAJournalLine) error {
	if faJnlLine.Fa_error_entry_no <= 0 {
		return nil
	}
	table := gtables.FAJournalLineTableName
	field := ""
	switch {
	case faJnlLine.Depr_until_fa_posting_date:
		field = "depr_until_fa_posting_date"
	case faJnlLine.Depr_acquisition_cost:
		field = "depr_acquisition_cost"
	case faJnlLine.Duplicate_in_depreciation_book != "":
		field = "duplicate_in_depreciation_book"
	case faJnlLine.Use_duplication_list:
		field = "use_duplication_list"
	case !faJnlLine.Salvage_value.IsZero():
		field = "salvage_value"
	case faJnlLine.Insurance_no != "":
		field = "insurance_no"
	}
	if field != "" {
		return apperrors.FANotWithErrorEntry(table, field, "fa_error_entry_no")
	}
	return nil
}
