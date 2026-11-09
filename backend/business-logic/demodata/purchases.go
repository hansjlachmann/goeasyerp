package demodata

import (
	"fmt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/blanketorder"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchgetdropshpt"
	"math/rand"
	"sort"
	"time"

	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// PurchPoster posts a purchase document (codeunit 90 Purch.-Post)
type PurchPoster interface {
	Run(purchHeader *gtables.PurchaseHeader) error
}

// postDemoPurchases enters the demo purchase documents and posts some of them through the
// purchase posting (codeunit 90): three months of orders received and invoiced (some received
// first and invoiced a week later), an invoice and a credit memo applied to an invoice; open
// quotes, open orders (one received in part) and an open invoice. Bought by the main warehouse
// from the domestic vendors, each vendor's items at its costs (Item Vendor / Last Direct Cost).
// Its own random generator: the sales documents stay the same.
func postDemoPurchases(src ftables.Source, open func(src ftables.Source) PurchPoster, today time.Time, size Size, counts *Counts, p *progress) error {
	rng := rand.New(rand.NewSource(seed + 37))
	purchPost := open(src)
	receiptOnInvoice := gtables.GetPurchasesPayablesSetup(src).Receipt_on_invoice
	vendors, vendorItems := demoPurchaseMasters(src)
	if len(vendors) == 0 {
		return nil
	}
	perMonth := map[Size]int{Small: 3, Large: 6, Heavy: 12}[size]
	if perMonth == 0 {
		perMonth = 3
	}
	p.setTotal(3*perMonth + perMonth + 8)

	// a document from a vendor on a date with 1-3 of its items
	newDocument := func(documentType gtables.PurchaseHeaderDocument_type, date time.Time, vendor types.Code) (*gtables.PurchaseHeader, error) {
		purchHeader := gtables.NewPurchaseHeader(src)
		purchHeader.Document_type = documentType
		err := purchHeader.InsertOrError(true)
		if err != nil {
			return nil, err
		}
		if vendor == "" {
			vendor = vendors[rng.Intn(len(vendors))]
		}
		day := types.NewDateFromTime(date)
		for _, step := range []struct {
			field string
			value interface{}
		}{
			{"buy_from_vendor_no", vendor},
			{"posting_date", day},
			{"order_date", day},
			{"document_date", day},
			{"expected_receipt_date", types.NewDateFromTime(date.AddDate(0, 0, 7))},
			{"location_code", types.Code("MAIN")},
		} {
			if step.field == "posting_date" && documentType == gtables.PurchaseHeader_Document_type.Quote {
				continue
			}
			err = purchHeader.Validate(step.field, step.value)
			if err != nil {
				return nil, fmt.Errorf("demo purchase %s %s: %w", purchHeader.No, step.field, err)
			}
		}
		err = purchHeader.ModifyOrError(true)
		if err != nil {
			return nil, err
		}
		items := vendorItems[purchHeader.Buy_from_vendor_no]
		lines := 1 + rng.Intn(3)
		for i := 1; i <= lines; i++ {
			purchLine := gtables.NewPurchaseLine(src)
			purchLine.Document_type = gtables.PurchaseLineDocument_type(documentType)
			purchLine.Document_no = purchHeader.No
			purchLine.Line_no = i * 10000
			for _, step := range []struct {
				field string
				value interface{}
			}{
				{"type", gtables.PurchaseLine_Type.Item},
				{"no", items[rng.Intn(len(items))]},
				{"quantity", types.NewDecimalFromInt(int64(5 + rng.Intn(20)))},
			} {
				err = purchLine.Validate(step.field, step.value)
				if err != nil {
					return nil, fmt.Errorf("demo purchase %s line %d %s: %w", purchHeader.No, i, step.field, err)
				}
			}
			err = purchLine.InsertOrError(true)
			if err != nil {
				return nil, err
			}
		}
		return purchHeader, nil
	}
	post := func(purchHeader *gtables.PurchaseHeader, receive, invoice bool, date time.Time) error {
		err := purchHeader.Validate("posting_date", types.NewDateFromTime(date))
		if err != nil {
			return err
		}
		// the vendor's document number (mandatory in the demo setup)
		if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
			purchHeader.Vendor_cr_memo_no = types.Code("VCR-" + purchHeader.No.String())
		} else if invoice {
			purchHeader.Vendor_invoice_no = types.Code(fmt.Sprintf("VI-%s-%s", purchHeader.No, date.Format("0102")))
		}
		purchHeader.Receive = receive
		purchHeader.Invoice = invoice
		err = purchPost.Run(purchHeader)
		if err != nil {
			return fmt.Errorf("demo purchase posting %s: %w", purchHeader.No, err)
		}
		p.step()
		// an order's receipt; an invoice writes one with the setup's Receipt on Invoice
		if receive && (purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Order ||
			purchHeader.Document_type == gtables.PurchaseHeader_Document_type.Invoice && receiptOnInvoice) {
			counts.PostedPurchReceipts++
		}
		if invoice {
			if purchHeader.Document_type == gtables.PurchaseHeader_Document_type.CreditMemo {
				counts.PostedPurchCrMemos++
			} else {
				counts.PostedPurchInvoices++
			}
		}
		return nil
	}

	// three months of orders, received and invoiced
	start := today.AddDate(0, -3, 0)
	for month := 0; month < 3; month++ {
		for i := 0; i < perMonth; i++ {
			date := start.AddDate(0, month, 2+i*(25/perMonth))
			order, err := newDocument(gtables.PurchaseHeader_Document_type.Order, date, "")
			if err != nil {
				return err
			}
			if i%3 == 1 {
				// received first, invoiced a week later
				err = post(order, true, false, date)
				if err != nil {
					return err
				}
				err = post(order, false, true, date.AddDate(0, 0, 7))
			} else {
				err = post(order, true, true, date)
			}
			if err != nil {
				return err
			}
		}
	}
	// an invoice, and a credit memo applied to it (one item sent back)
	date := today.AddDate(0, 0, -18)
	invoice, err := newDocument(gtables.PurchaseHeader_Document_type.Invoice, date, "")
	if err != nil {
		return err
	}
	err = post(invoice, true, true, date)
	if err != nil {
		return err
	}
	purchInvHeader := gtables.NewPurchInvHeader(src)
	purchInvHeader.SetRange("pre_assigned_no", invoice.No)
	purchInvLine := gtables.NewPurchInvLine(src)
	if purchInvHeader.FindFirst() {
		purchInvLine.SetRange("document_no", purchInvHeader.No)
		purchInvLine.SetRange("type", gtables.PurchInvLine_Type.Item)
	}
	if purchInvHeader.No != "" && purchInvLine.FindFirst() {
		day := today.AddDate(0, 0, -8)
		creditMemo := gtables.NewPurchaseHeader(src)
		creditMemo.Document_type = gtables.PurchaseHeader_Document_type.CreditMemo
		err = creditMemo.InsertOrError(true)
		if err != nil {
			return err
		}
		for _, step := range []struct {
			field string
			value interface{}
		}{
			{"buy_from_vendor_no", invoice.Buy_from_vendor_no},
			{"posting_date", types.NewDateFromTime(day)},
			{"document_date", types.NewDateFromTime(day)},
			{"location_code", types.Code("MAIN")},
			{"applies_to_doc_type", gtables.PurchaseHeader_Applies_to_doc_type.Invoice},
			{"applies_to_doc_no", purchInvHeader.No},
		} {
			err = creditMemo.Validate(step.field, step.value)
			if err != nil {
				return fmt.Errorf("demo purchase credit memo %s: %w", step.field, err)
			}
		}
		err = creditMemo.ModifyOrError(true)
		if err != nil {
			return err
		}
		purchLine := gtables.NewPurchaseLine(src)
		purchLine.Document_type = gtables.PurchaseLine_Document_type.CreditMemo
		purchLine.Document_no = creditMemo.No
		purchLine.Line_no = 10000
		for _, step := range []struct {
			field string
			value interface{}
		}{
			{"type", gtables.PurchaseLine_Type.Item},
			{"no", purchInvLine.No},
			{"quantity", types.NewDecimalFromInt(1)},
			{"direct_unit_cost", purchInvLine.Direct_unit_cost},
		} {
			err = purchLine.Validate(step.field, step.value)
			if err != nil {
				return fmt.Errorf("demo purchase credit memo line %s: %w", step.field, err)
			}
		}
		err = purchLine.InsertOrError(true)
		if err != nil {
			return err
		}
		err = post(creditMemo, false, true, day)
		if err != nil {
			return err
		}
	}

	// open documents: quotes, orders (one received in part), an invoice, a credit memo
	for i := 0; i < 2; i++ {
		_, err = newDocument(gtables.PurchaseHeader_Document_type.Quote, today.AddDate(0, 0, -i), "")
		if err != nil {
			return err
		}
		counts.PurchaseQuotes++
		p.step()
	}
	for i := 0; i < 3; i++ {
		order, err := newDocument(gtables.PurchaseHeader_Document_type.Order, today.AddDate(0, 0, -i), "")
		if err != nil {
			return err
		}
		counts.PurchaseOrders++
		p.step()
		if i == 0 {
			purchLine := gtables.NewPurchaseLine(src)
			err = purchLine.GetOrError(map[string]interface{}{"document_type": int(order.Document_type), "document_no": order.No, "line_no": 10000})
			if err != nil {
				return err
			}
			err = purchLine.Validate("qty_to_receive", types.NewDecimalFromInt(2))
			if err != nil {
				return err
			}
			err = purchLine.ModifyOrError(true)
			if err != nil {
				return err
			}
			err = post(order, true, false, today)
			if err != nil {
				return err
			}
		}
	}
	_, err = newDocument(gtables.PurchaseHeader_Document_type.Invoice, today, "")
	if err != nil {
		return err
	}
	counts.PurchaseInvoices++
	_, err = newDocument(gtables.PurchaseHeader_Document_type.CreditMemo, today, "")
	if err != nil {
		return err
	}
	counts.PurchaseCrMemos++

	// a blanket order called off in part: an order of half of each line (Make Order), received
	// and invoiced — the blanket lines show it
	blanket, err := newDocument(gtables.PurchaseHeader_Document_type.BlanketOrder, today.AddDate(0, -1, 0), "")
	if err != nil {
		return err
	}
	counts.PurchBlanketOrders++
	purchLine := gtables.NewPurchaseLine(src)
	purchLine.SetRange("document_type", gtables.PurchaseLine_Document_type.BlanketOrder)
	purchLine.SetRange("document_no", blanket.No)
	var blanketLines []*gtables.PurchaseLine
	for found := purchLine.FindSetBuffered(); found; found = purchLine.Next() {
		line := gtables.NewPurchaseLine(src)
		line.Copy(purchLine)
		blanketLines = append(blanketLines, line)
	}
	for _, line := range blanketLines {
		half := types.NewDecimalFromInt(int64(line.Quantity.Float64())/2 + 1)
		err = line.Validate("qty_to_receive", half)
		if err == nil {
			err = line.ModifyOrError(true)
		}
		if err != nil {
			return err
		}
	}
	order, err := blanketorder.PurchToOrder(blanket)
	if err != nil {
		return err
	}
	err = post(order, true, true, today.AddDate(0, 0, -7))
	if err != nil {
		return err
	}
	err = postDemoDropShipments(src, today, post, vendors, vendorItems, counts)
	if err != nil {
		return err
	}
	return postDemoForeignPurchase(src, today, post)
}

// postDemoDropShipments: two sales orders of a domestic customer with a drop shipment line of a
// vendor's item, each with a purchase order to the customer made by Get Sales Order (codeunit
// 76) — the first open, the second received (which ships the sales order)
func postDemoDropShipments(src ftables.Source, today time.Time, post func(*gtables.PurchaseHeader, bool, bool, time.Time) error,
	vendors []types.Code, vendorItems map[types.Code][]types.Code, counts *Counts) error {
	var vendor, itemNo types.Code
	for _, v := range vendors {
		if len(vendorItems[v]) > 0 {
			vendor = v
			itemNo = vendorItems[v][0]
			break
		}
	}
	cust := gtables.NewCustomer(src)
	cust.SetRange("currency_code", "")
	cust.SetRange("blocked", false)
	if vendor == "" || !cust.FindFirst() {
		return nil
	}
	day := types.NewDateFromTime(today)
	for i := 0; i < 2; i++ {
		salesHeader := gtables.NewSalesHeader(src)
		salesHeader.Document_type = gtables.SalesHeader_Document_type.Order
		err := salesHeader.InsertOrError(true)
		if err == nil {
			err = salesHeader.Validate("sell_to_customer_no", cust.No)
		}
		if err == nil {
			err = salesHeader.Validate("posting_date", day)
		}
		if err == nil {
			err = salesHeader.ModifyOrError(true)
		}
		if err != nil {
			return err
		}
		salesLine := gtables.NewSalesLine(src)
		salesLine.Document_type = gtables.SalesLine_Document_type.Order
		salesLine.Document_no = salesHeader.No
		salesLine.Line_no = 10000
		err = salesLine.Validate("type", gtables.SalesLine_Type.Item)
		if err == nil {
			err = salesLine.Validate("no", itemNo)
		}
		if err == nil {
			err = salesLine.Validate("quantity", types.NewDecimalFromInt(int64(2+i)))
		}
		if err == nil {
			err = salesLine.Validate("drop_shipment", true)
		}
		if err != nil {
			return err
		}
		salesLine.Shipment_date = day
		err = salesLine.InsertOrError(true)
		if err != nil {
			return err
		}
		counts.SalesOrders++

		purchHeader := gtables.NewPurchaseHeader(src)
		purchHeader.Document_type = gtables.PurchaseHeader_Document_type.Order
		err = purchHeader.InsertOrError(true)
		if err == nil {
			err = purchHeader.Validate("buy_from_vendor_no", vendor)
		}
		if err == nil {
			err = purchHeader.Validate("posting_date", day)
		}
		if err == nil {
			err = purchHeader.Validate("sell_to_customer_no", cust.No)
		}
		if err == nil && salesHeader.Ship_to_code != "" {
			err = purchHeader.Validate("ship_to_code", salesHeader.Ship_to_code)
		}
		if err == nil {
			err = purchHeader.ModifyOrError(true)
		}
		if err == nil {
			err = purchgetdropshpt.Code(purchHeader, salesHeader.No)
		}
		if err != nil {
			return err
		}
		counts.PurchaseOrders++
		if i == 1 {
			err = post(purchHeader, true, false, today)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// postDemoForeignPurchase: an order received and invoiced from a vendor in Denmark (an EU
// country with an Intrastat Code), with the foreign trade statistics fields — the Intrastat
// journal's demo receipt
func postDemoForeignPurchase(src ftables.Source, today time.Time, post func(*gtables.PurchaseHeader, bool, bool, time.Time) error) error {
	purchHeader := gtables.NewPurchaseHeader(src)
	purchHeader.Document_type = gtables.PurchaseHeader_Document_type.Order
	err := purchHeader.InsertOrError(true)
	if err != nil {
		return err
	}
	day := types.NewDateFromTime(today)
	for _, step := range []struct {
		field string
		value interface{}
	}{
		{"buy_from_vendor_no", types.Code("V00060")},
		{"order_date", day},
		{"posting_date", day},
		{"location_code", types.Code("MAIN")},
		{"transaction_type", types.Code("11")},
		{"transport_method", types.Code("3")},
		{"entry_point", types.Code("SVINESUND")},
		{"area", types.Code("03")},
		{"transaction_specification", types.Code("10")},
	} {
		err = purchHeader.Validate(step.field, step.value)
		if err != nil {
			return fmt.Errorf("demo foreign purchase %s: %w", step.field, err)
		}
	}
	err = purchHeader.ModifyOrError(true)
	if err != nil {
		return err
	}
	for i, l := range []struct {
		item string
		qty  int64
	}{{"1000", 40}, {"1020", 10}} {
		purchLine := gtables.NewPurchaseLine(src)
		purchLine.Document_type = gtables.PurchaseLine_Document_type.Order
		purchLine.Document_no = purchHeader.No
		purchLine.Line_no = (i + 1) * 10000
		err = purchLine.Validate("type", gtables.PurchaseLine_Type.Item)
		if err == nil {
			err = purchLine.Validate("no", types.Code(l.item))
		}
		if err == nil {
			err = purchLine.Validate("quantity", types.NewDecimalFromInt(l.qty))
		}
		if err == nil {
			err = purchLine.InsertOrError(true)
		}
		if err != nil {
			return fmt.Errorf("demo foreign purchase line %d: %w", i+1, err)
		}
	}
	return post(purchHeader, true, true, today)
}

// demoPurchaseMasters: the domestic vendors that supply items (Vendor No. on the item) and their
// items
func demoPurchaseMasters(src ftables.Source) ([]types.Code, map[types.Code][]types.Code) {
	domestic := map[types.Code]bool{}
	vend := gtables.NewVendor(src)
	vend.SetRange("vendor_posting_group", types.Code("DOMESTIC"))
	for found := vend.FindSetBuffered(); found; found = vend.Next() {
		if !vend.Blocked {
			domestic[vend.No] = true
		}
	}
	vendorItems := map[types.Code][]types.Code{}
	item := gtables.NewItem(src)
	for found := item.FindSetBuffered(); found; found = item.Next() {
		if !item.Blocked && domestic[item.Vendor_no] && item.Inventory_posting_group != "" && item.Gen_prod_posting_group != "" {
			vendorItems[item.Vendor_no] = append(vendorItems[item.Vendor_no], item.No)
		}
	}
	var vendors []types.Code
	for vendorNo := range vendorItems {
		vendors = append(vendors, vendorNo)
	}
	sort.Slice(vendors, func(i, j int) bool { return vendors[i] < vendors[j] })
	return vendors, vendorItems
}
