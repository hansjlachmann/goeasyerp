package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// genProductPostingGroupLogic is the business logic of table 251 Gen_Product_Posting_Group.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/genproductpostinggroup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type genProductPostingGroupLogic struct{}

func init() { gtables.RegisterGenProductPostingGroupLogic(genProductPostingGroupLogic{}) }

// OnInsert runs before a new record is inserted
func (genProductPostingGroupLogic) OnInsert(rec *gtables.GenProductPostingGroup) error {
	return nil
}

// OnModify runs before a record is modified
func (genProductPostingGroupLogic) OnModify(rec *gtables.GenProductPostingGroup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (genProductPostingGroupLogic) OnDelete(rec *gtables.GenProductPostingGroup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (genProductPostingGroupLogic) OnRename(rec *gtables.GenProductPostingGroup) error {
	return nil
}

// ValidateTaxProdPostingGroup reads the Gen. Product Posting Group entered in a field (a blank
// code is a new, empty group) and tells whether its default tax posting group is filled in
// automatically (Auto Insert Default) — used when that field is validated
func ValidateTaxProdPostingGroup(group *gtables.GenProductPostingGroup, entered types.Code) (bool, error) {
	if entered == "" {
		group.InitRecord()
		return group.Auto_insert_default, nil
	}
	err := group.GetOrError(entered)
	if err != nil {
		return false, err
	}
	return group.Auto_insert_default, nil
}
