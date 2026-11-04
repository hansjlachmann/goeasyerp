package tables

import (
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// generalPostingSetupLogic is the business logic of table 252 General Posting Setup: its G/L
// accounts are posting accounts that are not blocked; the payment discount accounts are for
// Adjust for Payment Disc.
type generalPostingSetupLogic struct{}

func init() { gtables.RegisterGeneralPostingSetupLogic(generalPostingSetupLogic{}) }

// checkSetupGLAcc: a setup's G/L account is a posting account that is not blocked (CheckGLAcc)
func checkSetupGLAcc(rec ftables.Source, accountNo types.Code) error {
	if accountNo != "" {
		glAcc := gtables.NewGLAccount(rec)
		err := glAcc.GetOrError(accountNo)
		if err != nil {
			return err
		}
		return CheckGLAcc(glAcc)
	}
	return nil
}

func (generalPostingSetupLogic) OnValidate_Sales_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Sales_account)
}
func (generalPostingSetupLogic) OnValidate_Sales_line_disc_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Sales_line_disc_account)
}
func (generalPostingSetupLogic) OnValidate_Sales_inv_disc_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Sales_inv_disc_account)
}
func (generalPostingSetupLogic) OnValidate_Sales_payment_disc_account(rec *gtables.GeneralPostingSetup) error {
	err := checkSetupGLAcc(rec, rec.Sales_payment_disc_account)
	if err != nil {
		return err
	}
	if rec.Sales_payment_disc_account != "" {
		glSetup := gtables.GetGeneralLedgerSetup(rec)
		return glSetup.TestField("adjust_for_payment_disc", true)
	}
	return nil
}
func (generalPostingSetupLogic) OnValidate_Purch_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Purch_account)
}
func (generalPostingSetupLogic) OnValidate_Purch_line_disc_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Purch_line_disc_account)
}
func (generalPostingSetupLogic) OnValidate_Purch_inv_disc_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Purch_inv_disc_account)
}
func (generalPostingSetupLogic) OnValidate_Purch_payment_disc_account(rec *gtables.GeneralPostingSetup) error {
	err := checkSetupGLAcc(rec, rec.Purch_payment_disc_account)
	if err != nil {
		return err
	}
	if rec.Purch_payment_disc_account != "" {
		glSetup := gtables.GetGeneralLedgerSetup(rec)
		return glSetup.TestField("adjust_for_payment_disc", true)
	}
	return nil
}
func (generalPostingSetupLogic) OnValidate_Cogs_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Cogs_account)
}
func (generalPostingSetupLogic) OnValidate_Inventory_adjmt_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Inventory_adjmt_account)
}
func (generalPostingSetupLogic) OnValidate_Project_sales_adjmt_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Project_sales_adjmt_account)
}
func (generalPostingSetupLogic) OnValidate_Project_cost_adjmt_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Project_cost_adjmt_account)
}
func (generalPostingSetupLogic) OnValidate_Sales_credit_memo_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Sales_credit_memo_account)
}
func (generalPostingSetupLogic) OnValidate_Purch_credit_memo_account(rec *gtables.GeneralPostingSetup) error {
	return checkSetupGLAcc(rec, rec.Purch_credit_memo_account)
}
