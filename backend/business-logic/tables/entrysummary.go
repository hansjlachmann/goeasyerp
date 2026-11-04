package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// entrySummaryLogic is the business logic of table 338 Entry_Summary.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/entrysummary_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type entrySummaryLogic struct{}

func init() { gtables.RegisterEntrySummaryLogic(entrySummaryLogic{}) }

// OnInsert runs before a new record is inserted
func (entrySummaryLogic) OnInsert(rec *gtables.EntrySummary) error {
	return nil
}

// OnModify runs before a record is modified
func (entrySummaryLogic) OnModify(rec *gtables.EntrySummary) error {
	return nil
}

// OnDelete runs before a record is deleted
func (entrySummaryLogic) OnDelete(rec *gtables.EntrySummary) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (entrySummaryLogic) OnRename(rec *gtables.EntrySummary) error {
	return nil
}
