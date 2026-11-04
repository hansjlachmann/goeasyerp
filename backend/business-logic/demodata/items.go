package demodata

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// postDemoItems posts a year of inventory movements of the demo items through the item posting
// (codeunit 22, with Automatic Cost Posting also their value in the G/L): an opening stock a
// year ago and every quarter a purchase and two sales, at the main warehouse
func postDemoItems(src ftables.Source, open func(src ftables.Source) ItemJnlPoster, today time.Time, masterData []tableRecords, counts *Counts, p *progress) error {
	rng := rand.New(rand.NewSource(seed + 27))
	itemJnlPostLine := open(src)
	start := today.AddDate(-1, 0, 1)
	lineNo := 0
	post := func(date time.Time, entryType gtables.ItemJournalLineEntry_type, itemNo, documentNo string, qty int, unitAmount *types.Decimal, serialNo, lotNo string) error {
		lineNo = lineNo + 10000
		line := gtables.NewItemJournalLine(src)
		line.InitRecord()
		line.Journal_template_name = "ITEM"
		line.Journal_batch_name = "DEFAULT"
		line.Line_no = lineNo
		line.Source_code = "ITEMJNL"
		line.Document_no = types.Code(documentNo)
		for _, step := range []struct {
			field string
			value interface{}
		}{
			{"posting_date", types.NewDateFromTime(date)},
			{"entry_type", entryType},
			{"item_no", types.Code(itemNo)},
			{"location_code", types.Code("MAIN")},
			{"quantity", types.NewDecimalFromInt(int64(qty))},
		} {
			err := line.Validate(step.field, step.value)
			if err != nil {
				return fmt.Errorf("demo item journal %s %s: %w", itemNo, step.field, err)
			}
		}
		if unitAmount != nil {
			err := line.Validate("unit_amount", *unitAmount)
			if err != nil {
				return fmt.Errorf("demo item journal %s unit amount: %w", itemNo, err)
			}
		}
		line.Serial_no, line.Lot_no = types.Code(serialNo), types.Code(lotNo)
		err := itemJnlPostLine.Run(line)
		if err != nil {
			return fmt.Errorf("demo item journal %s %s: %w", itemNo, documentNo, err)
		}
		counts.ItemLedgerEntries++
		p.step()
		return nil
	}

	bomItems := demoBOMItems(masterData)
	// an opening stock and every quarter a purchase and two sales per item
	itemCount := 0
	for _, t := range masterData {
		if t.Table == gtables.ItemTableName {
			for _, r := range t.Records {
				if !bomItems[fmt.Sprint(r["no"])] {
					itemCount++
				}
			}
		}
	}
	p.setTotal(13*itemCount + 4)
	for _, t := range masterData {
		if t.Table != gtables.ItemTableName {
			continue
		}
		for _, r := range t.Records {
			itemNo := fmt.Sprint(r["no"])
			if bomItems[itemNo] {
				continue // assembled from its components (postDemoAssemblies)
			}
			cost, err := types.NewDecimalFromString(fmt.Sprint(r["last_direct_cost"]))
			if err != nil {
				return err
			}
			// coffee beans come in lots (sales take the oldest lot first)
			lotNo := func(documentNo string) string {
				if itemNo == demoLotItem {
					return "LOT-" + documentNo
				}
				return ""
			}
			err = post(start, gtables.ItemJournalLine_Entry_type.PositiveAdjmt, itemNo, "OPENING", 40+rng.Intn(160), nil, "", lotNo("OPENING"))
			if err != nil {
				return err
			}
			for q := 0; q < 4; q++ {
				quarter := start.AddDate(0, 3*q, 0)
				// purchases at the last direct cost ±10 %
				price := cost.Mul(types.NewDecimal(0.9 + rng.Float64()*0.2)).Round(2)
				err = post(quarter.AddDate(0, 0, 5), gtables.ItemJournalLine_Entry_type.Purchase, itemNo, fmt.Sprintf("PURCH%d", q+1), 20+rng.Intn(100), &price, "", lotNo(fmt.Sprintf("PURCH%d", q+1)))
				if err != nil {
					return err
				}
				for s := 0; s < 2; s++ {
					err = post(quarter.AddDate(0, 0, 30+30*s), gtables.ItemJournalLine_Entry_type.Sale, itemNo, fmt.Sprintf("SALE%d-%d", q+1, s+1), 5+rng.Intn(45), nil, "", "")
					if err != nil {
						return err
					}
				}
			}
		}
	}

	// office chairs with serial numbers: three come in last month, one of them was sold
	if demoItemExists(masterData, demoSerialItem) {
		chairCost := types.NewDecimalFromInt(1450)
		for i := 1; i <= 3; i++ {
			err := post(today.AddDate(0, -1, 0), gtables.ItemJournalLine_Entry_type.Purchase, demoSerialItem, "PURCH-SN", 1, &chairCost, fmt.Sprintf("ERGO-%04d", i), "")
			if err != nil {
				return err
			}
		}
		err := post(today.AddDate(0, 0, -10), gtables.ItemJournalLine_Entry_type.Sale, demoSerialItem, "SALE-SN", 1, nil, "ERGO-0002", "")
		if err != nil {
			return err
		}
	}
	return nil
}

// The demo's item tracking: coffee beans in lots, office chairs with serial numbers
const (
	demoLotItem    = "2000"
	demoSerialItem = "1300"
)

func demoItemExists(masterData []tableRecords, itemNo string) bool {
	for _, t := range masterData {
		if t.Table != gtables.ItemTableName {
			continue
		}
		for _, r := range t.Records {
			if fmt.Sprint(r["no"]) == itemNo {
				return true
			}
		}
	}
	return false
}

// demoBOMItems are the demo items that have a bill of materials
func demoBOMItems(masterData []tableRecords) map[string]bool {
	bomItems := map[string]bool{}
	for _, t := range masterData {
		if t.Table == gtables.BOMComponentTableName {
			for _, r := range t.Records {
				bomItems[fmt.Sprint(r["parent_item_no"])] = true
			}
		}
	}
	return bomItems
}

// postDemoAssemblies assembles the demo bills of materials through the BOM posting (codeunit
// 32): every quarter a batch of each, from the components in stock
func postDemoAssemblies(src ftables.Source, open func(src ftables.Source) BOMJnlPoster, today time.Time, masterData []tableRecords, counts *Counts, p *progress) error {
	rng := rand.New(rand.NewSource(seed + 90))
	bomJnlPostLine := open(src)
	start := today.AddDate(-1, 0, 1)
	lineNo := 0
	var itemNos []string
	for itemNo := range demoBOMItems(masterData) {
		itemNos = append(itemNos, itemNo)
	}
	sort.Strings(itemNos)
	p.setTotal(4 * len(itemNos))
	for q := 0; q < 4; q++ {
		quarter := start.AddDate(0, 3*q, 10)
		for _, itemNo := range itemNos {
			// as many as the components on hand allow, at most the planned quantity
			qty := types.NewDecimalFromInt(int64(2 + rng.Intn(4)))
			bomComp := gtables.NewBOMComponent(src)
			bomComp.SetRange("parent_item_no", types.Code(itemNo))
			bomComp.SetRange("type", gtables.BOMComponent_Type.Item)
			for found := bomComp.FindSetBuffered(); found; found = bomComp.Next() {
				item := gtables.NewItem(src)
				if item.Get(bomComp.No) && !bomComp.Quantity_per.IsZero() {
					item.CalcFields("quantity_on_hand")
					possible := system.Round(item.Quantity_on_hand.Div(bomComp.Quantity_per), types.NewDecimalFromInt(1), "<")
					if possible.LessThan(qty) {
						qty = possible
					}
				}
			}
			if !qty.IsPositive() {
				continue
			}
			lineNo = lineNo + 10000
			line := gtables.NewBOMJournalLine(src)
			line.InitRecord()
			line.Journal_template_name = "BOM"
			line.Journal_batch_name = "DEFAULT"
			line.Line_no = lineNo
			line.Source_code = "BOMJNL"
			line.Document_no = types.Code(fmt.Sprintf("ASM%d-%s", q+1, itemNo))
			for _, step := range []struct {
				field string
				value interface{}
			}{
				{"posting_date", types.NewDateFromTime(quarter)},
				{"item_no", types.Code(itemNo)},
				{"location_code", types.Code("MAIN")},
				{"quantity", qty},
			} {
				err := line.Validate(step.field, step.value)
				if err != nil {
					return fmt.Errorf("demo BOM journal %s %s: %w", itemNo, step.field, err)
				}
			}
			err := bomJnlPostLine.Run(line)
			if err != nil {
				return fmt.Errorf("demo BOM journal %s %s: %w", itemNo, line.Document_no, err)
			}
			counts.BOMLedgerEntries++
			p.step()
		}
	}
	return nil
}
