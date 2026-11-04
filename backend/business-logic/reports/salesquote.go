package reports

import gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"

// Report 204 "Sales - Quote": prints sales quotes (salesDocument). Layout:
// backend/definitions/reports/204-sales-quote.yaml.
const SalesQuoteID = 204

func init() {
	Register(SalesQuoteID, "sales-quote", func() Report {
		return &salesDocument{documentType: gtables.SalesHeader_Document_type.Quote}
	})
}
