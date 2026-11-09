package salestax

import (
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Functions › Recalculate Sales Tax on the sales and purchase documents (classic RecalcSalesYN /
// RecalcPurchYN): after asking, the document's tax is recalculated per group of lines, in one
// transaction; the page reloads
func init() {
	functions.Register("table", gtables.SalesHeaderTableName, "RecalculateSalesTax", functions.For(RecalcSalesYN), functions.WithDialog())
	functions.Register("table", gtables.PurchaseHeaderTableName, "RecalculateSalesTax", functions.For(RecalcPurchYN), functions.WithDialog())
}

// RecalcSalesYN asks, then recalculates the sales document's tax
func RecalcSalesYN(rec *gtables.SalesHeader) (fcodeunits.Result, error) {
	if !system.Confirm(i18n.GetInstance().Message("RECALCULATE_SALES_TAX_CONFIRM", fcodeunits.CurrentLanguage())) {
		return fcodeunits.Result{Success: true}, nil
	}
	tx, err := ftables.BeginTransaction(rec.GetDB())
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, rec.GetCompany(), rec.GetDBType())
	salesHeader := gtables.NewSalesHeader(src)
	err = salesHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err == nil {
		err = New(src).RecalcSalesLines(salesHeader)
	}
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

// RecalcPurchYN asks, then recalculates the purchase document's tax
func RecalcPurchYN(rec *gtables.PurchaseHeader) (fcodeunits.Result, error) {
	if !system.Confirm(i18n.GetInstance().Message("RECALCULATE_SALES_TAX_CONFIRM", fcodeunits.CurrentLanguage())) {
		return fcodeunits.Result{Success: true}, nil
	}
	tx, err := ftables.BeginTransaction(rec.GetDB())
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, rec.GetCompany(), rec.GetDBType())
	purchHeader := gtables.NewPurchaseHeader(src)
	err = purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err == nil {
		err = New(src).RecalcPurchLines(purchHeader)
	}
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
