package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// countryRegionLogic is the business logic of table 9 Country_Region.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/countryregion_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type countryRegionLogic struct{}

func init() { gtables.RegisterCountryRegionLogic(countryRegionLogic{}) }

// OnInsert runs before a new record is inserted
func (countryRegionLogic) OnInsert(rec *gtables.CountryRegion) error {
	return nil
}

// OnModify runs before a record is modified
func (countryRegionLogic) OnModify(rec *gtables.CountryRegion) error {
	return nil
}

// OnDelete runs before a record is deleted
func (countryRegionLogic) OnDelete(rec *gtables.CountryRegion) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (countryRegionLogic) OnRename(rec *gtables.CountryRegion) error {
	return nil
}
