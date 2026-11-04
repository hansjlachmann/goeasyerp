package bomjnl_test

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/bomjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

// demoCompany is a company with the demo data: items with a year of postings and the demo
// bills of materials assembled every quarter
func demoCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	counts, err := demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
		BOMJnlPostLine:  func(src ftables.Source) demodata.BOMJnlPoster { return bomjnl.NewBOMJnlPostLine(src) },
	})
	must(t, err)
	if counts.BOMLedgerEntries != 8 {
		t.Errorf("%d demo assemblies, want 8", counts.BOMLedgerEntries)
	}
	return ftables.DBSource(db, "demo", database.DBTypeSQLite)
}

// inTx runs a posting in a transaction, as the posting codeunits do
func inTx(t *testing.T, src ftables.Source, post func(txSrc ftables.Source) error) {
	t.Helper()
	tx, err := ftables.BeginTransaction(src.GetDB())
	must(t, err)
	err = post(ftables.DBSource(tx, src.GetCompany(), src.GetDBType()))
	if err != nil {
		_ = ftables.Rollback(tx)
		t.Fatal(err)
	}
	must(t, ftables.Commit(tx))
}

func qtyOnHand(t *testing.T, src ftables.Source, itemNo string) types.Decimal {
	t.Helper()
	item := gtables.NewItem(src)
	must(t, item.GetOrError(types.Code(itemNo)))
	item.CalcFields("quantity_on_hand")
	return item.Quantity_on_hand
}

// Posting a BOM journal line: the assembled item comes in, each component goes out (items as
// negative adjustments, the resource as usage), BOM ledger entries point at them in one
// register, and the assembled item's entry costs what its components cost
func TestPostBOMJournal(t *testing.T) {
	src := demoCompany(t)
	before := map[string]types.Decimal{}
	for _, no := range []string{"2100", "2000", "2010", "2030", "1200"} {
		before[no] = qtyOnHand(t, src, no)
	}

	line := gtables.NewBOMJournalLine(src)
	line.InitRecord()
	line.Journal_template_name, line.Journal_batch_name, line.Line_no = "BOM", "DEFAULT", 10000
	must(t, tables.SetUpNewBOMJnlLine(line, gtables.NewBOMJournalLine(src)))
	must(t, line.Validate("posting_date", types.NewDate(2026, 10, 6)))
	must(t, line.Validate("item_no", types.Code("2100")))
	must(t, line.Validate("location_code", types.Code("MAIN")))
	must(t, line.Validate("quantity", dec("4")))
	if line.Document_no != "B00001" || line.Unit_of_measure_code != "PCS" || !line.Quantity_base.Equal(dec("4")) || line.Description != "Coffee gift basket" || line.Source_code != "BOMJNL" {
		t.Errorf("line: %s %s %q %s", line.Unit_of_measure_code, line.Quantity_base, line.Description, line.Source_code)
	}
	must(t, line.InsertOrError(true))

	var result bomjnl.PostBatchResult
	inTx(t, src, func(txSrc ftables.Source) error {
		l := gtables.NewBOMJournalLine(txSrc)
		l.Journal_template_name, l.Journal_batch_name = "BOM", "DEFAULT"
		var err error
		result, err = bomjnl.NewBOMJnlPostBatch(txSrc).Run(l)
		return err
	})
	if result.RegisterNo == 0 {
		t.Fatal("nothing posted")
	}
	// 2100: + 4; per basket 0.5 kg coffee, 1 tea, 2 biscuits, 1 box
	for no, change := range map[string]string{"2100": "4", "2000": "-2", "2010": "-4", "2030": "-8", "1200": "-4"} {
		if got := qtyOnHand(t, src, no).Sub(before[no]); !got.Equal(dec(change)) {
			t.Errorf("item %s changed by %s, want %s", no, got, change)
		}
	}
	bomReg := gtables.NewBOMRegister(src)
	must(t, bomReg.GetOrError(result.RegisterNo))
	bomLedgEntry := gtables.NewBOMLedgerEntry(src)
	bomLedgEntry.SetRange("entry_no", bomReg.From_entry_no, bomReg.To_entry_no)
	if bomLedgEntry.Count() != 5 {
		t.Errorf("%d BOM ledger entries in the register, want 5 (the BOM and its 4 components)", bomLedgEntry.Count())
	}
	must(t, bomLedgEntry.GetOrError(bomReg.From_entry_no))
	if bomLedgEntry.Entry_type != gtables.BOMLedgerEntry_Entry_type.BOM || bomLedgEntry.Reference_type != gtables.BOMLedgerEntry_Reference_type.Item {
		t.Errorf("first BOM ledger entry: %v %v", bomLedgEntry.Entry_type, bomLedgEntry.Reference_type)
	}
	// the assembled entry's cost is the components' (the negative adjustments' amounts)
	itemLedgEntry := gtables.NewItemLedgerEntry(src)
	must(t, itemLedgEntry.GetOrError(bomLedgEntry.Reference_entry_no))
	components := gtables.NewItemLedgerEntry(src)
	components.SetRange("document_no", types.Code("B00001"))
	components.SetRange("entry_type", gtables.ItemLedgerEntry_Entry_type.NegativeAdjmt)
	must(t, components.CalcSums("adjusted_cost_qty"))
	if !itemLedgEntry.Adjusted_cost_qty.Equal(components.Adjusted_cost_qty.Neg()) || itemLedgEntry.Adjusted_cost_qty.IsZero() {
		t.Errorf("assembled cost %s, components %s", itemLedgEntry.Adjusted_cost_qty, components.Adjusted_cost_qty)
	}
	if !itemLedgEntry.Cost_posted_to_gl.Equal(itemLedgEntry.Adjusted_cost_qty) {
		t.Errorf("cost posted to G/L %s, adjusted cost %s (Automatic Cost Posting)", itemLedgEntry.Cost_posted_to_gl, itemLedgEntry.Adjusted_cost_qty)
	}
	// the G/L still balances
	glEntry := gtables.NewGLEntry(src)
	must(t, glEntry.CalcSums("amount"))
	if !glEntry.Amount.Round(2).IsZero() {
		t.Errorf("G/L entries add up to %s", glEntry.Amount)
	}
}

// The resource component of the office starter kit is posted as usage
func TestPostBOMWithResource(t *testing.T) {
	src := demoCompany(t)
	resLedgEntry := gtables.NewResLedgerEntry(src)
	resLedgEntry.SetRange("resource_no", types.Code("R0060"))
	resLedgEntry.SetRange("source_code", types.Code("BOMJNL"))
	if resLedgEntry.Count() != 4 { // one per demo assembly of 1500
		t.Errorf("%d resource entries from the demo assemblies, want 4", resLedgEntry.Count())
	}
}

// Explode BOM (codeunit 51): a component that is a bill of materials becomes its components;
// Calc. Standard Cost (codeunit 54, this level) sums the components' unit costs
func TestExplodeBOMAndCalcCost(t *testing.T) {
	src := demoCompany(t)
	// a box of 2 starter kits
	comp := gtables.NewBOMComponent(src)
	comp.Parent_item_no, comp.Line_no = "1210", 10000
	must(t, comp.Validate("type", gtables.BOMComponent_Type.Item))
	must(t, comp.Validate("no", types.Code("1500")))
	comp.Quantity_per = dec("2")
	comp.Position = "K%1"
	must(t, comp.InsertOrError(true))
	if !comp.Bill_of_materials || comp.Description != "Office starter kit" {
		t.Errorf("component: BOM %v %q", comp.Bill_of_materials, comp.Description)
	}
	_, err := bomjnl.ExplodeBOM(comp)
	must(t, err)
	comps := gtables.NewBOMComponent(src)
	comps.SetRange("parent_item_no", types.Code("1210"))
	if comps.Count() != 6 {
		t.Fatalf("%d components after the explode, want the kit's 6", comps.Count())
	}
	comps.SetRange("no", types.Code("1100"))
	if !comps.FindFirst() || !comps.Quantity_per.Equal(dec("10")) || comps.Position != "KA" {
		t.Errorf("pens per box %s, position %s; want 10 (2 kits × 5), KA", comps.Quantity_per, comps.Position)
	}

	// Calc. Standard Cost of the gift basket (this level):
	// 0.5 coffee + 1 tea + 2 biscuits + 1 box at their unit costs
	want := types.Decimal{}
	for no, per := range map[string]string{"2000": "0.5", "2010": "1", "2030": "2", "1200": "1"} {
		item := gtables.NewItem(src)
		must(t, item.GetOrError(types.Code(no)))
		want = want.Add(item.Unit_cost.Mul(dec(per)))
	}
	basket := gtables.NewItem(src)
	must(t, basket.GetOrError(types.Code("2100")))
	must(t, bomjnl.CalcItemStandardCost(basket, false))
	must(t, basket.GetOrError(types.Code("2100")))
	if !basket.Standard_cost.Equal(want.Round(5)) {
		t.Errorf("standard cost %s, want %s", basket.Standard_cost, want.Round(5))
	}
}
