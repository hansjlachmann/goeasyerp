package codeunits

import (
	"encoding/base64"
	"errors"

	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// PurchPostPrint - Codeunit 92 "Purch.-Post + Print": Post and Print (Shift+F11) of a purchase
// document — posts it as Purch.-Post (Yes/No) does, then prints the posted invoice or credit memo
// (report 406, 407), or the receipt when the order was only received (408), to the browser's print
// dialog.
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
	var reportID int
	var context interface{}
	switch {
	case purchPost.InvoiceNo != "":
		reportID = reports.PurchaseInvoiceID
		purchInvHeader := gtables.NewPurchInvHeader(src)
		purchInvHeader.No = purchPost.InvoiceNo
		context = purchInvHeader
	case purchPost.CrMemoNo != "":
		reportID = reports.PurchaseCrMemoID
		purchCrMemoHeader := gtables.NewPurchCrMemoHdr(src)
		purchCrMemoHeader.No = purchPost.CrMemoNo
		context = purchCrMemoHeader
	case purchPost.ReceiptNo != "":
		reportID = reports.PurchaseReceiptID
		purchRcptHeader := gtables.NewPurchRcptHeader(src)
		purchRcptHeader.No = purchPost.ReceiptNo
		context = purchRcptHeader
	default:
		return result, nil
	}
	printout, err := reports.Execute(reportID, c.db, c.company, c.dbType, reports.RunOptions{Output: reports.OutputPrint, Context: context})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if result.Data == nil {
		result.Data = map[string]interface{}{}
	}
	result.Data["pdf"] = base64.StdEncoding.EncodeToString(printout.PDF)
	result.Data["filename"] = printout.FileName
	result.Data["output"] = printout.Output
	return result, nil
}
