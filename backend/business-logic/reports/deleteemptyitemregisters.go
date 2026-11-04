package reports

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 799 Delete Empty Item Registers: the item registers without item or phys. inventory
// ledger entries in their ranges are deleted (deleteEmptyRegisters)
const DeleteEmptyItemRegistersID = 799

func init() {
	Register(DeleteEmptyItemRegistersID, "delete-empty-item-registers", func() Report {
		r := &deleteEmptyRegisters{name: "Item_Register", table: gtables.ItemRegisterTableName}
		r.trigger = Trigger(func(itemReg *gtables.ItemRegister, run *Run) error {
			itemLedgEntry := gtables.NewItemLedgerEntry(run)
			itemLedgEntry.SetRange("entry_no", itemReg.From_entry_no, itemReg.To_entry_no)
			physInvtLedgEntry := gtables.NewPhysInventoryLedgerEntry(run)
			physInvtLedgEntry.SetRange("entry_no", itemReg.From_phys_inventory_entry_no, itemReg.To_phys_inventory_entry_no)
			if itemLedgEntry.FindFirst() || physInvtLedgEntry.FindFirst() {
				return Skip()
			}
			return r.deleteRegister(itemReg.No, itemReg.Creation_date, itemReg.DeleteOrError(false))
		})
		return r
	})
}
