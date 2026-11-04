package reports_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// quoteCompany: the company (with a logo), customer C1 with a ship-to name, item 1000 at 12.50
// and quote SQ1 with lines lines of 2 PCS (a text line after the first)
func quoteCompany(t *testing.T, lines int) (ftables.Source, *gtables.SalesHeader) {
	t.Helper()
	return documentCompany(t, lines, gtables.SalesHeader_Document_type.Quote, "SQ1")
}

// documentCompany: as quoteCompany, with a document of the type and number
func documentCompany(t *testing.T, lines int, documentType gtables.SalesHeaderDocument_type, no types.Code) (ftables.Source, *gtables.SalesHeader) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "quote", database.DBTypeSQLite))
	// as the API runs the report on SQLite: its reads are buffered in a transaction
	tx, err := ftables.BeginTransaction(db)
	mustOK(t, err)
	t.Cleanup(func() { _ = ftables.Rollback(tx) })
	src := ftables.DBSource(tx, "quote", database.DBTypeSQLite)

	var logo bytes.Buffer
	mustOK(t, png.Encode(&logo, image.NewRGBA(image.Rect(0, 0, 60, 20))))
	companyInfo := gtables.GetCompanyInformation(src)
	companyInfo.Name = "Nordkapp Kontor AS"
	companyInfo.Address = "Stortingsgata 12"
	companyInfo.Post_code = "0159"
	companyInfo.City = "Oslo"
	companyInfo.Phone_no = "+47 22 00 00 00"
	companyInfo.Picture = logo.Bytes()
	mustOK(t, companyInfo.InsertOrError(false))

	cust := gtables.NewCustomer(src)
	cust.No = "C1"
	cust.Name = "Fjord Møbler AS"
	cust.Address = "Bryggen 5"
	cust.City = "Bergen"
	cust.Post_code = "5003"
	cust.Customer_posting_group = "DOM"
	cust.Gen_bus_posting_group = "DOM"
	mustOK(t, cust.InsertOrError(false))
	unitOfMeasure := gtables.NewUnitOfMeasure(src)
	unitOfMeasure.Code = "PCS"
	mustOK(t, unitOfMeasure.InsertOrError(false))
	item := gtables.NewItem(src)
	item.No = "1000"
	item.Description = "Office Chair"
	item.Base_unit_of_measure = "PCS"
	item.Sales_unit_of_measure = "PCS"
	item.Unit_price = types.MustDecimal("12.5")
	item.Inventory_posting_group = "RESALE"
	item.Gen_prod_posting_group = "RETAIL"
	mustOK(t, item.InsertOrError(false))
	itemUnitOfMeasure := gtables.NewItemUnitOfMeasure(src)
	itemUnitOfMeasure.Item_no = "1000"
	itemUnitOfMeasure.Code = "PCS"
	itemUnitOfMeasure.Qty_per_unit_of_measure = types.NewDecimalFromInt(1)
	mustOK(t, itemUnitOfMeasure.InsertOrError(false))

	salesHeader := gtables.NewSalesHeader(src)
	salesHeader.Document_type = documentType
	salesHeader.No = no
	mustOK(t, salesHeader.InsertOrError(true))
	mustOK(t, salesHeader.Validate("sell_to_customer_no", types.Code("C1")))
	salesHeader.Ship_to_name = "Fjord Møbler Lager"
	salesHeader.Your_reference = "PO-77"
	mustOK(t, salesHeader.ModifyOrError(true))
	for i := 1; i <= lines; i++ {
		salesLine := gtables.NewSalesLine(salesHeader)
		salesLine.Document_type = gtables.SalesLineDocument_type(documentType)
		salesLine.Document_no = no
		salesLine.Line_no = i * 10000
		if i == 2 {
			mustOK(t, salesLine.Validate("description", types.Text("Delivered in October")))
		} else {
			mustOK(t, salesLine.Validate("type", gtables.SalesLine_Type.Item))
			mustOK(t, salesLine.Validate("no", types.Code("1000")))
			mustOK(t, salesLine.Validate("quantity", types.NewDecimalFromInt(2)))
		}
		err := salesLine.InsertOrError(true)
		if err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
	}
	return src, salesHeader
}

func printQuote(t *testing.T, src ftables.Source, output string, options map[string]string) reports.Result {
	t.Helper()
	result, err := reports.Execute(reports.SalesQuoteID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: output, Options: options})
	mustOK(t, err)
	if !bytes.HasPrefix(result.PDF, []byte("%PDF-")) {
		t.Fatalf("no PDF")
	}
	return result
}

func printedTexts(result reports.Result, page int) string {
	var texts []string
	for _, text := range result.Texts {
		if page == 0 || text.Page == page {
			texts = append(texts, text.Text)
		}
	}
	return strings.Join(texts, "|")
}

func noPrinted(t *testing.T, src ftables.Source) int {
	t.Helper()
	salesHeader := gtables.NewSalesHeader(src)
	mustOK(t, salesHeader.GetOrError(map[string]interface{}{"document_type": 0, "no": "SQ1"}))
	return salesHeader.No_printed
}

func TestSalesQuoteReport(t *testing.T) {
	src, _ := quoteCompany(t, 3)
	result := printQuote(t, src, reports.OutputPDF, nil)
	if result.Pages != 1 || result.FileName != "reports.204.texts.FILE_NAME.pdf" && !strings.HasSuffix(result.FileName, "SQ1.pdf") {
		t.Errorf("pages %d, file %q", result.Pages, result.FileName)
	}
	texts := printedTexts(result, 1)
	for _, want := range []string{
		"Nordkapp Kontor AS", "Stortingsgata 12", "0159 Oslo", "+47 22 00 00 00", // the company
		"Fjord Møbler AS", "Bryggen 5", "5003 Bergen", "Fjord Møbler Lager", // bill-to, ship-to
		"SQ1", "C1", "PO-77", // quote no., customer ID, P.O. number
		"1000", "PCS", "Office Chair", "2", "12.50", "25.00", // an item line
		"Delivered in October", // the text line
		"50.00",                // subtotal and total: 2 lines of 25.00
	} {
		if !strings.Contains("|"+texts+"|", "|"+want+"|") {
			t.Errorf("%q not printed: %s", want, texts)
		}
	}
	if noPrinted(t, src) != 1 {
		t.Errorf("No. Printed %d after a print, want 1", noPrinted(t, src))
	}
	// A preview does not count
	printQuote(t, src, reports.OutputPreview, nil)
	if noPrinted(t, src) != 1 {
		t.Errorf("No. Printed %d after a preview, want 1", noPrinted(t, src))
	}
}

func TestSalesQuoteReportPagesAndCopies(t *testing.T) {
	src, _ := quoteCompany(t, 70)
	result := printQuote(t, src, reports.OutputPrint, map[string]string{"no_of_copies": "1", "print_company_address": "false"})
	// 70 lines: more than a page per copy; the copy starts on page 1 again
	if result.Pages < 4 || result.Pages%2 != 0 {
		t.Fatalf("pages %d", result.Pages)
	}
	perCopy := result.Pages / 2
	all := printedTexts(result, 0)
	if strings.Contains(all, "Nordkapp Kontor AS") {
		t.Error("the company is printed without Print Company Address")
	}
	if strings.Contains(printedTexts(result, 1), "reports.204.texts.COPY") && !strings.Contains(printedTexts(result, 1), "COPY") {
		t.Error("the original says COPY")
	}
	copyTexts := printedTexts(result, perCopy+1)
	if !strings.Contains(copyTexts, "COPY") && !strings.Contains(copyTexts, "KOPI") {
		t.Errorf("the copy does not say so: %s", copyTexts)
	}
	for page := 1; page <= result.Pages; page++ {
		texts := printedTexts(result, page)
		if !strings.Contains(texts, "Fjord Møbler AS") {
			t.Errorf("page %d without the header", page)
		}
		lastOfCopy := page%perCopy == 0
		continued := strings.Contains(texts, "CONTINUED|") || strings.Contains(texts, "Continued|")
		if continued == lastOfCopy {
			t.Errorf("page %d: carried-over total %v", page, continued)
		}
	}
	// 69 item lines of 25.00 each
	if !strings.Contains(all, "1,725.00") {
		t.Errorf("total 1,725.00 not printed")
	}
	if noPrinted(t, src) != 1 {
		t.Errorf("No. Printed %d after a print with a copy, want 1 (once per print)", noPrinted(t, src))
	}
}

func TestPrintedLayoutsLoad(t *testing.T) {
	layouts, err := reports.Layouts()
	mustOK(t, err)
	if layouts[reports.SalesQuoteID] == nil {
		t.Fatal("no layout for report 204")
	}
	for id, layout := range layouts {
		err := reports.CheckLayout(layout)
		if err != nil {
			t.Error(err)
		}
		if !reports.Printed(id) {
			t.Errorf("report %d not printed", id)
		}
	}
	if reports.Printed(93) {
		t.Error("report 93 (processing only) is printed")
	}
	_ = fmt.Sprint()
}

// Report 205 prints orders as the quote report prints quotes: its title and the order's dates
func TestOrderConfirmationReport(t *testing.T) {
	src, salesHeader := documentCompany(t, 2, gtables.SalesHeader_Document_type.Order, "SO1")
	result, err := reports.Execute(reports.OrderConfirmationID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF})
	mustOK(t, err)
	texts := "|" + printedTexts(result, 1) + "|"
	for _, want := range []string{"ORDER CONFIRMATION", "Order No.", "SO1", "Shipment Date", "Fjord Møbler AS", "Office Chair", "25.00"} {
		if !strings.Contains(texts, "|"+want+"|") {
			t.Errorf("%q not printed: %s", want, texts)
		}
	}
	if result.FileName != "Order Confirmation SO1.pdf" {
		t.Errorf("file %q", result.FileName)
	}
	stored := gtables.NewSalesHeader(src)
	mustOK(t, stored.GetOrError(map[string]interface{}{"document_type": 1, "no": "SO1"}))
	if stored.No_printed != 1 {
		t.Errorf("No. Printed %d, want 1", stored.No_printed)
	}
	// Only orders: the quote report does not print it
	result, err = reports.Execute(reports.SalesQuoteID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF})
	mustOK(t, err)
	if strings.Contains(printedTexts(result, 0), "SO1") {
		t.Errorf("the quote report printed order %s", salesHeader.No)
	}
}
