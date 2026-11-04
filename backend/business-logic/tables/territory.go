package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// territoryLogic is the business logic of table 286 Territory.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/territory_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type territoryLogic struct{}

func init() { gtables.RegisterTerritoryLogic(territoryLogic{}) }

// OnInsert runs before a new record is inserted
func (territoryLogic) OnInsert(rec *gtables.Territory) error {
	return nil
}

// OnModify runs before a record is modified
func (territoryLogic) OnModify(rec *gtables.Territory) error {
	return nil
}

// OnDelete runs before a record is deleted
func (territoryLogic) OnDelete(rec *gtables.Territory) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (territoryLogic) OnRename(rec *gtables.Territory) error {
	return nil
}
