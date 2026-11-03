package reports

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Reports 704 Serial Number Sold History and 705 Serial Number Status/Aging (from the classic
// US reports 10160/10161, renumbered in the base range): per item the sales of serial-numbered
// pieces (customer, serial number, date, price and cost) and the pieces on inventory (location,
// serial number, date, days aged since, cost; the average age and the number of pieces). An
// item prints only with entries (classic PrintOnlyIfDetail): its line is printed before its
// first entry. The item's Date, Location, Dimension 1 and Dimension 2 Filters limit the entries
// (DataItemLink to the FlowFilters).
const (
	SerialNumberSoldHistoryID = 704
	SerialNumberStatusAgingID = 705
)

func init() {
	Register(SerialNumberSoldHistoryID, "serial-number-sold-history", func() Report { return &serialNumberSoldHistory{} })
	Register(SerialNumberStatusAgingID, "serial-number-status-aging", func() Report { return &serialNumberStatusAging{} })
}

var serialItemFilterFields = []string{"no", "inventory_posting_group", "vendor_no", "bill_of_materials",
	"dimension_1_filter", "dimension_2_filter", "location_filter"}

// linkItemFilters (DataItemLink Posting Date=FIELD(Date Filter), …): the item's entries in its
// FlowFilters; without a serial number filter of the user, the entries with a serial number
func linkItemFilters(itemLedgEntry *gtables.ItemLedgerEntry, item *gtables.Item) {
	itemLedgEntry.FilterGroup(4)
	itemLedgEntry.SetRange("item_no", item.No)
	if item.Date_filter != "" {
		itemLedgEntry.SetFilter("posting_date", item.Date_filter)
	}
	if item.Location_filter != "" {
		itemLedgEntry.SetFilter("location_code", item.Location_filter)
	}
	if item.Dimension_1_filter != "" {
		itemLedgEntry.SetFilter("dimension_1_code", item.Dimension_1_filter)
	}
	if item.Dimension_2_filter != "" {
		itemLedgEntry.SetFilter("dimension_2_code", item.Dimension_2_filter)
	}
	itemLedgEntry.FilterGroup(0)
	if itemLedgEntry.GetFilter("serial_no") == "" {
		itemLedgEntry.SetFilter("serial_no", "<>''")
	}
}

// serialReportHead: the head values (company, date, user; the work date the ages count to)
func serialReportHead(run *Run) error {
	setReportHeadValues(run)
	run.SetValue("work_date", system.WorkDate())
	return nil
}

// Report 704 Serial Number Sold History
type serialNumberSoldHistory struct {
	item       *gtables.Item
	firstEntry bool
}

func (r *serialNumberSoldHistory) Definition() *Definition {
	return &Definition{
		OnPreReport: serialReportHead,
		DataItems: []*DataItem{{
			Name:                "Item",
			Table:               gtables.ItemTableName,
			Key:                 []string{"no"},
			RequestFilterFields: serialItemFilterFields,
			OnPreDataItem:       headWithFilters[*gtables.Item](nil),
			OnAfterGetRecord: Trigger(func(item *gtables.Item, run *Run) error {
				r.item = item
				r.firstEntry = true
				return nil
			}),
			DataItems: []*DataItem{{
				Name:  "Item_Ledger_Entry",
				Table: gtables.ItemLedgerEntryTableName,
				Key:   []string{"entry_type", "item_no", "variant_code", "drop_shipment", "location_code", "posting_date"},
				TableView: []ftables.FieldFilter{
					{Field: "entry_type", Expression: "1"}, // Sale
					{Field: "unit_amount", Expression: "<>0"},
				},
				RequestFilterFields: []string{"serial_no", "source_no"},
				OnPreDataItem: Trigger(func(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) error {
					linkItemFilters(itemLedgEntry, r.item)
					return nil
				}),
				OnAfterGetRecord: Trigger(func(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) error {
					customer := gtables.NewCustomer(run)
					if !customer.Get(itemLedgEntry.Source_no) {
						customer.Init()
					}
					run.SetValue("customer_name", customer.Name.String())
					run.SetValue("first_entry", r.firstEntry)
					run.SetValue("item_no", r.item.No.String())
					run.SetValue("item_description", r.item.Description.String())
					r.firstEntry = false
					return nil
				}),
			}},
		}},
	}
}

// Report 705 Serial Number Status/Aging
type serialNumberStatusAging struct {
	item          *gtables.Item
	firstEntry    bool
	totalDaysAged int
	numberOfItems int
}

func (r *serialNumberStatusAging) Definition() *Definition {
	return &Definition{
		OnPreReport: serialReportHead,
		DataItems: []*DataItem{{
			Name:                "Item",
			Table:               gtables.ItemTableName,
			Key:                 []string{"no"},
			RequestFilterFields: serialItemFilterFields,
			OnPreDataItem:       headWithFilters[*gtables.Item](nil),
			OnAfterGetRecord: Trigger(func(item *gtables.Item, run *Run) error {
				r.item = item
				r.firstEntry = true
				return nil
			}),
			DataItems: []*DataItem{{
				Name:                "Item_Ledger_Entry",
				Table:               gtables.ItemLedgerEntryTableName,
				Key:                 []string{"item_no", "variant_code", "drop_shipment", "location_code", "posting_date"},
				TableView:           []ftables.FieldFilter{{Field: "open", Expression: "true"}},
				RequestFilterFields: []string{"serial_no"},
				OnPreDataItem: Trigger(func(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) error {
					linkItemFilters(itemLedgEntry, r.item)
					r.totalDaysAged = 0
					r.numberOfItems = 0
					run.SetValue("number_of_items", 0)
					return nil
				}),
				OnAfterGetRecord: Trigger(func(itemLedgEntry *gtables.ItemLedgerEntry, run *Run) error {
					daysAged := itemLedgEntry.Posting_date.DaysBetween(system.WorkDate())
					r.totalDaysAged += daysAged
					r.numberOfItems++
					run.SetValue("days_aged", daysAged)
					run.SetValue("first_entry", r.firstEntry)
					run.SetValue("item_no", r.item.No.String())
					run.SetValue("item_description", r.item.Description.String())
					run.SetValue("item_base_unit", r.item.Base_unit_of_measure.String())
					// the footer's values (set before the last record ends)
					averageAge := types.Decimal{}
					if r.numberOfItems != 0 {
						averageAge = system.Round(types.NewDecimalFromInt(int64(r.totalDaysAged)).Div(types.NewDecimalFromInt(int64(r.numberOfItems))), types.NewDecimalFromInt(1))
					}
					run.SetValue("average_age", averageAge)
					run.SetValue("number_of_items", r.numberOfItems)
					run.SetValue("total_quantity_text", run.Text("TOTAL_QUANTITY", r.numberOfItems))
					r.firstEntry = false
					return nil
				}),
			}},
		}},
	}
}
