// Package purchprinted is codeunits 317 Purch.Header-Printed, 318 Purch.Rcpt.-Printed, 319
// Purch.Inv.-Printed and 320 Purch.Cr.Memo-Printed: a purchase document a report printed (not a
// preview) counts one more print in No. Printed (saved without triggers).
package purchprinted

import (
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Run is codeunit 317 Purch.Header-Printed: a quote or order printed
func Run(rec *gtables.PurchaseHeader) error {
	purchHeader := gtables.NewPurchaseHeader(rec)
	err := purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		return err
	}
	purchHeader.No_printed = purchHeader.No_printed + 1
	return purchHeader.ModifyOrError(false)
}

// RunReceipt is codeunit 318 Purch.Rcpt.-Printed: a posted receipt printed
func RunReceipt(rec *gtables.PurchRcptHeader) error {
	purchRcptHeader := gtables.NewPurchRcptHeader(rec)
	err := purchRcptHeader.GetOrError(rec.No)
	if err != nil {
		return err
	}
	purchRcptHeader.No_printed = purchRcptHeader.No_printed + 1
	return purchRcptHeader.ModifyOrError(false)
}

// RunInvoice is codeunit 319 Purch.Inv.-Printed: a posted invoice printed
func RunInvoice(rec *gtables.PurchInvHeader) error {
	purchInvHeader := gtables.NewPurchInvHeader(rec)
	err := purchInvHeader.GetOrError(rec.No)
	if err != nil {
		return err
	}
	purchInvHeader.No_printed = purchInvHeader.No_printed + 1
	return purchInvHeader.ModifyOrError(false)
}

// RunCrMemo is codeunit 320 Purch.Cr.Memo-Printed: a posted credit memo printed
func RunCrMemo(rec *gtables.PurchCrMemoHdr) error {
	purchCrMemoHeader := gtables.NewPurchCrMemoHdr(rec)
	err := purchCrMemoHeader.GetOrError(rec.No)
	if err != nil {
		return err
	}
	purchCrMemoHeader.No_printed = purchCrMemoHeader.No_printed + 1
	return purchCrMemoHeader.ModifyOrError(false)
}
