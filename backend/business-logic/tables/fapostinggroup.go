package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fAPostingGroupLogic is the business logic of table 5606 FA Posting Group: every account must
// be a posting account that is not blocked (CheckGLAcc)
type fAPostingGroupLogic struct{}

func init() { gtables.RegisterFAPostingGroupLogic(fAPostingGroupLogic{}) }

func (fAPostingGroupLogic) OnValidate_Acquisition_cost_account(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Acquisition_cost_account, false, false)
}

func (fAPostingGroupLogic) OnValidate_Accum_depreciation_account(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Accum_depreciation_account, false, false)
}

func (fAPostingGroupLogic) OnValidate_Write_down_account(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Write_down_account, false, false)
}

func (fAPostingGroupLogic) OnValidate_Appreciation_account(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Appreciation_account, false, false)
}

func (fAPostingGroupLogic) OnValidate_Custom_1_account(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Custom_1_account, false, false)
}

func (fAPostingGroupLogic) OnValidate_Custom_2_account(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Custom_2_account, false, false)
}

func (fAPostingGroupLogic) OnValidate_Acq_cost_acc_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Acq_cost_acc_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Accum_depr_acc_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Accum_depr_acc_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Write_down_acc_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Write_down_acc_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Appreciation_acc_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Appreciation_acc_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Custom_1_account_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Custom_1_account_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Custom_2_account_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Custom_2_account_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Gains_acc_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Gains_acc_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Losses_acc_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Losses_acc_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Book_value_acc_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Book_value_acc_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Sales_acc_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Sales_acc_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Write_down_bal_acc_on_disp(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Write_down_bal_acc_on_disp, false, false)
}

func (fAPostingGroupLogic) OnValidate_Apprec_bal_acc_on_disp(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Apprec_bal_acc_on_disp, false, false)
}

func (fAPostingGroupLogic) OnValidate_Custom_1_bal_acc_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Custom_1_bal_acc_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Custom_2_bal_acc_on_disposal(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Custom_2_bal_acc_on_disposal, false, false)
}

func (fAPostingGroupLogic) OnValidate_Maintenance_expense_account(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Maintenance_expense_account, false, false)
}

func (fAPostingGroupLogic) OnValidate_Maintenance_bal_acc(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Maintenance_bal_acc, false, false)
}

func (fAPostingGroupLogic) OnValidate_Acquisition_cost_bal_acc(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Acquisition_cost_bal_acc, false, false)
}

func (fAPostingGroupLogic) OnValidate_Depreciation_expense_acc(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Depreciation_expense_acc, false, false)
}

func (fAPostingGroupLogic) OnValidate_Write_down_expense_acc(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Write_down_expense_acc, false, false)
}

func (fAPostingGroupLogic) OnValidate_Appreciation_bal_account(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Appreciation_bal_account, false, false)
}

func (fAPostingGroupLogic) OnValidate_Custom_1_expense_acc(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Custom_1_expense_acc, false, false)
}

func (fAPostingGroupLogic) OnValidate_Custom_2_expense_acc(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Custom_2_expense_acc, false, false)
}

func (fAPostingGroupLogic) OnValidate_Sales_bal_acc(rec *gtables.FAPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.Sales_bal_acc, false, false)
}
