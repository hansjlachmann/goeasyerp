package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salescalcdisc"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// SalesDiscYesNo - Codeunit 61 "Sales-Disc. (Yes/No)": a sales document's Calculate Invoice
// Discount. After asking, it calculates the invoice discount (Sales-Calc. Discount) in one
// transaction and the page shows the new amounts.
const SalesDiscYesNoID = 61
const SalesDiscYesNoName = "sales-disc-yes-no"

func init() {
	Register(SalesDiscYesNoID, SalesDiscYesNoName, NewSalesDiscYesNo)
}

type SalesDiscYesNo struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewSalesDiscYesNo creates a new instance of the codeunit
func NewSalesDiscYesNo(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &SalesDiscYesNo{db: db, company: company, dbType: dbType}
}

func (c *SalesDiscYesNo) ID() int                    { return SalesDiscYesNoID }
func (c *SalesDiscYesNo) Name() string               { return SalesDiscYesNoName }
func (c *SalesDiscYesNo) SourceTable() string        { return gtables.SalesHeaderTableName }
func (c *SalesDiscYesNo) UsesProgress() bool         { return true } // it asks (Confirm)
func (c *SalesDiscYesNo) GetDB() database.Executor   { return c.db }
func (c *SalesDiscYesNo) GetCompany() string         { return c.company }
func (c *SalesDiscYesNo) GetDBType() database.DBType { return c.dbType }

// Run calculates the invoice discount of the document (the card's record)
func (c *SalesDiscYesNo) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.SalesHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 61 runs on a sales header")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *SalesDiscYesNo) code(rec *gtables.SalesHeader) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if !system.Confirm(ts.Message("SALES_CALC_INV_DISC_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	salesHeader := gtables.NewSalesHeader(src)
	err = salesHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = salescalcdisc.CalculateInvoiceDiscount(salesHeader)
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
