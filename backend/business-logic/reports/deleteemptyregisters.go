package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
)

// The Delete Empty … Registers reports (99 G/L, 799 Item, 899 BOM, 1199 Resource, 5693 FA, 5695
// Insurance Registers, filter Creation Date) delete the registers whose entries are all gone
// after date compressions; each has its file with its register's check.

// deleteEmptyRegisters is a Delete Empty … Registers report: the data item over the registers
// (by No.) asks first and deletes those whose OnAfterGetRecord trigger finds no entries
type deleteEmptyRegisters struct {
	name        string
	table       string
	trigger     TriggerFunc
	noOfDeleted int
	window      *system.Window
}

func (r *deleteEmptyRegisters) Definition() *Definition {
	return &Definition{
		Transaction: true,
		DataItems: []*DataItem{{
			Name: r.name, Table: r.table, Key: []string{"no"},
			RequestFilterFields: []string{"creation_date"},
			OnPreDataItem: func(rec interface{}, run *Run) error {
				if system.GuiAllowed() && !system.Confirm(run.Text("CONFIRM")) {
					return Break()
				}
				r.window = system.OpenWindow(run.Text("WINDOW"))
				return nil
			},
			OnAfterGetRecord: r.trigger,
			OnPostDataItem: func(rec interface{}, run *Run) error {
				r.window.Close()
				return nil
			},
		}},
	}
}

// deleteRegister: the register deleted (err: the delete's error), counted in the window
func (r *deleteEmptyRegisters) deleteRegister(no int, creationDate interface{}, err error) error {
	if err != nil {
		return err
	}
	r.noOfDeleted++
	r.window.Update(1, no)
	r.window.Update(2, creationDate)
	r.window.Update(3, r.noOfDeleted)
	return nil
}
