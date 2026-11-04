// Package insurancejnl is the insurance journal: codeunits 5651 Insurance Jnl.-Check Line, 5652
// Insurance Jnl.-Post Line with 5657 Make Ins. Coverage Ledg. Entry, 5653 Insurance Jnl.-Post
// Batch and 5656 InsuranceJnlManagement. A posted line is an Ins. Coverage Ledger Entry — the
// asset's value covered by the insurance policy — in an Insurance Register.
package insurancejnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// InsuranceJnlCheckLine is codeunit 5651 Insurance Jnl.-Check Line (the FA Setup's Insurance
// Depr. Book is checked on the first call)
type InsuranceJnlCheckLine struct {
	callNo int
}

// Run checks a line: insurance, document no., posting date and fixed asset
func (c *InsuranceJnlCheckLine) Run(rec *gtables.InsuranceJournalLine) error {
	if rec.Insurance_no == "" {
		return nil
	}
	for _, field := range []string{"insurance_no", "document_no", "posting_date", "fa_no"} {
		err := rec.TestField(field)
		if err != nil {
			return err
		}
	}
	if c.callNo == 0 {
		faSetup := gtables.GetFASetup(rec)
		err := faSetup.TestField("insurance_depr_book")
		if err != nil {
			return err
		}
	}
	c.callNo = 1
	return nil
}

// InsuranceJnlPostLine is codeunit 5652 Insurance Jnl.-Post Line: one instance per posting (the
// register and the next entry no. carry over from line to line). Open it on the posting's
// transaction.
type InsuranceJnlPostLine struct {
	src                   ftables.Source
	insuranceJnlCheckLine InsuranceJnlCheckLine
	insuranceReg          *gtables.InsuranceRegister
	nextEntryNo           int
}

// NewInsuranceJnlPostLine opens the line posting in the posting's database (its transaction)
func NewInsuranceJnlPostLine(src ftables.Source) *InsuranceJnlPostLine {
	return &InsuranceJnlPostLine{src: src}
}

// Run checks and posts a line (OnRun)
func (p *InsuranceJnlPostLine) Run(insuranceJnlLine *gtables.InsuranceJournalLine) error {
	return p.code(insuranceJnlLine, true)
}

// RunWithOutCheck posts a line checked before (the batch posting)
func (p *InsuranceJnlPostLine) RunWithOutCheck(insuranceJnlLine *gtables.InsuranceJournalLine) error {
	return p.code(insuranceJnlLine, false)
}

func (p *InsuranceJnlPostLine) code(insuranceJnlLine *gtables.InsuranceJournalLine, checkLine bool) error {
	if insuranceJnlLine.Insurance_no == "" {
		return nil
	}
	if checkLine {
		err := p.insuranceJnlCheckLine.Run(insuranceJnlLine)
		if err != nil {
			return err
		}
	}
	insurance := gtables.NewInsurance(p.src)
	err := insurance.GetOrError(insuranceJnlLine.Insurance_no)
	if err != nil {
		return err
	}
	err = insurance.TestField("blocked", false)
	if err != nil {
		return err
	}
	fa := gtables.NewFixedAsset(p.src)
	err = fa.GetOrError(insuranceJnlLine.Fa_no)
	if err != nil {
		return err
	}
	for _, field := range []string{"budgeted_asset", "blocked", "inactive"} {
		err = fa.TestField(field, false)
		if err != nil {
			return err
		}
	}
	insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(p.src)
	err = copyFromJnlLine(insCoverageLedgEntry, insuranceJnlLine)
	if err != nil {
		return err
	}
	copyFromInsuranceCard(insCoverageLedgEntry, insurance)

	if p.nextEntryNo == 0 {
		insCoverageLedgEntry.LockTable()
		insCoverageLedgEntry2 := gtables.NewInsCoverageLedgerEntry(p.src)
		if insCoverageLedgEntry2.FindLast() {
			p.nextEntryNo = insCoverageLedgEntry2.Entry_no
		}
		p.insuranceReg = gtables.NewInsuranceRegister(p.src)
		p.insuranceReg.LockTable()
		no := 1
		if p.insuranceReg.FindLast() {
			no = p.insuranceReg.No + 1
		}
		p.insuranceReg.Init()
		p.insuranceReg.No = no
		p.insuranceReg.From_entry_no = p.nextEntryNo + 1
		p.insuranceReg.Creation_date = system.Today()
		p.insuranceReg.Source_code = insuranceJnlLine.Source_code
		p.insuranceReg.Journal_batch_name = insuranceJnlLine.Journal_batch_name
		p.insuranceReg.User_id = types.Code(system.UserId())
	}
	p.nextEntryNo = p.nextEntryNo + 1
	insCoverageLedgEntry.Entry_no = p.nextEntryNo
	err = insCoverageLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	if p.insuranceReg.To_entry_no == 0 {
		p.insuranceReg.To_entry_no = p.nextEntryNo
		return p.insuranceReg.InsertOrError(false)
	}
	p.insuranceReg.To_entry_no = p.nextEntryNo
	return p.insuranceReg.ModifyOrError(false)
}

// copyFromJnlLine (codeunit 5657 CopyFromJnlLine): the entry from the journal line
func copyFromJnlLine(insCoverageLedgEntry *gtables.InsCoverageLedgerEntry, insuranceJnlLine *gtables.InsuranceJournalLine) error {
	insCoverageLedgEntry.User_id = types.Code(system.UserId())
	insCoverageLedgEntry.Insurance_no = insuranceJnlLine.Insurance_no
	insCoverageLedgEntry.Fa_no = insuranceJnlLine.Fa_no
	insCoverageLedgEntry.Fa_description = insuranceJnlLine.Fa_description
	insCoverageLedgEntry.Posting_date = insuranceJnlLine.Posting_date
	insCoverageLedgEntry.Document_type = gtables.InsCoverageLedgerEntryDocument_type(insuranceJnlLine.Document_type)
	insCoverageLedgEntry.Document_date = insuranceJnlLine.Document_date
	if insCoverageLedgEntry.Document_date.IsZero() {
		insCoverageLedgEntry.Document_date = insCoverageLedgEntry.Posting_date
	}
	insCoverageLedgEntry.Document_no = insuranceJnlLine.Document_no
	insCoverageLedgEntry.External_document_no = insuranceJnlLine.External_document_no
	insCoverageLedgEntry.Amount = insuranceJnlLine.Amount
	insCoverageLedgEntry.Description = insuranceJnlLine.Description
	insCoverageLedgEntry.Index_entry = insuranceJnlLine.Index_entry
	insCoverageLedgEntry.Dimension_1_code = insuranceJnlLine.Dimension_1_code
	insCoverageLedgEntry.Dimension_2_code = insuranceJnlLine.Dimension_2_code
	insCoverageLedgEntry.Source_code = insuranceJnlLine.Source_code
	insCoverageLedgEntry.Journal_batch_name = insuranceJnlLine.Journal_batch_name
	insCoverageLedgEntry.Reason_code = insuranceJnlLine.Reason_code
	disposedFA, err := setDisposedFA(insCoverageLedgEntry, insCoverageLedgEntry.Fa_no)
	if err != nil {
		return err
	}
	insCoverageLedgEntry.Disposed_fa = disposedFA
	insCoverageLedgEntry.No_series = insuranceJnlLine.Posting_no_series
	return nil
}

// copyFromInsuranceCard (CopyFromInsuranceCard): the insurance's class, subclass and locations
func copyFromInsuranceCard(insCoverageLedgEntry *gtables.InsCoverageLedgerEntry, insurance *gtables.Insurance) {
	insCoverageLedgEntry.Fa_class_code = insurance.Fa_class_code
	insCoverageLedgEntry.Fa_subclass_code = insurance.Fa_subclass_code
	insCoverageLedgEntry.Fa_location_code = insurance.Fa_location_code
	insCoverageLedgEntry.Location_code = insurance.Location_code
}

// setDisposedFA (SetDisposedFA): whether the asset is disposed in the FA Setup's insurance book
func setDisposedFA(src ftables.Source, faNo types.Code) (bool, error) {
	faSetup := gtables.GetFASetup(src)
	err := faSetup.TestField("insurance_depr_book")
	if err != nil {
		return false, err
	}
	faDeprBook := gtables.NewFADepreciationBook(src)
	if faDeprBook.Get(map[string]interface{}{"fa_no": faNo, "depreciation_book_code": faSetup.Insurance_depr_book}) {
		return !faDeprBook.Disposal_date.IsZero(), nil
	}
	return false, nil
}
