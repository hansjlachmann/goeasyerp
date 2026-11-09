package reports

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Reports 299 Delete Invoiced Sales Orders, 291 Delete Invd Blnkt Sales Orders, 499 Delete
// Invoiced Purch. Orders and 491 Delete Invd Blnkt Purch Orders: the orders (blanket orders)
// whose lines are all shipped (received) and invoiced — at least one line invoiced, nothing
// outstanding, nothing shipped (received) and not invoiced; a blanket order also no longer used
// by an order line — are deleted with their lines and comments, without their triggers.
const (
	DeleteInvdBlnktSalesOrdersID = 291
	DeleteInvoicedSalesOrdersID  = 299
	DeleteInvdBlnktPurchOrdersID = 491
	DeleteInvoicedPurchOrdersID  = 499
)

func init() {
	Register(DeleteInvdBlnktSalesOrdersID, "delete-invd-blnkt-sales-orders", func() Report {
		return &deleteInvoicedSalesOrders{documentType: gtables.SalesHeader_Document_type.BlanketOrder}
	})
	Register(DeleteInvoicedSalesOrdersID, "delete-invoiced-sales-orders", func() Report {
		return &deleteInvoicedSalesOrders{documentType: gtables.SalesHeader_Document_type.Order}
	})
	Register(DeleteInvdBlnktPurchOrdersID, "delete-invd-blnkt-purch-orders", func() Report {
		return &deleteInvoicedPurchOrders{documentType: gtables.PurchaseHeader_Document_type.BlanketOrder}
	})
	Register(DeleteInvoicedPurchOrdersID, "delete-invoiced-purch-orders", func() Report {
		return &deleteInvoicedPurchOrders{documentType: gtables.PurchaseHeader_Document_type.Order}
	})
}

// deleteInvoicedSalesOrders is report 299 (orders) or 291 (blanket orders)
type deleteInvoicedSalesOrders struct {
	documentType gtables.SalesHeaderDocument_type
	window       *system.Window
	noOfDeleted  int
}

func (r *deleteInvoicedSalesOrders) Definition() *Definition {
	return &Definition{
		Transaction: true,
		DataItems: []*DataItem{{
			Name: "Sales_Header", Table: gtables.SalesHeaderTableName, Key: []string{"document_type", "no"},
			TableView:           []ftables.FieldFilter{{Field: "document_type", Expression: strconv.Itoa(int(r.documentType))}},
			RequestFilterFields: []string{"no", "sell_to_customer_no", "bill_to_customer_no"},
			OnPreDataItem: func(rec any, run *Run) error {
				r.window = system.OpenWindow(run.Text("WINDOW"))
				return nil
			},
			OnAfterGetRecord: Trigger(r.onAfterGetRecord),
			OnPostDataItem: func(rec any, run *Run) error {
				r.window.Close()
				run.Message(run.Text("DELETED", r.noOfDeleted))
				return nil
			},
		}},
	}
}

func (r *deleteInvoicedSalesOrders) onAfterGetRecord(salesHeader *gtables.SalesHeader, run *Run) error {
	r.window.Update(1, salesHeader.No)

	salesLine := gtables.NewSalesLine(run)
	salesLine.SetRange("document_type", gtables.SalesLineDocument_type(salesHeader.Document_type))
	salesLine.SetRange("document_no", salesHeader.No)
	salesLine.SetFilter("quantity_invoiced", "<>0")
	if salesLine.FindFirst() {
		salesLine.SetRange("quantity_invoiced")
		salesLine.SetFilter("outstanding_quantity", "<>0")
		if !salesLine.FindFirst() {
			salesLine.SetRange("outstanding_quantity")
			salesLine.SetFilter("qty_shipped_not_invoiced", "<>0")
			if !salesLine.FindFirst() {
				salesLine.SetRange("qty_shipped_not_invoiced")
				if r.documentType == gtables.SalesHeader_Document_type.BlanketOrder {
					salesLine2 := gtables.NewSalesLine(run)
					salesLine2.SetRange("blanket_order_no", salesHeader.No)
					if salesLine2.FindFirst() {
						return nil
					}
				}
				salesLine.DeleteAll()

				salesCommentLine := gtables.NewSalesCommentLine(run)
				salesCommentLine.SetRange("document_type", gtables.SalesCommentLineDocument_type(salesHeader.Document_type))
				salesCommentLine.SetRange("no", salesHeader.No)
				salesCommentLine.DeleteAll()

				err := salesHeader.DeleteOrError(false)
				if err != nil {
					return err
				}
				r.noOfDeleted++
			}
		}
	}
	return nil
}

// deleteInvoicedPurchOrders is report 499 (orders) or 491 (blanket orders)
type deleteInvoicedPurchOrders struct {
	documentType gtables.PurchaseHeaderDocument_type
	window       *system.Window
	noOfDeleted  int
}

func (r *deleteInvoicedPurchOrders) Definition() *Definition {
	return &Definition{
		Transaction: true,
		DataItems: []*DataItem{{
			Name: "Purchase_Header", Table: gtables.PurchaseHeaderTableName, Key: []string{"document_type", "no"},
			TableView:           []ftables.FieldFilter{{Field: "document_type", Expression: strconv.Itoa(int(r.documentType))}},
			RequestFilterFields: []string{"no", "buy_from_vendor_no", "pay_to_vendor_no"},
			OnPreDataItem: func(rec any, run *Run) error {
				r.window = system.OpenWindow(run.Text("WINDOW"))
				return nil
			},
			OnAfterGetRecord: Trigger(r.onAfterGetRecord),
			OnPostDataItem: func(rec any, run *Run) error {
				r.window.Close()
				run.Message(run.Text("DELETED", r.noOfDeleted))
				return nil
			},
		}},
	}
}

func (r *deleteInvoicedPurchOrders) onAfterGetRecord(purchHeader *gtables.PurchaseHeader, run *Run) error {
	r.window.Update(1, purchHeader.No)

	purchLine := gtables.NewPurchaseLine(run)
	purchLine.SetRange("document_type", gtables.PurchaseLineDocument_type(purchHeader.Document_type))
	purchLine.SetRange("document_no", purchHeader.No)
	purchLine.SetFilter("quantity_invoiced", "<>0")
	if purchLine.FindFirst() {
		purchLine.SetRange("quantity_invoiced")
		purchLine.SetFilter("outstanding_quantity", "<>0")
		if !purchLine.FindFirst() {
			purchLine.SetRange("outstanding_quantity")
			purchLine.SetFilter("qty_rcd_not_invoiced", "<>0")
			if !purchLine.FindFirst() {
				purchLine.SetRange("qty_rcd_not_invoiced")
				if r.documentType == gtables.PurchaseHeader_Document_type.BlanketOrder {
					purchLine2 := gtables.NewPurchaseLine(run)
					purchLine2.SetRange("blanket_order_no", purchHeader.No)
					if purchLine2.FindFirst() {
						return nil
					}
				}
				purchLine.DeleteAll()

				purchCommentLine := gtables.NewPurchCommentLine(run)
				purchCommentLine.SetRange("document_type", gtables.PurchCommentLineDocument_type(purchHeader.Document_type))
				purchCommentLine.SetRange("no", purchHeader.No)
				purchCommentLine.DeleteAll()

				err := purchHeader.DeleteOrError(false)
				if err != nil {
					return err
				}
				r.noOfDeleted++
			}
		}
	}
	return nil
}
