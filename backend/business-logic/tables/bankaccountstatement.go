package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bankAccountStatementLogic is the business logic of table 275 Bank_Account_Statement.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/bankaccountstatement_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type bankAccountStatementLogic struct{}

func init() { gtables.RegisterBankAccountStatementLogic(bankAccountStatementLogic{}) }

// OnInsert runs before a new record is inserted
func (bankAccountStatementLogic) OnInsert(rec *gtables.BankAccountStatement) error {
	return nil
}

// OnModify runs before a record is modified
func (bankAccountStatementLogic) OnModify(rec *gtables.BankAccountStatement) error {
	return nil
}

// OnDelete runs before a record is deleted
func (bankAccountStatementLogic) OnDelete(rec *gtables.BankAccountStatement) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (bankAccountStatementLogic) OnRename(rec *gtables.BankAccountStatement) error {
	return nil
}
