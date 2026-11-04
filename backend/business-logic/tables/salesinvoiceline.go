package tables

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// salesInvoiceLineLogic is the business logic of table 113 Sales Invoice Line: a line of a posted sales invoice (written by
// sales posting, codeunit 80); it has no triggers of its own.
type salesInvoiceLineLogic struct{}

func init() { gtables.RegisterSalesInvoiceLineLogic(salesInvoiceLineLogic{}) }
