package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 5411 Items by Bin: the items against the bins (classic form 5411, a matrix over table
// 5400 Bin) — a column per bin of the Location Filter (and Bin Filter), "Location, Bin"; a cell
// is the item's Quantity on Hand in the bin (the classic sets the Location and Bin Filter to the
// column's bin and calculates Quantity on Hand; here one pass over the item ledger entries)
const itemsByBinPageID = 5411

func init() {
	pages.OnMatrixCalculate(itemsByBinPageID, func(card *gtables.Item, item *gtables.Item) ([]pages.MatrixColumn, map[string][]string, error) {
		// the columns: the bins in the filters
		bin := gtables.NewBin(item)
		if item.Location_filter != "" {
			bin.SetFilter("location_code", item.Location_filter)
		}
		if item.Bin_filter != "" {
			bin.SetFilter("code", item.Bin_filter)
		}
		type binKey struct{ locationCode, code types.Code }
		var bins []binKey
		var columns []pages.MatrixColumn
		column := map[binKey]int{}
		for found := bin.FindSet(); found; found = bin.Next() {
			key := binKey{bin.Location_code, bin.Code}
			column[key] = len(bins)
			bins = append(bins, key)
			columns = append(columns, pages.MatrixColumn{Name: bin.Location_code.String() + ", " + bin.Code.String(), DecimalPlaces: "0:5"})
		}

		// the cells: the quantity on hand per item and bin
		quantities := map[types.Code][]types.Decimal{}
		itemLedgEntry := gtables.NewItemLedgerEntry(item)
		itemLedgEntry.SetFilter("bin_code", "<>''")
		if item.Location_filter != "" {
			itemLedgEntry.SetFilter("location_code", item.Location_filter)
		}
		if item.Bin_filter != "" {
			itemLedgEntry.SetFilter("bin_code", item.Bin_filter)
		}
		if item.Variant_filter != "" {
			itemLedgEntry.SetFilter("variant_code", item.Variant_filter)
		}
		for found := itemLedgEntry.FindSetBuffered(); found; found = itemLedgEntry.Next() {
			i, ok := column[binKey{itemLedgEntry.Location_code, itemLedgEntry.Bin_code}]
			if !ok {
				continue
			}
			if quantities[itemLedgEntry.Item_no] == nil {
				quantities[itemLedgEntry.Item_no] = make([]types.Decimal, len(bins))
			}
			quantities[itemLedgEntry.Item_no][i] = quantities[itemLedgEntry.Item_no][i].Add(itemLedgEntry.Quantity)
		}

		values := map[string][]string{}
		for found := item.FindSet(); found; found = item.Next() {
			cells := make([]string, len(bins))
			for i, qty := range quantities[item.No] {
				if !qty.IsZero() {
					cells[i] = qty.String()
				}
			}
			values[item.No.String()] = cells
		}
		return columns, values, nil
	})
}
