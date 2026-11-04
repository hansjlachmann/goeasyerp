package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// PurchPostYesNo - Codeunit 91 "Purch.-Post (Yes/No)": the Post (F11) of a purchase document. An
// order asks Receive, Invoice or Receive and Invoice, an invoice or credit memo whether to post it;
// then Purch.-Post (codeunit 90) posts it in one transaction. The page reloads the order that
// goes on, or opens the posted invoice or credit memo of a document posted completely.
const PurchPostYesNoID = 91
const PurchPostYesNoName = "purch-post-yes-no"

// The posted documents' cards a completely posted document opens on
const (
	postedPurchInvoiceCardID = 138
	postedPurchCrMemoCardID  = 140
	postedPurchReceiptCardID = 136
)

func init() {
	Register(PurchPostYesNoID, PurchPostYesNoName, NewPurchPostYesNo)
}

type PurchPostYesNo struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewPurchPostYesNo creates a new instance of the codeunit
func NewPurchPostYesNo(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &PurchPostYesNo{db: db, company: company, dbType: dbType}
}

func (c *PurchPostYesNo) ID() int                    { return PurchPostYesNoID }
func (c *PurchPostYesNo) Name() string               { return PurchPostYesNoName }
func (c *PurchPostYesNo) SourceTable() string        { return gtables.PurchaseHeaderTableName }
func (c *PurchPostYesNo) UsesProgress() bool         { return true } // it asks (StrMenu, Confirm)
func (c *PurchPostYesNo) GetDB() database.Executor   { return c.db }
func (c *PurchPostYesNo) GetCompany() string         { return c.company }
func (c *PurchPostYesNo) GetDBType() database.DBType { return c.dbType }

// Run posts the document (the card's or the list's record)
func (c *PurchPostYesNo) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.PurchaseHeader)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 91 runs on a purchase header")
	}
	result, _, err := c.post(rec)
	return result, err
}

// askReceiveInvoice asks what to post of an order (Receive, Invoice, Receive and Invoice); ok is false
// when the user cancelled
func askReceiveInvoice() (receive, invoice, ok bool) {
	ts := i18n.GetInstance()
	selection := system.StrMenu(ts.Message("PURCH_POST_RECEIVE_INVOICE", fcodeunits.CurrentLanguage()), 3)
	if selection == 0 {
		return false, false, false
	}
	return selection == 1 || selection == 3, selection == 2 || selection == 3, true
}

// postPurchDocument posts the document (codeunit 90) in src, the posting's transaction: an
// order receives and/or invoices as asked
func postPurchDocument(src ftables.Source, rec *gtables.PurchaseHeader, receive, invoice bool) (*purchposting.PurchPost, error) {
	purchHeader := gtables.NewPurchaseHeader(src)
	err := purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		return nil, err
	}
	if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order {
		purchHeader.Receive = receive
		purchHeader.Invoice = invoice
	}
	purchPost := purchposting.NewPurchPost(src)
	err = purchPost.Run(purchHeader)
	if err != nil {
		return nil, err
	}
	return purchPost, nil
}

// post asks, posts and returns the page's result and what was posted (nil when the user
// cancelled)
func (c *PurchPostYesNo) post(rec *gtables.PurchaseHeader) (fcodeunits.Result, *purchposting.PurchPost, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	receive, invoice := false, false
	if rec.Document_type == gtables.PurchaseHeader_Document_type.Order {
		var ok bool
		receive, invoice, ok = askReceiveInvoice()
		if !ok {
			return fcodeunits.Result{Success: true}, nil, nil
		}
	} else {
		documentType := ts.OptionCaption(gtables.PurchaseHeaderTableName, "document_type", i18n.Key(rec.Document_type.String()), lang)
		if !system.Confirm(system.StrSubstNo(ts.Message("PURCH_POST_CONFIRM", lang), documentType)) {
			return fcodeunits.Result{Success: true}, nil, nil
		}
	}

	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, nil, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	purchPost, err := postPurchDocument(src, rec, receive, invoice)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, nil, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, nil, err
	}

	if !purchPost.Deleted {
		return fcodeunits.Result{Success: true, Refresh: true}, purchPost, nil
	}
	result := fcodeunits.Result{Success: true}
	switch {
	case purchPost.InvoiceNo != "":
		result.OpenPage = &fcodeunits.PageLink{PageID: postedPurchInvoiceCardID, Key: purchPost.InvoiceNo.String()}
	case purchPost.CrMemoNo != "":
		result.OpenPage = &fcodeunits.PageLink{PageID: postedPurchCrMemoCardID, Key: purchPost.CrMemoNo.String()}
	case purchPost.ReceiptNo != "":
		result.OpenPage = &fcodeunits.PageLink{PageID: postedPurchReceiptCardID, Key: purchPost.ReceiptNo.String()}
	}
	return result, purchPost, nil
}
