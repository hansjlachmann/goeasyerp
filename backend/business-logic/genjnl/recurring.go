package genjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Recurring journals in codeunit 13 Gen. Jnl.-Post Batch. A line's Recurring Method says what
// it posts: F Fixed its amount, V Variable its amount (then cleared), B Balance the account's
// balance on the posting date (allocated to other accounts); the R… methods post the same and
// the reverse the day after. Document No. and Description may hold %1 day, %2 week, %3 month,
// %4 month name, %5 accounting period name of the posting date.

// isReversingMethod: RF, RV or RB — the line is reversed the day after
func isReversingMethod(method gtables.GenJournalLineRecurring_method) bool {
	return method >= gtables.GenJournalLine_Recurring_method.RFReversingFixed
}

// checkRecurringLine: a recurring journal's line has a method and a frequency and no balancing
// account; a variable line an amount, a balance line none; other journals' lines have neither
func (b *GenJnlPostBatch) checkRecurringLine(genJnlLine2 *gtables.GenJournalLine) error {
	if genJnlLine2.Account_no == "" {
		return nil
	}
	if !b.genJnlTemplate.Recurring {
		err := genJnlLine2.TestField("recurring_method", gtables.GenJournalLine_Recurring_method.Blank)
		if err != nil {
			return err
		}
		return genJnlLine2.TestField("recurring_frequency", types.Code(""))
	}
	err := genJnlLine2.TestField("recurring_method")
	if err != nil {
		return err
	}
	err = genJnlLine2.TestField("recurring_frequency")
	if err != nil {
		return err
	}
	if genJnlLine2.Bal_account_no != "" {
		return apperrors.JnlRecurringNoBalAccount(jnlLine)
	}
	switch genJnlLine2.Recurring_method {
	case gtables.GenJournalLine_Recurring_method.VVariable, gtables.GenJournalLine_Recurring_method.RVReversingVariable:
		return genJnlLine2.TestField("amount")
	case gtables.GenJournalLine_Recurring_method.BBalance, gtables.GenJournalLine_Recurring_method.RBReversingBalance:
		return genJnlLine2.TestField("amount", types.Decimal{})
	}
	return nil
}

// updateRecurringAmt: a balance line's amount is the reverse of its G/L account's balance on the
// posting date — saved, with its allocations' shares
func (b *GenJnlPostBatch) updateRecurringAmt(genJnlLine2 *gtables.GenJournalLine) error {
	if genJnlLine2.Account_no == "" || !tables.IsBalanceRecurringMethod(genJnlLine2.Recurring_method) {
		return nil
	}
	if genJnlLine2.Account_type != gtables.GenJournalLine_Account_type.GLAccount {
		return apperrors.JnlRecurringBalanceGLOnly()
	}
	glAcc := gtables.NewGLAccount(b.src)
	glAcc.No = genJnlLine2.Account_no
	err := glAcc.SetFlowFilter("date_filter", ".."+genJnlLine2.Posting_date.String())
	if err != nil {
		return err
	}
	glAcc.CalcFields("net_change")
	err = genJnlLine2.Validate("amount", glAcc.Net_change.Neg())
	if err != nil {
		return err
	}
	return genJnlLine2.ModifyOrError(false)
}

// allocations: the line's allocations
func allocations(genJnlLine *gtables.GenJournalLine) *gtables.GenJnlAllocation {
	genJnlAlloc := gtables.NewGenJnlAllocation(genJnlLine)
	genJnlAlloc.SetRange("journal_template_name", genJnlLine.Journal_template_name)
	genJnlAlloc.SetRange("journal_batch_name", genJnlLine.Journal_batch_name)
	genJnlAlloc.SetRange("journal_line_no", genJnlLine.Line_no)
	return genJnlAlloc
}

// checkAllocations: allocations with amounts only in recurring journals, each with an account
func (b *GenJnlPostBatch) checkAllocations(genJnlLine2 *gtables.GenJournalLine) error {
	if genJnlLine2.Account_no == "" {
		return nil
	}
	genJnlAlloc := allocations(genJnlLine2)
	genJnlAlloc.SetFilter("amount", "<>0")
	if !genJnlAlloc.FindFirst() {
		return nil
	}
	if !b.genJnlTemplate.Recurring {
		return apperrors.JnlAllocRecurringOnly()
	}
	genJnlAlloc.SetRange("account_no", types.Code(""))
	if genJnlAlloc.FindFirst() {
		return genJnlAlloc.TestField("account_no")
	}
	return nil
}

// makeRecurringText: the placeholders of a recurring line's document no. and description
func (b *GenJnlPostBatch) makeRecurringText(genJnlLine2 *gtables.GenJournalLine) {
	if genJnlLine2.Account_no == "" || genJnlLine2.Recurring_method == gtables.GenJournalLine_Recurring_method.Blank {
		return
	}
	genJnlLine2.Document_no, genJnlLine2.Description = tables.RecurringText(b.src, genJnlLine2.Posting_date,
		genJnlLine2.Document_no, genJnlLine2.Description, genJnlLine2.MaxStrLen("document_no"), genJnlLine2.MaxStrLen("description"))
}

// postAllocations: each allocation posts as a G/L account line of the line's document — the
// reverse on the reversing date; a variable or balance line's allocations lose their amounts
// once posted (the reversing methods after the reversal)
func (b *GenJnlPostBatch) postAllocations(allocateGenJnlLine *gtables.GenJournalLine, reversing bool) error {
	if allocateGenJnlLine.Account_no == "" {
		return nil
	}
	genJnlAlloc := allocations(allocateGenJnlLine)
	genJnlAlloc.SetFilter("account_no", "<>''")
	var allocs []*gtables.GenJnlAllocation
	for found := genJnlAlloc.FindSet(); found; found = genJnlAlloc.Next() {
		a := gtables.NewGenJnlAllocation(b.src)
		a.Copy(genJnlAlloc)
		allocs = append(allocs, a)
	}
	method := allocateGenJnlLine.Recurring_method
	clear := !reversing && (method == gtables.GenJournalLine_Recurring_method.VVariable || method == gtables.GenJournalLine_Recurring_method.BBalance) ||
		reversing && (method == gtables.GenJournalLine_Recurring_method.RVReversingVariable || method == gtables.GenJournalLine_Recurring_method.RBReversingBalance)
	for _, a := range allocs {
		genJnlLine2 := gtables.NewGenJournalLine(b.src)
		genJnlLine2.Init()
		genJnlLine2.Account_type = gtables.GenJournalLine_Account_type.GLAccount
		genJnlLine2.Posting_date = allocateGenJnlLine.Posting_date
		genJnlLine2.Document_type = allocateGenJnlLine.Document_type
		genJnlLine2.Document_no = allocateGenJnlLine.Document_no
		genJnlLine2.Description = allocateGenJnlLine.Description
		genJnlLine2.Source_code = allocateGenJnlLine.Source_code
		genJnlLine2.Journal_batch_name = allocateGenJnlLine.Journal_batch_name
		genJnlLine2.Line_no = allocateGenJnlLine.Line_no
		genJnlLine2.Reason_code = allocateGenJnlLine.Reason_code
		genJnlLine2.Recurring_method = method
		genJnlLine2.Account_no = a.Account_no
		genJnlLine2.Amount = a.Amount
		genJnlLine2.Amount_lcy = a.Amount
		genJnlLine2.Department_code = a.Department_code
		genJnlLine2.Project_code = a.Project_code
		genJnlLine2.Gen_posting_type = gtables.GenJournalLineGen_posting_type(a.Gen_posting_type)
		genJnlLine2.Gen_bus_posting_group = a.Gen_bus_posting_group
		genJnlLine2.Gen_prod_posting_group = a.Gen_prod_posting_group
		genJnlLine2.Tax_bus_posting_group = a.Tax_bus_posting_group
		genJnlLine2.Tax_prod_posting_group = a.Tax_prod_posting_group
		genJnlLine2.Tax_area_code = a.Tax_area_code
		genJnlLine2.Tax_liable = a.Tax_liable
		genJnlLine2.Tax_group_code = a.Tax_group_code
		genJnlLine2.Use_tax = a.Use_tax
		genJnlLine2.Tax_calculation_type = gtables.GenJournalLineTax_calculation_type(a.Tax_calculation_type)
		genJnlLine2.Tax_amount = a.Tax_amount
		genJnlLine2.Tax_base_amount = a.Amount.Sub(a.Tax_amount)
		genJnlLine2.Tax_percent = a.Tax_percent
		if reversing {
			multiplyAmounts(genJnlLine2, types.NewDecimalFromInt(-1))
			genJnlLine2.Reversing_entry = true
		}
		err := b.genJnlPostLine.Run(genJnlLine2)
		if err != nil {
			return err
		}
		if clear {
			a.Amount = types.Decimal{}
			a.Additional_currency_amount = types.Decimal{}
			err = a.ModifyOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// multiplyAmounts: the line's amounts times a factor (-1: the reversing line, 0: cleared)
func multiplyAmounts(genJnlLine2 *gtables.GenJournalLine, factor types.Decimal) {
	if genJnlLine2.Account_no == "" {
		return
	}
	genJnlLine2.Amount = genJnlLine2.Amount.Mul(factor)
	genJnlLine2.Debit_amount = genJnlLine2.Debit_amount.Mul(factor)
	genJnlLine2.Credit_amount = genJnlLine2.Credit_amount.Mul(factor)
	genJnlLine2.Amount_lcy = genJnlLine2.Amount_lcy.Mul(factor)
	genJnlLine2.Balance_lcy = genJnlLine2.Balance_lcy.Mul(factor)
	genJnlLine2.Sales_purch_lcy = genJnlLine2.Sales_purch_lcy.Mul(factor)
	genJnlLine2.Profit_lcy = genJnlLine2.Profit_lcy.Mul(factor)
	genJnlLine2.Inv_discount_lcy = genJnlLine2.Inv_discount_lcy.Mul(factor)
	genJnlLine2.Quantity = genJnlLine2.Quantity.Mul(factor)
	genJnlLine2.Tax_amount = genJnlLine2.Tax_amount.Mul(factor)
	genJnlLine2.Tax_base_amount = genJnlLine2.Tax_base_amount.Mul(factor)
	genJnlLine2.Tax_amount_lcy = genJnlLine2.Tax_amount_lcy.Mul(factor)
	genJnlLine2.Tax_base_amount_lcy = genJnlLine2.Tax_base_amount_lcy.Mul(factor)
	genJnlLine2.Source_currency_amount = genJnlLine2.Source_currency_amount.Mul(factor)
}

// updateRecurringLines: after posting, each line of the recurring journal moves on to its next
// date (Recurring Frequency); variable and balance lines lose their amounts
func (b *GenJnlPostBatch) updateRecurringLines(genJnlLine *gtables.GenJournalLine, noOfRecords int) error {
	genJnlLine2 := gtables.NewGenJournalLine(b.src)
	genJnlLine2.CopyFilters(genJnlLine)
	var lines []*gtables.GenJournalLine
	for found := genJnlLine2.FindSet(); found; found = genJnlLine2.Next() {
		l := gtables.NewGenJournalLine(b.src)
		l.Copy(genJnlLine2)
		lines = append(lines, l)
	}
	for i, l := range lines {
		b.window.Update(9, i+1)
		b.window.Update(10, system.Round(types.NewDecimalFromInt(int64((i+1)*10000/noOfRecords)), 1))
		if !l.Posting_date.IsZero() {
			nextDate, err := tables.NextRecurringDate(l.Posting_date, l.Recurring_frequency)
			if err != nil {
				return err
			}
			err = l.Validate("posting_date", nextDate)
			if err != nil {
				return err
			}
		}
		if l.Recurring_method != gtables.GenJournalLine_Recurring_method.FFixed &&
			l.Recurring_method != gtables.GenJournalLine_Recurring_method.RFReversingFixed {
			multiplyAmounts(l, types.Decimal{})
			// the allocations by quantity or percentage follow the cleared amount (validating the
			// posting date recalculated them from the old one)
			err := tables.UpdateGenJnlAllocations(l)
			if err != nil {
				return err
			}
		}
		err := l.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}
