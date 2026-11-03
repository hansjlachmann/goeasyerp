package tables

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// projectsSetupLogic is the business logic of table 315 Projects_Setup.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/projectssetup_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type projectsSetupLogic struct{}

func init() { gtables.RegisterProjectsSetupLogic(projectsSetupLogic{}) }

// OnInsert runs before a new record is inserted
func (projectsSetupLogic) OnInsert(rec *gtables.ProjectsSetup) error {
	return nil
}

// OnModify runs before a record is modified
func (projectsSetupLogic) OnModify(rec *gtables.ProjectsSetup) error {
	return nil
}

// OnDelete runs before a record is deleted
func (projectsSetupLogic) OnDelete(rec *gtables.ProjectsSetup) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (projectsSetupLogic) OnRename(rec *gtables.ProjectsSetup) error {
	return nil
}
