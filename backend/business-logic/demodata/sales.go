package demodata

import (
	"fmt"
	"github.com/hansjlachmann/openerp/backend/business-logic/blanketorder"
	"math/rand"
	"time"

	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// SalesPoster posts a sales document (codeunit 80 Sales-Post)
type SalesPoster interface {
	Run(salesHeader *gtables.SalesHeader) error
}

// postDemoSales enters the demo sales documents and posts some of them through the sales
// posting (codeunit 80): three months of orders shipped and invoiced (some shipped first and
// invoiced a week later), an invoice and a credit memo applied to an invoice; open quotes, open
// orders (one shipped in part) and an open invoice. Sold from the main warehouse to the domestic
// customers, items and resources at their prices.
func postDemoSales(src ftables.Source, open func(src ftables.Source) SalesPoster, today time.Time, size Size, counts *Counts, p *progress) error {
	rng := rand.New(rand.NewSource(seed + 36))
	salesPost := open(src)
	customers, items, resources, err := demoSalesMasters(src)
	if err != nil {
		return err
	}
	if len(customers) == 0 || len(items) == 0 {
		return nil
	}
	perMonth := map[Size]int{Small: 4, Large: 10, Heavy: 20}[size]
	if perMonth == 0 {
		perMonth = 4
	}
	// the postings and the open documents
	p.setTotal(3*perMonth + perMonth + 10)

	// a document for a customer on a date with 1-3 lines (an item, sometimes a resource)
	newDocument := func(documentType gtables.SalesHeaderDocument_type, date time.Time) (*gtables.SalesHeader, error) {
		salesHeader := gtables.NewSalesHeader(src)
		salesHeader.Document_type = documentType
		err := salesHeader.InsertOrError(true)
		if err != nil {
			return nil, err
		}
		day := types.NewDateFromTime(date)
		for _, step := range []struct {
			field string
			value interface{}
		}{
			{"sell_to_customer_no", customers[rng.Intn(len(customers))]},
			{"posting_date", day},
			{"order_date", day},
			{"document_date", day},
			{"location_code", types.Code("MAIN")},
		} {
			if step.field == "posting_date" && documentType == gtables.SalesHeader_Document_type.Quote {
				continue
			}
			err = salesHeader.Validate(step.field, step.value)
			if err != nil {
				return nil, fmt.Errorf("demo sales %s %s: %w", salesHeader.No, step.field, err)
			}
		}
		err = salesHeader.ModifyOrError(true)
		if err != nil {
			return nil, err
		}
		lines := 1 + rng.Intn(3)
		for i := 1; i <= lines; i++ {
			lineType := gtables.SalesLine_Type.Item
			no := items[rng.Intn(len(items))]
			qty := 1 + rng.Intn(10)
			if i == 3 && len(resources) > 0 {
				lineType = gtables.SalesLine_Type.Resource
				no = resources[rng.Intn(len(resources))]
				qty = 2 + rng.Intn(6)
			}
			salesLine := gtables.NewSalesLine(src)
			salesLine.Document_type = gtables.SalesLineDocument_type(documentType)
			salesLine.Document_no = salesHeader.No
			salesLine.Line_no = i * 10000
			for _, step := range []struct {
				field string
				value interface{}
			}{
				{"type", lineType},
				{"no", no},
				{"quantity", types.NewDecimalFromInt(int64(qty))},
			} {
				err = salesLine.Validate(step.field, step.value)
				if err != nil {
					return nil, fmt.Errorf("demo sales %s line %d %s: %w", salesHeader.No, i, step.field, err)
				}
			}
			err = salesLine.InsertOrError(true)
			if err != nil {
				return nil, err
			}
		}
		return salesHeader, nil
	}
	post := func(salesHeader *gtables.SalesHeader, ship, invoice bool, date time.Time) error {
		err := salesHeader.Validate("posting_date", types.NewDateFromTime(date))
		if err != nil {
			return err
		}
		salesHeader.Ship = ship
		salesHeader.Invoice = invoice
		err = salesPost.Run(salesHeader)
		if err != nil {
			return fmt.Errorf("demo sales posting %s: %w", salesHeader.No, err)
		}
		p.step()
		if ship && salesHeader.Document_type != gtables.SalesHeader_Document_type.CreditMemo {
			counts.PostedSalesShipments++
		}
		if invoice {
			if salesHeader.Document_type == gtables.SalesHeader_Document_type.CreditMemo {
				counts.PostedSalesCrMemos++
			} else {
				counts.PostedSalesInvoices++
			}
		}
		return nil
	}

	// three months of orders, shipped and invoiced
	start := today.AddDate(0, -3, 0)
	for month := 0; month < 3; month++ {
		for i := 0; i < perMonth; i++ {
			date := start.AddDate(0, month, 1+i*(25/perMonth))
			order, err := newDocument(gtables.SalesHeader_Document_type.Order, date)
			if err != nil {
				return err
			}
			if i%3 == 2 {
				// shipped first, invoiced a week later
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
	// an invoice, and a credit memo applied to it
	date := today.AddDate(0, 0, -20)
	invoice, err := newDocument(gtables.SalesHeader_Document_type.Invoice, date)
	if err != nil {
		return err
	}
	invoiceCustomer := invoice.Sell_to_customer_no
	err = post(invoice, true, true, date)
	if err != nil {
		return err
	}
	salesInvHeader := gtables.NewSalesInvoiceHeader(src)
	salesInvHeader.SetRange("pre_assigned_no", invoice.No)
	if salesInvHeader.FindFirst() {
		creditMemo := gtables.NewSalesHeader(src)
		creditMemo.Document_type = gtables.SalesHeader_Document_type.CreditMemo
		err = creditMemo.InsertOrError(true)
		if err != nil {
			return err
		}
		day := types.NewDateFromTime(today.AddDate(0, 0, -10))
		for _, step := range []struct {
			field string
			value interface{}
		}{
			{"sell_to_customer_no", invoiceCustomer},
			{"posting_date", day},
			{"document_date", day},
			{"location_code", types.Code("MAIN")},
			{"applies_to_doc_type", gtables.SalesHeader_Applies_to_doc_type.Invoice},
			{"applies_to_doc_no", salesInvHeader.No},
		} {
			err = creditMemo.Validate(step.field, step.value)
			if err != nil {
				return fmt.Errorf("demo credit memo %s: %w", step.field, err)
			}
		}
		err = creditMemo.ModifyOrError(true)
		if err != nil {
			return err
		}
		salesInvLine := gtables.NewSalesInvoiceLine(src)
		salesInvLine.SetRange("document_no", salesInvHeader.No)
		salesInvLine.SetRange("type", gtables.SalesInvoiceLine_Type.Item)
		if salesInvLine.FindFirst() {
			salesLine := gtables.NewSalesLine(src)
			salesLine.Document_type = gtables.SalesLine_Document_type.CreditMemo
			salesLine.Document_no = creditMemo.No
			salesLine.Line_no = 10000
			for _, step := range []struct {
				field string
				value interface{}
			}{
				{"type", gtables.SalesLine_Type.Item},
				{"no", salesInvLine.No},
				{"quantity", types.NewDecimalFromInt(1)},
				{"unit_price", salesInvLine.Unit_price},
			} {
				err = salesLine.Validate(step.field, step.value)
				if err != nil {
					return fmt.Errorf("demo credit memo line %s: %w", step.field, err)
				}
			}
			err = salesLine.InsertOrError(true)
			if err != nil {
				return err
			}
			err = post(creditMemo, false, true, today.AddDate(0, 0, -10))
			if err != nil {
				return err
			}
		}
	}

	// open documents: quotes, orders (one shipped in part), an invoice
	for i := 0; i < 3; i++ {
		_, err = newDocument(gtables.SalesHeader_Document_type.Quote, today.AddDate(0, 0, -i))
		if err != nil {
			return err
		}
		counts.SalesQuotes++
		p.step()
	}
	for i := 0; i < 3; i++ {
		order, err := newDocument(gtables.SalesHeader_Document_type.Order, today.AddDate(0, 0, -i))
		if err != nil {
			return err
		}
		counts.SalesOrders++
		p.step()
		if i == 0 {
			salesLine := gtables.NewSalesLine(src)
			err = salesLine.GetOrError(map[string]interface{}{"document_type": int(order.Document_type), "document_no": order.No, "line_no": 10000})
			if err != nil {
				return err
			}
			if salesLine.Quantity.GreaterThan(types.NewDecimalFromInt(1)) {
				err = salesLine.Validate("qty_to_ship", types.NewDecimalFromInt(1))
				if err != nil {
					return err
				}
				err = salesLine.ModifyOrError(true)
				if err != nil {
					return err
				}
			}
			err = post(order, true, false, today)
			if err != nil {
				return err
			}
		}
	}
	_, err = newDocument(gtables.SalesHeader_Document_type.Invoice, today)
	if err != nil {
		return err
	}
	counts.SalesInvoices++

	// a blanket order called off in part: an order of half of each line (Make Order), shipped
	// and invoiced — the blanket lines show it
	blanket, err := newDocument(gtables.SalesHeader_Document_type.BlanketOrder, today.AddDate(0, -1, 0))
	if err != nil {
		return err
	}
	counts.SalesBlanketOrders++
	salesLine := gtables.NewSalesLine(src)
	salesLine.SetRange("document_type", gtables.SalesLine_Document_type.BlanketOrder)
	salesLine.SetRange("document_no", blanket.No)
	var blanketLines []*gtables.SalesLine
	for found := salesLine.FindSetBuffered(); found; found = salesLine.Next() {
		line := gtables.NewSalesLine(src)
		line.Copy(salesLine)
		blanketLines = append(blanketLines, line)
	}
	for _, line := range blanketLines {
		half := types.NewDecimalFromInt(int64(line.Quantity.Float64())/2 + 1)
		err = line.Validate("qty_to_ship", half)
		if err == nil {
			err = line.ModifyOrError(true)
		}
		if err != nil {
			return err
		}
	}
	order, err := blanketorder.SalesToOrder(blanket)
	if err != nil {
		return err
	}
	err = post(order, true, true, today.AddDate(0, 0, -7))
	if err != nil {
		return err
	}
	return postDemoForeignTrade(src, today, post)
}

// postDemoForeignTrade: an order shipped and invoiced to a customer in Denmark (an EU country
// with an Intrastat Code), with the foreign trade statistics fields — the Intrastat journal's
// demo shipment
func postDemoForeignTrade(src ftables.Source, today time.Time, post func(*gtables.SalesHeader, bool, bool, time.Time) error) error {
	salesHeader := gtables.NewSalesHeader(src)
	salesHeader.Document_type = gtables.SalesHeader_Document_type.Order
	err := salesHeader.InsertOrError(true)
	if err != nil {
		return err
	}
	day := types.NewDateFromTime(today)
	for _, step := range []struct {
		field string
		value interface{}
	}{
		{"sell_to_customer_no", types.Code("C00080")},
		{"order_date", day},
		{"posting_date", day},
		{"location_code", types.Code("MAIN")},
		{"transaction_type", types.Code("11")},
		{"transport_method", types.Code("3")},
		{"exit_point", types.Code("SVINESUND")},
		{"area", types.Code("03")},
		{"transaction_specification", types.Code("10")},
	} {
		err = salesHeader.Validate(step.field, step.value)
		if err != nil {
			return fmt.Errorf("demo foreign trade %s: %w", step.field, err)
		}
	}
	err = salesHeader.ModifyOrError(true)
	if err != nil {
		return err
	}
	for i, l := range []struct {
		item string
		qty  int64
	}{{"1300", 4}, {"1310", 2}} {
		salesLine := gtables.NewSalesLine(src)
		salesLine.Document_type = gtables.SalesLine_Document_type.Order
		salesLine.Document_no = salesHeader.No
		salesLine.Line_no = (i + 1) * 10000
		err = salesLine.Validate("type", gtables.SalesLine_Type.Item)
		if err == nil {
			err = salesLine.Validate("no", types.Code(l.item))
		}
		if err == nil {
			err = salesLine.Validate("quantity", types.NewDecimalFromInt(l.qty))
		}
		if err == nil {
			err = salesLine.InsertOrError(true)
		}
		if err != nil {
			return fmt.Errorf("demo foreign trade line %d: %w", i+1, err)
		}
	}
	return post(salesHeader, true, true, today)
}

// demoSalesMasters: the domestic customers, the items on inventory at the main warehouse and the
// resources the demo sales sell
func demoSalesMasters(src ftables.Source) ([]types.Code, []types.Code, []types.Code, error) {
	var customers, items, resources []types.Code
	cust := gtables.NewCustomer(src)
	cust.SetRange("customer_posting_group", types.Code("DOMESTIC"))
	for found := cust.FindSetBuffered(); found; found = cust.Next() {
		if !cust.Blocked {
			customers = append(customers, cust.No)
		}
	}
	item := gtables.NewItem(src)
	for found := item.FindSetBuffered(); found; found = item.Next() {
		if !item.Blocked && item.Inventory_posting_group != "" && item.Gen_prod_posting_group != "" {
			items = append(items, item.No)
		}
	}
	res := gtables.NewResource(src)
	for found := res.FindSetBuffered(); found; found = res.Next() {
		if !res.Blocked && res.Gen_prod_posting_group != "" {
			resources = append(resources, res.No)
		}
	}
	return customers, items, resources, nil
}
