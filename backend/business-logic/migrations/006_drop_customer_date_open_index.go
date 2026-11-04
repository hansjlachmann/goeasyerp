package migrations

import (
	"fmt"

	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

func init() {
	Register(&Migration006DropCustomerDateOpenIndex{})
}

// Migration006DropCustomerDateOpenIndex drops the index of the removed Customer Ledger Entry
// key customer_date_open (customer, posting date, open). The SIFT sync drops the key's totals
// table and triggers itself; table sync never drops indexes, so the plain index would stay and
// cost an update on every posting.
type Migration006DropCustomerDateOpenIndex struct{}

func (m *Migration006DropCustomerDateOpenIndex) Version() int {
	return 6
}

func (m *Migration006DropCustomerDateOpenIndex) Name() string {
	return "drop_customer_date_open_index"
}

func (m *Migration006DropCustomerDateOpenIndex) Description() string {
	return "Drop the index of the removed Customer Ledger Entry key customer_date_open"
}

func (m *Migration006DropCustomerDateOpenIndex) Up(ctx *fmigrations.Context) error {
	for _, company := range ctx.Companies {
		err := ctx.DropIndex(fmt.Sprintf("%s$Customer Ledger Entry$customer_date_open", company))
		if err != nil {
			return fmt.Errorf("company %s: %w", company, err)
		}
	}
	return nil
}
