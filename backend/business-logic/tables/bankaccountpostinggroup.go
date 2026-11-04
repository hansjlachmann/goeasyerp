package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bankAccountPostingGroupLogic is the business logic of table 277 Bank Account Posting Group
type bankAccountPostingGroupLogic struct{}

func init() { gtables.RegisterBankAccountPostingGroupLogic(bankAccountPostingGroupLogic{}) }

// OnValidate_G_l_bank_account_no: a posting account that is not blocked
func (bankAccountPostingGroupLogic) OnValidate_G_l_bank_account_no(rec *gtables.BankAccountPostingGroup) error {
	return checkPostingGroupGLAcc(rec, rec.G_l_bank_account_no, false, false)
}
