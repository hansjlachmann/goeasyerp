package salespost_test

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salescalcdisc"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salespost"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func dec(s string) types.Decimal { return types.MustDecimal(s) }

func wantDec(t *testing.T, what string, got types.Decimal, want string) {
	t.Helper()
	if !got.Equal(dec(want)) {
		t.Errorf("%s = %s, want %s", what, got, want)
	}
}

func insert(t *testing.T, rec interface{ InsertOrError(bool) error }) {
	t.Helper()
	must(t, rec.InsertOrError(false))
}

// salesCompany: quote numbers SQ000001…; customer C1 (price group RETAIL, discount group LARGE,
// payment terms 14 days / 2 % in 8 days, tax area AREA with a 10 % state tax on group TAXABLE,
// invoice discount 3 % from 100 and a 5.00 service charge below); customer C2 without groups;
// item 1000 (10.00, cost 6.00, PCS and BOX of 10, RETAIL price 9.00, quantity discount 5 % from
// 10 PCS, LARGE/GRP1 customer/item discount 10 %); resource R1 (50.00 an hour, cost 30.00);
// G/L account 6810 (service charges)
func salesCompany(t *testing.T) ftables.Source {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	must(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	must(t, tables.CreateCompanyTables(db, "sales", database.DBTypeSQLite))
	// as the API and the codeunits write: in a transaction (loops read their rows at once)
	tx, err := ftables.BeginTransaction(db)
	must(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	src := ftables.DBSource(tx, "sales", database.DBTypeSQLite)

	noSeries := gtables.NewNoSeries(src)
	noSeries.Code = "S-QUO"
	noSeries.Default_nos = true
	insert(t, noSeries)
	noSeriesLine := gtables.NewNoSeriesLine(src)
	noSeriesLine.Series_code = "S-QUO"
	noSeriesLine.Line_no = 10000
	noSeriesLine.Starting_no = "SQ000001"
	noSeriesLine.Increment_by_no = 1
	noSeriesLine.Open = true
	insert(t, noSeriesLine)
	salesSetup := gtables.GetSalesReceivablesSetup(src)
	salesSetup.Quote_nos = "S-QUO"
	insert(t, salesSetup)

	paymentTerms := gtables.NewPaymentTerms(src)
	paymentTerms.Code = "14D"
	paymentTerms.Due_date_calculation = "14D"
	paymentTerms.Discount_date_calculation = "8D"
	paymentTerms.Discount_percent = dec("2")
	insert(t, paymentTerms)

	custPostingGroup := gtables.NewCustomerPostingGroup(src)
	custPostingGroup.Code = "DOM"
	custPostingGroup.Service_charge_acc = "6810"
	insert(t, custPostingGroup)
	glAcc := gtables.NewGLAccount(src)
	glAcc.No = "6810"
	glAcc.Name = "Service Charges"
	glAcc.Direct_posting = true
	glAcc.Gen_prod_posting_group = "SERVICES"
	insert(t, glAcc)

	jurisdiction := gtables.NewTaxJurisdiction(src)
	jurisdiction.Code = "STATE"
	insert(t, jurisdiction)
	taxAreaLine := gtables.NewTaxAreaLine(src)
	taxAreaLine.Tax_area = "AREA"
	taxAreaLine.Tax_jurisdiction_code = "STATE"
	insert(t, taxAreaLine)
	taxDetail := gtables.NewTaxDetail(src)
	taxDetail.Tax_jurisdiction_code = "STATE"
	taxDetail.Tax_group_code = "TAXABLE"
	taxDetail.Tax_below_maximum = dec("10")
	insert(t, taxDetail)
	taxPostingSetup := gtables.NewTaxPostingSetup(src)
	taxPostingSetup.Tax_calculation_type = gtables.TaxPostingSetup_Tax_calculation_type.SalesTax
	insert(t, taxPostingSetup)

	for _, c := range []struct {
		no, priceGroup, discGroup, terms, area string
	}{{"C1", "RETAIL", "LARGE", "14D", "AREA"}, {"C2", "", "", "", ""}} {
		cust := gtables.NewCustomer(src)
		cust.No = types.Code(c.no)
		cust.Name = types.Text("Customer " + c.no)
		cust.Address = "Main Street 1"
		cust.City = "Springfield"
		cust.Customer_posting_group = "DOM"
		cust.Gen_bus_posting_group = "DOM"
		cust.Payment_terms_code = types.Code(c.terms)
		cust.Price_group_code = types.Code(c.priceGroup)
		cust.Cust_item_disc_gr = types.Code(c.discGroup)
		cust.Invoice_disc_code = cust.No
		cust.Allow_quantity_disc = true
		cust.Tax_area_code = types.Code(c.area)
		cust.Tax_liable = c.area != ""
		insert(t, cust)
	}
	for _, d := range []struct{ minimum, pct, charge string }{{"0", "0", "5"}, {"100", "3", "0"}} {
		custInvDisc := gtables.NewCustInvoiceDisc(src)
		custInvDisc.Code = "C1"
		custInvDisc.Minimum_amount = dec(d.minimum)
		custInvDisc.Discount_percent = dec(d.pct)
		custInvDisc.Service_charge = dec(d.charge)
		insert(t, custInvDisc)
	}

	for _, u := range []string{"PCS", "BOX", "HOUR"} {
		unitOfMeasure := gtables.NewUnitOfMeasure(src)
		unitOfMeasure.Code = types.Code(u)
		unitOfMeasure.Description = types.Text(u + " unit")
		insert(t, unitOfMeasure)
	}
	item := gtables.NewItem(src)
	item.No = "1000"
	item.Description = "Office Chair"
	item.Base_unit_of_measure = "PCS"
	item.Sales_unit_of_measure = "PCS"
	item.Unit_price = dec("10")
	item.Unit_cost = dec("6")
	item.Inventory_posting_group = "RESALE"
	item.Gen_prod_posting_group = "RETAIL"
	item.Tax_group_code = "TAXABLE"
	item.Sales_qty_disc_code = "QD1"
	item.Item_cust_disc_gr = "GRP1"
	item.Allow_invoice_disc = true
	item.Units_per_parcel = dec("5")
	item.Gross_weight = dec("2")
	insert(t, item)
	for _, u := range []struct {
		code string
		qty  int64
	}{{"PCS", 1}, {"BOX", 10}} {
		itemUnitOfMeasure := gtables.NewItemUnitOfMeasure(src)
		itemUnitOfMeasure.Item_no = "1000"
		itemUnitOfMeasure.Code = types.Code(u.code)
		itemUnitOfMeasure.Qty_per_unit_of_measure = types.NewDecimalFromInt(u.qty)
		insert(t, itemUnitOfMeasure)
	}
	itemPrice := gtables.NewItemPrice(src)
	itemPrice.Item_no = "1000"
	itemPrice.Price_group_code = "RETAIL"
	itemPrice.Unit_price = dec("9")
	itemPrice.Allow_quantity_disc = true
	itemPrice.Allow_cust_item_disc = true
	itemPrice.Allow_invoice_disc = true
	insert(t, itemPrice)
	itemSalesQtyDisc := gtables.NewItemSalesQtyDisc(src)
	itemSalesQtyDisc.Code = "QD1"
	itemSalesQtyDisc.Minimum_quantity = dec("10")
	itemSalesQtyDisc.Discount_percent = dec("5")
	insert(t, itemSalesQtyDisc)
	custItemDisc := gtables.NewCustItemDiscount(src)
	custItemDisc.Customer_disc_group = "LARGE"
	custItemDisc.Item_discount_group = "GRP1"
	custItemDisc.Discount_percent = dec("10")
	insert(t, custItemDisc)

	res := gtables.NewResource(src)
	res.No = "R1"
	res.Name = "Consultant"
	res.Unit_of_measure_code = "HOUR"
	res.Unit_price = dec("50")
	res.Unit_cost = dec("30")
	res.Gen_prod_posting_group = "SERVICES"
	insert(t, res)
	return src
}

// newQuote inserts a quote for a customer (its number from the series)
func newQuote(t *testing.T, src ftables.Source, customer string) *gtables.SalesHeader {
	t.Helper()
	salesHeader := gtables.NewSalesHeader(src)
	salesHeader.Document_type = gtables.SalesHeader_Document_type.Quote
	must(t, salesHeader.InsertOrError(true))
	must(t, salesHeader.Validate("sell_to_customer_no", types.Code(customer)))
	must(t, salesHeader.ModifyOrError(true))
	return salesHeader
}

// addLine enters a line as the subpage does: type, no., (unit of measure,) quantity
func addLine(t *testing.T, salesHeader *gtables.SalesHeader, lineNo int, lineType gtables.SalesLineType, no, uom, qty string) *gtables.SalesLine {
	t.Helper()
	salesLine := gtables.NewSalesLine(salesHeader)
	salesLine.Document_type = gtables.SalesLineDocument_type(salesHeader.Document_type)
	salesLine.Document_no = salesHeader.No
	salesLine.Line_no = lineNo
	must(t, salesLine.Validate("type", lineType))
	must(t, salesLine.Validate("no", types.Code(no)))
	if uom != "" {
		must(t, salesLine.Validate("unit_of_measure_code", types.Code(uom)))
	}
	must(t, salesLine.Validate("quantity", dec(qty)))
	must(t, salesLine.InsertOrError(true))
	return salesLine
}

// A quote: its number, the customer's fields; item lines priced by price group, unit of measure,
// quantity and customer/item discounts with sales tax; a resource line; the invoice discount;
// the statistics; a new customer recreates the lines; deleting the quote deletes its lines
func TestSalesQuote(t *testing.T) {
	src := salesCompany(t)
	salesHeader := newQuote(t, src, "C1")
	if salesHeader.No != "SQ000001" || salesHeader.No_series != "S-QUO" {
		t.Fatalf("quote no. %s series %s", salesHeader.No, salesHeader.No_series)
	}
	if salesHeader.Bill_to_customer_no != "C1" || salesHeader.Bill_to_name != "Customer C1" || salesHeader.Sell_to_customer_name != "Customer C1" ||
		salesHeader.Ship_to_name != "Customer C1" || salesHeader.Price_group_code != "RETAIL" || salesHeader.Cust_item_disc_gr != "LARGE" ||
		salesHeader.Invoice_disc_code != "C1" || salesHeader.Tax_area_code != "AREA" || !salesHeader.Tax_liable || salesHeader.Customer_posting_group != "DOM" {
		t.Errorf("customer fields: %+v", salesHeader.ToMap())
	}
	if !salesHeader.Posting_date.IsZero() || salesHeader.Document_date != system.WorkDate() || salesHeader.Order_date != system.WorkDate() {
		t.Errorf("quote dates: posting %s document %s order %s", salesHeader.Posting_date, salesHeader.Document_date, salesHeader.Order_date)
	}
	dueDate, err := system.CalcDate("14D", system.WorkDate())
	must(t, err)
	if salesHeader.Due_date != dueDate {
		t.Errorf("due date %s, want %s", salesHeader.Due_date, dueDate)
	}
	wantDec(t, "payment discount %", salesHeader.Payment_discount_percent, "2")

	// 12 PCS at the RETAIL price 9.00: 5 % quantity + 10 % customer/item discount = 15 %
	line1 := addLine(t, salesHeader, 10000, gtables.SalesLine_Type.Item, "1000", "", "12")
	if line1.Description != "Office Chair" || line1.Unit_of_measure_code != "PCS" || line1.Unit_of_measure != "PCS unit" || line1.Sell_to_customer_no != "C1" {
		t.Errorf("item line: %+v", line1.ToMap())
	}
	wantDec(t, "unit price", line1.Unit_price, "9")
	wantDec(t, "quantity disc. %", line1.Quantity_disc_percent, "5")
	wantDec(t, "cust./item disc. %", line1.Cust_item_disc_percent, "10")
	wantDec(t, "line discount %", line1.Line_discount_percent, "15")
	wantDec(t, "line discount amount", line1.Line_discount_amount, "16.2")
	wantDec(t, "amount", line1.Amount, "91.8")
	wantDec(t, "amount incl. tax", line1.Amount_including_tax, "100.98")
	wantDec(t, "tax %", line1.Tax_percent, "10")
	wantDec(t, "outstanding amount", line1.Outstanding_amount, "100.98")
	wantDec(t, "unit cost (LCY)", line1.Unit_cost_lcy, "6")

	// a resource: its card's price and cost, no tax group
	line2 := addLine(t, salesHeader, 20000, gtables.SalesLine_Type.Resource, "R1", "", "2")
	if line2.Unit_of_measure_code != "HOUR" || line2.Description != "Consultant" {
		t.Errorf("resource line: %+v", line2.ToMap())
	}
	wantDec(t, "resource unit price", line2.Unit_price, "50")
	wantDec(t, "resource amount incl. tax", line2.Amount_including_tax, "100")
	wantDec(t, "resource unit cost", line2.Unit_cost_lcy, "30")

	// 1 BOX = 10 PCS: the price per box, the quantity discount on the base quantity
	line3 := addLine(t, salesHeader, 30000, gtables.SalesLine_Type.Item, "1000", "BOX", "1")
	wantDec(t, "box qty per unit", line3.Qty_per_unit_of_measure, "10")
	wantDec(t, "box quantity (base)", line3.Quantity_base, "10")
	wantDec(t, "box unit price", line3.Unit_price, "90")
	wantDec(t, "box line discount %", line3.Line_discount_percent, "15")
	wantDec(t, "box amount", line3.Amount, "76.5")
	wantDec(t, "box unit cost (LCY)", line3.Unit_cost_lcy, "60")
	wantDec(t, "box gross weight", line3.Gross_weight, "20")

	// a text line
	textLine := gtables.NewSalesLine(src)
	textLine.Document_type = gtables.SalesLine_Document_type.Quote
	textLine.Document_no = salesHeader.No
	textLine.Line_no = 40000
	textLine.Description = "Delivery within two weeks"
	insert(t, textLine)

	salesHeader.CalcFields("amount", "amount_including_tax")
	wantDec(t, "header amount", salesHeader.Amount, "268.3")
	wantDec(t, "header amount incl. tax", salesHeader.Amount_including_tax, "285.13")

	// invoice discount: 3 % from 100 on the lines allowing it
	must(t, salescalcdisc.CalculateInvoiceDiscount(salesHeader))
	salesLine := gtables.NewSalesLine(src)
	salesLine.SetRange("document_type", salesHeader.Document_type)
	salesLine.SetRange("document_no", salesHeader.No)
	must(t, salesLine.CalcSums("inv_discount_amount", "amount"))
	wantDec(t, "invoice discount", salesLine.Inv_discount_amount, "8.049")
	wantDec(t, "amount after invoice discount", salesLine.Amount, "260.251")
	if salesLine.Count() != 4 {
		t.Errorf("%d lines: no service charge above the minimum", salesLine.Count())
	}

	// statistics
	totals, err := salespost.SumSalesLines(salesHeader, salespost.General)
	must(t, err)
	wantDec(t, "total amount", totals.TotalSalesLine.Amount, "260.25")
	wantDec(t, "total inv. discount", totals.TotalSalesLine.Inv_discount_amount, "8.05")
	wantDec(t, "total quantity", totals.TotalSalesLine.Quantity, "15")
	wantDec(t, "total gross weight", totals.TotalSalesLine.Gross_weight, "44") // 12 × 2 + 1 × 20
	wantDec(t, "parcels", totals.TotalSalesLine.Units_per_parcel, "4")         // 12/5 → 3 and 1/5 → 1, rounded up per line
	wantDec(t, "cost (LCY)", totals.TotalSalesLineLCY.Unit_cost_lcy, "192")
	wantDec(t, "profit (LCY)", totals.ProfitLCY, "68.25")
	wantDec(t, "tax amount", totals.TaxAmount, "16.32") // 8.90 + 7.42: the tax of each line rounded

	// another customer recreates the lines with its prices
	must(t, salesHeader.Validate("sell_to_customer_no", types.Code("C2")))
	must(t, salesHeader.ModifyOrError(true))
	salesLine = gtables.NewSalesLine(src)
	salesLine.SetRange("document_type", salesHeader.Document_type)
	salesLine.SetRange("document_no", salesHeader.No)
	salesLine.SetRange("line_no", 10000)
	if !salesLine.FindFirst() {
		t.Fatal("line 10000 recreated")
	}
	if salesLine.Sell_to_customer_no != "C2" {
		t.Errorf("recreated line for %s", salesLine.Sell_to_customer_no)
	}
	wantDec(t, "C2 unit price", salesLine.Unit_price, "10")
	wantDec(t, "C2 line discount %", salesLine.Line_discount_percent, "5")
	wantDec(t, "C2 quantity", salesLine.Quantity, "12")
	wantDec(t, "C2 amount incl. tax (no tax area)", salesLine.Amount_including_tax, "114")

	// deleting the quote deletes its lines
	must(t, salesHeader.DeleteOrError(true))
	salesLine = gtables.NewSalesLine(src)
	if salesLine.Count() != 0 {
		t.Errorf("%d lines left", salesLine.Count())
	}
}

// Below the invoice discount's minimum the customer pays the service charge: a G/L line on the
// customer posting group's Service Charge Acc.; calculating again updates it
func TestSalesServiceCharge(t *testing.T) {
	src := salesCompany(t)
	salesHeader := newQuote(t, src, "C1")
	addLine(t, salesHeader, 10000, gtables.SalesLine_Type.Item, "1000", "", "2")
	must(t, salescalcdisc.CalculateInvoiceDiscount(salesHeader))
	must(t, salescalcdisc.CalculateInvoiceDiscount(salesHeader))
	salesLine := gtables.NewSalesLine(src)
	salesLine.SetRange("type", gtables.SalesLine_Type.GLAccount)
	if salesLine.Count() != 1 || !salesLine.FindFirst() {
		t.Fatalf("%d service charge lines", salesLine.Count())
	}
	if salesLine.No != "6810" || salesLine.Allow_invoice_disc || salesLine.Line_no != 20000 {
		t.Errorf("service charge line: %+v", salesLine.ToMap())
	}
	wantDec(t, "service charge", salesLine.Unit_price, "5")
	wantDec(t, "service charge amount", salesLine.Amount, "5")
}
