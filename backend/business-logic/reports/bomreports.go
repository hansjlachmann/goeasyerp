package reports

import (
	"strconv"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The BOM reports (main menu Items & Inventory → BOM Reports; the classic local reports
// renumbered):
//   - 804 Sub-Assemblies: the items with a bill of materials that are a component of another
//     bill of materials, with their inventory (in the FlowFilters), reorder point, lead time,
//     alternative item, unit cost and value on hand.
//   - 805 Where-Used List: per item the bills of materials it is a component of — the parent
//     item, the quantity per, its inventory and quantities on sales and purchase orders; items
//     that are no component are left out (PrintOnlyIfDetail).
const (
	SubAssembliesID = 804
	WhereUsedListID = 805
)

func init() {
	Register(SubAssembliesID, "sub-assemblies", func() Report { return &subAssemblies{} })
	Register(WhereUsedListID, "where-used-list", func() Report { return &whereUsedList{} })
}

// bomComponentOfItem: the BOM components that are the item
func bomComponentOfItem(src ftables.Source, itemNo any) *gtables.BOMComponent {
	bomComp := gtables.NewBOMComponent(src)
	bomComp.SetCurrentKey("type", "no")
	bomComp.SetRange("type", gtables.BOMComponent_Type.Item)
	bomComp.SetRange("no", itemNo)
	return bomComp
}

// Report 804 Sub-Assemblies
type subAssemblies struct{}

func (r *subAssemblies) Definition() *Definition {
	return &Definition{
		OnPreReport: reportHead,
		DataItems: []*DataItem{{
			Name: "Item", Table: gtables.ItemTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "base_unit_of_measure", "shelf_bin_no", "dimension_1_filter", "dimension_2_filter", "location_filter"},
			OnPreDataItem:       headWithFilters[*gtables.Item](nil),
			OnAfterGetRecord: Trigger(func(item *gtables.Item, run *Run) error {
				// DataItemTableView WHERE(Bill of Materials=CONST(Yes)): the FlowField
				item.CalcFields("bill_of_materials")
				if !item.Bill_of_materials {
					return Skip()
				}
				bomComp := bomComponentOfItem(run, item.No)
				if !bomComp.FindFirst() { // Not part of a BOM
					return Skip()
				}
				item.CalcFields("quantity_on_hand")
				run.SetValue("value_on_hand", item.Quantity_on_hand.Mul(item.Unit_cost))
				return nil
			}),
		}},
	}
}

// Report 805 Where-Used List
type whereUsedList struct{}

func (r *whereUsedList) Definition() *Definition {
	return &Definition{
		OnPreReport: reportHead,
		DataItems: []*DataItem{{
			Name: "Item", Table: gtables.ItemTableName, Key: []string{"no"},
			RequestFilterFields: []string{"no", "search_description"},
			OnPreDataItem:       headWithFilters[*gtables.Item](nil),
			OnAfterGetRecord: Trigger(func(item *gtables.Item, run *Run) error {
				bomComp := bomComponentOfItem(run, item.No)
				if !bomComp.FindFirst() { // PrintOnlyIfDetail
					return Skip()
				}
				return nil
			}),
			DataItems: []*DataItem{{
				Name: "BOM_Component", Table: gtables.BOMComponentTableName, Key: []string{"type", "no"},
				TableView: []ftables.FieldFilter{{Field: "type", Expression: strconv.Itoa(int(gtables.BOMComponent_Type.Item))}},
				Link:      map[string]string{"no": "no"},
				DataItems: []*DataItem{{
					Name: "Item2", Table: gtables.ItemTableName, Key: []string{"no"},
					Link: map[string]string{"no": "parent_item_no"},
					OnAfterGetRecord: Trigger(func(item2 *gtables.Item, run *Run) error {
						item2.CalcFields("quantity_on_hand", "qty_on_purch_order", "qty_on_sales_order")
						return nil
					}),
				}},
			}},
		}},
	}
}
