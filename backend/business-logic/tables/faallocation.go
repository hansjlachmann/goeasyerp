package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// fAAllocationLogic is the business logic of table 5615 FA Allocation: an FA posting group's
// balancing amounts of an allocation type split over G/L accounts (with department and project)
// by Allocation % — what is not allocated goes to the group's own balancing account.
type fAAllocationLogic struct{}

func init() { gtables.RegisterFAAllocationLogic(fAAllocationLogic{}) }

// OnValidate_Account_no: a posting account that is not blocked; its name
func (fAAllocationLogic) OnValidate_Account_no(rec *gtables.FAAllocation) error {
	if rec.Account_no == "" {
		return nil
	}
	glAcc := gtables.NewGLAccount(rec)
	err := glAcc.GetOrError(rec.Account_no)
	if err != nil {
		return err
	}
	err = CheckGLAcc(glAcc)
	if err != nil {
		return err
	}
	rec.Description = glAcc.Name
	return nil
}
