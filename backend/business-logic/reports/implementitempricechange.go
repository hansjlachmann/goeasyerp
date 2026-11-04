package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 793 Implement Item Price Change: the suggested price changes become the item prices
// (inserted or modified, with their tax and discount settings); afterwards the user may delete
// the suggestions.
const ImplementItemPriceChangeID = 793

func init() {
	Register(ImplementItemPriceChangeID, "implement-item-price-change", func() Report { return &implementItemPriceChange{} })
}

type implementItemPriceChange struct {
	window *system.Window
}

func (r *implementItemPriceChange) Definition() *Definition {
	return &Definition{
		DataItems: []*DataItem{{
			Name: "Item_Price_Change", Table: "Item_Price_Change",
			Key:                 []string{"item_no", "price_group_code", "unit_of_measure_code", "currency_code", "starting_date"},
			RequestFilterFields: []string{"item_no", "price_group_code", "unit_of_measure_code", "currency_code"},
			OnPreDataItem:       Trigger(r.onPreDataItem),
			OnAfterGetRecord:    Trigger(r.onAfterGetRecord),
			OnPostDataItem:      Trigger(r.onPostDataItem),
		}},
	}
}

func (r *implementItemPriceChange) onPreDataItem(itemPriceChange *gtables.ItemPriceChange, run *Run) error {
	r.window = system.OpenWindow(run.Text("WINDOW"))
	return nil
}

func (r *implementItemPriceChange) onAfterGetRecord(itemPriceChange *gtables.ItemPriceChange, run *Run) error {
	r.window.Update(1, itemPriceChange.Item_no)
	r.window.Update(2, itemPriceChange.Price_group_code)
	r.window.Update(3, itemPriceChange.Currency_code)
	r.window.Update(4, itemPriceChange.Starting_date)
	itemPrice := gtables.NewItemPrice(run)
	itemPrice.Item_no = itemPriceChange.Item_no
	itemPrice.Price_group_code = itemPriceChange.Price_group_code
	itemPrice.Unit_of_measure_code = itemPriceChange.Unit_of_measure_code
	itemPrice.Currency_code = itemPriceChange.Currency_code
	itemPrice.Starting_date = itemPriceChange.Starting_date
	itemPrice.Unit_price = itemPriceChange.New_unit_price
	itemPrice.Price_includes_tax = itemPriceChange.Price_includes_tax
	itemPrice.Allow_quantity_disc = itemPriceChange.Allow_quantity_disc
	itemPrice.Allow_cust_item_disc = itemPriceChange.Allow_cust_item_disc
	itemPrice.Allow_invoice_disc = itemPriceChange.Allow_invoice_disc
	itemPrice.Tax_bus_posting_gr_price = itemPriceChange.Tax_bus_posting_gr_price
	if !itemPrice.Insert(false) {
		return itemPrice.ModifyOrError(false)
	}
	return nil
}

// onPostDataItem asks whether to delete the suggestions that were carried out — the data
// item's records in its filters (classic: in the footer section, after a COMMIT)
func (r *implementItemPriceChange) onPostDataItem(itemPriceChange *gtables.ItemPriceChange, run *Run) error {
	if system.GuiAllowed() && system.Confirm(run.Text("DELETE_SUGGESTIONS")) {
		itemPriceChange.DeleteAll()
	}
	return nil
}
