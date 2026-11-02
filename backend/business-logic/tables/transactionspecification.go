package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// transactionSpecificationLogic is the business logic of table 285 Transaction_Specification.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/transactionspecification_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type transactionSpecificationLogic struct{}

func init() { gtables.RegisterTransactionSpecificationLogic(transactionSpecificationLogic{}) }

// OnInsert runs before a new record is inserted
func (transactionSpecificationLogic) OnInsert(rec *gtables.TransactionSpecification) error {
	return nil
}

// OnModify runs before a record is modified
func (transactionSpecificationLogic) OnModify(rec *gtables.TransactionSpecification) error {
	return nil
}

// OnDelete runs before a record is deleted
func (transactionSpecificationLogic) OnDelete(rec *gtables.TransactionSpecification) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (transactionSpecificationLogic) OnRename(rec *gtables.TransactionSpecification) error {
	return nil
}
