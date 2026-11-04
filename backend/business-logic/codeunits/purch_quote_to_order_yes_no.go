package codeunits

import (
	"errors"
	"fmt"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchquotetoorder"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// PurchQuoteToOrderYesNo - Codeunit 93 "Purch.-Quote to Order (Yes/No)": a quote's Make Order.
// After asking, the quote becomes an order (Purch.-Quote to Order, codeunit 96) in one
// transaction; the message names both, and the page opens the order (Purchase Order card 50).
const PurchQuoteToOrderYesNoID = 93
const PurchQuoteToOrderYesNoName = "purch-quote-to-order-yes-no"

// purchOrderCardID: the Purchase Order card the new order opens on
const purchOrderCardID = 50

func init() {
	Register(PurchQuoteToOrderYesNoID, PurchQuoteToOrderYesNoName, NewPurchQuoteToOrderYesNo)
}

type PurchQuoteToOrderYesNo struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewPurchQuoteToOrderYesNo creates a new instance of the codeunit
func NewPurchQuoteToOrderYesNo(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &PurchQuoteToOrderYesNo{db: db, company: company, dbType: dbType}
}

func (c *PurchQuoteToOrderYesNo) ID() int                    { return PurchQuoteToOrderYesNoID }
func (c *PurchQuoteToOrderYesNo) Name() string               { return PurchQuoteToOrderYesNoName }
func (c *PurchQuoteToOrderYesNo) SourceTable() string        { return gtables.PurchaseHeaderTableName }
func (c *PurchQuoteToOrderYesNo) UsesProgress() bool         { return true } // it asks (Confirm)
func (c *PurchQuoteToOrderYesNo) GetDB() database.Executor   { return c.db }
func (c *PurchQuoteToOrderYesNo) GetCompany() string         { return c.company }
func (c *PurchQuoteToOrderYesNo) GetDBType() database.DBType { return c.dbType }

// Run makes an order of the quote (the card's or the list's record)
func (c *PurchQuoteToOrderYesNo) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.PurchaseHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 93 runs on a purchase header")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *PurchQuoteToOrderYesNo) code(rec *gtables.PurchaseHeader) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	err := rec.TestField("document_type", gtables.PurchaseHeader_Document_type.Quote)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !system.Confirm(ts.Message("PURCH_QUOTE_TO_ORDER_CONFIRM", lang)) {
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
	purchHeader2, err := purchquotetoorder.Run(purchHeader)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	result := system.Message(ts.Message("PURCH_QUOTE_CHANGED_TO_ORDER", lang), rec.No, purchHeader2.No)
	result.OpenPage = &fcodeunits.PageLink{PageID: purchOrderCardID, Key: fmt.Sprintf("%d,%s", int(purchHeader2.Document_type), purchHeader2.No)}
	return result, nil
}
