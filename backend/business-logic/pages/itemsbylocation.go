package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 491 Items by Location: the items against the locations (classic form 491, a matrix over
// table 14 Location) — a column per location of the Location Filter; a cell is the item's
// Quantity on Hand at the location (the classic sets the Location Filter to the column's code and
// calculates Quantity on Hand; here one pass over the item ledger entries), drilling down to the
// item's entries at the location
const itemsByLocationPageID = 491

func init() {
	pages.OnMatrixCalculate(itemsByLocationPageID, func(card *gtables.Item, item *gtables.Item) ([]pages.MatrixColumn, map[string][]string, error) {
		// the columns: the locations in the filter
		location := gtables.NewLocation(item)
		if item.Location_filter != "" {
			location.SetFilter("code", item.Location_filter)
		}
		var columns []pages.MatrixColumn
		column := map[types.Code]int{}
		for found := location.FindSet(); found; found = location.Next() {
			column[location.Code] = len(columns)
			columns = append(columns, pages.MatrixColumn{Name: location.Code.String(), DecimalPlaces: "0:5",
				DrilldownFilters: map[string]string{"location_code": location.Code.String()}})
		}

		// the cells: the quantity on hand per item and location
		quantities := map[types.Code][]types.Decimal{}
		itemLedgEntry := gtables.NewItemLedgerEntry(item)
		if item.Location_filter != "" {
			itemLedgEntry.SetFilter("location_code", item.Location_filter)
		}
		if item.Variant_filter != "" {
			itemLedgEntry.SetFilter("variant_code", item.Variant_filter)
		}
		for found := itemLedgEntry.FindSetBuffered(); found; found = itemLedgEntry.Next() {
			i, ok := column[itemLedgEntry.Location_code]
			if !ok {
				continue
			}
			if quantities[itemLedgEntry.Item_no] == nil {
				quantities[itemLedgEntry.Item_no] = make([]types.Decimal, len(columns))
			}
			quantities[itemLedgEntry.Item_no][i] = quantities[itemLedgEntry.Item_no][i].Add(itemLedgEntry.Quantity)
		}

		values := map[string][]string{}
		for found := item.FindSet(); found; found = item.Next() {
			cells := make([]string, len(columns))
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
