package reports

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 1299 Delete Empty Project Registers: the project registers without project ledger
// entries in their range are deleted (deleteEmptyRegisters)
const DeleteEmptyProjectRegistersID = 1299

func init() {
	Register(DeleteEmptyProjectRegistersID, "delete-empty-project-registers", func() Report {
		r := &deleteEmptyRegisters{name: "Project_Register", table: gtables.ProjectRegisterTableName}
		r.trigger = Trigger(func(projectReg *gtables.ProjectRegister, run *Run) error {
			projectLedgEntry := gtables.NewProjectLedgerEntry(run)
			projectLedgEntry.SetRange("entry_no", projectReg.From_entry_no, projectReg.To_entry_no)
			if projectLedgEntry.FindFirst() {
				return Skip()
			}
			return r.deleteRegister(projectReg.No, projectReg.Creation_date, projectReg.DeleteOrError(false))
		})
		return r
	})
}
