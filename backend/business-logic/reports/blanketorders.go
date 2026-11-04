package reports

import gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"

// Reports 210 "Blanket Sales Order" and 410 "Blanket Purchase Order": print blanket orders like
// the orders (salesDocument / purchDocument). Layouts: backend/definitions/reports/210-…, 410-….
const (
	BlanketSalesOrderID    = 210
	BlanketPurchaseOrderID = 410
)

func init() {
	Register(BlanketSalesOrderID, "blanket-sales-order", func() Report {
		return &salesDocument{documentType: gtables.SalesHeader_Document_type.BlanketOrder}
	})
	Register(BlanketPurchaseOrderID, "blanket-purchase-order", func() Report {
		return &purchDocument{documentType: gtables.PurchaseHeader_Document_type.BlanketOrder}
	})
}
