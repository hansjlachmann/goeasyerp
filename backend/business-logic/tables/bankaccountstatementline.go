package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// bankAccountStatementLineLogic is the business logic of table 276 Bank_Account_Statement_Line.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/bankaccountstatementline_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type bankAccountStatementLineLogic struct{}

func init() { gtables.RegisterBankAccountStatementLineLogic(bankAccountStatementLineLogic{}) }

// OnInsert runs before a new record is inserted
func (bankAccountStatementLineLogic) OnInsert(rec *gtables.BankAccountStatementLine) error {
	return nil
}

// OnModify runs before a record is modified
func (bankAccountStatementLineLogic) OnModify(rec *gtables.BankAccountStatementLine) error {
	return nil
}

// OnDelete runs before a record is deleted
func (bankAccountStatementLineLogic) OnDelete(rec *gtables.BankAccountStatementLine) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (bankAccountStatementLineLogic) OnRename(rec *gtables.BankAccountStatementLine) error {
	return nil
}
