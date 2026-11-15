package tables

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Every auto-numbered table (auto_key) whose OnInsert takes a number series is known to
// NewRecordNoSeries — the new card's tip ("press Enter to get the next number") depends on it
func TestNewRecordNoSeriesKnowsEveryAutoKeyTable(t *testing.T) {
	// Extended Text Header numbers its Text No. per record and language, without a series
	withoutSeries := map[string]bool{"Extended_Text_Header": true}
	files, err := filepath.Glob(filepath.Join("..", "..", "definitions", "tables", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("table definitions: %v", err)
	}
	autoKey := regexp.MustCompile(`(?m)^\s+auto_key: true`)
	name := regexp.MustCompile(`(?m)^\s+name: "?([^"\s]+)"?`)
	db := newTestDB(t)
	checked := 0
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !autoKey.Match(content) {
			continue
		}
		tableName := name.FindStringSubmatch(string(content))[1]
		if withoutSeries[tableName] {
			continue
		}
		factory, ok := gtables.GetTableFactory(tableName)
		if !ok {
			t.Errorf("%s: no table factory", tableName)
			continue
		}
		rec := factory()
		rec.InitWithDBType(db, "c1", database.DBTypeSQLite)
		src, ok := rec.(ftables.Source)
		if !ok {
			t.Errorf("%s: not a record source", tableName)
			continue
		}
		if _, known := newRecordNoSeries(src); !known {
			t.Errorf("%s is auto_key but NewRecordNoSeries does not know its number series", tableName)
		}
		checked++
	}
	if checked < 10 {
		t.Errorf("checked %d auto_key tables, want at least 10", checked)
	}
}

// A new customer gets a number when Customer Nos. names a series with Default Nos.; without the
// series, or with a series for typed numbers only, it does not. A sales document takes its
// document type's series.
func TestNewRecordGetsNumber(t *testing.T) {
	db := newTestDB(t)
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	cust := gtables.NewCustomer(src)
	if NewRecordGetsNumber(cust) {
		t.Error("a customer gets a number without Customer Nos.")
	}

	salesSetup := gtables.GetSalesReceivablesSetup(src)
	salesSetup.Customer_nos = "CUST"
	salesSetup.Order_nos = "S-ORD"
	if !salesSetup.Insert(false) {
		t.Fatal("insert the setup")
	}
	if NewRecordGetsNumber(cust) {
		t.Error("a customer gets a number from a series that does not exist")
	}

	noSeries := gtables.NewNoSeries(src)
	noSeries.Code, noSeries.Default_nos = "CUST", true
	if !noSeries.Insert(false) {
		t.Fatal("insert the series")
	}
	if !NewRecordGetsNumber(cust) {
		t.Error("a customer does not get a number from Customer Nos. with Default Nos.")
	}
	noSeries.Default_nos = false
	noSeries.Manual_nos = true
	if !noSeries.Modify(false) {
		t.Fatal("modify the series")
	}
	if NewRecordGetsNumber(cust) {
		t.Error("a customer gets a number from a series without Default Nos.")
	}

	order := gtables.NewSalesHeader(src)
	order.Document_type = gtables.SalesHeader_Document_type.Order
	if got := NewRecordNoSeries(order); got != types.Code("S-ORD") {
		t.Errorf("a sales order's series = %q, want S-ORD", got)
	}
	invoice := gtables.NewSalesHeader(src)
	invoice.Document_type = gtables.SalesHeader_Document_type.Invoice
	if got := NewRecordNoSeries(invoice); got != "" {
		t.Errorf("a sales invoice's series = %q, want none (Invoice Nos. blank)", got)
	}
}
