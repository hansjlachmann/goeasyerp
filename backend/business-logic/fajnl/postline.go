package fajnl

import (
	"strconv"

	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// FAJnlPostLine is codeunit 5632 FA Jnl.-Post Line with the codeunits it runs: 5604/5647 Make
// FA/Maintenance Ledger Entry (the entry from the journal line and the asset's card and
// depreciation book), 5600 FA Insert Ledger Entry (the entry numbered, Part of Book Value / Part
// of Depreciable Basis by posting type, Amount (LCY), the FA register, a cancelled entry (FA Error
// Entry No.) and the entry that cancels it both taken off the asset) and 5606 FA Check
// Consistency. An acquisition cost with a salvage value posts the salvage value too. One
// FAJnlPostLine posts the lines of one posting: they share its FA register.
//
// Not migrated yet: the general journal's fixed asset lines (FA2 — the G/L accounts of codeunit
// 5601), automatic depreciation (Depr. until FA Posting Date, Depr. Acquisition Cost — FA3),
// disposal (FA4), duplication in other books (FA5), budgeted assets.
type FAJnlPostLine struct {
	src                    ftables.Source
	faJnlCheckLine         FAJnlCheckLine
	nextEntryNo            int
	nextMaintenanceEntryNo int
	faReg                  *gtables.FARegister
}

// NewFAJnlPostLine opens the line posting in the posting's database (its transaction)
func NewFAJnlPostLine(src ftables.Source) *FAJnlPostLine {
	return &FAJnlPostLine{src: src}
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
	if faJnlLine.Fa_posting_type == gtables.FAJournalLine_Fa_posting_type.Maintenance {
		maintenanceLedgEntry := makeMaintenanceLedgEntry(p.src, faJnlLine)
		return p.maintenanceCode(maintenanceLedgEntry)
	}
	faLedgEntry := makeFALedgEntry(p.src, faJnlLine)
	return p.code(faLedgEntry, faJnlLine.Salvage_value)
}

// code (Code): the asset may be posted to; the entry (and its salvage value) inserted
func (p *FAJnlPostLine) code(faLedgEntry *gtables.FALedgerEntry, salvageValue types.Decimal) error {
	fa, faDeprBook, err := p.getFA(faLedgEntry.Fa_no, faLedgEntry.Depreciation_book_code)
	if err != nil {
		return err
	}
	copyFromFACard(faLedgEntry, fa, faDeprBook)
	err = p.insertFALedgEntry(faLedgEntry, fa)
	if err != nil {
		return err
	}
	// PostSalvageValue: an acquisition's salvage value
	if salvageValue.IsZero() || faLedgEntry.Fa_posting_type != gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost {
		return nil
	}
	faLedgEntry.Entry_no = 0
	faLedgEntry.Automatic_entry = true
	faLedgEntry.Amount = salvageValue
	faLedgEntry.Fa_posting_type = gtables.FALedgerEntry_Fa_posting_type.SalvageValue
	return p.insertFALedgEntry(faLedgEntry, fa)
}

// maintenanceCode (MaintenanceCode): the maintenance entry inserted
func (p *FAJnlPostLine) maintenanceCode(maintenanceLedgEntry *gtables.MaintenanceLedgerEntry) error {
	fa, faDeprBook, err := p.getFA(maintenanceLedgEntry.Fa_no, maintenanceLedgEntry.Depreciation_book_code)
	if err != nil {
		return err
	}
	maintenanceLedgEntry.Fa_class_code = fa.Fa_class_code
	maintenanceLedgEntry.Fa_subclass_code = fa.Fa_subclass_code
	maintenanceLedgEntry.Fa_location_code = fa.Fa_location_code
	maintenanceLedgEntry.Location_code = fa.Location_code
	maintenanceLedgEntry.Fa_exchange_rate = getExchangeRate(faDeprBook)
	return p.insertMaintenance(maintenanceLedgEntry, fa, faDeprBook)
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
	e.Department_code = faJnlLine.Department_code
	e.Project_code = faJnlLine.Project_code
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
	e.Department_code = faJnlLine.Department_code
	e.Project_code = faJnlLine.Project_code
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
	faLedgEntry.Amount_lcy = amountLCY(faLedgEntry.Amount, faLedgEntry.Fa_exchange_rate)
	// G/L integration (FA Insert G/L Account) comes with the general journal's fixed asset lines
	faLedgEntry.G_l_entry_no = 0
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
	entry.Amount_lcy = amountLCY(entry.Amount, entry.Fa_exchange_rate)
	entry.Debit_amount, entry.Credit_amount = setDebitCredit(entry.Amount, entry.Correction)
	err = entry.InsertOrError(false)
	if err != nil {
		return err
	}
	// SetMaintenanceLastDate
	maintenanceLedgEntry := gtables.NewMaintenanceLedgerEntry(p.src)
	maintenanceLedgEntry.SetCurrentKey("fa_no", "depreciation_book_code", "fa_posting_date")
	maintenanceLedgEntry.SetRange("fa_no", entry.Fa_no)
	maintenanceLedgEntry.SetRange("depreciation_book_code", entry.Depreciation_book_code)
	faDeprBook.Last_maintenance_date = types.Date{}
	if maintenanceLedgEntry.FindLast() {
		faDeprBook.Last_maintenance_date = maintenanceLedgEntry.Fa_posting_date
	}
	err = faDeprBook.ModifyOrError(false)
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
