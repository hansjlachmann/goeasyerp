package reports

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Report 5695 Delete Empty Insurance Reg.: the insurance registers without insurance coverage
// ledger entries in their range are deleted (deleteEmptyRegisters)
const DeleteEmptyInsuranceRegistersID = 5695

func init() {
	Register(DeleteEmptyInsuranceRegistersID, "delete-empty-insurance-registers", func() Report {
		r := &deleteEmptyRegisters{name: "Insurance_Register", table: gtables.InsuranceRegisterTableName}
		r.trigger = Trigger(func(insuranceReg *gtables.InsuranceRegister, run *Run) error {
			insCoverageLedgEntry := gtables.NewInsCoverageLedgerEntry(run)
			insCoverageLedgEntry.SetRange("entry_no", insuranceReg.From_entry_no, insuranceReg.To_entry_no)
			if insCoverageLedgEntry.FindFirst() {
				return Skip()
			}
			return r.deleteRegister(insuranceReg.No, insuranceReg.Creation_date, insuranceReg.DeleteOrError(false))
		})
		return r
	})
}
