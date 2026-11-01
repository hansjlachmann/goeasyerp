package codeunits

import (
	"errors"

	"github.com/hansjlachmann/openerp/backend/business-logic/purchgetreceipt"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// PurchGetReceipt - Codeunit 74 "Purch.-Get Receipt": Get Receipt Lines on a purchase invoice —
// the vendor's receipt lines not invoiced on page 5709; the lines of the picked line's
// receipt go onto the invoice (business-logic/purchgetreceipt), and the page reloads.
const PurchGetReceiptID = 74
const PurchGetReceiptName = "purch-get-receipt"

func init() {
	Register(PurchGetReceiptID, PurchGetReceiptName, NewPurchGetReceipt)
}

type PurchGetReceipt struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewPurchGetReceipt creates a new instance of the codeunit
func NewPurchGetReceipt(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &PurchGetReceipt{db: db, company: company, dbType: dbType}
}

func (c *PurchGetReceipt) ID() int                    { return PurchGetReceiptID }
func (c *PurchGetReceipt) Name() string               { return PurchGetReceiptName }
func (c *PurchGetReceipt) SourceTable() string        { return gtables.PurchaseHeaderTableName }
func (c *PurchGetReceipt) UsesProgress() bool         { return true } // it opens a page (RunModal)
func (c *PurchGetReceipt) GetDB() database.Executor   { return c.db }
func (c *PurchGetReceipt) GetCompany() string         { return c.company }
func (c *PurchGetReceipt) GetDBType() database.DBType { return c.dbType }

// Run gets receipt lines onto the invoice (the card's record)
func (c *PurchGetReceipt) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.PurchaseHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 74 runs on a purchase header")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *PurchGetReceipt) code(rec *gtables.PurchaseHeader) (fcodeunits.Result, error) {
	src := ftables.DBSource(c.db, c.company, c.dbType)
	purchHeader := gtables.NewPurchaseHeader(src)
	err := purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	// the user picks outside the transaction (the page waits for the user)
	receiptNo, picked, err := purchgetreceipt.SelectReceipt(purchHeader)
	if err != nil || !picked {
		return fcodeunits.Result{Success: true}, err
	}
	receiptDescription := i18n.GetInstance().Message("PURCH_RECEIPT_NO_DESCRIPTION", fcodeunits.CurrentLanguage())
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	purchHeader.SetDB(tx)
	err = purchgetreceipt.CreateInvLines(purchHeader, receiptNo, receiptDescription)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}
