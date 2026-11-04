package reports_test

import (
	"database/sql"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// costCompany is a demo company (items with a year of postings, Automatic Cost Posting) plus
// item T1, a FIFO copy of item 1000
func costCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite))
	_, err = demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{
		Today:           time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local),
		ItemJnlPostLine: func(src ftables.Source) demodata.ItemJnlPoster { return itemjnl.NewItemJnlPostLine(src) },
	})
	mustOK(t, err)
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	item1000 := gtables.NewItem(src)
	mustOK(t, item1000.GetOrError(types.Code("1000")))
	item := gtables.NewItem(src)
	item.TransferFields(item1000)
	item.No, item.Costing_method = "T1", gtables.Item_Costing_method.FIFO
	item.Unit_cost, item.Average_cost, item.Last_direct_cost = types.MustDecimal("90"), types.MustDecimal("90"), types.MustDecimal("90")
	mustOK(t, item.InsertOrError(false))
	uom := gtables.NewItemUnitOfMeasure(src)
	uom.Item_no, uom.Code, uom.Qty_per_unit_of_measure = "T1", item.Base_unit_of_measure, types.NewDecimalFromInt(1)
	mustOK(t, uom.InsertOrError(false))
	return src
}

// postItem posts an item journal line in a transaction
func postItem(t *testing.T, src ftables.Source, entryType gtables.ItemJournalLineEntry_type, qty string, unitAmount string) {
	t.Helper()
	tx, err := ftables.BeginTransaction(src.GetDB())
	mustOK(t, err)
	txSrc := ftables.DBSource(tx, src.GetCompany(), src.GetDBType())
	line := gtables.NewItemJournalLine(txSrc)
	line.InitRecord()
	line.Journal_template_name, line.Journal_batch_name, line.Source_code = "ITEM", "DEFAULT", "ITEMJNL"
	line.Document_no = "T-COST"
	for _, step := range []struct {
		field string
		value interface{}
	}{
		{"posting_date", types.NewDate(2026, 10, 5)},
		{"entry_type", entryType},
		{"item_no", types.Code("T1")},
		{"location_code", types.Code("MAIN")},
		{"quantity", types.MustDecimal(qty)},
	} {
		mustOK(t, line.Validate(step.field, step.value))
	}
	if unitAmount != "" {
		mustOK(t, line.Validate("unit_amount", types.MustDecimal(unitAmount)))
	}
	err = itemjnl.NewItemJnlPostLine(txSrc).Run(line)
	if err != nil {
		_ = ftables.Rollback(tx)
		t.Fatal(err)
	}
	mustOK(t, ftables.Commit(tx))
}

func lastEntry(t *testing.T, src ftables.Source) *gtables.ItemLedgerEntry {
	t.Helper()
	e := gtables.NewItemLedgerEntry(src)
	e.SetRange("item_no", types.Code("T1"))
	if !e.FindLast() {
		t.Fatal("no entry of T1")
	}
	return e
}

func glBalance(src ftables.Source, account string) types.Decimal {
	glEntry := gtables.NewGLEntry(src)
	glEntry.SetRange("gl_account_no", types.Code(account))
	_ = glEntry.CalcSums("amount")
	return glEntry.Amount.Round(2)
}

// A FIFO sale posted at the item's unit cost (the average 110 of purchases at 100 and 120) gets
// the cost of the first purchase it was applied to (100) from Adjust Cost; Post Inventory Cost
// to G/L (for T1's entries) moves the 40 difference from COGS back to inventory, and running it
// again posts nothing
func TestAdjustCostAndPostInventoryCost(t *testing.T) {
	src := costCompany(t)
	postItem(t, src, gtables.ItemJournalLine_Entry_type.Purchase, "10", "100")
	postItem(t, src, gtables.ItemJournalLine_Entry_type.Purchase, "10", "120")
	postItem(t, src, gtables.ItemJournalLine_Entry_type.Sale, "4", "")
	sale := lastEntry(t, src)
	if !sale.Adjusted_cost_qty.Equal(types.MustDecimal("-440")) {
		t.Fatalf("sale posted at %s, want -440 (the item's unit cost, the average 110)", sale.Adjusted_cost_qty)
	}
	invtPostingGr := gtables.NewInventoryPostingGroup(src)
	mustOK(t, invtPostingGr.GetOrError(sale.Inventory_posting_group))
	genPostingSetup := gtables.NewGeneralPostingSetup(src)
	mustOK(t, genPostingSetup.GetOrError(map[string]interface{}{"gen_bus_posting_group": sale.Gen_bus_posting_group, "gen_prod_posting_group": sale.Gen_prod_posting_group}))
	inventoryBefore := glBalance(src, invtPostingGr.Inventory_account.String())
	cogsBefore := glBalance(src, genPostingSetup.Cogs_account.String())

	runReport(t, src, reports.AdjustCostItemEntriesID, nil)
	sale = lastEntry(t, src)
	if !sale.Adjusted_cost_qty.Equal(types.MustDecimal("-400")) || !sale.Adjusted_cost_invoiced_qty.Equal(types.MustDecimal("-400")) || !sale.Cost_is_adjusted {
		t.Errorf("adjusted sale: %s / %s, adjusted %v; want -400, adjusted", sale.Adjusted_cost_qty, sale.Adjusted_cost_invoiced_qty, sale.Cost_is_adjusted)
	}

	postT1 := func() reports.Result {
		t.Helper()
		t1Entries := gtables.NewItemLedgerEntry(src)
		t1Entries.SetRange("item_no", types.Code("T1"))
		result, err := reports.Execute(reports.PostInventoryCostToGLID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
			Record: t1Entries, Options: map[string]string{"post_level": "2", "posting_date": "2026-10-06"}})
		mustOK(t, err)
		return result
	}
	firstGLEntry := gtables.NewGLEntry(src).Count() + 1
	result := postT1()
	// the texts keep their values: the message's amounts, the entries' descriptions
	if !strings.Contains(result.Message, "COGS -40, inventory adjustments 0.") {
		t.Errorf("message %q without the posted amounts", result.Message)
	}
	posted := gtables.NewGLEntry(src)
	posted.SetFilter("entry_no", strconv.Itoa(firstGLEntry)+"..")
	for found := posted.FindSet(); found; found = posted.Next() {
		if !strings.HasPrefix(posted.Description.String(), "Sale ") || !strings.HasSuffix(posted.Description.String(), " on 2026-10-05") {
			t.Errorf("G/L entry %d description %q lost its values", posted.Entry_no, posted.Description)
		}
	}
	sale = lastEntry(t, src)
	if !sale.Cost_posted_to_gl.Equal(types.MustDecimal("-400")) {
		t.Errorf("cost posted to G/L %s, want -400", sale.Cost_posted_to_gl)
	}
	if got := glBalance(src, invtPostingGr.Inventory_account.String()).Sub(inventoryBefore); !got.Equal(types.MustDecimal("40")) {
		t.Errorf("inventory account changed by %s, want 40", got)
	}
	if got := glBalance(src, genPostingSetup.Cogs_account.String()).Sub(cogsBefore); !got.Equal(types.MustDecimal("-40")) {
		t.Errorf("COGS account changed by %s, want -40", got)
	}
	glEntry := gtables.NewGLEntry(src)
	mustOK(t, glEntry.CalcSums("amount"))
	if !glEntry.Amount.Round(2).IsZero() {
		t.Errorf("G/L entries add up to %s", glEntry.Amount)
	}

	// again: nothing left to post
	before := gtables.NewGLEntry(src).Count()
	postT1()
	if after := gtables.NewGLEntry(src).Count(); after != before {
		t.Errorf("a second run posted %d G/L entries", after-before)
	}
}

// Per inventory posting group a document no. is required; per entry it is refused
func TestPostInventoryCostDocumentNo(t *testing.T) {
	src := resPriceCompany(t)
	_, err := reports.Execute(reports.PostInventoryCostToGLID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"post_level": "1", "posting_date": "2026-10-06"}})
	if err == nil || !strings.Contains(err.Error(), "Document No.") || !strings.Contains(err.Error(), "per Inventory Posting Group") {
		t.Errorf("per posting group without document no.: %v", err)
	}
	_, err = reports.Execute(reports.PostInventoryCostToGLID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"post_level": "2", "posting_date": "2026-10-06", "document_no": "X"}})
	if err == nil || !strings.Contains(err.Error(), "Document No.") || !strings.Contains(err.Error(), "per Item Ledger Entry") {
		t.Errorf("per entry with document no.: %v", err)
	}
}
