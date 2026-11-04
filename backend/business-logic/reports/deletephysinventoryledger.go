package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 789 Delete Phys. Inventory Ledger: the phys. inventory ledger entries between the
// Starting and Ending Date of closed fiscal years are deleted.
const DeletePhysInventoryLedgerID = 789

func init() {
	Register(DeletePhysInventoryLedgerID, "delete-phys-inventory-ledger", func() Report { return &deletePhysInventoryLedger{} })
}

type deletePhysInventoryLedger struct {
	dateCompression
	noOfDeleted int
	window      *system.Window
}

func (r *deletePhysInventoryLedger) Definition() *Definition {
	r.tableID = gtables.PhysInventoryLedgerEntryTableID
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options: []Option{
			{Name: "starting_date", Type: "date"},
			{Name: "ending_date", Type: "date", Required: true},
		},
		Transaction: true,
		OnInitReport: func(run *Run) error {
			run.SetOption("ending_date", system.Today().String())
			return nil
		},
		OnPreReport: func(run *Run) error {
			r.read(run)
			return nil
		},
		DataItems: []*DataItem{{
			Name: "Phys_Inventory_Ledger_Entry", Table: gtables.PhysInventoryLedgerEntryTableName,
			RequestFilterFields: []string{"item_no", "inventory_posting_group"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *deletePhysInventoryLedger) onPreDataItem(physInvtLedgEntry *gtables.PhysInventoryLedgerEntry, run *Run) error {
	err := r.confirm(run)
	if err != nil {
		return err
	}
	r.window = system.OpenWindow(run.Text("WINDOW"))
	physInvtLedgEntry2 := gtables.NewPhysInventoryLedgerEntry(run)
	lastEntryNo := 0
	if physInvtLedgEntry2.FindLast() {
		lastEntryNo = physInvtLedgEntry2.Entry_no
	}
	physInvtLedgEntry.SetRange("entry_no", 0, lastEntryNo)
	physInvtLedgEntry.SetRange("posting_date", r.entrdDateComprJnl.Starting_date, r.entrdDateComprJnl.Ending_date)
	return nil
}

// onAfterGetRecord: the entry of a closed fiscal year is deleted
func (r *deletePhysInventoryLedger) onAfterGetRecord(physInvtLedgEntry *gtables.PhysInventoryLedgerEntry, run *Run) error {
	_, err := r.dateComprMgt.GetDateFilter(physInvtLedgEntry.Posting_date, r.entrdDateComprJnl, true)
	if err != nil {
		return err
	}
	r.window.Update(1, physInvtLedgEntry.Item_no)
	r.window.Update(2, physInvtLedgEntry.Posting_date)
	err = physInvtLedgEntry.DeleteOrError(false)
	if err != nil {
		return err
	}
	r.noOfDeleted++
	r.window.Update(3, r.noOfDeleted)
	return nil
}

func (r *deletePhysInventoryLedger) onPostDataItem(physInvtLedgEntry *gtables.PhysInventoryLedgerEntry, run *Run) error {
	r.window.Close()
	return nil
}
