package codeunits

import (
	"errors"
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesquotetoorder"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// SalesQuoteToOrderYesNo - Codeunit 83 "Sales-Quote to Order (Yes/No)": a quote's Make Order.
// After asking, the quote becomes an order (Sales-Quote to Order, codeunit 86) in one
// transaction; the message names both, and the page opens the order (Sales Order card 42).
const SalesQuoteToOrderYesNoID = 83
const SalesQuoteToOrderYesNoName = "sales-quote-to-order-yes-no"

// salesOrderCardID: the Sales Order card the new order opens on
const salesOrderCardID = 42

func init() {
	Register(SalesQuoteToOrderYesNoID, SalesQuoteToOrderYesNoName, NewSalesQuoteToOrderYesNo)
}

type SalesQuoteToOrderYesNo struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewSalesQuoteToOrderYesNo creates a new instance of the codeunit
func NewSalesQuoteToOrderYesNo(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &SalesQuoteToOrderYesNo{db: db, company: company, dbType: dbType}
}

func (c *SalesQuoteToOrderYesNo) ID() int                    { return SalesQuoteToOrderYesNoID }
func (c *SalesQuoteToOrderYesNo) Name() string               { return SalesQuoteToOrderYesNoName }
func (c *SalesQuoteToOrderYesNo) SourceTable() string        { return gtables.SalesHeaderTableName }
func (c *SalesQuoteToOrderYesNo) UsesProgress() bool         { return true } // it asks (Confirm)
func (c *SalesQuoteToOrderYesNo) GetDB() database.Executor   { return c.db }
func (c *SalesQuoteToOrderYesNo) GetCompany() string         { return c.company }
func (c *SalesQuoteToOrderYesNo) GetDBType() database.DBType { return c.dbType }

// Run makes an order of the quote (the card's or the list's record)
func (c *SalesQuoteToOrderYesNo) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.SalesHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 83 runs on a sales header")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *SalesQuoteToOrderYesNo) code(rec *gtables.SalesHeader) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := rec.TestField("document_type", gtables.SalesHeader_Document_type.Quote)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !system.Confirm(ts.Message("SALES_QUOTE_TO_ORDER_CONFIRM", lang)) {
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
	salesHeader2, err := salesquotetoorder.Run(salesHeader)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	result := system.Message(ts.Message("SALES_QUOTE_CHANGED_TO_ORDER", lang), rec.No, salesHeader2.No)
	result.OpenPage = &fcodeunits.PageLink{PageID: salesOrderCardID, Key: fmt.Sprintf("%d,%s", int(salesHeader2.Document_type), salesHeader2.No)}
	return result, nil
}
