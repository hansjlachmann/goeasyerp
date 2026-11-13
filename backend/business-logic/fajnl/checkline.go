package fajnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// FAJnlCheckLine is codeunit 5631 FA Jnl.-Check Line, for FA journal lines (CheckFAJnlLine) and
// general journal lines with a fixed asset (CheckGenJnlLine, OnRun): the line has an FA posting
// date, a book and (FA journal) a document no.; the FA posting date is allowed (the user's or the
// FA Setup's Allow FA Posting From/To) and the same as the posting date when the book wants it; a
// posting type the book posts to the G/L is posted in the general journal, the others in the FA
// journal; the fields the posting type does not use are blank; a cancelling line (FA Error Entry
// No.) uses none of the extras. A budgeted asset is posted in the FA journal only; a line's
// Budgeted FA No. (the budgeted asset its amount is taken from) is not for a disposal or a
// budgeted asset.
type FAJnlCheckLine struct {
	allowPostingFrom types.Date
	allowPostingTo   types.Date
	datesRead        bool
}

// faCheck: what the checks need of either journal line
type faCheck struct {
	src                  ftables.Source
	table                string // the line's table (errors)
	genJnlPosting        bool
	faNo                 types.Code
	deprBookCode         types.Code
	postingDate          types.Date
	faPostingDate        types.Date
	postingType          gtables.FAJournalLineFa_posting_type
	postingTypeValue     interface{} // the line's own option value (errors)
	deprUntilFAPostDate  bool
	deprAcquisitionCost  bool
	salvageValue         types.Decimal
	quantity             types.Decimal
	insuranceNo          types.Code
	budgetedFANo         types.Code
	maintenanceCode      types.Code
	noOfDepreciationDays int
	reclassification     bool
	errorEntryNo         int
	duplicateInDeprBook  types.Code
	useDuplicationList   bool
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
	postingDate := faJnlLine.Posting_date
	if postingDate.IsZero() {
		postingDate = faJnlLine.Fa_posting_date
	}
	return c.checkJnlLine(&faCheck{
		src: faJnlLine, table: gtables.FAJournalLineTableName,
		faNo: faJnlLine.Fa_no, deprBookCode: faJnlLine.Depreciation_book_code,
		postingDate: postingDate, faPostingDate: faJnlLine.Fa_posting_date,
		postingType: faJnlLine.Fa_posting_type, postingTypeValue: faJnlLine.Fa_posting_type,
		deprUntilFAPostDate: faJnlLine.Depr_until_fa_posting_date, deprAcquisitionCost: faJnlLine.Depr_acquisition_cost,
		salvageValue: faJnlLine.Salvage_value, quantity: faJnlLine.Quantity,
		insuranceNo: faJnlLine.Insurance_no, budgetedFANo: faJnlLine.Budgeted_fa_no, maintenanceCode: faJnlLine.Maintenance_code,
		noOfDepreciationDays: faJnlLine.No_of_depreciation_days, reclassification: faJnlLine.Fa_reclassification_entry,
		errorEntryNo:        faJnlLine.Fa_error_entry_no,
		duplicateInDeprBook: faJnlLine.Duplicate_in_depreciation_book, useDuplicationList: faJnlLine.Use_duplication_list,
	})
}

// CheckGenJnlLine (OnRun) checks a general journal line with a fixed asset (account or
// balancing account): an FA posting type and a book; not a fixed asset on both sides; an
// acquisition, disposal or maintenance with general posting groups has a general posting type,
// the other posting types have none
func (c *FAJnlCheckLine) CheckGenJnlLine(genJnlLine *gtables.GenJournalLine) error {
	table := gtables.GenJournalLineTableName
	err := genJnlLine.TestField("fa_posting_type")
	if err != nil {
		return err
	}
	err = genJnlLine.TestField("depreciation_book_code")
	if err != nil {
		return err
	}
	if genJnlLine.Duplicate_in_depreciation_book == genJnlLine.Depreciation_book_code {
		return apperrors.FANotDifferentThan(table, "duplicate_in_depreciation_book", "depreciation_book_code")
	}
	if int(genJnlLine.Account_type) == int(genJnlLine.Bal_account_type) {
		return apperrors.JnlAccountTypes(table, "account_type", "bal_account_type")
	}
	postingType := gtables.FAJournalLineFa_posting_type(genJnlLine.Fa_posting_type - 1)
	withGenPosting := postingType == gtables.FAJournalLine_Fa_posting_type.AcquisitionCost ||
		postingType == gtables.FAJournalLine_Fa_posting_type.Disposal ||
		postingType == gtables.FAJournalLine_Fa_posting_type.Maintenance
	var faNo types.Code
	if genJnlLine.Account_no != "" && genJnlLine.Account_type == gtables.GenJournalLine_Account_type.FixedAsset {
		if withGenPosting {
			if genJnlLine.Gen_bus_posting_group != "" || genJnlLine.Gen_prod_posting_group != "" {
				err = genJnlLine.TestField("gen_posting_type")
			}
		} else {
			err = testNoGenPosting(genJnlLine, "gen_posting_type", "gen_bus_posting_group", "gen_prod_posting_group")
		}
		if err != nil {
			return err
		}
		faNo = genJnlLine.Account_no
	}
	if genJnlLine.Bal_account_no != "" && genJnlLine.Bal_account_type == gtables.GenJournalLine_Bal_account_type.FixedAsset {
		if withGenPosting {
			if genJnlLine.Bal_gen_bus_posting_group != "" || genJnlLine.Bal_gen_prod_posting_group != "" {
				err = genJnlLine.TestField("bal_gen_posting_type")
			}
		} else {
			err = testNoGenPosting(genJnlLine, "bal_gen_posting_type", "bal_gen_bus_posting_group", "bal_gen_prod_posting_group")
		}
		if err != nil {
			return err
		}
		faNo = genJnlLine.Bal_account_no
	}
	faPostingDate := genJnlLine.Fa_posting_date
	if faPostingDate.IsZero() {
		faPostingDate = genJnlLine.Posting_date
	}
	return c.checkJnlLine(&faCheck{
		src: genJnlLine, table: table, genJnlPosting: true,
		faNo: faNo, deprBookCode: genJnlLine.Depreciation_book_code,
		postingDate: genJnlLine.Posting_date, faPostingDate: faPostingDate,
		postingType: postingType, postingTypeValue: genJnlLine.Fa_posting_type,
		deprUntilFAPostDate: genJnlLine.Depr_until_fa_posting_date, deprAcquisitionCost: genJnlLine.Depr_acquisition_cost,
		salvageValue: genJnlLine.Salvage_value, quantity: genJnlLine.Quantity,
		insuranceNo: genJnlLine.Insurance_no, budgetedFANo: genJnlLine.Budgeted_fa_no, maintenanceCode: genJnlLine.Maintenance_code,
		noOfDepreciationDays: genJnlLine.No_of_depreciation_days, reclassification: genJnlLine.Fa_reclassification_entry,
		errorEntryNo:        genJnlLine.Fa_error_entry_no,
		duplicateInDeprBook: genJnlLine.Duplicate_in_depreciation_book, useDuplicationList: genJnlLine.Use_duplication_list,
	})
}

// testNoGenPosting: a general posting type and general posting groups are blank
func testNoGenPosting(genJnlLine *gtables.GenJournalLine, fields ...string) error {
	for _, field := range fields {
		var err error
		if field == "gen_posting_type" {
			err = genJnlLine.TestField(field, gtables.GenJournalLine_Gen_posting_type.Blank)
		} else if field == "bal_gen_posting_type" {
			err = genJnlLine.TestField(field, gtables.GenJournalLine_Bal_gen_posting_type.Blank)
		} else {
			err = genJnlLine.TestField(field, types.Code(""))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// checkJnlLine (CheckJnlLine): the checks of both journals
func (c *FAJnlCheckLine) checkJnlLine(l *faCheck) error {
	fa := gtables.NewFixedAsset(l.src)
	err := fa.GetOrError(l.faNo)
	if err != nil {
		return err
	}
	deprBook := gtables.NewDepreciationBook(l.src)
	err = deprBook.GetOrError(l.deprBookCode)
	if err != nil {
		return err
	}
	faDeprBook := gtables.NewFADepreciationBook(l.src)
	err = faDeprBook.GetOrError(map[string]interface{}{"fa_no": l.faNo, "depreciation_book_code": l.deprBookCode})
	if err != nil {
		return err
	}
	err = c.checkFAPostingDate(l, deprBook)
	if err != nil {
		return err
	}
	err = checkFAIntegration(l, deprBook, fa)
	if err != nil {
		return err
	}
	err = checkConsistency(l, deprBook, fa)
	if err != nil {
		return err
	}
	return checkErrorNo(l)
}

// checkFAPostingDate (CheckFAPostingDate): not a closing date, within the allowed FA posting
// dates, the same as the posting date when the book uses the same FA and G/L posting dates
func (c *FAJnlCheckLine) checkFAPostingDate(l *faCheck, deprBook *gtables.DepreciationBook) error {
	faPostingDate := l.faPostingDate
	if !faPostingDate.Equal(system.NormalDate(faPostingDate)) {
		return apperrors.JnlClosingDateGLOnly(l.table, "fa_posting_date")
	}
	if !c.datesRead {
		c.datesRead = true
		userSetup := gtables.NewUserSetup(l.src)
		if system.UserId() != "" && userSetup.Get(types.Code(system.UserId())) {
			c.allowPostingFrom = userSetup.Allow_fa_posting_from
			c.allowPostingTo = userSetup.Allow_fa_posting_to
		}
		if c.allowPostingFrom.IsZero() && c.allowPostingTo.IsZero() {
			faSetup := gtables.GetFASetup(l.src)
			c.allowPostingFrom = faSetup.Allow_fa_posting_from
			c.allowPostingTo = faSetup.Allow_fa_posting_to
		}
	}
	if faPostingDate.Before(c.allowPostingFrom) || !c.allowPostingTo.IsZero() && faPostingDate.After(c.allowPostingTo) {
		return apperrors.JnlDateNotAllowed(l.table, "fa_posting_date")
	}
	if deprBook.Use_same_fa_g_l_posting_dates && !l.postingDate.Equal(faPostingDate) {
		if l.genJnlPosting {
			return apperrors.FAMustBeIdentical(l.table, "fa_posting_date", "posting_date")
		}
		return apperrors.FAMustBeIdentical(l.table, "posting_date", "fa_posting_date")
	}
	return nil
}

// checkFAIntegration (CheckFAIntegration): a posting type the book posts to the G/L is posted in
// the general journal, the others in the FA journal (a salvage value never goes to the G/L); the
// automatic depreciation of a line (Depr. until FA Posting Date, Depr. Acquisition Cost) follows
// the book's depreciation integration; a budgeted asset is never posted in the general journal
func checkFAIntegration(l *faCheck, deprBook *gtables.DepreciationBook, fa *gtables.FixedAsset) error {
	if l.genJnlPosting && fa.Budgeted_asset {
		return apperrors.FAMustNotBeBudgeted(l.table, "account_no")
	}
	if fa.Budgeted_asset {
		return nil
	}
	integrated := glIntegration(deprBook, l.postingType)
	if integrated && !l.genJnlPosting {
		return apperrors.FAPostInGenJnl(l.table, "fa_posting_type", l.postingTypeValue)
	}
	if !integrated && l.genJnlPosting {
		return apperrors.FAPostInFAJnl(l.table, "fa_posting_type", l.postingTypeValue)
	}
	deprIntegration := deprBook.G_l_integration_depreciation
	if l.genJnlPosting == !deprIntegration {
		if l.deprUntilFAPostDate {
			return apperrors.FADeprGLIntegration(l.table, "depr_until_fa_posting_date", l.deprBookCode.String(), deprIntegration)
		}
		if l.deprAcquisitionCost {
			return apperrors.FADeprGLIntegration(l.table, "depr_acquisition_cost", l.deprBookCode.String(), deprIntegration)
		}
	}
	return nil
}

// glIntegration: whether the book posts a posting type to the G/L
func glIntegration(deprBook *gtables.DepreciationBook, postingType gtables.FAJournalLineFa_posting_type) bool {
	return tables.DeprBookGLIntegration(deprBook, postingType)
}

// checkConsistency (CheckConsistency): the fields the posting type does not use are blank; a
// budgeted asset takes no Budgeted FA No.
func checkConsistency(l *faCheck, deprBook *gtables.DepreciationBook, fa *gtables.FixedAsset) error {
	postingType := l.postingType
	notTogether := func(field string) error {
		return apperrors.FANotTogetherWith(l.table, field, "fa_posting_type", l.postingTypeValue)
	}
	if l.errorEntryNo > 0 && postingType == gtables.FAJournalLine_Fa_posting_type.Maintenance {
		return notTogether("fa_error_entry_no")
	}
	if postingType != gtables.FAJournalLine_Fa_posting_type.AcquisitionCost {
		switch {
		case l.deprAcquisitionCost:
			return notTogether("depr_acquisition_cost")
		case !l.salvageValue.IsZero():
			return notTogether("salvage_value")
		case !l.quantity.IsZero() && postingType != gtables.FAJournalLine_Fa_posting_type.Maintenance:
			return notTogether("quantity")
		case l.insuranceNo != "":
			return notTogether("insurance_no")
		}
	}
	if postingType == gtables.FAJournalLine_Fa_posting_type.Maintenance && l.deprUntilFAPostDate {
		return notTogether("depr_until_fa_posting_date")
	}
	if postingType != gtables.FAJournalLine_Fa_posting_type.Maintenance && l.maintenanceCode != "" {
		return notTogether("maintenance_code")
	}
	if postingType != gtables.FAJournalLine_Fa_posting_type.Depreciation && postingType != gtables.FAJournalLine_Fa_posting_type.Custom1 &&
		l.noOfDepreciationDays != 0 {
		return notTogether("no_of_depreciation_days")
	}
	if postingType == gtables.FAJournalLine_Fa_posting_type.Disposal {
		if l.reclassification {
			return notTogether("fa_reclassification_entry")
		}
		if l.budgetedFANo != "" {
			return notTogether("budgeted_fa_no")
		}
	}
	faField := "fa_no"
	if l.genJnlPosting {
		faField = "account_no"
	}
	if fa.Budgeted_asset && l.budgetedFANo != "" {
		return apperrors.FANotWhenBudgeted(l.table, "budgeted_fa_no", faField)
	}
	if postingType == gtables.FAJournalLine_Fa_posting_type.AcquisitionCost && l.insuranceNo != "" {
		faSetup := gtables.GetFASetup(l.src)
		if deprBook.Code != faSetup.Insurance_depr_book {
			return apperrors.FieldMustBeValue(l.table, "insurance_no", "")
		}
	}
	if l.genJnlPosting && fa.Budgeted_asset {
		return apperrors.FAMustNotBeBudgeted(l.table, "account_no")
	}
	return nil
}

// checkErrorNo (CheckErrorNo): a line that cancels an entry uses none of the extras
func checkErrorNo(l *faCheck) error {
	if l.errorEntryNo <= 0 {
		return nil
	}
	field := ""
	switch {
	case l.deprUntilFAPostDate:
		field = "depr_until_fa_posting_date"
	case l.deprAcquisitionCost:
		field = "depr_acquisition_cost"
	case l.duplicateInDeprBook != "":
		field = "duplicate_in_depreciation_book"
	case l.useDuplicationList:
		field = "use_duplication_list"
	case !l.salvageValue.IsZero():
		field = "salvage_value"
	case l.insuranceNo != "":
		field = "insurance_no"
	case l.budgetedFANo != "":
		field = "budgeted_fa_no"
	}
	if field != "" {
		return apperrors.FANotWithErrorEntry(l.table, field, "fa_error_entry_no")
	}
	return nil
}
