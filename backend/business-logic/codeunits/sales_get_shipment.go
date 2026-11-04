package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesgetshipment"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// SalesGetShipment - Codeunit 64 "Sales-Get Shipment": Get Shipment Lines on a sales invoice —
// the customer's shipment lines not invoiced on page 5708; the lines of the picked line's
// shipment go onto the invoice (business-logic/salesgetshipment), and the page reloads.
const SalesGetShipmentID = 64
const SalesGetShipmentName = "sales-get-shipment"

func init() {
	Register(SalesGetShipmentID, SalesGetShipmentName, NewSalesGetShipment)
}

type SalesGetShipment struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewSalesGetShipment creates a new instance of the codeunit
func NewSalesGetShipment(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &SalesGetShipment{db: db, company: company, dbType: dbType}
}

func (c *SalesGetShipment) ID() int                    { return SalesGetShipmentID }
func (c *SalesGetShipment) Name() string               { return SalesGetShipmentName }
func (c *SalesGetShipment) SourceTable() string        { return gtables.SalesHeaderTableName }
func (c *SalesGetShipment) UsesProgress() bool         { return true } // it opens a page (RunModal)
func (c *SalesGetShipment) GetDB() database.Executor   { return c.db }
func (c *SalesGetShipment) GetCompany() string         { return c.company }
func (c *SalesGetShipment) GetDBType() database.DBType { return c.dbType }

// Run gets shipment lines onto the invoice (the card's record)
func (c *SalesGetShipment) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.SalesHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 64 runs on a sales header")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *SalesGetShipment) code(rec *gtables.SalesHeader) (fcodeunits.Result, error) {
	src := ftables.DBSource(c.db, c.company, c.dbType)
	salesHeader := gtables.NewSalesHeader(src)
	err := salesHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	// the user picks outside the transaction (the page waits for the user)
	shipmentNo, picked, err := salesgetshipment.SelectShipment(salesHeader)
	if err != nil || !picked {
		return fcodeunits.Result{Success: true}, err
	}
	shipmentDescription := i18n.GetInstance().Message("SALES_SHIPMENT_NO_DESCRIPTION", fcodeunits.CurrentLanguage())
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	salesHeader.SetDB(tx)
	err = salesgetshipment.CreateInvLines(salesHeader, shipmentNo, shipmentDescription)
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
