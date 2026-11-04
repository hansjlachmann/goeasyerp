package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// PurchPostPrint - Codeunit 92 "Purch.-Post + Print": Post and Print (Shift+F11) of a purchase
// document — posts it as Purch.-Post (Yes/No) does, then prints what was posted with the reports of
// Report Selections: an order's receipt (P.Receipt) and invoice (P.Invoice), an invoice
// (P.Invoice), a credit memo (P.Cr.Memo) — without request pages, to the output of the reports'
// Printer Selection (the browser's print dialog by default).
const PurchPostPrintID = 92
const PurchPostPrintName = "purch-post-print"

func init() {
	Register(PurchPostPrintID, PurchPostPrintName, NewPurchPostPrint)
}

type PurchPostPrint struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewPurchPostPrint creates a new instance of the codeunit
func NewPurchPostPrint(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &PurchPostPrint{db: db, company: company, dbType: dbType}
}

func (c *PurchPostPrint) ID() int                    { return PurchPostPrintID }
func (c *PurchPostPrint) Name() string               { return PurchPostPrintName }
func (c *PurchPostPrint) SourceTable() string        { return gtables.PurchaseHeaderTableName }
func (c *PurchPostPrint) UsesProgress() bool         { return true }
func (c *PurchPostPrint) GetDB() database.Executor   { return c.db }
func (c *PurchPostPrint) GetCompany() string         { return c.company }
func (c *PurchPostPrint) GetDBType() database.DBType { return c.dbType }

// Run posts the document and prints what was posted
func (c *PurchPostPrint) Run(record interface{}) (fcodeunits.Result, error) {
	result, err := c.code(record)
	return result, err
}

func (c *PurchPostPrint) code(record interface{}) (fcodeunits.Result, error) {
	purchPostYesNo := &PurchPostYesNo{db: c.db, company: c.company, dbType: c.dbType}
	rec, ok := record.(*gtables.PurchaseHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 92 runs on a purchase header")
	}
	result, purchPost, err := purchPostYesNo.post(rec)
	if err != nil || purchPost == nil {
		return result, err
	}
	src := ftables.DBSource(c.db, c.company, c.dbType)
	var printouts []reports.Result
	printReport := func(usage gtables.ReportSelectionsUsage, context ftables.Source) error {
		printout, err := reports.PrintDocuments(context, usage, false)
		if err != nil {
			return err
		}
		printouts = append(printouts, printout...)
		return nil
	}
	if rec.Document_type == gtables.PurchaseHeader_Document_type.Order && purchPost.ReceiptNo != "" {
		purchRcptHeader := gtables.NewPurchRcptHeader(src)
		purchRcptHeader.No = purchPost.ReceiptNo
		err = printReport(gtables.ReportSelections_Usage.PReceipt, purchRcptHeader)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	if purchPost.InvoiceNo != "" {
		purchInvHeader := gtables.NewPurchInvHeader(src)
		purchInvHeader.No = purchPost.InvoiceNo
		err = printReport(gtables.ReportSelections_Usage.PInvoice, purchInvHeader)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	if purchPost.CrMemoNo != "" {
		purchCrMemoHeader := gtables.NewPurchCrMemoHdr(src)
		purchCrMemoHeader.No = purchPost.CrMemoNo
		err = printReport(gtables.ReportSelections_Usage.PCrMemo, purchCrMemoHeader)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return reports.WithPrintouts(result, printouts), nil
}
