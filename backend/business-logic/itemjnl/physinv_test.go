package itemjnl

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// quantityAt is an item's quantity on hand at a location
func quantityAt(t *testing.T, src ftables.Source, itemNo, location string) types.Decimal {
	t.Helper()
	item := gtables.NewItem(src)
	must(t, item.GetOrError(types.Code(itemNo)))
	must(t, item.SetFlowFilter("location_filter", location))
	item.CalcFields("quantity_on_hand")
	return item.Quantity_on_hand
}

// A transfer moves the quantity to the new location with its cost: an outbound entry applied to
// the inventory and an inbound entry at the new location; the inventory value stays
func TestTransfer(t *testing.T) {
	src := demoCompany(t)
	testItem(t, src, "T6", 0, "")
	_, err := postItemLines(t, src,
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Purchase, itemNo: "T6", qty: "10", unitAmount: "100", day: 1})
	must(t, err)
	_, err = postItemLines(t, src,
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Transfer, itemNo: "T6", qty: "4", day: 2, documentNo: "T00002", newLocation: "OSLO"})
	must(t, err)
	if !quantityAt(t, src, "T6", "MAIN").Equal(dec("6")) || !quantityAt(t, src, "T6", "OSLO").Equal(dec("4")) {
		t.Errorf("MAIN %s, OSLO %s", quantityAt(t, src, "T6", "MAIN"), quantityAt(t, src, "T6", "OSLO"))
	}
	list := entries(src, "T6")
	if len(list) != 3 || list[2].Location_code != "OSLO" || !list[2].Remaining_quantity.Equal(dec("4")) || !list[2].Unit_cost.Equal(dec("100")) {
		t.Fatalf("transfer entries %d: %+v", len(list), list[len(list)-1])
	}
	if !glDocument(src, "1333", "T00002").IsZero() {
		t.Errorf("a transfer changed the inventory value: %s", glDocument(src, "1333", "T00002"))
	}
	// more than on inventory cannot be transferred
	_, err = postItemLines(t, src,
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Transfer, itemNo: "T6", qty: "9", day: 3, documentNo: "T00003", newLocation: "OSLO"})
	if err == nil {
		t.Error("a transfer of more than the inventory was posted")
	}
}

// Calculate Inventory (report 790) fills the phys. inventory journal with the quantities on
// inventory per location; a counted quantity below gives a negative adjustment, the posting
// writes phys. inventory entries and the register's range
func TestPhysInventory(t *testing.T) {
	src := demoCompany(t)
	testItem(t, src, "T7", 0, "")
	_, err := postItemLines(t, src,
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Purchase, itemNo: "T7", qty: "12", unitAmount: "50", day: 1})
	must(t, err)

	context := gtables.NewItemJournalLine(src)
	context.Journal_template_name, context.Journal_batch_name = "PHYS. INV.", "DEFAULT"
	item := gtables.NewItem(src)
	item.SetRange("no", "T7")
	_, err = reports.Execute(reports.CalculateInventoryID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Context: context, Record: item,
		Options: map[string]string{"posting_date": date(t, 20, 10, 2026).String(), "by_location": "true"}})
	must(t, err)
	line := gtables.NewItemJournalLine(src)
	line.SetRange("journal_template_name", "PHYS. INV.")
	if line.Count() != 1 || !line.FindFirst() {
		t.Fatalf("%d calculated lines", line.Count())
	}
	if !line.Phys_inventory || !line.Qty_calculated.Equal(dec("12")) || !line.Qty_phys_inventory.Equal(dec("12")) || line.Location_code != "MAIN" ||
		line.Document_no == "" || !line.Quantity.IsZero() {
		t.Fatalf("calculated line: phys %v calc %s counted %s loc %s doc %s qty %s", line.Phys_inventory, line.Qty_calculated, line.Qty_phys_inventory,
			line.Location_code, line.Document_no, line.Quantity)
	}
	// the user counts 10
	must(t, line.Validate("qty_phys_inventory", dec("10")))
	if line.Entry_type != gtables.ItemJournalLine_Entry_type.NegativeAdjmt || !line.Quantity.Equal(dec("2")) {
		t.Fatalf("counted: entry type %v quantity %s", line.Entry_type, line.Quantity)
	}
	must(t, line.ModifyOrError(true))

	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
	batchLine := gtables.NewItemJournalLine(txSrc)
	batchLine.Journal_template_name, batchLine.Journal_batch_name = "PHYS. INV.", "DEFAULT"
	result, err := NewItemJnlPostBatch(txSrc).Run(batchLine)
	if err != nil {
		_ = ftables.Rollback(tx)
		t.Fatal(err)
	}
	must(t, ftables.Commit(tx))

	if !quantityAt(t, src, "T7", "").Equal(dec("10")) {
		t.Errorf("on hand after the count %s", quantityAt(t, src, "T7", ""))
	}
	physEntry := gtables.NewPhysInventoryLedgerEntry(src)
	physEntry.SetRange("item_no", "T7")
	if physEntry.Count() != 1 || !physEntry.FindFirst() || !physEntry.Qty_calculated.Equal(dec("12")) || !physEntry.Qty_phys_inventory.Equal(dec("10")) {
		t.Errorf("phys. inventory entries %d", physEntry.Count())
	}
	itemReg := gtables.NewItemRegister(src)
	must(t, itemReg.GetOrError(result.RegisterNo))
	if itemReg.From_phys_inventory_entry_no != physEntry.Entry_no || itemReg.To_phys_inventory_entry_no != physEntry.Entry_no || itemReg.From_entry_no == 0 {
		t.Errorf("register %+v", itemReg)
	}
}
