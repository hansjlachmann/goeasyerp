package reports

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 1199 Delete Empty Res. Registers: the resource registers without resource ledger entries
// in their range are deleted (deleteEmptyRegisters)
const DeleteEmptyResRegistersID = 1199

func init() {
	Register(DeleteEmptyResRegistersID, "delete-empty-res-registers", func() Report {
		r := &deleteEmptyRegisters{name: "Resource_Register", table: gtables.ResourceRegisterTableName}
		r.trigger = Trigger(func(resReg *gtables.ResourceRegister, run *Run) error {
			resLedgEntry := gtables.NewResLedgerEntry(run)
			resLedgEntry.SetRange("entry_no", resReg.From_entry_no, resReg.To_entry_no)
			if resLedgEntry.FindFirst() {
				return Skip()
			}
			return r.deleteRegister(resReg.No, resReg.Creation_date, resReg.DeleteOrError(false))
		})
		return r
	})
}
