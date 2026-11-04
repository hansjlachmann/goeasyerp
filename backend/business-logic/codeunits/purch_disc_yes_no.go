package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchcalcdisc"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// PurchDiscYesNo - Codeunit 71 "Purch.-Disc. (Yes/No)": a purchase document's Calculate Invoice
// Discount. After asking, it calculates the invoice discount (Purch.-Calc.Discount) in one
// transaction and the page shows the new amounts.
const PurchDiscYesNoID = 71
const PurchDiscYesNoName = "purch-disc-yes-no"

func init() {
	Register(PurchDiscYesNoID, PurchDiscYesNoName, NewPurchDiscYesNo)
}

type PurchDiscYesNo struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewPurchDiscYesNo creates a new instance of the codeunit
func NewPurchDiscYesNo(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &PurchDiscYesNo{db: db, company: company, dbType: dbType}
}

func (c *PurchDiscYesNo) ID() int                    { return PurchDiscYesNoID }
func (c *PurchDiscYesNo) Name() string               { return PurchDiscYesNoName }
func (c *PurchDiscYesNo) SourceTable() string        { return gtables.PurchaseHeaderTableName }
func (c *PurchDiscYesNo) UsesProgress() bool         { return true } // it asks (Confirm)
func (c *PurchDiscYesNo) GetDB() database.Executor   { return c.db }
func (c *PurchDiscYesNo) GetCompany() string         { return c.company }
func (c *PurchDiscYesNo) GetDBType() database.DBType { return c.dbType }

// Run calculates the invoice discount of the document (the card's record)
func (c *PurchDiscYesNo) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.PurchaseHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 71 runs on a purchase header")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *PurchDiscYesNo) code(rec *gtables.PurchaseHeader) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if !system.Confirm(ts.Message("PURCH_CALC_INV_DISC_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	purchHeader := gtables.NewPurchaseHeader(src)
	err = purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = purchcalcdisc.CalculateInvoiceDiscount(purchHeader)
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
