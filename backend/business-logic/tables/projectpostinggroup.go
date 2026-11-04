package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// projectPostingGroupLogic is the business logic of table 208 Project Posting Group: its
// accounts must be posting accounts that are not blocked (CheckGLAcc)
type projectPostingGroupLogic struct{}

func init() { gtables.RegisterProjectPostingGroupLogic(projectPostingGroupLogic{}) }

func (projectPostingGroupLogic) OnValidate_Wip_cost_account(rec *gtables.ProjectPostingGroup) error {
	return checkSetupGLAcc(rec, rec.Wip_cost_account)
}

func (projectPostingGroupLogic) OnValidate_Wip_sales_account(rec *gtables.ProjectPostingGroup) error {
	return checkSetupGLAcc(rec, rec.Wip_sales_account)
}

func (projectPostingGroupLogic) OnValidate_Cost_account(rec *gtables.ProjectPostingGroup) error {
	return checkSetupGLAcc(rec, rec.Cost_account)
}

func (projectPostingGroupLogic) OnValidate_Sales_account(rec *gtables.ProjectPostingGroup) error {
	return checkSetupGLAcc(rec, rec.Sales_account)
}

func (projectPostingGroupLogic) OnValidate_G_l_exp_sales_acc(rec *gtables.ProjectPostingGroup) error {
	return checkSetupGLAcc(rec, rec.G_l_exp_sales_acc)
}
