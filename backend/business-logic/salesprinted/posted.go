package salesprinted

import (
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// RunShipment is codeunit 314 Sales Shpt.-Printed: a posted shipment printed (not a preview)
// counts one more print in No. Printed
func RunShipment(rec *gtables.SalesShipmentHeader) error {
	salesShptHeader := gtables.NewSalesShipmentHeader(rec)
	err := salesShptHeader.GetOrError(rec.No)
	if err != nil {
		return err
	}
	salesShptHeader.No_printed = salesShptHeader.No_printed + 1
	return salesShptHeader.ModifyOrError(false)
}

// RunInvoice is codeunit 315 Sales Inv.-Printed: a posted invoice printed (not a preview)
// counts one more print in No. Printed
func RunInvoice(rec *gtables.SalesInvoiceHeader) error {
	salesInvHeader := gtables.NewSalesInvoiceHeader(rec)
	err := salesInvHeader.GetOrError(rec.No)
	if err != nil {
		return err
	}
	salesInvHeader.No_printed = salesInvHeader.No_printed + 1
	return salesInvHeader.ModifyOrError(false)
}

// RunCrMemo is codeunit 316 Sales Cr. Memo-Printed: a posted credit memo printed (not a
// preview) counts one more print in No. Printed
func RunCrMemo(rec *gtables.SalesCrMemoHeader) error {
	salesCrMemoHeader := gtables.NewSalesCrMemoHeader(rec)
	err := salesCrMemoHeader.GetOrError(rec.No)
	if err != nil {
		return err
	}
	salesCrMemoHeader.No_printed = salesCrMemoHeader.No_printed + 1
	return salesCrMemoHeader.ModifyOrError(false)
}
