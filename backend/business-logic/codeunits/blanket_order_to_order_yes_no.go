package codeunits

import (
	"errors"

	"github.com/hansjlachmann/openerp/backend/business-logic/blanketorder"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Codeunits 84 "Blnkt Sales Ord. to Ord. (Y/N)" and 94 "Blnkt Purch Ord. to Ord. (Y/N)": a
// blanket order's Make Order. After asking, an order is made from the blanket order (codeunits
// 87 / 97) in one transaction; the message names both, and the blanket order stays open.
const (
	BlanketSalesOrderToOrderYesNoID = 84
	BlanketPurchOrderToOrderYesNoID = 94
)

func init() {
	Register(BlanketSalesOrderToOrderYesNoID, "blanket-sales-order-to-order-yes-no", func(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
		return &blanketOrderToOrderYesNo{db: db, company: company, dbType: dbType, purchase: false}
	})
	Register(BlanketPurchOrderToOrderYesNoID, "blanket-purch-order-to-order-yes-no", func(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
		return &blanketOrderToOrderYesNo{db: db, company: company, dbType: dbType, purchase: true}
	})
}

type blanketOrderToOrderYesNo struct {
	db       database.Executor
	company  string
	dbType   database.DBType
	purchase bool
}

func (c *blanketOrderToOrderYesNo) ID() int {
	if c.purchase {
		return BlanketPurchOrderToOrderYesNoID
	}
	return BlanketSalesOrderToOrderYesNoID
}

func (c *blanketOrderToOrderYesNo) Name() string {
	if c.purchase {
		return "blanket-purch-order-to-order-yes-no"
	}
	return "blanket-sales-order-to-order-yes-no"
}

func (c *blanketOrderToOrderYesNo) SourceTable() string {
	if c.purchase {
		return gtables.PurchaseHeaderTableName
	}
	return gtables.SalesHeaderTableName
}

func (c *blanketOrderToOrderYesNo) UsesProgress() bool         { return true } // it asks (Confirm)
func (c *blanketOrderToOrderYesNo) GetDB() database.Executor   { return c.db }
func (c *blanketOrderToOrderYesNo) GetCompany() string         { return c.company }
func (c *blanketOrderToOrderYesNo) GetDBType() database.DBType { return c.dbType }

// Run makes an order of the blanket order (the card's or the list's record)
func (c *blanketOrderToOrderYesNo) Run(record interface{}) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	var documentType interface{}
	var key map[string]interface{}
	var no string
	switch rec := record.(type) {
	case *gtables.SalesHeader:
		if c.purchase {
			return fcodeunits.Result{}, errors.New("codeunit 94 runs on a purchase header")
		}
		err := rec.TestField("document_type", gtables.SalesHeader_Document_type.BlanketOrder)
		if err != nil {
			return fcodeunits.Result{}, err
		}
		documentType, no = int(rec.Document_type), rec.No.String()
	case *gtables.PurchaseHeader:
		if !c.purchase {
			return fcodeunits.Result{}, errors.New("codeunit 84 runs on a sales header")
		}
		err := rec.TestField("document_type", gtables.PurchaseHeader_Document_type.BlanketOrder)
		if err != nil {
			return fcodeunits.Result{}, err
		}
		documentType, no = int(rec.Document_type), rec.No.String()
	default:
		return fcodeunits.Result{}, errors.New("a blanket order to order codeunit runs on a sales or purchase header")
	}
	key = map[string]interface{}{"document_type": documentType, "no": no}
	if !system.Confirm(ts.Message("BLANKET_ORDER_TO_ORDER_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	orderNo := ""
	if c.purchase {
		purchHeader := gtables.NewPurchaseHeader(src)
		err = purchHeader.GetOrError(key)
		if err == nil {
			var purchOrderHeader *gtables.PurchaseHeader
			purchOrderHeader, err = blanketorder.PurchToOrder(purchHeader)
			if err == nil {
				orderNo = purchOrderHeader.No.String()
			}
		}
	} else {
		salesHeader := gtables.NewSalesHeader(src)
		err = salesHeader.GetOrError(key)
		if err == nil {
			var salesOrderHeader *gtables.SalesHeader
			salesOrderHeader, err = blanketorder.SalesToOrder(salesHeader)
			if err == nil {
				orderNo = salesOrderHeader.No.String()
			}
		}
	}
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	result := system.Message(ts.Message("BLANKET_ORDER_CHANGED_TO_ORDER", lang), orderNo, no)
	result.Refresh = true
	return result, nil
}
