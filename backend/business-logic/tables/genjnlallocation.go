package tables

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// genJnlAllocationLogic is the business logic of table 221 Gen. Jnl. Allocation: the G/L
// accounts a recurring journal line's amount is allocated to — by Allocation Quantity (shares
// of the total quantity), Allocation % or a fixed Amount. The allocated amounts carry the
// opposite sign of the line, so the line and its allocations balance; the line's Balance (LCY)
// includes them (Allocated Amt. (LCY)).
//
// Not migrated yet: the additional reporting currency (UpdateAllocationsAddCurr).
type genJnlAllocationLogic struct{}

func init() { gtables.RegisterGenJnlAllocationLogic(genJnlAllocationLogic{}) }

const genJnlAllocTable = gtables.GenJnlAllocationTableName

// genJnlLineKey is the key of a Gen. Journal Line (template, batch, line no.)
func genJnlLineKey(template, batch types.Code, lineNo int) map[string]interface{} {
	return map[string]interface{}{"journal_template_name": template, "journal_batch_name": batch, "line_no": lineNo}
}

// getAllocGenJnlLine: the journal line of the allocation
func getAllocGenJnlLine(rec *gtables.GenJnlAllocation) (*gtables.GenJournalLine, error) {
	genJnlLine := gtables.NewGenJournalLine(rec)
	err := genJnlLine.GetOrError(genJnlLineKey(rec.Journal_template_name, rec.Journal_batch_name, rec.Journal_line_no))
	if err != nil {
		return nil, err
	}
	return genJnlLine, nil
}

// OnInsert: the allocation's journal line exists
func (genJnlAllocationLogic) OnInsert(rec *gtables.GenJnlAllocation) error {
	rec.LockTable()
	_, err := getAllocGenJnlLine(rec)
	return err
}

// OnDelete: the line's balance without the allocation (VALIDATE(Amount,0))
func (genJnlAllocationLogic) OnDelete(rec *gtables.GenJnlAllocation) error {
	return rec.Validate("amount", types.Decimal{})
}

// OnValidate_Account_no: a posting account with direct posting; its department, project and
// posting groups follow (Account Name is a FlowField)
func (genJnlAllocationLogic) OnValidate_Account_no(rec *gtables.GenJnlAllocation) error {
	glAcc := gtables.NewGLAccount(rec)
	if rec.Account_no == "" {
		glAcc.Init()
	} else {
		err := glAcc.GetOrError(rec.Account_no)
		if err != nil {
			return err
		}
		err = CheckGLAcc(glAcc)
		if err != nil {
			return err
		}
		err = glAcc.TestField("direct_posting", true)
		if err != nil {
			return err
		}
	}
	rec.Department_code = glAcc.Department_code
	rec.Project_code = glAcc.Project_code
	rec.Gen_posting_type = gtables.GenJnlAllocationGen_posting_type(glAcc.Gen_posting_type)
	rec.Gen_bus_posting_group = glAcc.Gen_bus_posting_group
	rec.Gen_prod_posting_group = glAcc.Gen_prod_posting_group
	rec.Tax_bus_posting_group = glAcc.Tax_bus_posting_group
	rec.Tax_prod_posting_group = glAcc.Tax_prod_posting_group
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

// OnValidate_Allocation_quantity: no quantity — no percentage or amount; the line's
// allocations are calculated again
func (genJnlAllocationLogic) OnValidate_Allocation_quantity(rec *gtables.GenJnlAllocation) error {
	if rec.Allocation_quantity.IsZero() {
		rec.Allocation_percent = types.Decimal{}
		rec.Amount = types.Decimal{}
	}
	return updateAllocationsFrom(rec)
}

// OnValidate_Allocation_percent: a percentage replaces the quantity; the allocations are
// calculated again
func (genJnlAllocationLogic) OnValidate_Allocation_percent(rec *gtables.GenJnlAllocation) error {
	rec.Allocation_quantity = types.Decimal{}
	if rec.Allocation_percent.IsZero() {
		rec.Amount = types.Decimal{}
	}
	return updateAllocationsFrom(rec)
}

// OnValidate_Amount: an amount replaces the quantity and percentage; the line's balance
// follows
func (genJnlAllocationLogic) OnValidate_Amount(rec *gtables.GenJnlAllocation) error {
	if !rec.Allocation_quantity.IsZero() || !rec.Allocation_percent.IsZero() {
		rec.Allocation_quantity = types.Decimal{}
		rec.Allocation_percent = types.Decimal{}
		return updateAllocationsFrom(rec)
	}
	rec.Modify(false) // a new allocation is not saved yet
	genJnlLine, err := getAllocGenJnlLine(rec)
	if err != nil {
		return err
	}
	return updateJnlBalance(genJnlLine, true)
}

func (genJnlAllocationLogic) OnValidate_Gen_posting_type(rec *gtables.GenJnlAllocation) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

// OnValidate_Gen_bus_posting_group: the group's default tax business posting group follows
func (genJnlAllocationLogic) OnValidate_Gen_bus_posting_group(rec *gtables.GenJnlAllocation) error {
	if rec.XRec().Gen_bus_posting_group != rec.Gen_bus_posting_group {
		genBusPostingGroup := gtables.NewGenBusinessPostingGroup(rec)
		autoInsert, err := ValidateTaxBusPostingGroup(genBusPostingGroup, rec.Gen_bus_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			return rec.Validate("tax_bus_posting_group", genBusPostingGroup.Def_tax_bus_posting_group)
		}
	}
	return nil
}

// OnValidate_Gen_prod_posting_group: the group's default tax product posting group follows
func (genJnlAllocationLogic) OnValidate_Gen_prod_posting_group(rec *gtables.GenJnlAllocation) error {
	if rec.XRec().Gen_prod_posting_group != rec.Gen_prod_posting_group {
		genProdPostingGroup := gtables.NewGenProductPostingGroup(rec)
		autoInsert, err := ValidateTaxProdPostingGroup(genProdPostingGroup, rec.Gen_prod_posting_group)
		if err != nil {
			return err
		}
		if autoInsert {
			return rec.Validate("tax_prod_posting_group", genProdPostingGroup.Def_tax_prod_posting_group)
		}
	}
	return nil
}

func (genJnlAllocationLogic) OnValidate_Tax_area_code(rec *gtables.GenJnlAllocation) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

func (genJnlAllocationLogic) OnValidate_Tax_liable(rec *gtables.GenJnlAllocation) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

func (genJnlAllocationLogic) OnValidate_Tax_group_code(rec *gtables.GenJnlAllocation) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

func (genJnlAllocationLogic) OnValidate_Use_tax(rec *gtables.GenJnlAllocation) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

func (genJnlAllocationLogic) OnValidate_Tax_bus_posting_group(rec *gtables.GenJnlAllocation) error {
	return rec.Validate("tax_prod_posting_group", rec.Tax_prod_posting_group)
}

// OnValidate_Tax_prod_posting_group: the allocation's tax as a journal line would calculate it
func (genJnlAllocationLogic) OnValidate_Tax_prod_posting_group(rec *gtables.GenJnlAllocation) error {
	genJnlLine, err := getAllocGenJnlLine(rec)
	if err != nil {
		return err
	}
	err = CheckGenJnlAllocTax(rec, genJnlLine)
	if err != nil {
		return err
	}
	return updateGenJnlAllocTax(rec, genJnlLine)
}

// CheckGenJnlAllocTax: an allocation cannot have a general posting type when its journal line
// has one (CheckTax)
func CheckGenJnlAllocTax(rec *gtables.GenJnlAllocation, genJnlLine *gtables.GenJournalLine) error {
	if rec.Gen_posting_type != gtables.GenJnlAllocation_Gen_posting_type.Blank &&
		genJnlLine.Gen_posting_type != gtables.GenJournalLine_Gen_posting_type.Blank {
		return apperrors.JnlAllocGenPostingType(gtables.GenJournalLineTableName, "gen_posting_type")
	}
	return nil
}

// updateGenJnlAllocTax: the allocation's tax calculation type, tax % and tax amount — a journal
// line with its account, amount and posting groups validated (UpdateTax)
func updateGenJnlAllocTax(rec *gtables.GenJnlAllocation, genJnlLine *gtables.GenJournalLine) error {
	genJnlLine2 := gtables.NewGenJournalLine(rec)
	genJnlLine2.Account_no = rec.Account_no
	genJnlLine2.Amount = rec.Amount
	genJnlLine2.Amount_lcy = rec.Amount
	genJnlLine2.Department_code = rec.Department_code
	genJnlLine2.Project_code = rec.Project_code
	genJnlLine2.Gen_posting_type = gtables.GenJournalLineGen_posting_type(rec.Gen_posting_type)
	genJnlLine2.Gen_bus_posting_group = rec.Gen_bus_posting_group
	genJnlLine2.Gen_prod_posting_group = rec.Gen_prod_posting_group
	genJnlLine2.Tax_bus_posting_group = rec.Tax_bus_posting_group
	genJnlLine2.Tax_prod_posting_group = rec.Tax_prod_posting_group
	genJnlLine2.Tax_area_code = rec.Tax_area_code
	genJnlLine2.Tax_liable = rec.Tax_liable
	genJnlLine2.Tax_group_code = rec.Tax_group_code
	genJnlLine2.Use_tax = rec.Use_tax
	genJnlLine2.Tax_calculation_type = gtables.GenJournalLineTax_calculation_type(rec.Tax_calculation_type)
	genJnlLine2.Tax_amount = rec.Tax_amount
	genJnlLine2.Tax_percent = rec.Tax_percent
	genJnlLine2.Posting_date = genJnlLine.Posting_date
	err := genJnlLine2.Validate("tax_prod_posting_group", genJnlLine2.Tax_prod_posting_group)
	if err != nil {
		return err
	}
	rec.Amount = genJnlLine2.Amount_lcy
	rec.Tax_calculation_type = gtables.GenJnlAllocationTax_calculation_type(genJnlLine2.Tax_calculation_type)
	rec.Tax_amount = genJnlLine2.Tax_amount
	rec.Tax_percent = genJnlLine2.Tax_percent
	return nil
}

// updateAllocationsFrom: an allocation changed — saved with its tax, then the line's
// allocations calculated again (UpdateAllocations from the allocation)
func updateAllocationsFrom(rec *gtables.GenJnlAllocation) error {
	genJnlLine, err := getAllocGenJnlLine(rec)
	if err != nil {
		return err
	}
	err = updateGenJnlAllocTax(rec, genJnlLine)
	if err != nil {
		return err
	}
	rec.Modify(false) // a new allocation is not saved yet
	err = CheckGenJnlAllocTax(rec, genJnlLine)
	if err != nil {
		return err
	}
	err = updateGenJnlAllocations(genJnlLine, true)
	if err != nil {
		return err
	}
	saved := gtables.NewGenJnlAllocation(rec)
	if saved.GetByPK(rec.Journal_template_name, rec.Journal_batch_name, rec.Journal_line_no, rec.Line_no) {
		rec.Allocation_percent = saved.Allocation_percent
		rec.Amount = saved.Amount
		rec.Tax_amount = saved.Tax_amount
	}
	return nil
}

// UpdateGenJnlAllocations: a journal line's allocations by quantity or percentage get their
// share of the line's Amount (LCY), with the opposite sign and rounded so they add up; the
// line's balance includes them — the line itself is changed, saved by its caller
// (UpdateAllocations from the journal line)
func UpdateGenJnlAllocations(genJnlLine *gtables.GenJournalLine) error {
	return updateGenJnlAllocations(genJnlLine, false)
}

func updateGenJnlAllocations(genJnlLine *gtables.GenJournalLine, fromAllocations bool) error {
	if genJnlLine.Line_no == 0 {
		return nil
	}
	genJnlAlloc := gtables.NewGenJnlAllocation(genJnlLine)
	genJnlAlloc.SetRange("journal_template_name", genJnlLine.Journal_template_name)
	genJnlAlloc.SetRange("journal_batch_name", genJnlLine.Journal_batch_name)
	genJnlAlloc.SetRange("journal_line_no", genJnlLine.Line_no)
	updateGenJnlLine := fromAllocations
	var allocs []*gtables.GenJnlAllocation
	for found := genJnlAlloc.FindSet(); found; found = genJnlAlloc.Next() {
		a := gtables.NewGenJnlAllocation(genJnlLine)
		a.Copy(genJnlAlloc)
		allocs = append(allocs, a)
	}
	if len(allocs) > 0 {
		genJnlAlloc.LockTable()
		updateGenJnlLine = true
	}
	var totalQty, totalPct, totalPctRnded, totalAmountLCY, totalAmountLCY2, totalAmountLCYRnded, totalAmountLCYRnded2 types.Decimal
	hundred := types.NewDecimalFromInt(100)
	for _, a := range allocs {
		if a.Allocation_quantity.IsZero() && a.Allocation_percent.IsZero() {
			continue
		}
		if !fromAllocations {
			err := CheckGenJnlAllocTax(a, genJnlLine)
			if err != nil {
				return err
			}
		}
		if a.Allocation_quantity.IsZero() {
			totalAmountLCY2 = totalAmountLCY2.Sub(genJnlLine.Amount_lcy.Mul(a.Allocation_percent).Div(hundred))
			a.Amount = system.Round(totalAmountLCY2).Sub(totalAmountLCYRnded2)
			totalAmountLCYRnded2 = totalAmountLCYRnded2.Add(a.Amount)
		} else {
			if totalQty.IsZero() {
				for _, a2 := range allocs {
					totalQty = totalQty.Add(a2.Allocation_quantity)
				}
				if totalQty.IsZero() {
					totalQty = types.NewDecimalFromInt(1)
				}
			}
			totalPct = totalPct.Add(a.Allocation_quantity.Div(totalQty).Mul(hundred))
			a.Allocation_percent = system.Round(totalPct, types.MustDecimal("0.01")).Sub(totalPctRnded)
			totalPctRnded = totalPctRnded.Add(a.Allocation_percent)
			totalAmountLCY = totalAmountLCY.Sub(genJnlLine.Amount_lcy.Mul(a.Allocation_quantity).Div(totalQty))
			a.Amount = system.Round(totalAmountLCY).Sub(totalAmountLCYRnded)
			totalAmountLCYRnded = totalAmountLCYRnded.Add(a.Amount)
		}
		err := updateGenJnlAllocTax(a, genJnlLine)
		if err != nil {
			return err
		}
		err = a.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	if updateGenJnlLine {
		return updateJnlBalance(genJnlLine, fromAllocations)
	}
	return nil
}

// updateJnlBalance: the line's Balance (LCY) with its allocated amount — saved when the
// allocations changed it (UpdateJnlBalance)
func updateJnlBalance(genJnlLine *gtables.GenJournalLine, save bool) error {
	genJnlLine.CalcFields("allocated_amt_lcy")
	if genJnlLine.Bal_account_no == "" {
		genJnlLine.Balance_lcy = genJnlLine.Amount_lcy.Add(genJnlLine.Allocated_amt_lcy)
	} else {
		genJnlLine.Balance_lcy = genJnlLine.Allocated_amt_lcy
	}
	if save {
		return genJnlLine.ModifyOrError(false)
	}
	return nil
}
