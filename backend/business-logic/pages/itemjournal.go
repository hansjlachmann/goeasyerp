package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The item journal pages 40 Item Journal, 393 Item Transfer Journal, 392 Phys. Inventory
// Journal and 286 Recurring Item Jnl.: OnOpenPage opens a template of the page's type and one of its batches
// (ItemJnlManagement); a new line gets its values from the line above (SetUpNewLine); the
// footer shows the line's item description.
var itemJournalPages = map[int]gtables.ItemJournalTemplateType{
	40:  gtables.ItemJournalTemplate_Type.Item,
	286: gtables.ItemJournalTemplate_Type.Item, // Recurring Item Jnl.: the recurring item templates
	393: gtables.ItemJournalTemplate_Type.Transfer,
	392: gtables.ItemJournalTemplate_Type.PhysInventory,
}

func init() {
	for pageID, templateType := range itemJournalPages {
		pages.OnOpenPage(pageID, func(rec *gtables.ItemJournalLine, page *pages.Open) error {
			return itemjnl.OpenJournalPage(rec, page, templateType, pageID == tables.RecurringItemJnlPageID)
		})
		pages.OnNewRecord(pageID, func(rec, xRec *gtables.ItemJournalLine, belowXRec bool) {
			_ = tables.SetUpNewItemJnlLine(rec, xRec)
		})
		pages.OnAfterGetRecord(pageID, func(rec *gtables.ItemJournalLine, page *pages.Control) {
			page.SetValue("item_description", itemjnl.GetItem(rec))
		})
	}

	// Page 262 Item Journal Batches: a new batch takes the template's numbering and reason code
	// (SetupNewBatch)
	pages.OnNewRecord(262, func(rec, xRec *gtables.ItemJournalBatch, belowXRec bool) {
		_ = tables.SetupNewItemJnlBatch(rec)
	})
}
