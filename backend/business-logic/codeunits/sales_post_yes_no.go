package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// SalesPostYesNo - Codeunit 81 "Sales-Post (Yes/No)": the Post (F11) of a sales document. An
// order asks Ship, Invoice or Ship and Invoice, an invoice or credit memo whether to post it;
// then Sales-Post (codeunit 80) posts it in one transaction. The page reloads the order that
// goes on, or opens the posted invoice or credit memo of a document posted completely.
const SalesPostYesNoID = 81
const SalesPostYesNoName = "sales-post-yes-no"

// The posted documents' cards a completely posted document opens on
const (
	postedSalesInvoiceCardID  = 132
	postedSalesCrMemoCardID   = 134
	postedSalesShipmentCardID = 130
)

func init() {
	Register(SalesPostYesNoID, SalesPostYesNoName, NewSalesPostYesNo)
}

type SalesPostYesNo struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewSalesPostYesNo creates a new instance of the codeunit
func NewSalesPostYesNo(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &SalesPostYesNo{db: db, company: company, dbType: dbType}
}

func (c *SalesPostYesNo) ID() int                    { return SalesPostYesNoID }
func (c *SalesPostYesNo) Name() string               { return SalesPostYesNoName }
func (c *SalesPostYesNo) SourceTable() string        { return gtables.SalesHeaderTableName }
func (c *SalesPostYesNo) UsesProgress() bool         { return true } // it asks (StrMenu, Confirm)
func (c *SalesPostYesNo) GetDB() database.Executor   { return c.db }
func (c *SalesPostYesNo) GetCompany() string         { return c.company }
func (c *SalesPostYesNo) GetDBType() database.DBType { return c.dbType }

// Run posts the document (the card's or the list's record)
func (c *SalesPostYesNo) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.SalesHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 81 runs on a sales header")
	}
	result, _, err := c.post(rec)
	return result, err
}

// askShipInvoice asks what to post of an order (Ship, Invoice, Ship and Invoice); ok is false
// when the user cancelled
func askShipInvoice() (ship, invoice, ok bool) {
	ts := i18n.GetInstance()
	selection := system.StrMenu(ts.Message("SALES_POST_SHIP_INVOICE", fcodeunits.CurrentLanguage()), 3)
	if selection == 0 {
		return false, false, false
	}
	return selection == 1 || selection == 3, selection == 2 || selection == 3, true
}

// postSalesDocument posts the document (codeunit 80) in src, the posting's transaction: an
// order ships and/or invoices as asked
func postSalesDocument(src ftables.Source, rec *gtables.SalesHeader, ship, invoice bool) (*salesposting.SalesPost, error) {
	salesHeader := gtables.NewSalesHeader(src)
	err := salesHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		return nil, err
	}
	if salesHeader.Document_type == gtables.SalesHeader_Document_type.Order {
		salesHeader.Ship = ship
		salesHeader.Invoice = invoice
	}
	salesPost := salesposting.NewSalesPost(src)
	err = salesPost.Run(salesHeader)
	if err != nil {
		return nil, err
	}
	return salesPost, nil
}

// post asks, posts and returns the page's result and what was posted (nil when the user
// cancelled)
func (c *SalesPostYesNo) post(rec *gtables.SalesHeader) (fcodeunits.Result, *salesposting.SalesPost, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	ship, invoice := false, false
	if rec.Document_type == gtables.SalesHeader_Document_type.Order {
		var ok bool
		ship, invoice, ok = askShipInvoice()
		if !ok {
			return fcodeunits.Result{Success: true}, nil, nil
		}
	} else {
		documentType := ts.OptionCaption(gtables.SalesHeaderTableName, "document_type", i18n.Key(rec.Document_type.String()), lang)
		if !system.Confirm(system.StrSubstNo(ts.Message("SALES_POST_CONFIRM", lang), documentType)) {
			return fcodeunits.Result{Success: true}, nil, nil
		}
	}

	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, nil, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	salesPost, err := postSalesDocument(src, rec, ship, invoice)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, nil, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, nil, err
	}

	if !salesPost.Deleted {
		return fcodeunits.Result{Success: true, Refresh: true}, salesPost, nil
	}
	result := fcodeunits.Result{Success: true}
	switch {
	case salesPost.InvoiceNo != "":
		result.OpenPage = &fcodeunits.PageLink{PageID: postedSalesInvoiceCardID, Key: salesPost.InvoiceNo.String()}
	case salesPost.CrMemoNo != "":
		result.OpenPage = &fcodeunits.PageLink{PageID: postedSalesCrMemoCardID, Key: salesPost.CrMemoNo.String()}
	case salesPost.ShipmentNo != "":
		result.OpenPage = &fcodeunits.PageLink{PageID: postedSalesShipmentCardID, Key: salesPost.ShipmentNo.String()}
	}
	return result, salesPost, nil
}
