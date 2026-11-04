package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration025Items{})
}

// Migration025Items goes with the items (tables 27 Item, 30 Item Translation, 94 Inventory
// Posting Group, 99 Item Vendor, 204 Unit of Measure, 313 Inventory Setup, 5401 Item Variant,
// 5402 Unit of Measure Translation, 5404 Item Unit of Measure) and their prices and discounts
// (28 Item Price, 29 Item Sales Qty. Disc., 341 Item Discount Group, 20 Cust./Item
// Discount), the item journals and the item ledgers (82, 233, 83, 32, 339, 46, 281). The READER role
// may read them, as all business tables; the tables come from the table sync.
type Migration025Items struct{}

func (m *Migration025Items) Version() int {
	return 25
}

func (m *Migration025Items) Name() string {
	return "items"
}

func (m *Migration025Items) Description() string {
	return "READER reads the items, their prices and discounts, the item journals and the item ledger"
}

func (m *Migration025Items) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, itemTables)
}

var itemTables = []string{
	"ITEM", "ITEM_TRANSLATION", "INVENTORY_POSTING_GROUP", "ITEM_VENDOR", "UNIT_OF_MEASURE", "INVENTORY_SETUP",
	"ITEM_VARIANT", "UNIT_OF_MEASURE_TRANSLATION", "ITEM_UNIT_OF_MEASURE", "ITEM_PRICE", "ITEM_SALES_QTY_DISC",
	"ITEM_DISCOUNT_GROUP", "CUST_ITEM_DISCOUNT", "ITEM_JOURNAL_TEMPLATE", "ITEM_JOURNAL_BATCH", "ITEM_JOURNAL_LINE",
	"ITEM_LEDGER_ENTRY", "ITEM_APPLICATION_ENTRY", "ITEM_REGISTER", "PHYS_INVENTORY_LEDGER_ENTRY",
}
