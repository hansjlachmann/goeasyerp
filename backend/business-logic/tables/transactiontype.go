package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// transactionTypeLogic is the business logic of table 258 Transaction_Type.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/transactiontype_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type transactionTypeLogic struct{}

func init() { gtables.RegisterTransactionTypeLogic(transactionTypeLogic{}) }

// OnInsert runs before a new record is inserted
func (transactionTypeLogic) OnInsert(rec *gtables.TransactionType) error {
	return nil
}

// OnModify runs before a record is modified
func (transactionTypeLogic) OnModify(rec *gtables.TransactionType) error {
	return nil
}

// OnDelete runs before a record is deleted
func (transactionTypeLogic) OnDelete(rec *gtables.TransactionType) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (transactionTypeLogic) OnRename(rec *gtables.TransactionType) error {
	return nil
}
