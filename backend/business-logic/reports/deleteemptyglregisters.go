package reports

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 99 Delete Empty G/L Registers: the G/L registers without G/L, customer, vendor, tax or
// bank account entries in their range are deleted (deleteEmptyRegisters)
const DeleteEmptyGLRegistersID = 99

func init() {
	Register(DeleteEmptyGLRegistersID, "delete-empty-g-l-registers", func() Report {
		r := &deleteEmptyRegisters{name: "G_L_Register", table: gtables.GLRegisterTableName}
		r.trigger = Trigger(func(glReg *gtables.GLRegister, run *Run) error {
			glEntry := gtables.NewGLEntry(run)
			glEntry.SetRange("entry_no", glReg.From_entry_no, glReg.To_entry_no)
			custLedgEntry := gtables.NewCustomerLedgerEntry(run)
			custLedgEntry.SetRange("entry_no", glReg.From_entry_no, glReg.To_entry_no)
			vendLedgEntry := gtables.NewVendorLedgerEntry(run)
			vendLedgEntry.SetRange("entry_no", glReg.From_entry_no, glReg.To_entry_no)
			taxEntry := gtables.NewTaxEntry(run)
			taxEntry.SetRange("entry_no", glReg.From_entry_no, glReg.To_entry_no)
			bankAccLedgEntry := gtables.NewBankAccountLedgerEntry(run)
			bankAccLedgEntry.SetRange("entry_no", glReg.From_entry_no, glReg.To_entry_no)
			if glEntry.FindFirst() || custLedgEntry.FindFirst() || vendLedgEntry.FindFirst() || taxEntry.FindFirst() || bankAccLedgEntry.FindFirst() {
				return Skip()
			}
			return r.deleteRegister(glReg.No, glReg.Creation_date, glReg.DeleteOrError(false))
		})
		return r
	})
}
