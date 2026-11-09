package reports

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchcalcdisc"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salescalcdisc"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The batch posting of documents (Posting › Post Batch on the document lists; Sales/Purchases →
// Activities): 296 Batch Post Sales Orders, 297 Batch Post Sales Invoices, 298 Batch Post Sales
// Credit Memos, 496 Batch Post Purchase Orders, 497 Batch Post Purch. Invoices, 498 Batch Post
// Purch. Credit Memos. Every document in the request filters is posted by codeunit 80/90 in a
// transaction of its own — orders shipping/receiving and/or invoicing as asked —, with the
// invoice discount calculated first (Calc. Inv. Discount, from the setup) and the posting and
// document dates replaced when asked; a document that cannot be posted is left as it is and
// counted. The message tells how many of the documents were posted.
const (
	BatchPostSalesOrdersID   = 296
	BatchPostSalesInvoicesID = 297
	BatchPostSalesCrMemosID  = 298
	BatchPostPurchOrdersID   = 496
	BatchPostPurchInvoicesID = 497
	BatchPostPurchCrMemosID  = 498
	batchPostSalesHeader     = "Sales_Header"
	batchPostPurchHeader     = "Purchase_Header"
	batchPostOptionShip      = "ship"
	batchPostOptionReceive   = "receive"
	batchPostOptionInvoice   = "invoice"
)

// PurchPost posts a purchase document in src (codeunit 90 Purch.-Post); set by the codeunits
// package (the reports cannot import the posting)
var PurchPost func(src ftables.Source, purchHeader *gtables.PurchaseHeader) error

func init() {
	Register(BatchPostSalesOrdersID, "batch-post-sales-orders", func() Report {
		return &batchPostSales{documentType: gtables.SalesHeader_Document_type.Order}
	})
	Register(BatchPostSalesInvoicesID, "batch-post-sales-invoices", func() Report {
		return &batchPostSales{documentType: gtables.SalesHeader_Document_type.Invoice}
	})
	Register(BatchPostSalesCrMemosID, "batch-post-sales-credit-memos", func() Report {
		return &batchPostSales{documentType: gtables.SalesHeader_Document_type.CreditMemo}
	})
	Register(BatchPostPurchOrdersID, "batch-post-purchase-orders", func() Report {
		return &batchPostPurch{documentType: gtables.PurchaseHeader_Document_type.Order}
	})
	Register(BatchPostPurchInvoicesID, "batch-post-purch-invoices", func() Report {
		return &batchPostPurch{documentType: gtables.PurchaseHeader_Document_type.Invoice}
	})
	Register(BatchPostPurchCrMemosID, "batch-post-purch-credit-memos", func() Report {
		return &batchPostPurch{documentType: gtables.PurchaseHeader_Document_type.CreditMemo}
	})
}

// batchPostOptions: Ship/Receive and Invoice (orders), Posting Date, Replace Posting Date,
// Replace Document Date, Calc. Inv. Discount
func batchPostOptions(order bool, shipOption string, calcInvDiscount bool) []Option {
	var options []Option
	if order {
		options = append(options,
			Option{Name: shipOption, Type: "bool"},
			Option{Name: batchPostOptionInvoice, Type: "bool"})
	}
	calcInvDisc := "false"
	if calcInvDiscount {
		calcInvDisc = "true"
	}
	return append(options,
		Option{Name: "posting_date", Type: "date"},
		Option{Name: "replace_posting_date", Type: "bool"},
		Option{Name: "replace_document_date", Type: "bool"},
		Option{Name: "calc_inv_discount", Type: "bool", Default: calcInvDisc})
}

// batchPost is what the six reports share: the counters and the checks of the options
type batchPost struct {
	counter, counterOK int
}

// checkPostingDate: Replace Posting Date needs a posting date
func (b *batchPost) checkPostingDate(run *Run) error {
	if run.OptionBool("replace_posting_date") && run.OptionDate("posting_date").IsZero() {
		return system.Error(run.Text("ENTER_POSTING_DATE"))
	}
	return nil
}

// postInTransaction runs post in a transaction of its own (classic COMMIT + IF CODEUNIT.RUN):
// a refusal rolls it back and is only counted
func (b *batchPost) postInTransaction(run *Run, post func(src ftables.Source) error) error {
	b.counter++
	tx, err := ftables.BeginTransaction(run.GetDB())
	if err != nil {
		return err
	}
	err = post(ftables.DBSource(tx, run.GetCompany(), run.GetDBType()))
	if err != nil {
		_ = ftables.Rollback(tx)
		return nil
	}
	err = ftables.Commit(tx)
	if err != nil {
		return nil
	}
	b.counterOK++
	return nil
}

// Reports 296/297/298: the sales orders, invoices or credit memos
type batchPostSales struct {
	batchPost
	documentType gtables.SalesHeaderDocument_type
}

func (r *batchPostSales) Definition() *Definition {
	order := r.documentType == gtables.SalesHeader_Document_type.Order
	return &Definition{
		OptionsCaption: "OPTIONS",
		OnInitReport: func(run *Run) error {
			salesSetup := gtables.GetSalesReceivablesSetup(run)
			if salesSetup.Calc_inv_discount {
				run.SetOption("calc_inv_discount", "true")
			}
			return nil
		},
		Options: batchPostOptions(order, batchPostOptionShip, false),
		DataItems: []*DataItem{{
			Name: batchPostSalesHeader, Table: gtables.SalesHeaderTableName, Key: []string{"document_type", "no"},
			TableView:           []ftables.FieldFilter{{Field: "document_type", Expression: system.Format(int(r.documentType))}},
			RequestFilterFields: []string{"no"},
			OnPreDataItem: Trigger(func(salesHeader *gtables.SalesHeader, run *Run) error {
				return r.checkPostingDate(run)
			}),
			OnAfterGetRecord: Trigger(func(salesHeader *gtables.SalesHeader, run *Run) error {
				return r.postInTransaction(run, func(src ftables.Source) error {
					return r.post(run, src, salesHeader)
				})
			}),
			OnPostDataItem: Trigger(func(salesHeader *gtables.SalesHeader, run *Run) error {
				run.Message(run.Text("POSTED", r.counterOK, r.counter))
				return nil
			}),
		}},
	}
}

// post: the document read again in the transaction, its invoice discount, Ship/Invoice and dates
// set, then codeunit 80
func (r *batchPostSales) post(run *Run, src ftables.Source, rec *gtables.SalesHeader) error {
	if SalesPost == nil {
		return errors.New("batch post: no sales posting")
	}
	salesHeader := gtables.NewSalesHeader(src)
	err := salesHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		return err
	}
	if run.OptionBool("calc_inv_discount") {
		err = salescalcdisc.CalculateInvoiceDiscount(salesHeader)
		if err != nil {
			return err
		}
		err = salesHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
		if err != nil {
			return err
		}
	}
	if r.documentType == gtables.SalesHeader_Document_type.Order {
		salesHeader.Ship = run.OptionBool(batchPostOptionShip)
		salesHeader.Invoice = run.OptionBool(batchPostOptionInvoice)
	}
	if run.OptionBool("replace_posting_date") {
		err = salesHeader.Validate("posting_date", run.OptionDate("posting_date"))
		if err != nil {
			return err
		}
	}
	if run.OptionBool("replace_document_date") {
		err = salesHeader.Validate("document_date", run.OptionDate("posting_date"))
		if err != nil {
			return err
		}
	}
	err = salesHeader.ModifyOrError(false)
	if err != nil {
		return err
	}
	return SalesPost(src, salesHeader)
}

// Reports 496/497/498: the purchase orders, invoices or credit memos
type batchPostPurch struct {
	batchPost
	documentType gtables.PurchaseHeaderDocument_type
}

func (r *batchPostPurch) Definition() *Definition {
	order := r.documentType == gtables.PurchaseHeader_Document_type.Order
	return &Definition{
		OptionsCaption: "OPTIONS",
		OnInitReport: func(run *Run) error {
			purchSetup := gtables.GetPurchasesPayablesSetup(run)
			if purchSetup.Calc_inv_discount {
				run.SetOption("calc_inv_discount", "true")
			}
			return nil
		},
		Options: batchPostOptions(order, batchPostOptionReceive, false),
		DataItems: []*DataItem{{
			Name: batchPostPurchHeader, Table: gtables.PurchaseHeaderTableName, Key: []string{"document_type", "no"},
			TableView:           []ftables.FieldFilter{{Field: "document_type", Expression: system.Format(int(r.documentType))}},
			RequestFilterFields: []string{"no"},
			OnPreDataItem: Trigger(func(purchHeader *gtables.PurchaseHeader, run *Run) error {
				return r.checkPostingDate(run)
			}),
			OnAfterGetRecord: Trigger(func(purchHeader *gtables.PurchaseHeader, run *Run) error {
				return r.postInTransaction(run, func(src ftables.Source) error {
					return r.post(run, src, purchHeader)
				})
			}),
			OnPostDataItem: Trigger(func(purchHeader *gtables.PurchaseHeader, run *Run) error {
				run.Message(run.Text("POSTED", r.counterOK, r.counter))
				return nil
			}),
		}},
	}
}

// post: the document read again in the transaction, its invoice discount, Receive/Invoice and
// dates set, then codeunit 90
func (r *batchPostPurch) post(run *Run, src ftables.Source, rec *gtables.PurchaseHeader) error {
	if PurchPost == nil {
		return errors.New("batch post: no purchase posting")
	}
	purchHeader := gtables.NewPurchaseHeader(src)
	err := purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
	if err != nil {
		return err
	}
	if run.OptionBool("calc_inv_discount") {
		err = purchcalcdisc.CalculateInvoiceDiscount(purchHeader)
		if err != nil {
			return err
		}
		err = purchHeader.GetOrError(map[string]interface{}{"document_type": int(rec.Document_type), "no": rec.No})
		if err != nil {
			return err
		}
	}
	if r.documentType == gtables.PurchaseHeader_Document_type.Order {
		purchHeader.Receive = run.OptionBool(batchPostOptionReceive)
		purchHeader.Invoice = run.OptionBool(batchPostOptionInvoice)
	}
	if run.OptionBool("replace_posting_date") {
		err = purchHeader.Validate("posting_date", run.OptionDate("posting_date"))
		if err != nil {
			return err
		}
	}
	if run.OptionBool("replace_document_date") {
		err = purchHeader.Validate("document_date", run.OptionDate("posting_date"))
		if err != nil {
			return err
		}
	}
	err = purchHeader.ModifyOrError(false)
	if err != nil {
		return err
	}
	return PurchPost(src, purchHeader)
}
