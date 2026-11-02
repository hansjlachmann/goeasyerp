package tables

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// businessUnitLogic is the business logic of table 220 Business_Unit.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/businessunit_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type businessUnitLogic struct{}

func init() { gtables.RegisterBusinessUnitLogic(businessUnitLogic{}) }

// OnInsert runs before a new record is inserted
func (businessUnitLogic) OnInsert(rec *gtables.BusinessUnit) error {
	return nil
}

// OnModify runs before a record is modified
func (businessUnitLogic) OnModify(rec *gtables.BusinessUnit) error {
	return nil
}

// OnDelete runs before a record is deleted
func (businessUnitLogic) OnDelete(rec *gtables.BusinessUnit) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (businessUnitLogic) OnRename(rec *gtables.BusinessUnit) error {
	return nil
}

// OnValidate_Exch_rate_loss_acc: a posting account
func (businessUnitLogic) OnValidate_Exch_rate_loss_acc(rec *gtables.BusinessUnit) error {
	return checkBusinessUnitGLAcc(rec, rec.Exch_rate_loss_acc)
}

// OnValidate_Exch_rate_gains_acc: a posting account
func (businessUnitLogic) OnValidate_Exch_rate_gains_acc(rec *gtables.BusinessUnit) error {
	return checkBusinessUnitGLAcc(rec, rec.Exch_rate_gains_acc)
}

// OnValidate_Residual_account: a posting account
func (businessUnitLogic) OnValidate_Residual_account(rec *gtables.BusinessUnit) error {
	return checkBusinessUnitGLAcc(rec, rec.Residual_account)
}

// OnValidate_Currency_code: the currency's factors at the work date for the income statement
// and the balance sheet
func (businessUnitLogic) OnValidate_Currency_code(rec *gtables.BusinessUnit) error {
	factor, err := ExchangeRates(rec).ExchangeRate(system.WorkDate(), rec.Currency_code)
	if err != nil {
		return err
	}
	rec.Income_currency_factor = factor
	rec.Balance_currency_factor = factor
	return nil
}

// checkBusinessUnitGLAcc (CheckGLAcc): an account that is given is a posting account
func checkBusinessUnitGLAcc(rec ftables.Source, accNo types.Code) error {
	if accNo == "" {
		return nil
	}
	glAcc := gtables.NewGLAccount(rec)
	err := glAcc.GetOrError(accNo)
	if err != nil {
		return err
	}
	return CheckGLAcc(glAcc)
}
