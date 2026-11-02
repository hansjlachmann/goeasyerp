package reports

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 899 Delete Empty BOM Registers: the BOM registers without BOM ledger entries in their
// range are deleted (deleteEmptyRegisters)
const DeleteEmptyBOMRegistersID = 899

func init() {
	Register(DeleteEmptyBOMRegistersID, "delete-empty-bom-registers", func() Report {
		r := &deleteEmptyRegisters{name: "BOM_Register", table: gtables.BOMRegisterTableName}
		r.trigger = Trigger(func(bomReg *gtables.BOMRegister, run *Run) error {
			bomLedgEntry := gtables.NewBOMLedgerEntry(run)
			bomLedgEntry.SetRange("entry_no", bomReg.From_entry_no, bomReg.To_entry_no)
			if bomLedgEntry.FindFirst() {
				return Skip()
			}
			return r.deleteRegister(bomReg.No, bomReg.Creation_date, bomReg.DeleteOrError(false))
		})
		return r
	})
}
