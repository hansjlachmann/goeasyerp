package fajnl

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// FAGLPostBuffer is table 5637 FA G/L Posting Buffer (temporary): a G/L amount an FA posting
// makes — the account (from the FA posting group, codeunit 5602, or an FA allocation), its
// department and project, and the FA or maintenance entry it belongs to. Codeunit 12 posts each
// as a G/L entry.
type FAGLPostBuffer struct {
	EntryNo                int // the G/L entry no. it becomes
	AccountNo              types.Code
	Amount                 types.Decimal
	Correction             bool
	DepartmentCode         types.Code
	ProjectCode            types.Code
	FAEntryType            gtables.GLEntryFa_entry_type
	FAEntryNo              int
	AutomaticEntry         bool
	OriginalGeneralJnlLine bool // the amount of the journal line itself (else an automatic one)
}

// faInsertGLAcc is codeunit 5601 FA Insert G/L Account: the G/L amounts of a posting, in a
// buffer numbered from the journal line's G/L entry no.
type faInsertGLAcc struct {
	buffer        []*FAGLPostBuffer
	glEntryNo     int
	orgGenJnlLine bool
}

// insertBufferEntry (InsertBufferEntry): the next buffer line
func (g *faInsertGLAcc) insertBufferEntry(b *FAGLPostBuffer) *FAGLPostBuffer {
	nextEntryNo := g.glEntryNo
	if len(g.buffer) > 0 {
		nextEntryNo = g.buffer[len(g.buffer)-1].EntryNo + 1
	}
	b.EntryNo = nextEntryNo
	b.OriginalGeneralJnlLine = g.orgGenJnlLine
	g.buffer = append(g.buffer, b)
	return b
}

// insertFAEntry (OnRun): the G/L amount of an FA ledger entry on its account (codeunit 5602);
// the entry gets the G/L entry no.
func (g *faInsertGLAcc) insertFAEntry(src ftables.Source, entry *gtables.FALedgerEntry) error {
	accNo, err := getAccNo(src, entry)
	if err != nil {
		return err
	}
	b := &FAGLPostBuffer{AccountNo: accNo, Amount: entry.Amount, Correction: entry.Correction,
		DepartmentCode: entry.Department_code, ProjectCode: entry.Project_code, FAEntryNo: entry.Entry_no,
		AutomaticEntry: entry.Automatic_entry}
	if entry.Entry_no > 0 {
		b.FAEntryType = gtables.GLEntry_Fa_entry_type.FixedAsset
	}
	g.glEntryNo = entry.G_l_entry_no
	entry.G_l_entry_no = g.insertBufferEntry(b).EntryNo
	return nil
}

// insertMaintenanceAccNo (InsertMaintenanceAccNo): the G/L amount of a maintenance entry on the
// posting group's Maintenance Expense Account
func (g *faInsertGLAcc) insertMaintenanceAccNo(src ftables.Source, entry *gtables.MaintenanceLedgerEntry) error {
	faPostingGr := gtables.NewFAPostingGroup(src)
	err := faPostingGr.GetOrError(entry.Fa_posting_group)
	if err != nil {
		return err
	}
	err = faPostingGr.TestField("maintenance_expense_account")
	if err != nil {
		return err
	}
	g.glEntryNo = entry.G_l_entry_no
	b := g.insertBufferEntry(&FAGLPostBuffer{AccountNo: faPostingGr.Maintenance_expense_account, Amount: entry.Amount,
		Correction: entry.Correction, DepartmentCode: entry.Department_code, ProjectCode: entry.Project_code,
		FAEntryNo: entry.Entry_no, FAEntryType: gtables.GLEntry_Fa_entry_type.Maintenance})
	entry.G_l_entry_no = b.EntryNo
	return nil
}

// getAccNo is codeunit 5602 FA Get G/L Account No.: the FA posting group's account of an FA
// ledger entry's posting type (and category: a disposal's own accounts)
func getAccNo(src ftables.Source, entry *gtables.FALedgerEntry) (types.Code, error) {
	faPostingGr := gtables.NewFAPostingGroup(src)
	err := faPostingGr.GetOrError(entry.Fa_posting_group)
	if err != nil {
		return "", err
	}
	field, accNo := "", types.Code("")
	switch entry.Fa_posting_category {
	case gtables.FALedgerEntry_Fa_posting_category.Blank:
		switch entry.Fa_posting_type {
		case gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost:
			field, accNo = "acquisition_cost_account", faPostingGr.Acquisition_cost_account
		case gtables.FALedgerEntry_Fa_posting_type.Depreciation:
			field, accNo = "accum_depreciation_account", faPostingGr.Accum_depreciation_account
		case gtables.FALedgerEntry_Fa_posting_type.WriteDown:
			field, accNo = "write_down_account", faPostingGr.Write_down_account
		case gtables.FALedgerEntry_Fa_posting_type.Appreciation:
			field, accNo = "appreciation_account", faPostingGr.Appreciation_account
		case gtables.FALedgerEntry_Fa_posting_type.Custom1:
			field, accNo = "custom_1_account", faPostingGr.Custom_1_account
		case gtables.FALedgerEntry_Fa_posting_type.Custom2:
			field, accNo = "custom_2_account", faPostingGr.Custom_2_account
		case gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal:
			field, accNo = "sales_acc_on_disposal", faPostingGr.Sales_acc_on_disposal
		case gtables.FALedgerEntry_Fa_posting_type.GainLoss:
			switch entry.Result_on_disposal {
			case gtables.FALedgerEntry_Result_on_disposal.Gain:
				field, accNo = "gains_acc_on_disposal", faPostingGr.Gains_acc_on_disposal
			case gtables.FALedgerEntry_Result_on_disposal.Loss:
				field, accNo = "losses_acc_on_disposal", faPostingGr.Losses_acc_on_disposal
			}
		}
	case gtables.FALedgerEntry_Fa_posting_category.Disposal:
		switch entry.Fa_posting_type {
		case gtables.FALedgerEntry_Fa_posting_type.AcquisitionCost:
			field, accNo = "acq_cost_acc_on_disposal", faPostingGr.Acq_cost_acc_on_disposal
		case gtables.FALedgerEntry_Fa_posting_type.Depreciation:
			field, accNo = "accum_depr_acc_on_disposal", faPostingGr.Accum_depr_acc_on_disposal
		case gtables.FALedgerEntry_Fa_posting_type.WriteDown:
			field, accNo = "write_down_acc_on_disposal", faPostingGr.Write_down_acc_on_disposal
		case gtables.FALedgerEntry_Fa_posting_type.Appreciation:
			field, accNo = "appreciation_acc_on_disposal", faPostingGr.Appreciation_acc_on_disposal
		case gtables.FALedgerEntry_Fa_posting_type.Custom1:
			field, accNo = "custom_1_account_on_disposal", faPostingGr.Custom_1_account_on_disposal
		case gtables.FALedgerEntry_Fa_posting_type.Custom2:
			field, accNo = "custom_2_account_on_disposal", faPostingGr.Custom_2_account_on_disposal
		case gtables.FALedgerEntry_Fa_posting_type.BookValueOnDisposal:
			field, accNo = "book_value_acc_on_disposal", faPostingGr.Book_value_acc_on_disposal
		}
	case gtables.FALedgerEntry_Fa_posting_category.BalDisposal:
		switch entry.Fa_posting_type {
		case gtables.FALedgerEntry_Fa_posting_type.WriteDown:
			field, accNo = "write_down_bal_acc_on_disp", faPostingGr.Write_down_bal_acc_on_disp
		case gtables.FALedgerEntry_Fa_posting_type.Appreciation:
			field, accNo = "apprec_bal_acc_on_disp", faPostingGr.Apprec_bal_acc_on_disp
		case gtables.FALedgerEntry_Fa_posting_type.Custom1:
			field, accNo = "custom_1_bal_acc_on_disposal", faPostingGr.Custom_1_bal_acc_on_disposal
		case gtables.FALedgerEntry_Fa_posting_type.Custom2:
			field, accNo = "custom_2_bal_acc_on_disposal", faPostingGr.Custom_2_bal_acc_on_disposal
		}
	}
	if field == "" {
		return "", nil
	}
	err = faPostingGr.TestField(field)
	if err != nil {
		return "", err
	}
	return accNo, nil
}

// allocationType is the option of FA Allocation's Allocation Type (InsertBufferBalAcc's
// FAPostingType: Acquisition, Depr, WriteDown, Appr, Custom1, Custom2, Disposal, Maintenance,
// Gain, Loss, Book Value)
type allocationType = gtables.FAAllocationAllocation_type

// balAccount: the FA posting group's balancing account of an allocation type (its field and
// value) and its Allocated % (field and value)
func balAccount(faPostingGr *gtables.FAPostingGroup, allocType allocationType) (string, types.Code, string, types.Decimal) {
	switch allocType {
	case gtables.FAAllocation_Allocation_type.Acquisition:
		faPostingGr.CalcFields("allocated_acquisition_cost_percent")
		return "acquisition_cost_bal_acc", faPostingGr.Acquisition_cost_bal_acc, "allocated_acquisition_cost_percent", faPostingGr.Allocated_acquisition_cost_percent
	case gtables.FAAllocation_Allocation_type.Depreciation:
		faPostingGr.CalcFields("allocated_depreciation_percent")
		return "depreciation_expense_acc", faPostingGr.Depreciation_expense_acc, "allocated_depreciation_percent", faPostingGr.Allocated_depreciation_percent
	case gtables.FAAllocation_Allocation_type.WriteDown:
		faPostingGr.CalcFields("allocated_write_down_percent")
		return "write_down_expense_acc", faPostingGr.Write_down_expense_acc, "allocated_write_down_percent", faPostingGr.Allocated_write_down_percent
	case gtables.FAAllocation_Allocation_type.Appreciation:
		faPostingGr.CalcFields("allocated_appreciation_percent")
		return "appreciation_bal_account", faPostingGr.Appreciation_bal_account, "allocated_appreciation_percent", faPostingGr.Allocated_appreciation_percent
	case gtables.FAAllocation_Allocation_type.Custom1:
		faPostingGr.CalcFields("allocated_custom_1_percent")
		return "custom_1_expense_acc", faPostingGr.Custom_1_expense_acc, "allocated_custom_1_percent", faPostingGr.Allocated_custom_1_percent
	case gtables.FAAllocation_Allocation_type.Custom2:
		faPostingGr.CalcFields("allocated_custom_2_percent")
		return "custom_2_expense_acc", faPostingGr.Custom_2_expense_acc, "allocated_custom_2_percent", faPostingGr.Allocated_custom_2_percent
	case gtables.FAAllocation_Allocation_type.Disposal:
		faPostingGr.CalcFields("allocated_sales_price_percent")
		return "sales_bal_acc", faPostingGr.Sales_bal_acc, "allocated_sales_price_percent", faPostingGr.Allocated_sales_price_percent
	case gtables.FAAllocation_Allocation_type.Maintenance:
		faPostingGr.CalcFields("allocated_maintenance_percent")
		return "maintenance_bal_acc", faPostingGr.Maintenance_bal_acc, "allocated_maintenance_percent", faPostingGr.Allocated_maintenance_percent
	case gtables.FAAllocation_Allocation_type.Gain:
		faPostingGr.CalcFields("allocated_gain_percent")
		return "gains_acc_on_disposal", faPostingGr.Gains_acc_on_disposal, "allocated_gain_percent", faPostingGr.Allocated_gain_percent
	case gtables.FAAllocation_Allocation_type.Loss:
		faPostingGr.CalcFields("allocated_loss_percent")
		return "losses_acc_on_disposal", faPostingGr.Losses_acc_on_disposal, "allocated_loss_percent", faPostingGr.Allocated_loss_percent
	}
	faPostingGr.CalcFields("allocated_book_value_percent")
	return "book_value_acc_on_disposal", faPostingGr.Book_value_acc_on_disposal, "allocated_book_value_percent", faPostingGr.Allocated_book_value_percent
}

// insertBufferBalAcc (InsertBufferBalAcc): the balancing amount of a posting type split over
// the posting group's FA allocations of the type (rounded per the book), the rest on the group's
// balancing account
func (g *faInsertGLAcc) insertBufferBalAcc(src ftables.Source, allocType allocationType, allocAmount types.Decimal, deprBookCode, postingGrCode,
	departmentCode, projectCode types.Code, automaticEntry, correction bool) error {
	faPostingGr := gtables.NewFAPostingGroup(src)
	err := faPostingGr.GetOrError(postingGrCode)
	if err != nil {
		return err
	}
	accountField, glAccNo, allocatedField, allocated := balAccount(faPostingGr, allocType)
	err = faPostingGr.TestField(accountField)
	if err != nil {
		return err
	}
	if allocated.GreaterThan(types.NewDecimalFromInt(100)) {
		return apperrors.FieldErrorText(gtables.FAPostingGroupTableName, allocatedField, "> 100")
	}
	deprBook := gtables.NewDepreciationBook(src)
	err = deprBook.GetOrError(deprBookCode)
	if err != nil {
		return err
	}
	calcRounding := func(amount types.Decimal) types.Decimal {
		if deprBook.Use_rounding_in_periodic_depr {
			return system.Round(amount, types.NewDecimalFromInt(1))
		}
		return system.Round(amount)
	}
	var totalAllocAmount, totalPercent types.Decimal
	faAlloc := gtables.NewFAAllocation(src)
	faAlloc.SetRange("code", postingGrCode)
	faAlloc.SetRange("allocation_type", allocType)
	for found := faAlloc.FindSet(); found; found = faAlloc.Next() {
		if faAlloc.Account_no == "" && faAlloc.Allocation_percent.IsPositive() {
			err = faAlloc.TestField("account_no")
			if err != nil {
				return err
			}
		}
		totalPercent = totalPercent.Add(faAlloc.Allocation_percent)
		newAmount := calcRounding(allocAmount.Mul(totalPercent).Div(types.NewDecimalFromInt(100))).Sub(totalAllocAmount)
		totalAllocAmount = totalAllocAmount.Add(newAmount)
		if totalAllocAmount.Abs().GreaterThan(allocAmount.Abs()) {
			newAmount = allocAmount.Sub(totalAllocAmount.Sub(newAmount))
		}
		if !newAmount.IsZero() {
			g.insertBufferEntry(&FAGLPostBuffer{AccountNo: faAlloc.Account_no, DepartmentCode: faAlloc.Department_code,
				ProjectCode: faAlloc.Project_code, Amount: newAmount, AutomaticEntry: automaticEntry, Correction: correction})
		}
	}
	if totalAllocAmount.Abs().LessThan(allocAmount.Abs()) {
		newAmount := allocAmount.Sub(totalAllocAmount)
		g.insertBufferEntry(&FAGLPostBuffer{AccountNo: glAccNo, Amount: newAmount, DepartmentCode: departmentCode,
			ProjectCode: projectCode, AutomaticEntry: automaticEntry, Correction: correction})
	}
	return nil
}

// allocTypeOfEntry (GetPostingType): an FA ledger entry's allocation type
func allocTypeOfEntry(entry *gtables.FALedgerEntry) allocationType {
	switch entry.Fa_posting_type {
	case gtables.FALedgerEntry_Fa_posting_type.GainLoss:
		if entry.Result_on_disposal == gtables.FALedgerEntry_Result_on_disposal.Gain {
			return gtables.FAAllocation_Allocation_type.Gain
		}
		return gtables.FAAllocation_Allocation_type.Loss
	case gtables.FALedgerEntry_Fa_posting_type.BookValueOnDisposal:
		return gtables.FAAllocation_Allocation_type.BookValue
	case gtables.FALedgerEntry_Fa_posting_type.ProceedsOnDisposal:
		return gtables.FAAllocation_Allocation_type.Disposal
	case gtables.FALedgerEntry_Fa_posting_type.SalvageValue:
		return gtables.FAAllocation_Allocation_type.Acquisition
	}
	// Acquisition Cost … Custom 2 have the same numbers in both options
	return allocationType(entry.Fa_posting_type)
}

// insertBalAcc (InsertBalAcc): an automatic FA entry's balancing amount (the entry's amount the
// other way)
func (g *faInsertGLAcc) insertBalAcc(src ftables.Source, entry *gtables.FALedgerEntry) error {
	return g.insertBufferBalAcc(src, allocTypeOfEntry(entry), entry.Amount.Neg(), entry.Depreciation_book_code,
		entry.Fa_posting_group, entry.Department_code, entry.Project_code, entry.Automatic_entry, entry.Correction)
}
