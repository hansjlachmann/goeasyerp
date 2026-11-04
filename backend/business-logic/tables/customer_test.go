package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/noseries"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The classic Customer code: numbers from the Customer Nos. series, manual numbers only when
// the series allows them, Search Name, Post Code → City, Gen. Bus. → Tax Bus. Posting Group,
// Last Date Modified, and no delete while ledger entries exist
func TestCustomerTriggers(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.Customer{}, &gtables.CustomerLedgerEntry{}, &gtables.PostCode{},
		&gtables.GenBusinessPostingGroup{}, &gtables.TaxBusinessPostingGroup{})
	setupCustomerNos(t, db, "c1", false)
	newCust := func() *gtables.Customer {
		c := &gtables.Customer{}
		c.InitWithDBType(db, "c1", database.DBTypeSQLite)
		c.InitRecord()
		return c
	}

	// Numbers from the series; Invoice Disc. Code follows
	for _, want := range []string{"C00001", "C00002"} {
		c := newCust()
		c.Name = "Acme"
		if !c.Insert(true) || c.No != types.Code(want) || c.No_series != "CUST" || c.Invoice_disc_code != types.Code(want) {
			t.Fatalf("insert: no %q series %q disc %q, %v", c.No, c.No_series, c.Invoice_disc_code, c.TriggerError())
		}
	}
	// A typed number: refused while the series has no Manual Nos.
	c := newCust()
	if err := c.Validate("no", types.Code("X1")); err == nil || errCode(t, err) != apperrors.ErrNoSeriesManual {
		t.Errorf("manual number: %v", err)
	}

	// Search Name follows the name while it was blank or the old name
	c = newCust()
	_ = c.Validate("name", types.Text("Fjordkraft AS"))
	if c.Search_name != "FJORDKRAFT AS" {
		t.Errorf("search name %q", c.Search_name)
	}

	// Post Code gives the city; an unknown one is accepted as it is
	var pc gtables.PostCode
	pc.InitWithDBType(db, "c1", database.DBTypeSQLite)
	pc.Code, pc.City = "0159", "Oslo"
	if !pc.Insert(false) {
		t.Fatal("insert post code")
	}
	if err := c.Validate("post_code", types.Code("0159")); err != nil || c.City != "Oslo" {
		t.Errorf("post code 0159: city %q, %v", c.City, err)
	}
	c.City = "Elsewhere"
	if err := c.Validate("post_code", types.Code("9999")); err != nil || c.City != "Elsewhere" {
		t.Errorf("unknown post code: city %q, %v", c.City, err)
	}

	// Gen. Bus. Posting Group fills in its default Tax Bus. Posting Group
	var tax gtables.TaxBusinessPostingGroup
	tax.InitWithDBType(db, "c1", database.DBTypeSQLite)
	tax.Code = "DOMESTIC"
	var gen gtables.GenBusinessPostingGroup
	gen.InitWithDBType(db, "c1", database.DBTypeSQLite)
	gen.InitRecord()
	gen.Code, gen.Def_tax_bus_posting_group, gen.Auto_insert_default = "DOMESTIC", "DOMESTIC", true
	if !tax.Insert(false) || !gen.Insert(false) {
		t.Fatal("insert posting groups")
	}
	if err := c.Validate("gen_bus_posting_group", types.Code("DOMESTIC")); err != nil || c.Tax_bus_posting_group != "DOMESTIC" {
		t.Errorf("gen. bus. posting group: tax %q, %v", c.Tax_bus_posting_group, err)
	}

	// Last Date Modified on modify
	var stored gtables.Customer
	stored.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if !stored.Get(types.Code("C00001")) {
		t.Fatal("C00001")
	}
	stored.Name = "Acme Ltd"
	if !stored.Modify(true) || !stored.Last_date_modified.Equal(system.Today()) {
		t.Errorf("last date modified %v", stored.Last_date_modified)
	}

	// No delete while ledger entries exist (no closed fiscal years yet)
	var e gtables.CustomerLedgerEntry
	e.InitWithDBType(db, "c1", database.DBTypeSQLite)
	e.Entry_no, e.Customer_no, e.Amount = 1, "C00001", types.NewDecimal(10)
	if !e.Insert(false) {
		t.Fatal("insert entry")
	}
	if stored.Delete(true) || stored.TriggerError() == nil || errCode(t, stored.TriggerError()) != apperrors.ErrDeleteHasEntries {
		t.Errorf("delete with entries: %v", stored.TriggerError())
	} else if msg := stored.TriggerError().(*apperrors.AppError).Message("en-US"); msg != "You cannot delete Customer C00001 because it has ledger entries in a fiscal year that has not been closed yet." {
		t.Errorf("message %q", msg)
	}
	var other gtables.Customer
	other.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if !other.Get(types.Code("C00002")) || !other.Delete(true) {
		t.Errorf("delete without entries: %v", other.TriggerError())
	}

	// AssistEdit without a dialog (no page can open): nothing changes
	c = newCust()
	if changed, err := AssistEditCustomer(c); changed || err != nil || c.No != "" {
		t.Errorf("AssistEdit without a dialog: %v %v %q", changed, err, c.No)
	}
}

// Debit/Credit Amount from the amounts; the Payments FlowField is the reversed sum (-Sum)
func TestDebitCreditAndPayments(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.Customer{}, &gtables.CustomerLedgerEntry{})
	for i, a := range []struct {
		amount  string
		docType gtables.CustomerLedgerEntryDocument_type
	}{{"100", gtables.CustomerLedgerEntry_Document_type.Invoice}, {"-60", gtables.CustomerLedgerEntry_Document_type.Payment}, {"-15.5", gtables.CustomerLedgerEntry_Document_type.Payment}} {
		var e gtables.CustomerLedgerEntry
		e.InitWithDBType(db, "c1", database.DBTypeSQLite)
		e.Entry_no, e.Customer_no, e.Document_type = i+1, "C1", a.docType
		e.Amount, e.Amount_lcy = types.MustDecimal(a.amount), types.MustDecimal(a.amount)
		SetDebitCredit(&e)
		if !e.Insert(false) {
			t.Fatal("insert entry")
		}
	}
	var c gtables.Customer
	c.InitWithDBType(db, "c1", database.DBTypeSQLite)
	c.No = "C1"
	if !c.Insert(false) {
		t.Fatal("insert customer")
	}
	c.CalcFields()
	for name, got := range map[string]types.Decimal{"payments": c.Payments, "debit": c.Debit_amount, "credit": c.Credit_amount_lcy, "balance": c.Balance, "invoices": c.Invoice_amounts} {
		want := map[string]string{"payments": "75.5", "debit": "100", "credit": "75.5", "balance": "24.5", "invoices": "100"}[name]
		if !got.Equal(types.MustDecimal(want)) {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
}

// A new customer saved with the number a related series just issued (AssistEdit: SelectSeries,
// SetSeries) keeps that series, also when the Customer Nos. series allows no manual numbers
func TestCustomerNumberFromRelatedSeries(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.Customer{})
	setupCustomerNos(t, db, "c1", false)
	var s gtables.NoSeries
	s.InitWithDBType(db, "c1", database.DBTypeSQLite)
	s.Code, s.Default_nos = "CUST-EXP", true
	var l gtables.NoSeriesLine
	l.InitWithDBType(db, "c1", database.DBTypeSQLite)
	l.InitRecord()
	l.Series_code, l.Line_no, l.Starting_no = "CUST-EXP", 10000, "E00001"
	var rel gtables.NoSeriesRelationship
	rel.InitWithDBType(db, "c1", database.DBTypeSQLite)
	rel.Code, rel.Series_code = "CUST", "CUST-EXP"
	if !s.Insert(false) || !l.Insert(false) || !rel.Insert(false) {
		t.Fatal("insert series")
	}
	m := noseries.New(ftables.DBSource(db, "c1", database.DBTypeSQLite))
	no, err := m.GetNextNo("CUST-EXP", types.Date{}, true)
	if err != nil || no != "E00001" {
		t.Fatalf("next number %q, %v", no, err)
	}

	var c gtables.Customer
	c.InitWithDBType(db, "c1", database.DBTypeSQLite)
	c.InitRecord()
	if err := c.Validate("no", no); err != nil || c.No_series != "CUST-EXP" {
		t.Fatalf("issued number: series %q, %v", c.No_series, err)
	}
	if !c.Insert(true) {
		t.Fatal(c.TriggerError())
	}
	// A number no series issued is typed by hand: refused (CUST has no Manual Nos.)
	var d gtables.Customer
	d.InitWithDBType(db, "c1", database.DBTypeSQLite)
	d.InitRecord()
	if err := d.Validate("no", types.Code("E00007")); err == nil {
		t.Error("a number no series issued was accepted")
	}
}

// The generated record does what classic records do without code in the triggers: INSERT
// refuses a taken key, GET/MODIFY/DELETE "without IF" fail with the standard messages, VALIDATE
// checks the field's table relation, and a setup table is read with its getter
func TestGeneratedRecordErrors(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.Customer{}, &gtables.PaymentTerms{}, &gtables.SalesReceivablesSetup{})
	msg := func(err error) string {
		t.Helper()
		if err == nil {
			return "<nil>"
		}
		return err.(*apperrors.AppError).Message("en-US")
	}
	var c gtables.Customer
	c.InitWithDBType(db, "c1", database.DBTypeSQLite)
	c.InitRecord()
	c.No = "C1"
	if err := c.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	// INSERT of a taken key: refused, the record stays as it was
	var d gtables.Customer
	d.InitWithDBType(db, "c1", database.DBTypeSQLite)
	d.InitRecord()
	d.No, d.Name = "C1", "Other"
	if d.Insert(false) || msg(d.TriggerError()) != "Customer C1 already exists" {
		t.Errorf("taken key: %v", d.TriggerError())
	}
	err := c.GetOrError(types.Code("C1"))
	if err != nil || c.Name != "" {
		t.Errorf("the stored customer was changed: %q, %v", c.Name, err)
	}
	if got := msg(d.GetOrError(types.Code("NONE"))); got != "Customer NONE not found" {
		t.Errorf("GetOrError: %q", got)
	}
	// VALIDATE checks the table relation before OnValidate
	if got := msg(c.Validate("payment_terms_code", types.Code("NONE"))); got != "Payment Terms Code 'NONE' does not exist" {
		t.Errorf("relation: %q", got)
	}
	// A setup table read before its record exists: a blank one with the defaults
	if setup := gtables.GetSalesReceivablesSetup(&c); setup.Customer_nos != "" || !setup.Stockout_warning {
		t.Errorf("setup = %+v", setup.ToMap())
	}
}
