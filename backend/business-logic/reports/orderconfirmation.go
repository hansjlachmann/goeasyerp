package reports

import gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"

// Report 205 "Order Confirmation": prints sales orders for the customer to confirm
// (salesDocument). Layout: backend/definitions/reports/205-order-confirmation.yaml.
const OrderConfirmationID = 205

func init() {
	Register(OrderConfirmationID, "order-confirmation", func() Report {
		return &salesDocument{documentType: gtables.SalesHeader_Document_type.Order}
	})
}
