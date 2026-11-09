package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchgetdropshpt"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// PurchGetDropShpt - Codeunit 76 "Purch.-Get Drop Shpt.": Get Sales Order on a purchase order —
// the user picks a sales order of the purchase order's customer; its drop shipment lines become
// purchase order lines linked to them (business-logic/purchgetdropshpt), and the page reloads.
const PurchGetDropShptID = 76
const PurchGetDropShptName = "purch-get-drop-shpt"

func init() {
	Register(PurchGetDropShptID, PurchGetDropShptName, NewPurchGetDropShpt)
}

type PurchGetDropShpt struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewPurchGetDropShpt creates a new instance of the codeunit
func NewPurchGetDropShpt(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &PurchGetDropShpt{db: db, company: company, dbType: dbType}
}

func (c *PurchGetDropShpt) ID() int                    { return PurchGetDropShptID }
func (c *PurchGetDropShpt) Name() string               { return PurchGetDropShptName }
func (c *PurchGetDropShpt) SourceTable() string        { return gtables.PurchaseHeaderTableName }
func (c *PurchGetDropShpt) UsesProgress() bool         { return true } // it opens a page (RunModal)
func (c *PurchGetDropShpt) GetDB() database.Executor   { return c.db }
func (c *PurchGetDropShpt) GetCompany() string         { return c.company }
func (c *PurchGetDropShpt) GetDBType() database.DBType { return c.dbType }

// Run gets a sales order's drop shipment lines onto the purchase order (the card's record)
func (c *PurchGetDropShpt) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.PurchaseHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 76 runs on a purchase header")
	}
	src := ftables.DBSource(c.db, c.company, c.dbType)
	purchHeader := gtables.NewPurchaseHeader(src)
	err := purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	// the user picks outside the transaction (the page waits for the user)
	salesOrderNo, picked, err := purchgetdropshpt.SelectSalesOrder(purchHeader)
	if err != nil || !picked {
		return fcodeunits.Result{Success: true}, err
	}
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	purchHeader.SetDB(tx)
	err = purchgetdropshpt.Code(purchHeader, salesOrderNo)
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
