package fajnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunit 5640 Duplicate Depr. Book: a line posted with Duplicate in Depreciation Book (or Use
// Duplication List: every book marked Part of Duplication List) puts the same posting as a line
// in the other book's journal (FA Journal Setup — the general journal when that book posts the
// type to the G/L, codeunit 5639 JnlName), at the books' FA exchange rates when the book uses them.
// The user posts those lines. An acquisition with an Insurance No. (in the FA Setup's insurance
// book) gives the insurance coverage: posted at once with Automatic Insurance Posting, else a line
// in the book's insurance journal (InsertInsurance).

// dupLine: what a duplicated line copies (MakeGenJnlLine/MakeFAJnlLine/Adjust…)
type dupLine struct {
	journalTemplate, journalBatch  types.Code
	genJnl                         bool // the line comes from the general journal
	faNo                           types.Code
	postingType                    gtables.FAJournalLineFa_posting_type
	faPostingDate, postingDate     types.Date
	documentType                   int
	documentDate                   types.Date
	documentNo, externalDocNo      types.Code
	description                    types.Text
	amount, salvageValue           types.Decimal
	quantity                       types.Decimal
	correction                     bool
	noOfDeprDays                   int
	deprUntil, deprAcqCost         bool
	faPostingGroup                 types.Code
	maintenanceCode                types.Code
	dimension1Code, dimension2Code types.Code
	reclassification, indexEntry   bool
	budgetedFANo                   types.Code
}

// duplicateFAJnlLine (DuplicateFAJnlLine)
func (p *FAJnlPostLine) duplicateFAJnlLine(l *gtables.FAJournalLine) error {
	src := p.src
	if l.Insurance_no != "" {
		err := p.insertInsurance(l.Depreciation_book_code, &insuranceLine{
			journalBatchName: l.Journal_batch_name, sourceCode: l.Source_code, reasonCode: l.Reason_code,
			insuranceNo: l.Insurance_no, faNo: l.Fa_no, postingDate: l.Fa_posting_date, amount: l.Amount,
			documentType: int(l.Document_type), documentDate: l.Document_date, documentNo: l.Document_no, externalDocNo: l.External_document_no,
			dimension1Code: l.Dimension_1_code, dimension2Code: l.Dimension_2_code,
		})
		if err != nil {
			return err
		}
	}
	if l.Duplicate_in_depreciation_book == "" && !l.Use_duplication_list {
		return nil
	}
	return duplicate(src, l.Depreciation_book_code, l.Duplicate_in_depreciation_book, l.Use_duplication_list, &dupLine{
		journalTemplate: l.Journal_template_name, journalBatch: l.Journal_batch_name,
		faNo: l.Fa_no, postingType: l.Fa_posting_type, faPostingDate: l.Fa_posting_date, postingDate: l.Posting_date,
		documentType: int(l.Document_type), documentDate: l.Document_date, documentNo: l.Document_no, externalDocNo: l.External_document_no,
		description: l.Description, amount: l.Amount, salvageValue: l.Salvage_value, quantity: l.Quantity, correction: l.Correction,
		noOfDeprDays: l.No_of_depreciation_days, deprUntil: l.Depr_until_fa_posting_date, deprAcqCost: l.Depr_acquisition_cost,
		faPostingGroup: l.Fa_posting_group, maintenanceCode: l.Maintenance_code, dimension1Code: l.Dimension_1_code, dimension2Code: l.Dimension_2_code,
		reclassification: l.Fa_reclassification_entry, indexEntry: l.Index_entry, budgetedFANo: l.Budgeted_fa_no,
	})
}

// duplicateGenJnlLine (DuplicateGenJnlLine): the FA amount is the line's G/L entry amount
func (p *FAJnlPostLine) duplicateGenJnlLine(l *gtables.GenJournalLine, faAmount types.Decimal) error {
	src := p.src
	if l.Insurance_no != "" {
		postingDate := l.Fa_posting_date
		if postingDate.IsZero() {
			postingDate = l.Posting_date
		}
		err := p.insertInsurance(l.Depreciation_book_code, &insuranceLine{
			journalBatchName: l.Journal_batch_name, sourceCode: l.Source_code, reasonCode: l.Reason_code,
			insuranceNo: l.Insurance_no, faNo: l.Account_no, postingDate: postingDate, amount: faAmount,
			documentType: int(l.Document_type), documentDate: l.Document_date, documentNo: l.Document_no, externalDocNo: l.External_document_no,
			dimension1Code: l.Dimension_1_code, dimension2Code: l.Dimension_2_code,
		})
		if err != nil {
			return err
		}
	}
	if l.Duplicate_in_depreciation_book == "" && !l.Use_duplication_list {
		return nil
	}
	return duplicate(src, l.Depreciation_book_code, l.Duplicate_in_depreciation_book, l.Use_duplication_list, &dupLine{
		journalTemplate: l.Journal_template_name, journalBatch: l.Journal_batch_name, genJnl: true,
		faNo: l.Account_no, postingType: gtables.FAJournalLineFa_posting_type(l.Fa_posting_type - 1), faPostingDate: l.Fa_posting_date, postingDate: l.Posting_date,
		documentType: int(l.Document_type), documentDate: l.Document_date, documentNo: l.Document_no, externalDocNo: l.External_document_no,
		description: l.Description, amount: faAmount, salvageValue: l.Salvage_value, quantity: l.Quantity, correction: l.Correction,
		noOfDeprDays: l.No_of_depreciation_days, deprUntil: l.Depr_until_fa_posting_date, deprAcqCost: l.Depr_acquisition_cost,
		faPostingGroup: l.Posting_group, maintenanceCode: l.Maintenance_code, dimension1Code: l.Dimension_1_code, dimension2Code: l.Dimension_2_code,
		reclassification: l.Fa_reclassification_entry, indexEntry: l.Index_entry, budgetedFANo: l.Budgeted_fa_no,
	})
}

// duplicate: the line in the one book or in every book of the duplication list
func duplicate(src ftables.Source, deprBookCode, duplicateIn types.Code, useList bool, l *dupLine) error {
	deprBook := gtables.NewDepreciationBook(src)
	err := deprBook.GetOrError(deprBookCode)
	if err != nil {
		return err
	}
	exchangeRate := dupExchangeRate(src, l.faNo, deprBook)
	if duplicateIn != "" {
		target := gtables.NewDepreciationBook(src)
		err = target.GetOrError(duplicateIn)
		if err != nil {
			return err
		}
		return createDupLine(src, target, exchangeRate, l)
	}
	if useList {
		target := gtables.NewDepreciationBook(src)
		for found := target.FindSet(); found; found = target.Next() {
			if target.Part_of_duplication_list && target.Code != deprBookCode {
				book := gtables.NewDepreciationBook(src)
				book.Copy(target)
				err = createDupLine(src, book, exchangeRate, l)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// dupExchangeRate (GetExchangeRate): the asset's FA exchange rate in a book that uses them (100:
// none)
func dupExchangeRate(src ftables.Source, faNo types.Code, deprBook *gtables.DepreciationBook) types.Decimal {
	hundred := types.NewDecimalFromInt(100)
	if !deprBook.Use_fa_exch_rate_in_duplic {
		return hundred
	}
	faDeprBook := gtables.NewFADepreciationBook(src)
	if faDeprBook.Get(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBook.Code}) && faDeprBook.Fa_exchange_rate.IsPositive() {
		return faDeprBook.Fa_exchange_rate
	}
	if deprBook.Default_exchange_rate.IsPositive() {
		return deprBook.Default_exchange_rate
	}
	return hundred
}

// createDupLine (CreateLine, JnlName, InsertLine): the line in the target book's journal
func createDupLine(src ftables.Source, deprBook *gtables.DepreciationBook, exchangeRate types.Decimal, l *dupLine) error {
	fa := gtables.NewFixedAsset(src)
	err := fa.GetOrError(l.faNo)
	if err != nil {
		return err
	}
	toGenJnl := !fa.Budgeted_asset && glIntegration(deprBook, l.postingType)
	var templateName, batchName types.Code
	if toGenJnl {
		templateName, batchName, _, err = tables.GenJnlName(src, deprBook.Code)
	} else {
		templateName, batchName, _, err = tables.FAJnlName(src, deprBook.Code)
	}
	if err != nil {
		return err
	}
	if toGenJnl == l.genJnl && templateName == l.journalTemplate && batchName == l.journalBatch {
		return apperrors.FADuplicateCurrentJnl(gtables.FAJournalSetupTableName)
	}
	amount, salvageValue := l.amount, l.salvageValue
	if deprBook.Use_fa_exch_rate_in_duplic {
		rate := exchangeRate.Div(dupExchangeRate(src, l.faNo, deprBook))
		amount = system.Round(amount.Mul(rate))
		salvageValue = system.Round(salvageValue.Mul(rate))
	}
	if toGenJnl {
		_, _, lastLineNo, _ := tables.GenJnlName(src, deprBook.Code)
		g := gtables.NewGenJournalLine(src)
		g.Init()
		g.Journal_template_name = templateName
		g.Journal_batch_name = batchName
		g.Line_no = lastLineNo + 10000
		g.Account_type = gtables.GenJournalLine_Account_type.FixedAsset
		g.Account_no = l.faNo
		g.Fa_posting_type = gtables.GenJournalLineFa_posting_type(l.postingType + 1)
		g.Fa_posting_date = l.faPostingDate
		g.Posting_date = l.postingDate
		if g.Posting_date.IsZero() {
			g.Posting_date = l.faPostingDate
		}
		if g.Posting_date.Equal(g.Fa_posting_date) {
			g.Fa_posting_date = types.Date{}
		}
		g.Document_type = gtables.GenJournalLineDocument_type(l.documentType)
		g.Document_date = l.documentDate
		g.Document_no = l.documentNo
		g.External_document_no = l.externalDocNo
		g.Description = l.description
		if err = g.Validate("amount", amount); err != nil {
			return err
		}
		g.Salvage_value = salvageValue
		g.Quantity = l.quantity
		if err = g.Validate("correction", l.correction); err != nil {
			return err
		}
		g.No_of_depreciation_days = l.noOfDeprDays
		g.Depr_until_fa_posting_date = l.deprUntil
		g.Depr_acquisition_cost = l.deprAcqCost
		g.Posting_group = l.faPostingGroup
		g.Maintenance_code = l.maintenanceCode
		g.Dimension_1_code = l.dimension1Code
		g.Dimension_2_code = l.dimension2Code
		g.Budgeted_fa_no = l.budgetedFANo
		g.Fa_reclassification_entry = l.reclassification
		g.Index_entry = l.indexEntry
		if err = g.Validate("depreciation_book_code", deprBook.Code); err != nil {
			return err
		}
		g.Posting_no_series = tables.GetGenNoSeries(src, templateName, batchName)
		if err = tables.SetGenJnlTrailCodes(g); err != nil {
			return err
		}
		return g.InsertOrError(true)
	}
	_, _, lastLineNo, _ := tables.FAJnlName(src, deprBook.Code)
	f := gtables.NewFAJournalLine(src)
	f.Init()
	f.Journal_template_name = templateName
	f.Journal_batch_name = batchName
	f.Line_no = lastLineNo + 10000
	f.Fa_no = l.faNo
	f.Fa_posting_type = l.postingType
	f.Fa_posting_date = l.faPostingDate
	f.Posting_date = l.postingDate
	if f.Posting_date.Equal(f.Fa_posting_date) {
		f.Posting_date = types.Date{}
	}
	f.Document_type = gtables.FAJournalLineDocument_type(l.documentType)
	f.Document_date = l.documentDate
	f.Document_no = l.documentNo
	f.External_document_no = l.externalDocNo
	f.Description = l.description
	if err = f.Validate("amount", amount); err != nil {
		return err
	}
	f.Salvage_value = salvageValue
	f.Quantity = l.quantity
	if err = f.Validate("correction", l.correction); err != nil {
		return err
	}
	f.No_of_depreciation_days = l.noOfDeprDays
	f.Depr_until_fa_posting_date = l.deprUntil
	f.Depr_acquisition_cost = l.deprAcqCost
	f.Fa_posting_group = l.faPostingGroup
	f.Maintenance_code = l.maintenanceCode
	f.Dimension_1_code = l.dimension1Code
	f.Dimension_2_code = l.dimension2Code
	f.Budgeted_fa_no = l.budgetedFANo
	f.Fa_reclassification_entry = l.reclassification
	f.Index_entry = l.indexEntry
	if err = f.Validate("depreciation_book_code", deprBook.Code); err != nil {
		return err
	}
	f.Posting_no_series = tables.GetFANoSeries(src, templateName, batchName)
	return f.InsertOrError(true)
}

// insuranceLine: what an insurance journal line takes from the FA or general journal line
type insuranceLine struct {
	journalBatchName, sourceCode, reasonCode types.Code
	insuranceNo, faNo                        types.Code
	postingDate                              types.Date
	amount                                   types.Decimal
	documentType                             int
	documentDate                             types.Date
	documentNo, externalDocNo                types.Code
	dimension1Code, dimension2Code           types.Code
}

// insertInsurance (InsertInsurance, InitInsuranceJnlLine): the acquisition's insurance coverage —
// posted at once with the FA Setup's Automatic Insurance Posting (the batch, source and reason code
// of the line posted), else a line in the book's insurance journal (FA Journal Setup)
func (p *FAJnlPostLine) insertInsurance(deprBookCode types.Code, l *insuranceLine) error {
	faSetup := gtables.GetFASetup(p.src)
	err := faSetup.TestField("insurance_depr_book", deprBookCode)
	if err != nil {
		return err
	}
	insuranceJnlLine := gtables.NewInsuranceJournalLine(p.src)
	insuranceJnlLine.Init()
	if !faSetup.Automatic_insurance_posting {
		templateName, batchName, lastLineNo, err := tables.InsuranceJnlName(p.src, deprBookCode)
		if err != nil {
			return err
		}
		insuranceJnlLine.Journal_template_name = templateName
		insuranceJnlLine.Journal_batch_name = batchName
		insuranceJnlLine.LockTable()
		insuranceJnlLine.Line_no = lastLineNo + 10000
		insuranceJnlLine.Posting_no_series = tables.GetInsuranceNoSeries(p.src, templateName, batchName)
	}
	if faSetup.Automatic_insurance_posting {
		insuranceJnlLine.Journal_batch_name = l.journalBatchName
		insuranceJnlLine.Source_code = l.sourceCode
		insuranceJnlLine.Reason_code = l.reasonCode
	}
	err = insuranceJnlLine.Validate("insurance_no", l.insuranceNo)
	if err != nil {
		return err
	}
	err = insuranceJnlLine.Validate("fa_no", l.faNo)
	if err != nil {
		return err
	}
	insuranceJnlLine.Posting_date = l.postingDate
	err = insuranceJnlLine.Validate("amount", l.amount)
	if err != nil {
		return err
	}
	insuranceJnlLine.Document_type = gtables.InsuranceJournalLineDocument_type(l.documentType)
	insuranceJnlLine.Document_date = l.documentDate
	if insuranceJnlLine.Document_date.IsZero() {
		insuranceJnlLine.Document_date = insuranceJnlLine.Posting_date
	}
	insuranceJnlLine.Document_no = l.documentNo
	insuranceJnlLine.External_document_no = l.externalDocNo
	insuranceJnlLine.Dimension_1_code = l.dimension1Code
	insuranceJnlLine.Dimension_2_code = l.dimension2Code
	if faSetup.Automatic_insurance_posting {
		return p.insuranceJnlPostLine.Run(insuranceJnlLine)
	}
	return insuranceJnlLine.InsertOrError(true)
}
