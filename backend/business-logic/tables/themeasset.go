package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// themeAssetLogic is the business logic of table 902 Theme_Asset.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/themeasset_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type themeAssetLogic struct{}

func init() { gtables.RegisterThemeAssetLogic(themeAssetLogic{}) }

// OnInsert runs before a new record is inserted
func (themeAssetLogic) OnInsert(rec *gtables.ThemeAsset) error {
	return nil
}

// OnModify runs before a record is modified
func (themeAssetLogic) OnModify(rec *gtables.ThemeAsset) error {
	return nil
}

// OnDelete runs before a record is deleted
func (themeAssetLogic) OnDelete(rec *gtables.ThemeAsset) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (themeAssetLogic) OnRename(rec *gtables.ThemeAsset) error {
	return nil
}
