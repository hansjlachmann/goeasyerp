package fajnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/insurancejnl"
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fadepr"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// FAJnlPostLine is codeunit 5632 FA Jnl.-Post Line with the codeunits it runs: 5604/5647 Make
// FA/Maintenance Ledger Entry (the entry from the journal line and the asset's card and
// depreciation book), 5600 FA Insert Ledger Entry (the entry numbered, Part of Book Value / Part
// of Depreciable Basis by posting type, Amount (LCY), the FA register, a cancelled entry (FA Error
// Entry No.) and the entry that cancels it both taken off the asset) and 5606 FA Check
// Consistency. An acquisition cost with a salvage value posts the salvage value too. One
// FAJnlPostLine posts the lines of one posting: they share its FA register.
//
// A general journal line with a fixed asset posts through GenJnlPostLine: its FA or maintenance
// entry, and the G/L amounts (codeunit 5601, GLBuffer) codeunit 12 posts as G/L entries.
//
// A line with Depr. until FA Posting Date first posts the depreciation up to its date, with Depr.
// Acquisition Cost after an acquisition the depreciation it adds (package fadepr).
//
// Duplicate in Depreciation Book / Use Duplication List put the line into the other books' journals
// (codeunit 5640, duplicate.go).
//
// A line with Budgeted FA No. marks its entry with the budgeted asset (FA No./Budgeted FA No.) and
// posts the same amount — and salvage value — reversed to the budgeted asset, so the budget shows
// what is left of it (SetBudgetAssetNo, PostBudgetAsset).
type FAJnlPostLine struct {
	src                    ftables.Source
	faJnlCheckLine         FAJnlCheckLine
	nextEntryNo            int
	nextMaintenanceEntryNo int
	faReg                  *gtables.FARegister
	glAcc                  faInsertGLAcc
	insuranceJnlPostLine   *insurancejnl.InsuranceJnlPostLine // automatic insurance posting (codeunit 5640's)
}

// NewFAJnlPostLine opens the line posting in the posting's database (its transaction)
func NewFAJnlPostLine(src ftables.Source) *FAJnlPostLine {
	return &FAJnlPostLine{src: src, insuranceJnlPostLine: insurancejnl.NewInsuranceJnlPostLine(src)}
}

// FAJnlPostLine posts an FA journal line (checked first when checkLine): a maintenance writes a
// maintenance ledger entry, every other posting type an FA ledger entry
func (p *FAJnlPostLine) FAJnlPostLine(faJnlLine *gtables.FAJournalLine, checkLine bool) error {
	if faJnlLine.Fa_no == "" {
		return nil
	}
	if faJnlLine.Posting_date.IsZero() {
		faJnlLine.Posting_date = faJnlLine.Fa_posting_date
	}
	if checkLine {
		err := p.faJnlCheckLine.CheckFAJnlLine(faJnlLine)
		if err != nil {
			return err
		}
	}
	err := p.duplicateFAJnlLine(faJnlLine)
	if err != nil {
		return err
	}
	if faJnlLine.Fa_posting_type == gtables.FAJournalLine_Fa_posting_type.Maintenance {
		maintenanceLedgEntry := makeMaintenanceLedgEntry(p.src, faJnlLine)
		return p.maintenanceCode(maintenanceLedgEntry, faJnlLine.Budgeted_fa_no, faJnlLine.Amount)
	}
	faLedgEntry := makeFALedgEntry(p.src, faJnlLine)
	return p.code(faLedgEntry, lineValues{salvageValue: faJnlLine.Salvage_value, amount: faJnlLine.Amount, faPostingDate: faJnlLine.Fa_posting_date,
		deprUntilDate: faJnlLine.Depr_until_fa_posting_date, deprAcqCost: faJnlLine.Depr_acquisition_cost, budgetNo: faJnlLine.Budgeted_fa_no})
}

// GenJnlPostLine posts a general journal line's fixed asset (codeunit 12's PostFixedAsset): the
// FA or maintenance entry with the amount and tax of the line's G/L entry, its transaction and
// G/L entry no.; GLBuffer has the G/L amounts to post
func (p *FAJnlPostLine) GenJnlPostLine(genJnlLine *gtables.GenJournalLine, faAmount, taxAmount types.Decimal, nextTransactionNo, nextGLEntryNo int) error {
	p.glAcc.buffer = nil
	if genJnlLine.Account_no == "" {
		return nil
	}
	if genJnlLine.Fa_posting_date.IsZero() {
		genJnlLine.Fa_posting_date = genJnlLine.Posting_date
	}
	err := p.duplicateGenJnlLine(genJnlLine, faAmount)
	if err != nil {
		return err
	}
	if genJnlLine.Fa_posting_type == gtables.GenJournalLine_Fa_posting_type.Maintenance {
		maintenanceLedgEntry := makeMaintenanceLedgEntryFromGenJnl(p.src, genJnlLine)
		maintenanceLedgEntry.Amount = faAmount
		maintenanceLedgEntry.Tax_amount = taxAmount
		maintenanceLedgEntry.Transaction_no = nextTransactionNo
		maintenanceLedgEntry.G_l_entry_no = nextGLEntryNo
		return p.maintenanceCode(maintenanceLedgEntry, genJnlLine.Budgeted_fa_no, faAmount)
	}
	faLedgEntry := makeFALedgEntryFromGenJnl(p.src, genJnlLine)
	faLedgEntry.Amount = faAmount
	faLedgEntry.Tax_amount = taxAmount
	faLedgEntry.Transaction_no = nextTransactionNo
	faLedgEntry.G_l_entry_no = nextGLEntryNo
	return p.code(faLedgEntry, lineValues{salvageValue: genJnlLine.Salvage_value, amount: faAmount, faPostingDate: genJnlLine.Fa_posting_date,
		deprUntilDate: genJnlLine.Depr_until_fa_posting_date, deprAcqCost: genJnlLine.Depr_acquisition_cost, budgetNo: genJnlLine.Budgeted_fa_no})
}

// GLBuffer is the G/L amounts of the last general journal line (FindFirstGLAcc/GetNextGLAcc)
func (p *FAJnlPostLine) GLBuffer() []*FAGLPostBuffer {
	return p.glAcc.buffer
}

// FARegister is the posting's FA register (nil before its first entry)
func (p *FAJnlPostLine) FARegister() *gtables.FARegister {
	return p.faReg
}

// code (Code): the asset may be posted to; the entry (and its salvage value) inserted
// lineValues: what Code needs of the journal line (FANo … ErrorEntryNo of the classic codeunit)
type lineValues struct {
	salvageValue  types.Decimal
	amount        types.Decimal // Amount2: the line's FA amount
	faPostingDate types.Date
	deprUntilDate bool       // Depr. until FA Posting Date
	deprAcqCost   bool       // Depr. Acquisition Cost
	budgetNo      types.Code // Budgeted FA No.
}

func (p *FAJnlPostLine) code(faLedgEntry *gtables.FALedgerEntry, v lineValues) error {
	fa, faDeprBook, err := p.getFA(faLedgEntry.Fa_no, faLedgEntry.Depreciation_book_code)
	if err != nil {
		return err
	}
	copyFromFACard(faLedgEntry, fa, faDeprBook)
	if faLedgEntry.Fa_posting_group == "" && faLedgEntry.G_l_entry_no > 0 {
		err = faDeprBook.TestField("fa_posting_group")
		if err != nil {
			return err
		}
		faLedgEntry.Fa_posting_group = faDeprBook.Fa_posting_group
	}
	if v.deprUntilDate {
		err = p.postDeprUntilDate(faLedgEntry, fa, v, false)
		if err != nil {
			return err
		}
	}
	if faLedgEntry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal {
		err = p.postDisposalEntry(faLedgEntry, fa)
		if err != nil {
			return err
		}
		if v.deprAcqCost {
			return p.postDeprUntilDate(faLedgEntry, fa, v, true)
		}
		return nil
	}
	// DeprLine: a depreciation line without amount only depreciates until its date
	deprLine := v.amount.IsZero() && faLedgEntry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.Depreciation && v.deprUntilDate
	if v.budgetNo != "" {
		err = setBudgetAssetNo(p.src, v.budgetNo, &faLedgEntry.Fa_no_budgeted_fa_no)
		if err != nil {
			return err
		}
	}
	if !deprLine {
		p.glAcc.orgGenJnlLine = true
		err = p.insertFALedgEntry(faLedgEntry, fa)
		p.glAcc.orgGenJnlLine = false
		if err != nil {
			return err
		}
	}
	// PostSalvageValue: an acquisition's salvage value
	if !v.salvageValue.IsZero() && faLedgEntry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost {
		salvage := gtables.NewFALedgerEntry(p.src)
		salvage.Copy(faLedgEntry)
		salvage.Entry_no = 0
		salvage.Automatic_entry = true
		salvage.Amount = v.salvageValue
		salvage.Fa_posting_type = gtables.FALedgerEntry_Fa_posting_type.SalvageValue
		err = p.insertFALedgEntry(salvage, fa)
		if err != nil {
			return err
		}
	}
	if v.deprAcqCost {
		err = p.postDeprUntilDate(faLedgEntry, fa, v, true)
		if err != nil {
			return err
		}
	}
	if v.budgetNo != "" {
		return p.postBudgetAsset(faLedgEntry, v)
	}
	return nil
}

// setBudgetAssetNo (SetBudgetAssetNo): the entry names the budgeted asset its amount is taken from
// (FA No./Budgeted FA No.); the asset must be a budgeted asset
func setBudgetAssetNo(src ftables.Source, budgetNo types.Code, faNoBudgetedFANo *types.Code) error {
	fa2 := gtables.NewFixedAsset(src)
	err := fa2.GetOrError(budgetNo)
	if err != nil {
		return err
	}
	if !fa2.Budgeted_asset {
		return apperrors.FANotABudgetedAsset(fa2.No.String())
	}
	*faNoBudgetedFANo = budgetNo
	return nil
}

// budgetedAsset: the budgeted asset of a line (Budgeted FA No.), not blocked or inactive
func (p *FAJnlPostLine) budgetedAsset(budgetNo types.Code) (*gtables.FixedAsset, error) {
	fa2 := gtables.NewFixedAsset(p.src)
	err := fa2.GetOrError(budgetNo)
	if err != nil {
		return nil, err
	}
	err = fa2.TestField("blocked", false)
	if err != nil {
		return nil, err
	}
	err = fa2.TestField("inactive", false)
	if err != nil {
		return nil, err
	}
	return fa2, nil
}

// postBudgetAsset (PostBudgetAsset): the entry's amount — and an acquisition's salvage value —
// reversed on the budgeted asset, an automatic entry without G/L that names the asset posted
// (FA No./Budgeted FA No.)
func (p *FAJnlPostLine) postBudgetAsset(entry *gtables.FALedgerEntry, v lineValues) error {
	fa2, err := p.budgetedAsset(v.budgetNo)
	if err != nil {
		return err
	}
	budgetEntry := gtables.NewFALedgerEntry(p.src)
	budgetEntry.Copy(entry)
	budgetEntry.Automatic_entry = true
	budgetEntry.G_l_entry_no = 0
	budgetEntry.Fa_no_budgeted_fa_no = entry.Fa_no
	budgetEntry.Fa_no = v.budgetNo
	if !v.salvageValue.IsZero() {
		budgetEntry.Amount = v.salvageValue.Neg()
		faPostingType2 := budgetEntry.Fa_posting_type
		budgetEntry.Fa_posting_type = gtables.FALedgerEntry_Fa_posting_type.SalvageValue
		err = p.insertFALedgEntry(budgetEntry, fa2)
		if err != nil {
			return err
		}
		budgetEntry.Fa_posting_type = faPostingType2
	}
	budgetEntry.Amount = v.amount.Neg()
	return p.insertFALedgEntry(budgetEntry, fa2)
}

// postDeprUntilDate (PostDeprUntilDate): the automatic custom 1 and depreciation entries of a
// line — the depreciation until its FA posting date (codeunit 5610), or the depreciation an
// acquisition adds (Depr. Acquisition Cost, codeunit 5613); with G/L integration their balancing
// amounts (InsertBalAcc)
func (p *FAJnlPostLine) postDeprUntilDate(entry *gtables.FALedgerEntry, fa *gtables.FixedAsset, v lineValues, acqCost bool) error {
	var deprAmount, custom1Amount types.Decimal
	var numberOfDays, custom1NumberOfDays int
	if acqCost {
		var err error
		deprAmount, custom1Amount, err = fadepr.AcqCostDepr(p.src, fa.No, entry.Depreciation_book_code, v.amount.Add(v.salvageValue), v.amount)
		if err != nil {
			return err
		}
	} else {
		result, err := fadepr.Calculate(p.src, fa.No, entry.Depreciation_book_code, v.faPostingDate, fadepr.EntryAmounts{}, types.Date{}, 0)
		if err != nil {
			return err
		}
		deprAmount, custom1Amount = result.DeprAmount, result.Custom1Amount
		numberOfDays, custom1NumberOfDays = result.NumberOfDays, result.Custom1NumberOfDays
	}
	post := func(postingType gtables.FALedgerEntryFa_posting_type, amount types.Decimal, days int) error {
		auto := gtables.NewFALedgerEntry(p.src)
		auto.Copy(entry)
		auto.Automatic_entry = true
		auto.Fa_no_budgeted_fa_no = ""
		auto.Fa_posting_category = gtables.FALedgerEntry_Fa_posting_category.Blank
		auto.Fa_posting_type = postingType
		auto.Amount = amount
		auto.No_of_depreciation_days = days
		err := p.insertFALedgEntry(auto, fa)
		if err != nil {
			return err
		}
		if auto.G_l_entry_no > 0 {
			return p.glAcc.insertBalAcc(p.src, auto)
		}
		return nil
	}
	if !custom1Amount.IsZero() {
		err := post(gtables.FALedgerEntry_Fa_posting_type.Custom1, custom1Amount, custom1NumberOfDays)
		if err != nil {
			return err
		}
	}
	if !deprAmount.IsZero() {
		return post(gtables.FALedgerEntry_Fa_posting_type.Depreciation, deprAmount, numberOfDays)
	}
	return nil
}

// maintenanceCode (MaintenanceCode): the maintenance entry inserted; with a budgeted asset
// (budgetNo) the entry names it and its amount (amount2) is posted to it reversed
func (p *FAJnlPostLine) maintenanceCode(maintenanceLedgEntry *gtables.MaintenanceLedgerEntry, budgetNo types.Code, amount2 types.Decimal) error {
	fa, faDeprBook, err := p.getFA(maintenanceLedgEntry.Fa_no, maintenanceLedgEntry.Depreciation_book_code)
	if err != nil {
		return err
	}
	maintenanceLedgEntry.Fa_class_code = fa.Fa_class_code
	maintenanceLedgEntry.Fa_subclass_code = fa.Fa_subclass_code
	maintenanceLedgEntry.Fa_location_code = fa.Fa_location_code
	maintenanceLedgEntry.Location_code = fa.Location_code
	maintenanceLedgEntry.Fa_exchange_rate = getExchangeRate(faDeprBook)
	if maintenanceLedgEntry.Fa_posting_group == "" && maintenanceLedgEntry.G_l_entry_no > 0 {
		err = faDeprBook.TestField("fa_posting_group")
		if err != nil {
			return err
		}
		maintenanceLedgEntry.Fa_posting_group = faDeprBook.Fa_posting_group
	}
	if budgetNo != "" {
		err = setBudgetAssetNo(p.src, budgetNo, &maintenanceLedgEntry.Fa_no_budgeted_fa_no)
		if err != nil {
			return err
		}
	}
	p.glAcc.orgGenJnlLine = true
	err = p.insertMaintenance(maintenanceLedgEntry, fa, faDeprBook)
	p.glAcc.orgGenJnlLine = false
	if err != nil {
		return err
	}
	if budgetNo == "" {
		return nil
	}
	// PostBudgetAsset for a maintenance
	fa2, err := p.budgetedAsset(budgetNo)
	if err != nil {
		return err
	}
	faDeprBook2 := gtables.NewFADepreciationBook(p.src)
	err = faDeprBook2.GetOrError(map[string]interface{}{"fa_no": budgetNo, "depreciation_book_code": maintenanceLedgEntry.Depreciation_book_code})
	if err != nil {
		return err
	}
	budgetEntry := gtables.NewMaintenanceLedgerEntry(p.src)
	budgetEntry.Copy(maintenanceLedgEntry)
	budgetEntry.Automatic_entry = true
	budgetEntry.G_l_entry_no = 0
	budgetEntry.Fa_no_budgeted_fa_no = maintenanceLedgEntry.Fa_no
	budgetEntry.Fa_no = budgetNo
	budgetEntry.Amount = amount2.Neg()
	return p.insertMaintenance(budgetEntry, fa2, faDeprBook2)
}

// getFA: the asset (not blocked or inactive) and its depreciation book
func (p *FAJnlPostLine) getFA(faNo, deprBookCode types.Code) (*gtables.FixedAsset, *gtables.FADepreciationBook, error) {
	fa := gtables.NewFixedAsset(p.src)
	fa.LockTable()
	deprBook := gtables.NewDepreciationBook(p.src)
	err := deprBook.GetOrError(deprBookCode)
	if err != nil {
		return nil, nil, err
	}
	err = fa.GetOrError(faNo)
	if err != nil {
		return nil, nil, err
	}
	err = fa.TestField("blocked", false)
	if err != nil {
		return nil, nil, err
	}
	err = fa.TestField("inactive", false)
	if err != nil {
		return nil, nil, err
	}
	faDeprBook := gtables.NewFADepreciationBook(p.src)
	err = faDeprBook.GetOrError(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": deprBookCode})
	if err != nil {
		return nil, nil, err
	}
	return fa, faDeprBook, nil
}

// makeFALedgEntry is codeunit 5604 Make FA Ledger Entry's CopyFromFAJnlLine
func makeFALedgEntry(src ftables.Source, faJnlLine *gtables.FAJournalLine) *gtables.FALedgerEntry {
	e := gtables.NewFALedgerEntry(src)
	e.Init()
	e.User_id = types.Code(system.UserId())
	e.Entry_no = faJnlLine.Fa_error_entry_no
	e.G_l_entry_no = 0
	e.Depreciation_book_code = faJnlLine.Depreciation_book_code
	e.Fa_no = faJnlLine.Fa_no
	e.Fa_posting_date = faJnlLine.Fa_posting_date
	e.Posting_date = faJnlLine.Posting_date
	e.Document_date = faJnlLine.Document_date
	if e.Document_date.IsZero() {
		e.Document_date = e.Posting_date
	}
	e.Document_type = gtables.FALedgerEntryDocument_type(faJnlLine.Document_type)
	e.Document_no = faJnlLine.Document_no
	e.External_document_no = faJnlLine.External_document_no
	e.Description = faJnlLine.Description
	e.Amount = faJnlLine.Amount
	e.Quantity = faJnlLine.Quantity
	e.Correction = faJnlLine.Correction
	e.Reclassification_entry = faJnlLine.Fa_reclassification_entry
	e.Index_entry = faJnlLine.Index_entry
	e.No_of_depreciation_days = faJnlLine.No_of_depreciation_days
	e.Fa_posting_group = faJnlLine.Fa_posting_group
	e.Dimension_1_code = faJnlLine.Dimension_1_code
	e.Dimension_2_code = faJnlLine.Dimension_2_code
	e.Reason_code = faJnlLine.Reason_code
	e.Source_code = faJnlLine.Source_code
	e.Journal_batch_name = faJnlLine.Journal_batch_name
	e.Fa_posting_type = tables.FAJnlLineConvertToLedgEntry(faJnlLine.Fa_posting_type)
	e.No_series = faJnlLine.Posting_no_series
	return e
}

// copyFromFACard is codeunit 5604's CopyFromFACard: the asset's class, subclass and locations and
// its depreciation book's method and terms
func copyFromFACard(e *gtables.FALedgerEntry, fa *gtables.FixedAsset, faDeprBook *gtables.FADepreciationBook) {
	e.Fa_class_code = fa.Fa_class_code
	e.Fa_subclass_code = fa.Fa_subclass_code
	e.Fa_location_code = fa.Fa_location_code
	e.Location_code = fa.Location_code
	e.Fa_exchange_rate = getExchangeRate(faDeprBook)
	e.Depreciation_method = gtables.FALedgerEntryDepreciation_method(faDeprBook.Depreciation_method)
	e.Depreciation_starting_date = faDeprBook.Depreciation_starting_date
	e.Depreciation_ending_date = faDeprBook.Depreciation_ending_date
	e.Straight_line_percent = faDeprBook.Straight_line_percent
	e.No_of_depreciation_years = faDeprBook.No_of_depreciation_years
	e.Fixed_depr_amount = faDeprBook.Fixed_depr_amount
	e.Declining_balance_percent = faDeprBook.Declining_balance_percent
	e.Depreciation_table_code = faDeprBook.Depreciation_table_code
	e.Use_fa_ledger_check = faDeprBook.Use_fa_ledger_check
	e.Depr_starting_date_custom_1 = faDeprBook.Depr_starting_date_custom_1
	e.Depr_ending_date_custom_1 = faDeprBook.Depr_ending_date_custom_1
	e.Accum_depr_percent_custom_1 = faDeprBook.Accum_depr_percent_custom_1
	e.Depr_percent_this_year_custom_1 = faDeprBook.Depr_this_year_percent_custom_1
	e.Property_class_custom_1 = gtables.FALedgerEntryProperty_class_custom_1(faDeprBook.Property_class_custom_1)
}

// makeMaintenanceLedgEntry is codeunit 5647 Make Maintenance Ledger Entry's CopyFromFAJnlLine
func makeMaintenanceLedgEntry(src ftables.Source, faJnlLine *gtables.FAJournalLine) *gtables.MaintenanceLedgerEntry {
	e := gtables.NewMaintenanceLedgerEntry(src)
	e.Init()
	e.User_id = types.Code(system.UserId())
	e.G_l_entry_no = 0
	e.Depreciation_book_code = faJnlLine.Depreciation_book_code
	e.Fa_no = faJnlLine.Fa_no
	e.Fa_posting_date = faJnlLine.Fa_posting_date
	e.Posting_date = faJnlLine.Posting_date
	e.Document_date = faJnlLine.Document_date
	if e.Document_date.IsZero() {
		e.Document_date = e.Posting_date
	}
	e.Document_type = gtables.MaintenanceLedgerEntryDocument_type(faJnlLine.Document_type)
	e.Document_no = faJnlLine.Document_no
	e.External_document_no = faJnlLine.External_document_no
	e.Description = faJnlLine.Description
	e.Amount = faJnlLine.Amount
	e.Quantity = faJnlLine.Quantity
	e.Index_entry = faJnlLine.Index_entry
	e.Fa_posting_group = faJnlLine.Fa_posting_group
	e.Dimension_1_code = faJnlLine.Dimension_1_code
	e.Dimension_2_code = faJnlLine.Dimension_2_code
	e.Reason_code = faJnlLine.Reason_code
	e.Source_code = faJnlLine.Source_code
	e.Journal_batch_name = faJnlLine.Journal_batch_name
	e.Maintenance_code = faJnlLine.Maintenance_code
	e.Correction = faJnlLine.Correction
	e.No_series = faJnlLine.Posting_no_series
	return e
}

// getExchangeRate is FA Depreciation Book GetExchangeRate: with the book's Use FA Exch. Rate in
// Duplic. the asset's FA Exchange Rate, else the book's default (0: none)
func getExchangeRate(faDeprBook *gtables.FADepreciationBook) types.Decimal {
	deprBook := gtables.NewDepreciationBook(faDeprBook)
	if !deprBook.Get(faDeprBook.Depreciation_book_code) || !deprBook.Use_fa_exch_rate_in_duplic {
		return types.Decimal{}
	}
	if faDeprBook.Fa_exchange_rate.IsPositive() {
		return faDeprBook.Fa_exchange_rate
	}
	return deprBook.Default_exchange_rate
}

// amountLCY: an amount at an FA exchange rate (in percent; none: the amount)
func amountLCY(amount, exchangeRate types.Decimal) types.Decimal {
	if !exchangeRate.IsPositive() {
		return system.Round(amount)
	}
	return system.Round(amount.Mul(exchangeRate).Div(types.NewDecimalFromInt(100)))
}

// setDebitCredit: Debit/Credit Amount of an amount (a correction the other way round)
func setDebitCredit(amount types.Decimal, correction bool) (types.Decimal, types.Decimal) {
	if amount.IsPositive() && !correction || amount.IsNegative() && correction {
		return amount, types.Decimal{}
	}
	return types.Decimal{}, amount.Neg()
}

// insertFALedgEntry is codeunit 5600 FA Insert Ledger Entry's OnRun: the entry numbered in the FA
// register, a main asset refused without Allow Posting to Main Assets, Part of Book Value / Part
// of Depreciable Basis by posting type, Amount (LCY), inserted; a cancelling entry and the entry
// it cancels taken off the asset (Canceled from FA No.); the consistency check
func (p *FAJnlPostLine) insertFALedgEntry(entry *gtables.FALedgerEntry, fa *gtables.FixedAsset) error {
	if p.nextEntryNo == 0 {
		last := gtables.NewFALedgerEntry(p.src)
		last.LockTable()
		if last.FindLast() {
			p.nextEntryNo = last.Entry_no
		}
		err := p.initRegister(entry.G_l_entry_no, entry.Source_code, entry.Journal_batch_name)
		if err != nil {
			return err
		}
	}
	p.nextEntryNo++
	err := checkMainAsset(p.src, fa, entry.Depreciation_book_code)
	if err != nil {
		return err
	}
	faLedgEntry := gtables.NewFALedgerEntry(p.src)
	faLedgEntry.Copy(entry)
	errorEntryNo := faLedgEntry.Entry_no
	faLedgEntry.Entry_no = p.nextEntryNo
	err = setFAPostingType(faLedgEntry)
	if err != nil {
		return err
	}
	if faLedgEntry.Automatic_entry {
		adjustFALedgEntry(faLedgEntry, fa)
	}
	faLedgEntry.Amount_lcy = amountLCY(faLedgEntry.Amount, faLedgEntry.Fa_exchange_rate)
	glIntegration, err := calcGLIntegration(p.src, faLedgEntry)
	if err != nil {
		return err
	}
	if glIntegration {
		err = p.glAcc.insertFAEntry(p.src, faLedgEntry)
		if err != nil {
			return err
		}
	} else {
		faLedgEntry.G_l_entry_no = 0
	}
	err = faLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	if errorEntryNo > 0 {
		err = cancelErrorEntry(p.src, faLedgEntry, errorEntryNo)
		if err != nil {
			return err
		}
	}
	if faLedgEntry.Fa_posting_category == gtables.FALedgerEntry_Fa_posting_category.Blank &&
		faLedgEntry.Fa_posting_type <= gtables.FALedgerEntry_Fa_posting_type.SalvageValue {
		checked := gtables.NewFALedgerEntry(p.src)
		checked.Copy(faLedgEntry)
		checked.Fa_no = entry.Fa_no
		err = checkConsistencyOfEntry(checked)
		if err != nil {
			return err
		}
	}
	return p.insertRegister(false, p.nextEntryNo)
}

// cancelErrorEntry: the entry a line cancels (FA Error Entry No.) must be its opposite; both
// keep the asset in Canceled from FA No. and leave it (FA No. blank)
func cancelErrorEntry(src ftables.Source, faLedgEntry *gtables.FALedgerEntry, errorEntryNo int) error {
	faNo := faLedgEntry.Fa_no.String()
	deprBookCode := faLedgEntry.Depreciation_book_code.String()
	faLedgEntry2 := gtables.NewFALedgerEntry(src)
	if !faLedgEntry2.Get(errorEntryNo) {
		return apperrors.FAErrorEntryNotFound(faNo, deprBookCode, strconv.Itoa(errorEntryNo))
	}
	if faLedgEntry2.Depreciation_book_code != faLedgEntry.Depreciation_book_code ||
		faLedgEntry2.Fa_no != faLedgEntry.Fa_no ||
		faLedgEntry2.Fa_posting_category != faLedgEntry.Fa_posting_category ||
		faLedgEntry2.Fa_posting_type != faLedgEntry.Fa_posting_type ||
		!faLedgEntry2.Amount.Equal(faLedgEntry.Amount.Neg()) ||
		!faLedgEntry2.Fa_posting_date.Equal(faLedgEntry.Fa_posting_date) {
		return apperrors.FAErrorEntryNoMatch(faNo, deprBookCode, strconv.Itoa(errorEntryNo))
	}
	faLedgEntry.Canceled_from_fa_no = faLedgEntry.Fa_no
	faLedgEntry2.Canceled_from_fa_no = faLedgEntry2.Fa_no
	faLedgEntry2.Fa_no = ""
	faLedgEntry.Fa_no = ""
	err := faLedgEntry2.ModifyOrError(false)
	if err != nil {
		return err
	}
	return faLedgEntry.ModifyOrError(false)
}

// insertMaintenance is codeunit 5600's InsertMaintenance: the maintenance entry numbered in the
// FA register, Amount (LCY), Debit/Credit Amount, inserted; the FA depreciation book's Last
// Maintenance Date
func (p *FAJnlPostLine) insertMaintenance(entry *gtables.MaintenanceLedgerEntry, fa *gtables.FixedAsset, faDeprBook *gtables.FADepreciationBook) error {
	if p.nextMaintenanceEntryNo == 0 {
		last := gtables.NewMaintenanceLedgerEntry(p.src)
		last.LockTable()
		if last.FindLast() {
			p.nextMaintenanceEntryNo = last.Entry_no
		}
		err := p.initRegister(entry.G_l_entry_no, entry.Source_code, entry.Journal_batch_name)
		if err != nil {
			return err
		}
	}
	p.nextMaintenanceEntryNo++
	err := checkMainAsset(p.src, fa, entry.Depreciation_book_code)
	if err != nil {
		return err
	}
	entry.Entry_no = p.nextMaintenanceEntryNo
	if entry.Automatic_entry {
		adjustMaintenanceLedgEntry(entry, fa)
	}
	entry.Amount_lcy = amountLCY(entry.Amount, entry.Fa_exchange_rate)
	entry.Debit_amount, entry.Credit_amount = setDebitCredit(entry.Amount, entry.Correction)
	if entry.G_l_entry_no > 0 {
		err = p.glAcc.insertMaintenanceAccNo(p.src, entry)
		if err != nil {
			return err
		}
	}
	err = entry.InsertOrError(false)
	if err != nil {
		return err
	}
	err = setMaintenanceLastDate(entry, faDeprBook)
	if err != nil {
		return err
	}
	return p.insertRegister(true, p.nextMaintenanceEntryNo)
}

// checkMainAsset (CheckMainAsset): a component's main asset has the book; a main asset is posted
// to only with the FA Setup's Allow Posting to Main Assets
func checkMainAsset(src ftables.Source, fa *gtables.FixedAsset, deprBookCode types.Code) error {
	if fa.Main_asset_component == gtables.FixedAsset_Main_asset_component.Component {
		faDeprBook2 := gtables.NewFADepreciationBook(src)
		err := faDeprBook2.GetOrError(map[string]interface{}{"fa_no": fa.Component_of_main_asset, "depreciation_book_code": deprBookCode})
		if err != nil {
			return err
		}
	}
	faSetup := gtables.GetFASetup(src)
	if faSetup.Allow_posting_to_main_assets {
		return nil
	}
	if fa.Main_asset_component == gtables.FixedAsset_Main_asset_component.MainAsset {
		return apperrors.FAMainAssetPosting(fa.No.String(), deprBookCode.String())
	}
	return nil
}

// setFAPostingType (SetFAPostingType): Debit/Credit Amount; an acquisition cost and a salvage
// value are part of the depreciable basis, an acquisition cost and depreciation part of the book
// value — write-down, appreciation and custom 1/2 as their FA Posting Type Setup says
func setFAPostingType(e *gtables.FALedgerEntry) error {
	e.Debit_amount, e.Credit_amount = setDebitCredit(e.Amount, e.Correction)
	e.Part_of_book_value = false
	e.Part_of_depreciable_basis = false
	if e.Fa_posting_category != gtables.FALedgerEntry_Fa_posting_category.Blank {
		return nil
	}
	switch e.Fa_posting_type {
	case gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost:
		e.Part_of_depreciable_basis = true
		e.Part_of_book_value = true
	case gtables.FALedgerEntry_Fa_posting_type.SalvageValue:
		e.Part_of_depreciable_basis = true
	case gtables.FALedgerEntry_Fa_posting_type.Depreciation:
		e.Part_of_book_value = true
	default:
		setupType, ok := postingTypeSetupType(e.Fa_posting_type)
		if !ok {
			return nil
		}
		err := tables.EnsureFAPostingTypeSetups(e, e.Depreciation_book_code)
		if err != nil {
			return err
		}
		faPostingTypeSetup := gtables.NewFAPostingTypeSetup(e)
		err = faPostingTypeSetup.GetOrError(map[string]interface{}{"depreciation_book_code": e.Depreciation_book_code, "fa_posting_type": int(setupType)})
		if err != nil {
			return err
		}
		e.Part_of_depreciable_basis = faPostingTypeSetup.Part_of_depreciable_basis
		e.Part_of_book_value = faPostingTypeSetup.Part_of_book_value
	}
	return nil
}

// initRegister (InitRegister): the posting's FA register, on its first entry (an FA journal
// posting: Journal Type Fixed Asset)
func (p *FAJnlPostLine) initRegister(glEntryNo int, sourceCode, batchName types.Code) error {
	if p.faReg != nil {
		return nil
	}
	faReg := gtables.NewFARegister(p.src)
	faReg.LockTable()
	no := 1
	if faReg.FindLast() {
		no = faReg.No + 1
	}
	faReg.Init()
	faReg.No = no
	if glEntryNo == 0 {
		faReg.Journal_type = gtables.FARegister_Journal_type.FixedAsset
	}
	faReg.Creation_date = system.Today()
	faReg.Source_code = sourceCode
	faReg.Journal_batch_name = batchName
	faReg.User_id = types.Code(system.UserId())
	err := faReg.InsertOrError(false)
	if err != nil {
		return err
	}
	p.faReg = faReg
	return nil
}

// insertRegister (InsertRegister): the register's entry range grows by the entry
func (p *FAJnlPostLine) insertRegister(maintenance bool, entryNo int) error {
	if maintenance {
		if p.faReg.From_maintenance_entry_no == 0 {
			p.faReg.From_maintenance_entry_no = entryNo
		}
		p.faReg.To_maintenance_entry_no = entryNo
	} else {
		if p.faReg.From_entry_no == 0 {
			p.faReg.From_entry_no = entryNo
		}
		p.faReg.To_entry_no = entryNo
	}
	return p.faReg.ModifyOrError(false)
}

// calcGLIntegration (CalcGLIntegration): an FA entry of a general journal line goes to the G/L —
// not a salvage value, nor the proceeds (Net) or the gain/loss (Gross) of a disposal
func calcGLIntegration(src ftables.Source, entry *gtables.FALedgerEntry) (bool, error) {
	if entry.G_l_entry_no == 0 || entry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.SalvageValue {
		return false, nil
	}
	deprBook := gtables.NewDepreciationBook(src)
	err := deprBook.GetOrError(entry.Depreciation_book_code)
	if err != nil {
		return false, err
	}
	switch deprBook.Disposal_calculation_method {
	case gtables.DepreciationBook_Disposal_calculation_method.Net:
		if entry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal {
			return false, nil
		}
	case gtables.DepreciationBook_Disposal_calculation_method.Gross:
		if entry.Fa_posting_type == gtables.FALedgerEntry_Fa_posting_type.GainLoss {
			return false, nil
		}
	}
	return true, nil
}

// makeFALedgEntryFromGenJnl is codeunit 5604 Make FA Ledger Entry's CopyFromGenJnlLine
func makeFALedgEntryFromGenJnl(src ftables.Source, genJnlLine *gtables.GenJournalLine) *gtables.FALedgerEntry {
	e := gtables.NewFALedgerEntry(src)
	e.Init()
	e.User_id = types.Code(system.UserId())
	e.Entry_no = genJnlLine.Fa_error_entry_no
	e.G_l_entry_no = 1
	e.Depreciation_book_code = genJnlLine.Depreciation_book_code
	e.Fa_no = genJnlLine.Account_no
	e.Fa_posting_date = genJnlLine.Fa_posting_date
	e.Posting_date = genJnlLine.Posting_date
	e.Document_date = genJnlLine.Document_date
	if e.Document_date.IsZero() {
		e.Document_date = e.Posting_date
	}
	e.Document_type = gtables.FALedgerEntryDocument_type(genJnlLine.Document_type)
	e.Document_no = genJnlLine.Document_no
	e.External_document_no = genJnlLine.External_document_no
	e.Description = genJnlLine.Description
	e.Quantity = genJnlLine.Quantity
	e.Correction = genJnlLine.Correction
	e.Index_entry = genJnlLine.Index_entry
	e.Reclassification_entry = genJnlLine.Fa_reclassification_entry
	e.No_of_depreciation_days = genJnlLine.No_of_depreciation_days
	e.Fa_posting_group = genJnlLine.Posting_group
	e.Dimension_1_code = genJnlLine.Dimension_1_code
	e.Dimension_2_code = genJnlLine.Dimension_2_code
	e.Reason_code = genJnlLine.Reason_code
	e.Source_code = genJnlLine.Source_code
	e.Journal_batch_name = genJnlLine.Journal_batch_name
	e.Bal_account_type = gtables.FALedgerEntryBal_account_type(genJnlLine.Bal_account_type)
	e.Bal_account_no = genJnlLine.Bal_account_no
	e.Gen_posting_type = gtables.FALedgerEntryGen_posting_type(genJnlLine.Gen_posting_type)
	e.Gen_bus_posting_group = genJnlLine.Gen_bus_posting_group
	e.Gen_prod_posting_group = genJnlLine.Gen_prod_posting_group
	e.Tax_bus_posting_group = genJnlLine.Tax_bus_posting_group
	e.Tax_prod_posting_group = genJnlLine.Tax_prod_posting_group
	e.Tax_area_code = genJnlLine.Tax_area_code
	e.Tax_liable = genJnlLine.Tax_liable
	e.Tax_group_code = genJnlLine.Tax_group_code
	e.Use_tax = genJnlLine.Use_tax
	e.No_series = genJnlLine.Posting_no_series
	e.Fa_posting_type = tables.FAJnlLineConvertToLedgEntry(gtables.FAJournalLineFa_posting_type(genJnlLine.Fa_posting_type - 1))
	return e
}

// makeMaintenanceLedgEntryFromGenJnl is codeunit 5647 Make Maintenance Ledger Entry's
// CopyFromGenJnlLine
func makeMaintenanceLedgEntryFromGenJnl(src ftables.Source, genJnlLine *gtables.GenJournalLine) *gtables.MaintenanceLedgerEntry {
	e := gtables.NewMaintenanceLedgerEntry(src)
	e.Init()
	e.User_id = types.Code(system.UserId())
	e.Entry_no = genJnlLine.Fa_error_entry_no
	e.G_l_entry_no = 1
	e.Depreciation_book_code = genJnlLine.Depreciation_book_code
	e.Fa_no = genJnlLine.Account_no
	e.Fa_posting_date = genJnlLine.Fa_posting_date
	e.Posting_date = genJnlLine.Posting_date
	e.Document_date = genJnlLine.Document_date
	if e.Document_date.IsZero() {
		e.Document_date = e.Posting_date
	}
	e.Document_type = gtables.MaintenanceLedgerEntryDocument_type(genJnlLine.Document_type)
	e.Document_no = genJnlLine.Document_no
	e.External_document_no = genJnlLine.External_document_no
	e.Description = genJnlLine.Description
	e.Quantity = genJnlLine.Quantity
	e.Correction = genJnlLine.Correction
	e.Index_entry = genJnlLine.Index_entry
	e.Fa_posting_group = genJnlLine.Posting_group
	e.Dimension_1_code = genJnlLine.Dimension_1_code
	e.Dimension_2_code = genJnlLine.Dimension_2_code
	e.Maintenance_code = genJnlLine.Maintenance_code
	e.Reason_code = genJnlLine.Reason_code
	e.Source_code = genJnlLine.Source_code
	e.Journal_batch_name = genJnlLine.Journal_batch_name
	e.Bal_account_type = gtables.MaintenanceLedgerEntryBal_account_type(genJnlLine.Bal_account_type)
	e.Bal_account_no = genJnlLine.Bal_account_no
	e.Gen_posting_type = gtables.MaintenanceLedgerEntryGen_posting_type(genJnlLine.Gen_posting_type)
	e.Gen_bus_posting_group = genJnlLine.Gen_bus_posting_group
	e.Gen_prod_posting_group = genJnlLine.Gen_prod_posting_group
	e.Tax_bus_posting_group = genJnlLine.Tax_bus_posting_group
	e.Tax_prod_posting_group = genJnlLine.Tax_prod_posting_group
	e.Tax_area_code = genJnlLine.Tax_area_code
	e.Tax_liable = genJnlLine.Tax_liable
	e.Tax_group_code = genJnlLine.Tax_group_code
	e.Use_tax = genJnlLine.Use_tax
	e.No_series = genJnlLine.Posting_no_series
	return e
}

// adjustFALedgEntry is codeunit 5607 FA Automatic Entry's AdjustFALedgEntry: an automatic entry
// has no quantity (but a budgeted asset's), balancing account, tax or general posting groups and
// is no reclassification or index entry
func adjustFALedgEntry(e *gtables.FALedgerEntry, fa *gtables.FixedAsset) {
	if !fa.Budgeted_asset {
		e.Quantity = types.Decimal{}
	}
	e.Bal_account_type = gtables.FALedgerEntry_Bal_account_type.GLAccount
	e.Bal_account_no = ""
	e.Tax_amount = types.Decimal{}
	e.Gen_posting_type = gtables.FALedgerEntry_Gen_posting_type.Blank
	e.Gen_bus_posting_group = ""
	e.Gen_prod_posting_group = ""
	e.Reclassification_entry = false
	e.Index_entry = false
}

// adjustMaintenanceLedgEntry is codeunit 5607 FA Automatic Entry's AdjustMaintenanceLedgEntry: an
// automatic maintenance entry has no quantity (but a budgeted asset's), balancing account, tax or
// general posting groups
func adjustMaintenanceLedgEntry(e *gtables.MaintenanceLedgerEntry, fa *gtables.FixedAsset) {
	if !fa.Budgeted_asset {
		e.Quantity = types.Decimal{}
	}
	e.Bal_account_type = gtables.MaintenanceLedgerEntry_Bal_account_type.GLAccount
	e.Bal_account_no = ""
	e.Tax_amount = types.Decimal{}
	e.Gen_posting_type = gtables.MaintenanceLedgerEntry_Gen_posting_type.Blank
	e.Gen_bus_posting_group = ""
	e.Gen_prod_posting_group = ""
	e.Index_entry = false
}

// postDisposalEntry (PostDisposalEntry): the proceeds of a disposal and — the first disposal —
// the reversal of the types set to Reverse before Disposal, the asset's amounts taken out
// (category Disposal), the book value on disposal or the balancing (by the book's Disposal
// Calculation Method) and the gain or loss; a second disposal adds its proceeds to the gain or
// loss; a cancelling line (FA Error Entry No.) cancels the last disposal's entries
func (p *FAJnlPostLine) postDisposalEntry(entry *gtables.FALedgerEntry, fa *gtables.FixedAsset) error {
	faNo, deprBookCode := entry.Fa_no, entry.Depreciation_book_code
	deprBook := gtables.NewDepreciationBook(p.src)
	err := deprBook.GetOrError(deprBookCode)
	if err != nil {
		return err
	}
	errorEntryNo := entry.Entry_no
	entry.Disposal_calculation_method = gtables.FALedgerEntryDisposal_calculation_method(deprBook.Disposal_calculation_method + 1)
	dt, oldMethod, maxDisposalNo, salesEntryNo := getDisposalType(p.src, faNo, deprBookCode, errorEntryNo)
	if maxDisposalNo > 0 && entry.Disposal_calculation_method != oldMethod {
		return apperrors.FADisposalMethodChanged(faNo.String(), deprBookCode.String(), gtables.DepreciationBookTableName, "disposal_calculation_method", deprBook.Disposal_calculation_method)
	}
	if errorEntryNo == 0 {
		entry.Disposal_entry_no = maxDisposalNo + 1
	} else if salesEntryNo != errorEntryNo {
		return apperrors.FADisposalCancelFirst(faNo.String(), deprBookCode.String(), strconv.Itoa(maxDisposalNo))
	}
	if dt == firstDisposal {
		err = p.postReverseType(entry, fa)
		if err != nil {
			return err
		}
	}
	p.glAcc.orgGenJnlLine = true
	err = p.insertFALedgEntry(entry, fa)
	p.glAcc.orgGenJnlLine = false
	if err != nil {
		return err
	}
	insertAmount := func(i int, amount types.Decimal, cancelEntryNo int) error {
		e := gtables.NewFALedgerEntry(p.src)
		e.Copy(entry)
		e.Automatic_entry = true
		e.Entry_no = cancelEntryNo
		e.Fa_posting_category = disposalPostingCategory(i)
		e.Fa_posting_type = disposalPostingType(i)
		e.Amount = amount
		switch i {
		case 0:
			e.Result_on_disposal = gtables.FALedgerEntry_Result_on_disposal.Gain
		case 1:
			e.Result_on_disposal = gtables.FALedgerEntry_Result_on_disposal.Loss
		default:
			e.Result_on_disposal = gtables.FALedgerEntry_Result_on_disposal.Blank
		}
		err := p.insertFALedgEntry(e, fa)
		if err != nil {
			return err
		}
		return p.postAllocation(e, deprBook)
	}
	switch dt {
	case firstDisposal:
		amounts, err := calcGainLoss(p.src, faNo, deprBookCode)
		if err != nil {
			return err
		}
		for i, amount := range amounts {
			if !amount.IsZero() {
				if err = insertAmount(i, amount, 0); err != nil {
					return err
				}
			}
		}
	case secondDisposal:
		amounts, err := calcSecondGainLoss(p.src, faNo, deprBookCode, entry.Amount)
		if err != nil {
			return err
		}
		for i := 0; i < 2; i++ {
			if !amounts[i].IsZero() {
				if err = insertAmount(i, amounts[i], 0); err != nil {
					return err
				}
			}
		}
	default:
		amounts, numbers := getErrorDisposal(p.src, faNo, deprBookCode, dt == errorDisposal)
		last := 14
		if dt == errorDisposal {
			last = 2
		}
		for i := 0; i < last; i++ {
			if numbers[i] != 0 {
				if err = insertAmount(i, amounts[i], numbers[i]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// postReverseType (PostReverseType): the types set to Reverse before Disposal reversed (with G/L
// integration their balancing amounts)
func (p *FAJnlPostLine) postReverseType(entry *gtables.FALedgerEntry, fa *gtables.FixedAsset) error {
	amounts, err := calcReverseAmounts(p.src, entry.Fa_no, entry.Depreciation_book_code)
	if err != nil {
		return err
	}
	for i, amount := range amounts {
		if amount.IsZero() {
			continue
		}
		e := gtables.NewFALedgerEntry(p.src)
		e.Copy(entry)
		e.Entry_no = 0
		e.Fa_posting_category = gtables.FALedgerEntry_Fa_posting_category.Blank
		e.Automatic_entry = true
		e.Amount = amount
		e.Fa_posting_type = reverseType(i)
		err = p.insertFALedgEntry(e, fa)
		if err != nil {
			return err
		}
		if e.G_l_entry_no > 0 {
			err = p.glAcc.insertBalAcc(p.src, e)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// postAllocation (PostAllocation): with G/L integration, a disposal's gain/loss (method Net) and
// book value on disposal allocated by the posting group's Allocated Gain / Loss / Book Value %
// (PostGLBalAcc)
func (p *FAJnlPostLine) postAllocation(entry *gtables.FALedgerEntry, deprBook *gtables.DepreciationBook) error {
	if entry.G_l_entry_no == 0 {
		return nil
	}
	faPostingGr := gtables.NewFAPostingGroup(p.src)
	err := faPostingGr.GetOrError(entry.Fa_posting_group)
	if err != nil {
		return err
	}
	faPostingGr.CalcFields("allocated_gain_percent", "allocated_loss_percent", "allocated_book_value_percent")
	switch entry.Fa_posting_type {
	case gtables.FALedgerEntry_Fa_posting_type.GainLoss:
		if deprBook.Disposal_calculation_method == gtables.DepreciationBook_Disposal_calculation_method.Net {
			if entry.Result_on_disposal == gtables.FALedgerEntry_Result_on_disposal.Gain {
				return p.postGLBalAcc(entry, faPostingGr.Allocated_gain_percent)
			}
			return p.postGLBalAcc(entry, faPostingGr.Allocated_loss_percent)
		}
	case gtables.FALedgerEntry_Fa_posting_type.BookValueOnDisposal:
		return p.postGLBalAcc(entry, faPostingGr.Allocated_book_value_percent)
	}
	return nil
}

// postGLBalAcc (PostGLBalAcc): with an allocation, the entry's amount moved off its account
// (the other way) and onto the allocation's accounts
func (p *FAJnlPostLine) postGLBalAcc(entry *gtables.FALedgerEntry, allocatedPct types.Decimal) error {
	if !allocatedPct.IsPositive() {
		return nil
	}
	e := gtables.NewFALedgerEntry(p.src)
	e.Copy(entry)
	e.Entry_no = 0
	e.Automatic_entry = true
	e.Amount = entry.Amount.Neg()
	e.Correction = !entry.Correction
	err := p.glAcc.insertFAEntry(p.src, e)
	if err != nil {
		return err
	}
	e.Correction = !e.Correction
	return p.glAcc.insertBalAcc(p.src, e)
}

// setMaintenanceLastDate (SetMaintenanceLastDate): the FA depreciation book's Last Maintenance
// Date — the last maintenance entry's FA posting date
func setMaintenanceLastDate(entry *gtables.MaintenanceLedgerEntry, faDeprBook *gtables.FADepreciationBook) error {
	maintenanceLedgEntry := gtables.NewMaintenanceLedgerEntry(entry)
	maintenanceLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	maintenanceLedgEntry.SetRange("fa_no", entry.Fa_no)
	maintenanceLedgEntry.SetRange("depreciation_book_code", entry.Depreciation_book_code)
	faDeprBook.Last_maintenance_date = types.Date{}
	if maintenanceLedgEntry.FindLast() {
		faDeprBook.Last_maintenance_date = maintenanceLedgEntry.Fa_posting_date
	}
	return faDeprBook.ModifyOrError(false)
}

// SetMaintenanceLastDate (FA Insert Ledger Entry SetMaintenanceLastDate, from outside the
// posting): the asset's Last Maintenance Date in the book again — after a date compression
func SetMaintenanceLastDate(entry *gtables.MaintenanceLedgerEntry) error {
	faDeprBook := gtables.NewFADepreciationBook(entry)
	err := faDeprBook.GetOrError(map[string]interface{}{"fa_no": entry.Fa_no, "depreciation_book_code": entry.Depreciation_book_code})
	if err != nil {
		return err
	}
	return setMaintenanceLastDate(entry, faDeprBook)
}
