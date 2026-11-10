package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The Item Availability pages' calculated period columns (classic form 353 Item Availability
// Lines and the lines forms of 492/5414/5412): Expected Inventory — the inventory with the
// purchase and sales orders up to the end of the row's period — and Available Inventory — the
// inventory less every sales order plus the purchase orders up to the end of the period
// (codeunit 5400 ExpectedQtyOnHand)
func init() {
	for _, pageID := range []int{157, 492, 5414, 5412} {
		pages.OnAfterGetRecord(pageID, func(rec *gtables.Item, page *pages.Control) {
			var qtyAvailable types.Decimal
			expectedInventory := tables.ExpectedQtyOnHand(rec, true, true, types.Decimal{}, &qtyAvailable, types.Date{})
			page.SetValue("expected_inventory", expectedInventory)
			page.SetValue("available_inventory", qtyAvailable)
		})
	}
}
