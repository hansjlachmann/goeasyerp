package tables

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/reservmgt"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// Blanket orders on the sales and purchase lines (fields 97/98 Blanket Order No./Line No.): a line
// taken from a blanket order line has its type, number and customer/vendor; a blanket order line
// with order lines keeps its type and number and is not deleted

// OnModify: a blanket order line whose order lines take from it keeps its type and number; a
// line with reservations still matches them
func (salesLineLogic) OnModify(rec *gtables.SalesLine) error {
	xRec := rec.XRec()
	if rec.Document_type == gtables.SalesLine_Document_type.BlanketOrder &&
		(rec.Type != xRec.Type || rec.No != xRec.No) {
		salesLine2 := gtables.NewSalesLine(rec)
		salesLine2.SetCurrentKey("document_type", "blanket_order_no", "blanket_order_line_no")
		salesLine2.SetRange("blanket_order_no", rec.Document_no)
		salesLine2.SetRange("blanket_order_line_no", rec.Line_no)
		for found := salesLine2.FindSetBuffered(); found; found = salesLine2.Next() {
			err := salesLine2.TestField("type", rec.Type)
			if err != nil {
				return err
			}
			err = salesLine2.TestField("no", rec.No)
			if err != nil {
				return err
			}
		}
	}
	// the line's reservations (classic ReservMgt.…VerifyChange)
	return reservmgt.New(rec).SalesLineVerifyChange(rec)
}

// checkSalesBlanketOrderLineDelete: a blanket order line that order lines take from stays
func checkSalesBlanketOrderLineDelete(rec *gtables.SalesLine) error {
	if rec.Document_type == gtables.SalesLine_Document_type.BlanketOrder {
		salesLine2 := gtables.NewSalesLine(rec)
		salesLine2.SetCurrentKey("document_type", "blanket_order_no", "blanket_order_line_no")
		salesLine2.SetRange("blanket_order_no", rec.Document_no)
		salesLine2.SetRange("blanket_order_line_no", rec.Line_no)
		if salesLine2.FindFirst() {
			return salesLine2.TestField("blanket_order_line_no", 0)
		}
	}
	return nil
}

// OnValidate_Blanket_order_no: no blanket order, no line of it
func (salesLineLogic) OnValidate_Blanket_order_no(rec *gtables.SalesLine) error {
	if rec.Blanket_order_no == "" {
		rec.Blanket_order_line_no = 0
		return nil
	}
	return rec.Validate("blanket_order_line_no", rec.Blanket_order_line_no)
}

// OnValidate_Blanket_order_line_no: the blanket order line has the line's type, number and
// customers
func (salesLineLogic) OnValidate_Blanket_order_line_no(rec *gtables.SalesLine) error {
	if rec.Blanket_order_line_no != 0 {
		salesLine2 := gtables.NewSalesLine(rec)
		err := salesLine2.GetOrError(map[string]interface{}{"document_type": int(gtables.SalesLine_Document_type.BlanketOrder),
			"document_no": rec.Blanket_order_no, "line_no": rec.Blanket_order_line_no})
		if err != nil {
			return err
		}
		err = salesLine2.TestField("type", rec.Type)
		if err == nil {
			err = salesLine2.TestField("no", rec.No)
		}
		if err == nil {
			err = salesLine2.TestField("bill_to_customer_no", rec.Bill_to_customer_no)
		}
		if err == nil {
			err = salesLine2.TestField("sell_to_customer_no", rec.Sell_to_customer_no)
		}
		return err
	}
	return nil
}

// OnModify: a blanket order line whose order lines take from it keeps its type and number; a
// line with reservations still matches them
func (purchaseLineLogic) OnModify(rec *gtables.PurchaseLine) error {
	xRec := rec.XRec()
	if rec.Document_type == gtables.PurchaseLine_Document_type.BlanketOrder &&
		(rec.Type != xRec.Type || rec.No != xRec.No) {
		purchLine2 := gtables.NewPurchaseLine(rec)
		purchLine2.SetCurrentKey("document_type", "blanket_order_no", "blanket_order_line_no")
		purchLine2.SetRange("blanket_order_no", rec.Document_no)
		purchLine2.SetRange("blanket_order_line_no", rec.Line_no)
		for found := purchLine2.FindSetBuffered(); found; found = purchLine2.Next() {
			err := purchLine2.TestField("type", rec.Type)
			if err != nil {
				return err
			}
			err = purchLine2.TestField("no", rec.No)
			if err != nil {
				return err
			}
		}
	}
	// the line's reservations (classic ReservMgt.…VerifyChange)
	return reservmgt.New(rec).PurchLineVerifyChange(rec)
}

// checkPurchBlanketOrderLineDelete: a blanket order line that order lines take from stays
func checkPurchBlanketOrderLineDelete(rec *gtables.PurchaseLine) error {
	if rec.Document_type == gtables.PurchaseLine_Document_type.BlanketOrder {
		purchLine2 := gtables.NewPurchaseLine(rec)
		purchLine2.SetCurrentKey("document_type", "blanket_order_no", "blanket_order_line_no")
		purchLine2.SetRange("blanket_order_no", rec.Document_no)
		purchLine2.SetRange("blanket_order_line_no", rec.Line_no)
		if purchLine2.FindFirst() {
			return purchLine2.TestField("blanket_order_line_no", 0)
		}
	}
	return nil
}

// OnValidate_Blanket_order_no: no blanket order, no line of it
func (purchaseLineLogic) OnValidate_Blanket_order_no(rec *gtables.PurchaseLine) error {
	if rec.Blanket_order_no == "" {
		rec.Blanket_order_line_no = 0
		return nil
	}
	return rec.Validate("blanket_order_line_no", rec.Blanket_order_line_no)
}

// OnValidate_Blanket_order_line_no: the blanket order line has the line's type, number and
// vendors
func (purchaseLineLogic) OnValidate_Blanket_order_line_no(rec *gtables.PurchaseLine) error {
	if rec.Blanket_order_line_no != 0 {
		purchLine2 := gtables.NewPurchaseLine(rec)
		err := purchLine2.GetOrError(map[string]interface{}{"document_type": int(gtables.PurchaseLine_Document_type.BlanketOrder),
			"document_no": rec.Blanket_order_no, "line_no": rec.Blanket_order_line_no})
		if err != nil {
			return err
		}
		err = purchLine2.TestField("type", rec.Type)
		if err == nil {
			err = purchLine2.TestField("no", rec.No)
		}
		if err == nil {
			err = purchLine2.TestField("pay_to_vendor_no", rec.Pay_to_vendor_no)
		}
		if err == nil {
			err = purchLine2.TestField("buy_from_vendor_no", rec.Buy_from_vendor_no)
		}
		return err
	}
	return nil
}
