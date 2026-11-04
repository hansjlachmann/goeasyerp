package itemjnl

import (
	"testing"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// testItem inserts an item with a base unit PCS and a BOX of 5
func testItem(t *testing.T, src ftables.Source, no string, costing gtables.ItemCosting_method, standardCost string) {
	t.Helper()
	item := gtables.NewItem(src)
	item.InitRecord()
	item.No, item.Description = types.Code(no), types.Text("Test item "+no)
	item.Inventory_posting_group, item.Gen_prod_posting_group = "RESALE", "RETAIL"
	item.Unit_price = dec("200")
	must(t, item.InsertOrError(true))
	for code, qty := range map[string]int64{"PCS": 1, "BOX": 5} {
		uom := gtables.NewItemUnitOfMeasure(src)
		uom.InitRecord()
		uom.Item_no, uom.Code, uom.Qty_per_unit_of_measure = types.Code(no), types.Code(code), types.NewDecimalFromInt(qty)
		must(t, uom.InsertOrError(false))
	}
	must(t, item.Validate("base_unit_of_measure", types.Code("PCS")))
	must(t, item.Validate("costing_method", costing))
	if standardCost != "" {
		must(t, item.Validate("standard_cost", dec(standardCost)))
	}
	must(t, item.ModifyOrError(true))
}

type itemLine struct {
	entryType   gtables.ItemJournalLineEntry_type
	itemNo      string
	qty         string
	unitAmount  string // purchases: the price paid ("" = the item's)
	uom         string
	appliesTo   int
	day         int
	documentNo  string
	newLocation string
	serialNo    string
	lotNo       string
	newSerialNo string
	newLotNo    string
	binCode     string
	newBinCode  string
}

// postItemLines enters lines in the ITEM journal's DEFAULT batch and posts the batch in one
// transaction (codeunit 23)
func postItemLines(t *testing.T, src ftables.Source, lines ...itemLine) (PostBatchResult, error) {
	t.Helper()
	for i, l := range lines {
		line := gtables.NewItemJournalLine(src)
		line.InitRecord()
		line.Journal_template_name, line.Journal_batch_name, line.Line_no = "ITEM", "DEFAULT", (i+1)*10000
		line.Source_code = "ITEMJNL"
		day := l.day
		if day == 0 {
			day = 10
		}
		must(t, line.Validate("posting_date", date(t, day, 10, 2026)))
		line.Document_no = "T00001"
		if l.documentNo != "" {
			line.Document_no = types.Code(l.documentNo)
		}
		must(t, line.Validate("entry_type", l.entryType))
		must(t, line.Validate("item_no", types.Code(l.itemNo)))
		must(t, line.Validate("location_code", types.Code("MAIN")))
		if l.uom != "" {
			must(t, line.Validate("unit_of_measure_code", types.Code(l.uom)))
		}
		must(t, line.Validate("quantity", dec(l.qty)))
		if l.unitAmount != "" {
			must(t, line.Validate("unit_amount", dec(l.unitAmount)))
		}
		if l.appliesTo != 0 {
			must(t, line.Validate("applies_to_entry", l.appliesTo))
		}
		if l.newLocation != "" {
			must(t, line.Validate("new_location_code", types.Code(l.newLocation)))
		}
		line.Serial_no, line.Lot_no = types.Code(l.serialNo), types.Code(l.lotNo)
		line.Bin_code, line.New_bin_code = types.Code(l.binCode), types.Code(l.newBinCode)
		line.New_serial_no, line.New_lot_no = types.Code(l.newSerialNo), types.Code(l.newLotNo)
		must(t, line.InsertOrError(true))
	}
	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
	line := gtables.NewItemJournalLine(txSrc)
	line.Journal_template_name, line.Journal_batch_name = "ITEM", "DEFAULT"
	result, err := NewItemJnlPostBatch(txSrc).Run(line)
	if err != nil {
		_ = ftables.Rollback(tx)
		return result, err
	}
	return result, ftables.Commit(tx)
}

// entries returns the item's ledger entries in entry order
func entries(src ftables.Source, itemNo string) []*gtables.ItemLedgerEntry {
	var list []*gtables.ItemLedgerEntry
	e := gtables.NewItemLedgerEntry(src)
	e.SetRange("item_no", itemNo)
	for found := e.FindSet(); found; found = e.Next() {
		c := gtables.NewItemLedgerEntry(src)
		c.Copy(e)
		list = append(list, c)
	}
	return list
}

// glDocument is the balance of a G/L account from one document
func glDocument(src ftables.Source, account, documentNo string) types.Decimal {
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("gl_account_no", account)
	glEntry.SetRange("document_no", documentNo)
	_ = glEntry.CalcSums("amount")
	return glEntry.Amount.Round(2)
}

// Two purchases (10 at 100, 10 at 120) and then a sale of 15 by costing method: FIFO takes the
// sale's quantity from the first purchase on, LIFO from the last; the sale costs the item's unit
// cost when it is entered (the average cost 110; Standard: the standard cost) — the actual
// FIFO/LIFO cost comes with Adjust Cost - Item Entries. The item's last direct and average
// cost, the inventory value and COGS in the G/L follow.
func TestCostingMethods(t *testing.T) {
	cases := []struct {
		name             string
		costing          gtables.ItemCosting_method
		standard         string
		cogs             string
		remain1, remain2 string
	}{
		{"FIFO", 0, "", "1650", "0", "5"},
		{"LIFO", 1, "", "1650", "5", "0"},
		{"Average", 3, "", "1650", "0", "5"},
		{"Standard", 4, "105", "1575", "0", "5"},
	}
	for _, c := range cases {
		src := demoCompany(t)
		testItem(t, src, "T1", c.costing, c.standard)
		result, err := postItemLines(t, src,
			itemLine{entryType: gtables.ItemJournalLine_Entry_type.Purchase, itemNo: "T1", qty: "10", unitAmount: "100", day: 1},
			itemLine{entryType: gtables.ItemJournalLine_Entry_type.Purchase, itemNo: "T1", qty: "10", unitAmount: "120", day: 2})
		must(t, err)
		if result.RegisterNo == 0 {
			t.Fatalf("%s: no register", c.name)
		}
		_, err = postItemLines(t, src, itemLine{entryType: gtables.ItemJournalLine_Entry_type.Sale, itemNo: "T1", qty: "15", day: 3, documentNo: "T00002"})
		must(t, err)
		list := entries(src, "T1")
		if len(list) != 3 {
			t.Fatalf("%s: %d entries", c.name, len(list))
		}
		sale := list[2]
		if !sale.Adjusted_cost_invoiced_qty.Neg().Equal(dec(c.cogs)) || !glDocument(src, "4135", "T00002").Equal(dec(c.cogs)) {
			t.Errorf("%s: sale cost %s, COGS %s, want %s", c.name, sale.Adjusted_cost_invoiced_qty.Neg(), glDocument(src, "4135", "T00002"), c.cogs)
		}
		purchases := dec("2200")
		if c.costing == 4 {
			purchases = dec("2100")
		}
		if !glDocument(src, "1333", "T00001").Equal(purchases) || !glDocument(src, "1333", "T00002").Equal(dec(c.cogs).Neg()) {
			t.Errorf("%s: inventory %s / %s", c.name, glDocument(src, "1333", "T00001"), glDocument(src, "1333", "T00002"))
		}
		if !list[0].Remaining_quantity.Equal(dec(c.remain1)) || !list[1].Remaining_quantity.Equal(dec(c.remain2)) || sale.Open {
			t.Errorf("%s: remaining %s / %s, sale open %v", c.name, list[0].Remaining_quantity, list[1].Remaining_quantity, sale.Open)
		}
		item := gtables.NewItem(src)
		must(t, item.GetOrError(types.Code("T1")))
		if c.costing != 4 && (!item.Last_direct_cost.Equal(dec("120")) || !item.Average_cost.Equal(dec("110"))) {
			t.Errorf("%s: last direct cost %s, average cost %s", c.name, item.Last_direct_cost, item.Average_cost)
		}
		// the posted lines are gone (the batch has a number series: no new empty line)
		lines := gtables.NewItemJournalLine(src)
		lines.SetRange("journal_template_name", "ITEM")
		if lines.Count() != 0 {
			t.Errorf("%s: %d lines left", c.name, lines.Count())
		}
		// application entries: the two purchases (themselves) and the sale's two applications
		appl := gtables.NewItemApplicationEntry(src)
		appl.SetRange("item_no", "T1")
		if appl.Count() != 4 {
			t.Errorf("%s: %d application entries", c.name, appl.Count())
		}
	}
}

// Applies-to Entry: the sale takes its cost from the chosen purchase; a unit of measure posts
// in the base unit
func TestAppliesToAndUnitOfMeasure(t *testing.T) {
	src := demoCompany(t)
	testItem(t, src, "T2", 0, "")
	_, err := postItemLines(t, src,
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Purchase, itemNo: "T2", qty: "2", unitAmount: "500", uom: "BOX", day: 1},
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Purchase, itemNo: "T2", qty: "10", unitAmount: "150", day: 2})
	must(t, err)
	list := entries(src, "T2")
	if len(list) != 2 || !list[0].Quantity.Equal(dec("10")) || !list[0].Unit_cost.Equal(dec("100")) || list[0].Unit_of_measure_code != "BOX" {
		t.Fatalf("box purchase: qty %s unit cost %s uom %s", list[0].Quantity, list[0].Unit_cost, list[0].Unit_of_measure_code)
	}
	_, err = postItemLines(t, src,
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Sale, itemNo: "T2", qty: "4", appliesTo: list[1].Entry_no, documentNo: "T00002"})
	must(t, err)
	list = entries(src, "T2")
	if !list[2].Adjusted_cost_invoiced_qty.Equal(dec("-500")) || !list[1].Remaining_quantity.Equal(dec("6")) || !list[0].Remaining_quantity.Equal(dec("10")) {
		t.Errorf("applied sale cost %s, remaining %s / %s", list[2].Adjusted_cost_invoiced_qty, list[0].Remaining_quantity, list[1].Remaining_quantity)
	}
}

// A sale without inventory leaves an open negative entry; the next purchase closes it
func TestNegativeInventory(t *testing.T) {
	src := demoCompany(t)
	testItem(t, src, "T3", 0, "")
	_, err := postItemLines(t, src,
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Sale, itemNo: "T3", qty: "3", day: 1},
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Purchase, itemNo: "T3", qty: "5", unitAmount: "80", day: 2})
	must(t, err)
	list := entries(src, "T3")
	if list[0].Open || !list[1].Remaining_quantity.Equal(dec("2")) {
		t.Errorf("sale open %v, purchase remaining %s", list[0].Open, list[1].Remaining_quantity)
	}
}

// Reserve Always (no reservations yet) and Costing Method Specific (no serial numbers yet) are
// refused; nothing is posted
func TestRefusedItems(t *testing.T) {
	src := demoCompany(t)
	testItem(t, src, "T4", 2, "")
	_, err := postItemLines(t, src,
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Purchase, itemNo: "T4", qty: "5", unitAmount: "80"},
		itemLine{entryType: gtables.ItemJournalLine_Entry_type.Sale, itemNo: "T4", qty: "1"})
	if err == nil {
		t.Error("a sale of a Specific item was posted")
	}
	if len(entries(src, "T4")) != 0 {
		t.Error("entries written by a refused posting")
	}
	lines := gtables.NewItemJournalLine(src)
	lines.DeleteAll()
	testItem(t, src, "T5", 0, "")
	item := gtables.NewItem(src)
	must(t, item.GetOrError(types.Code("T5")))
	item.Reserve = gtables.Item_Reserve.Always
	must(t, item.ModifyOrError(false))
	_, err = postItemLines(t, src, itemLine{entryType: gtables.ItemJournalLine_Entry_type.Sale, itemNo: "T5", qty: "1"})
	if err == nil {
		t.Error("a sale of a Reserve Always item was posted")
	}
}
