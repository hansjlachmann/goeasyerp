package tables

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// genBusinessPostingGroupLogic is the business logic of table 250 Gen_Business_Posting_Group.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/genbusinesspostinggroup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type genBusinessPostingGroupLogic struct{}

func init() { gtables.RegisterGenBusinessPostingGroupLogic(genBusinessPostingGroupLogic{}) }

// OnInsert runs before a new record is inserted
func (genBusinessPostingGroupLogic) OnInsert(rec *gtables.GenBusinessPostingGroup) error {
	return nil
}

// OnModify runs before a record is modified
func (genBusinessPostingGroupLogic) OnModify(rec *gtables.GenBusinessPostingGroup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (genBusinessPostingGroupLogic) OnDelete(rec *gtables.GenBusinessPostingGroup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (genBusinessPostingGroupLogic) OnRename(rec *gtables.GenBusinessPostingGroup) error {
	return nil
}

// ValidateTaxBusPostingGroup reads the Gen. Business Posting Group entered in a field (a blank
// code is a new, empty group) and tells whether its default tax posting group is filled in
// automatically (Auto Insert Default) — used when that field is validated
func ValidateTaxBusPostingGroup(group *gtables.GenBusinessPostingGroup, entered types.Code) (bool, error) {
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
