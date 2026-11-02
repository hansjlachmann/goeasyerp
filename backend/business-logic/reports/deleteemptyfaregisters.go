package reports

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 5693 Delete Empty FA Registers: the FA registers without FA or maintenance ledger
// entries in their ranges are deleted (deleteEmptyRegisters)
const DeleteEmptyFARegistersID = 5693

func init() {
	Register(DeleteEmptyFARegistersID, "delete-empty-fa-registers", func() Report {
		r := &deleteEmptyRegisters{name: "FA_Register", table: gtables.FARegisterTableName}
		r.trigger = Trigger(func(faReg *gtables.FARegister, run *Run) error {
			faLedgEntry := gtables.NewFALedgerEntry(run)
			faLedgEntry.SetRange("entry_no", faReg.From_entry_no, faReg.To_entry_no)
			if faLedgEntry.FindFirst() {
				return Skip()
			}
			maintenanceLedgEntry := gtables.NewMaintenanceLedgerEntry(run)
			maintenanceLedgEntry.SetRange("entry_no", faReg.From_maintenance_entry_no, faReg.To_maintenance_entry_no)
			if maintenanceLedgEntry.FindFirst() {
				return Skip()
			}
			return r.deleteRegister(faReg.No, faReg.Creation_date, faReg.DeleteOrError(false))
		})
		return r
	})
}
