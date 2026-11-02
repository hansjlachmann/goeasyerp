package reports

import (
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The item and resource list reports (main menu Inventory → Item Reports, Resources →
// Reports):
//   - 701 Inventory - List: the items with their unit, inventory (in the FlowFilters, e.g. a
//     location), costs and prices;
//   - 1001 Inventory Valuation: per item its open inbound entries — the remaining quantity at
//     the entry's cost (the adjusted cost of its quantity; Average items at the average cost) —
//     the item's inventory value and the total;
//   - 1101 Resource - List: the resources with their group, unit, cost and price.
const (
	InventoryListID      = 701
	InventoryValuationID = 1001
	ResourceListID       = 1101
)

func init() {
	Register(InventoryListID, "inventory-list", func() Report { return &inventoryList{} })
	Register(InventoryValuationID, "inventory-valuation", func() Report { return &inventoryValuation{} })
	Register(ResourceListID, "resource-list", func() Report { return &resourceList{} })
}

// Report 701 Inventory - List
type inventoryList struct{}

func (r *inventoryList) Definition() *Definition {
	return &Definition{
		OnPreReport: reportHead,
		DataItems: []*DataItem{{
			Name:                "Item",
			Table:               gtables.ItemTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "search_description", "inventory_posting_group", "location_filter"},
			OnPreDataItem:       headWithFilters[*gtables.Item](nil),
			OnAfterGetRecord: Trigger(func(rec *gtables.Item, run *Run) error {
				rec.CalcFields("quantity_on_hand")
				return nil
			}),
		}},
	}
}

// Report 1001 Inventory Valuation
type inventoryValuation struct {
	item       *gtables.Item
	itemValue  types.Decimal
	totalValue types.Decimal
}

func (r *inventoryValuation) Definition() *Definition {
	return &Definition{
		OnPreReport: reportHead,
		DataItems: []*DataItem{{
			Name:                "Item",
			Table:               gtables.ItemTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "description", "inventory_posting_group", "costing_method", "location_filter"},
			OnPreDataItem:       headWithFilters[*gtables.Item](nil),
			OnAfterGetRecord:    Trigger(r.itemOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:      "ItemLedgerEntry",
				Table:     gtables.ItemLedgerEntryTableName,
				Key:       []string{"item_no", "posting_date"},
				Link:      map[string]string{"item_no": "no"},
				TableView: []ftables.FieldFilter{{Field: "open", Expression: "true"}, {Field: "positive", Expression: "true"}},
				OnPreDataItem: Trigger(func(rec *gtables.ItemLedgerEntry, run *Run) error {
					if r.item.Location_filter != "" {
						rec.SetFilter("location_code", r.item.Location_filter)
					}
					return nil
				}),
				OnAfterGetRecord: Trigger(func(rec *gtables.ItemLedgerEntry, run *Run) error {
					cost, value := r.entryValue(rec)
					run.SetValue("cost", cost)
					run.SetValue("inventory_value", value)
					return nil
				}),
			}},
		}},
	}
}

// entryValue: an open entry's unit cost and the value of its remaining quantity
func (r *inventoryValuation) entryValue(rec *gtables.ItemLedgerEntry) (cost, value types.Decimal) {
	switch {
	case r.item.Costing_method == gtables.Item_Costing_method.Average:
		cost = r.item.Average_cost
	case !rec.Quantity.IsZero():
		cost = rec.Adjusted_cost_qty.Div(rec.Quantity)
	}
	return cost, rec.Remaining_quantity.Mul(cost).Round(2)
}

// itemOnAfterGetRecord: the item's inventory value (its open entries); items without inventory
// are left out
func (r *inventoryValuation) itemOnAfterGetRecord(rec *gtables.Item, run *Run) error {
	r.item = rec
	entries := gtables.NewItemLedgerEntry(run)
	entries.SetRange("item_no", rec.No)
	entries.SetRange("open", true)
	entries.SetRange("positive", true)
	if rec.Location_filter != "" {
		entries.SetFilter("location_code", rec.Location_filter)
	}
	r.itemValue = types.Decimal{}
	var quantity types.Decimal
	for found := entries.FindSetBuffered(); found; found = entries.Next() {
		_, value := r.entryValue(entries)
		r.itemValue = r.itemValue.Add(value)
		quantity = quantity.Add(entries.Remaining_quantity)
	}
	if quantity.IsZero() {
		return Skip()
	}
	r.totalValue = r.totalValue.Add(r.itemValue)
	run.SetValue("item_quantity", quantity)
	run.SetValue("item_value", r.itemValue)
	run.SetValue("total_value", r.totalValue)
	return nil
}

// Report 1101 Resource - List
type resourceList struct{}

func (r *resourceList) Definition() *Definition {
	return &Definition{
		OnPreReport: reportHead,
		DataItems: []*DataItem{{
			Name:                "Resource",
			Table:               gtables.ResourceTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "type", "resource_group_no", "gen_prod_posting_group"},
			OnPreDataItem:       headWithFilters[*gtables.Resource](nil),
		}},
	}
}
