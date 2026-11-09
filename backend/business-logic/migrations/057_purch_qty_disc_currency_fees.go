package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration057PurchQtyDiscCurrencyFees{})
}

// Migration057PurchQtyDiscCurrencyFees goes with the vendors' quantity discounts (table 100 Item
// Purch. Qty. Disc., page 911) and the reminder and finance charge fees per currency (tables 328
// Currency for Fin. Charge Terms, 329 Currency for Reminder Level, pages 477/478): the READER role
// may read them, as all business tables; the tables come from the table sync.
type Migration057PurchQtyDiscCurrencyFees struct{}

func (m *Migration057PurchQtyDiscCurrencyFees) Version() int {
	return 57
}

func (m *Migration057PurchQtyDiscCurrencyFees) Name() string {
	return "purch_qty_disc_currency_fees"
}

func (m *Migration057PurchQtyDiscCurrencyFees) Description() string {
	return "READER reads the purchase quantity discounts and the fees per currency"
}

func (m *Migration057PurchQtyDiscCurrencyFees) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, []string{"ITEM_PURCH_QTY_DISC", "CURRENCY_FOR_FIN_CHARGE_TERMS", "CURRENCY_FOR_REMINDER_LEVEL"})
}
