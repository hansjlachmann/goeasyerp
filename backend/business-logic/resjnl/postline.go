package resjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

var unitPrecision = types.MustDecimal("0.00001")

// ResJnlPostLine is codeunit 212 Res. Jnl.-Post Line: it posts a resource journal line as a
// resource ledger entry (a sale with the opposite sign) and keeps the Resource Register. Open it
// on the posting's transaction; one instance per posting keeps the entry numbers and the
// register.
type ResJnlPostLine struct {
	src ftables.Source

	resJnlLine      *gtables.ResJournalLine
	resReg          *gtables.ResourceRegister
	genPostingSetup *gtables.GeneralPostingSetup
	resJnlCheckLine ResJnlCheckLine
	nextEntryNo     int
}

// NewResJnlPostLine opens the posting codeunit in the posting's database (its transaction)
func NewResJnlPostLine(src ftables.Source) *ResJnlPostLine {
	return &ResJnlPostLine{src: src}
}

// Run checks and posts a journal line (OnRun)
func (p *ResJnlPostLine) Run(resJnlLine *gtables.ResJournalLine) error {
	p.resJnlLine = gtables.NewResJournalLine(p.src)
	p.resJnlLine.Copy(resJnlLine)
	return p.code()
}

// GetResReg returns the Resource Register of the posting (No. 0 before the first entry)
func (p *ResJnlPostLine) GetResReg() *gtables.ResourceRegister {
	if p.resReg == nil {
		return gtables.NewResourceRegister(p.src)
	}
	return p.resReg
}

func (p *ResJnlPostLine) code() error {
	resJnlLine := p.resJnlLine
	if tables.ResJnlLineEmptyLine(resJnlLine) {
		return nil
	}

	err := p.resJnlCheckLine.RunCheck(resJnlLine)
	if err != nil {
		return err
	}

	if p.nextEntryNo == 0 {
		resLedgEntry := gtables.NewResLedgerEntry(p.src)
		resLedgEntry.LockTable()
		if resLedgEntry.FindLast() {
			p.nextEntryNo = resLedgEntry.Entry_no
		}
		p.nextEntryNo = p.nextEntryNo + 1
	}

	if resJnlLine.Document_date.IsZero() {
		resJnlLine.Document_date = resJnlLine.Posting_date
	}

	if p.resReg == nil || p.resReg.No == 0 {
		p.resReg = gtables.NewResourceRegister(p.src)
		p.resReg.LockTable()
		if !p.resReg.FindLast() || p.resReg.To_entry_no != 0 {
			no := p.resReg.No
			p.resReg.Init()
			p.resReg.No = no + 1
			p.resReg.From_entry_no = p.nextEntryNo
			p.resReg.To_entry_no = p.nextEntryNo
			p.resReg.Creation_date = system.Today()
			p.resReg.Source_code = resJnlLine.Source_code
			p.resReg.Journal_batch_name = resJnlLine.Journal_batch_name
			p.resReg.User_id = types.Code(system.UserId())
			err = p.resReg.InsertOrError(false)
			if err != nil {
				return err
			}
		}
	}
	p.resReg.To_entry_no = p.nextEntryNo
	err = p.resReg.ModifyOrError(false)
	if err != nil {
		return err
	}

	res := gtables.NewResource(p.src)
	err = res.GetOrError(resJnlLine.Resource_no)
	if err != nil {
		return err
	}
	err = res.TestField("blocked", false)
	if err != nil {
		return err
	}

	if p.genPostingSetup == nil ||
		p.genPostingSetup.Gen_bus_posting_group != resJnlLine.Gen_bus_posting_group ||
		p.genPostingSetup.Gen_prod_posting_group != resJnlLine.Gen_prod_posting_group {
		p.genPostingSetup = gtables.NewGeneralPostingSetup(p.src)
		err = p.genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": resJnlLine.Gen_bus_posting_group, "gen_prod_posting_group": resJnlLine.Gen_prod_posting_group})
		if err != nil {
			return err
		}
	}

	resJnlLine.Resource_group_no = res.Resource_group_no

	resLedgEntry := gtables.NewResLedgerEntry(p.src)
	resLedgEntry.Init()
	resLedgEntry.Entry_type = gtables.ResLedgerEntryEntry_type(resJnlLine.Entry_type)
	resLedgEntry.Document_no = resJnlLine.Document_no
	resLedgEntry.External_document_no = resJnlLine.External_document_no
	resLedgEntry.Posting_date = resJnlLine.Posting_date
	resLedgEntry.Document_date = resJnlLine.Document_date
	resLedgEntry.Resource_no = resJnlLine.Resource_no
	resLedgEntry.Resource_group_no = resJnlLine.Resource_group_no
	resLedgEntry.Description = resJnlLine.Description
	resLedgEntry.Work_type_code = resJnlLine.Work_type_code
	resLedgEntry.Project_no = resJnlLine.Project_no
	resLedgEntry.Unit_of_measure_code = resJnlLine.Unit_of_measure_code
	resLedgEntry.Quantity = resJnlLine.Quantity
	resLedgEntry.Direct_unit_cost = resJnlLine.Direct_unit_cost
	resLedgEntry.Unit_cost = resJnlLine.Unit_cost
	resLedgEntry.Total_cost = resJnlLine.Total_cost
	resLedgEntry.Unit_price = resJnlLine.Unit_price
	resLedgEntry.Total_price = resJnlLine.Total_price
	resLedgEntry.Dimension_1_code = resJnlLine.Dimension_1_code
	resLedgEntry.Dimension_2_code = resJnlLine.Dimension_2_code
	resLedgEntry.Source_code = resJnlLine.Source_code
	resLedgEntry.Chargeable = resJnlLine.Chargeable
	resLedgEntry.Journal_batch_name = resJnlLine.Journal_batch_name
	resLedgEntry.Reason_code = resJnlLine.Reason_code
	resLedgEntry.Gen_bus_posting_group = resJnlLine.Gen_bus_posting_group
	resLedgEntry.Gen_prod_posting_group = resJnlLine.Gen_prod_posting_group
	resLedgEntry.No_series = resJnlLine.Posting_no_series

	resLedgEntry.Total_cost = system.Round(resLedgEntry.Total_cost)
	resLedgEntry.Total_price = system.Round(resLedgEntry.Total_price)
	if resLedgEntry.Entry_type == gtables.ResLedgerEntry_Entry_type.Sale {
		resLedgEntry.Chargeable = true
		resLedgEntry.Quantity = resLedgEntry.Quantity.Neg()
		resLedgEntry.Total_cost = resLedgEntry.Total_cost.Neg()
		resLedgEntry.Total_price = resLedgEntry.Total_price.Neg()
	}
	if !resLedgEntry.Quantity.IsZero() {
		resLedgEntry.Unit_cost = resLedgEntry.Total_cost.Div(resLedgEntry.Quantity)
		resLedgEntry.Unit_price = resLedgEntry.Total_price.Div(resLedgEntry.Quantity)
	}
	resLedgEntry.Direct_unit_cost = system.Round(resLedgEntry.Direct_unit_cost, unitPrecision)
	resLedgEntry.Unit_cost = system.Round(resLedgEntry.Unit_cost, unitPrecision)
	resLedgEntry.Unit_price = system.Round(resLedgEntry.Unit_price, unitPrecision)
	if resLedgEntry.Description == res.Name {
		resLedgEntry.Description = ""
	}
	resLedgEntry.User_id = types.Code(system.UserId())
	resLedgEntry.Entry_no = p.nextEntryNo
	err = resLedgEntry.InsertOrError(false)
	if err != nil {
		return err
	}
	p.nextEntryNo = p.nextEntryNo + 1
	return nil
}
