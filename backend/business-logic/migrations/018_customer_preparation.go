package migrations

import (
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration018CustomerPreparation{})
}

// Migration018CustomerPreparation goes with the tables the customer relates to: customer posting
// groups (92), post codes (225), territories (286), payment methods (289), shipping agents
// (291), reminder terms, levels and texts (292–294), customer discount groups (340), number
// series with lines and relationships (308–310) and the Sales & Receivables Setup (311). The
// READER role may read them, as all business tables; the tables come from the table sync.
type Migration018CustomerPreparation struct{}

func (m *Migration018CustomerPreparation) Version() int {
	return 18
}

func (m *Migration018CustomerPreparation) Name() string {
	return "customer_preparation"
}

func (m *Migration018CustomerPreparation) Description() string {
	return "READER reads the customer posting groups, codes, reminder terms, number series and sales setup"
}

func (m *Migration018CustomerPreparation) Up(ctx *fmigrations.Context) error {
	return grantReaderRead(ctx, customerPreparationTables)
}

var customerPreparationTables = []string{
	"CUSTOMER_POSTING_GROUP", "ZIP_CODE", "TERRITORY", "PAYMENT_METHOD", "SHIPPING_AGENT",
	"REMINDER_TERMS", "REMINDER_LEVEL", "REMINDER_TEXT", "CUSTOMER_DISCOUNT_GROUP",
	"NO_SERIES", "NO_SERIES_LINE", "NO_SERIES_RELATIONSHIP", "SALES_RECEIVABLES_SETUP",
}
