package codeunits

import (
	"errors"

	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// SalesPostPrint - Codeunit 82 "Sales-Post + Print": Post and Print (Shift+F11) of a sales
// document — posts it as Sales-Post (Yes/No) does, then prints what was posted with the reports of
// Report Selections: an order's shipment (S.Shipment) and invoice (S.Invoice), an invoice
// (S.Invoice), a credit memo (S.Cr.Memo) — without request pages, to the output of the reports'
// Printer Selection (the browser's print dialog by default).
const SalesPostPrintID = 82
const SalesPostPrintName = "sales-post-print"

func init() {
	Register(SalesPostPrintID, SalesPostPrintName, NewSalesPostPrint)
}

type SalesPostPrint struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewSalesPostPrint creates a new instance of the codeunit
func NewSalesPostPrint(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &SalesPostPrint{db: db, company: company, dbType: dbType}
}

func (c *SalesPostPrint) ID() int                    { return SalesPostPrintID }
func (c *SalesPostPrint) Name() string               { return SalesPostPrintName }
func (c *SalesPostPrint) SourceTable() string        { return gtables.SalesHeaderTableName }
func (c *SalesPostPrint) UsesProgress() bool         { return true }
func (c *SalesPostPrint) GetDB() database.Executor   { return c.db }
func (c *SalesPostPrint) GetCompany() string         { return c.company }
func (c *SalesPostPrint) GetDBType() database.DBType { return c.dbType }

// Run posts the document and prints what was posted
func (c *SalesPostPrint) Run(record interface{}) (fcodeunits.Result, error) {
	result, err := c.code(record)
	return result, err
}

func (c *SalesPostPrint) code(record interface{}) (fcodeunits.Result, error) {
	salesPostYesNo := &SalesPostYesNo{db: c.db, company: c.company, dbType: c.dbType}
	rec, ok := record.(*gtables.SalesHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 82 runs on a sales header")
	}
	result, salesPost, err := salesPostYesNo.post(rec)
	if err != nil || salesPost == nil {
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
	if rec.Document_type == gtables.SalesHeader_Document_type.Order && salesPost.ShipmentNo != "" {
		salesShptHeader := gtables.NewSalesShipmentHeader(src)
		salesShptHeader.No = salesPost.ShipmentNo
		err = printReport(gtables.ReportSelections_Usage.SShipment, salesShptHeader)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	if salesPost.InvoiceNo != "" {
		salesInvHeader := gtables.NewSalesInvoiceHeader(src)
		salesInvHeader.No = salesPost.InvoiceNo
		err = printReport(gtables.ReportSelections_Usage.SInvoice, salesInvHeader)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	if salesPost.CrMemoNo != "" {
		salesCrMemoHeader := gtables.NewSalesCrMemoHeader(src)
		salesCrMemoHeader.No = salesPost.CrMemoNo
		err = printReport(gtables.ReportSelections_Usage.SCrMemo, salesCrMemoHeader)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return reports.WithPrintouts(result, printouts), nil
}
