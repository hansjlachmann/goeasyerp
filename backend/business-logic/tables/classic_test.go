package tables

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// TransferFields copies by field number and type, also between tables
func TestTransferFields(t *testing.T) {
	from := &gtables.Customer{}
	from.No, from.Name, from.City = types.NewCode("C1"), types.NewText("Acme"), types.NewText("Oslo")
	from.Blocked = true

	to := &gtables.Customer{}
	to.No = types.NewCode("C2")
	to.TransferFields(from, false) // keep the key
	if to.No != "C2" || to.Name != "Acme" || to.City != "Oslo" || !to.Blocked {
		t.Errorf("same table: %+v", to.ToMap())
	}
	to.TransferFields(from)
	if to.No != "C1" {
		t.Errorf("with the key: no = %s", to.No)
	}

	// Country/Region 1 = Code, 2 = Name: the same numbers and types as Customer No. and Name
	var country gtables.CountryRegion
	country.TransferFields(from)
	if country.Code != "C1" || country.Name != "Acme" {
		t.Errorf("other table: %+v", country.ToMap())
	}
	// A value of another type is not set
	if country.SetFieldValue(2, types.NewCode("X")) || country.Name != "Acme" {
		t.Error("a Code value was set into a Text field")
	}
}

func TestTestFieldAndFieldError(t *testing.T) {
	var c gtables.Customer
	err := c.TestField("name")
	var appErr *apperrors.AppError
	if !asAppError(err, &appErr) || appErr.Code != apperrors.ErrRequiredField || appErr.Field != "name" {
		t.Errorf("empty name: %v", err)
	}
	c.Name = types.NewText("Acme")
	if err := c.TestField("Name"); err != nil {
		t.Errorf("name with a value: %v", err)
	}
	c.Application_method = gtables.Customer_Application_method.Manual
	if err := c.TestField("application_method", gtables.Customer_Application_method.Manual); err != nil {
		t.Errorf("application method Manual: %v", err)
	}
	err = c.TestField("application_method", gtables.Customer_Application_method.ApplyToOldest)
	if !asAppError(err, &appErr) || appErr.Code != apperrors.ErrFieldMustBe || appErr.Message("en-US") != "Application Method must be Apply to Oldest" {
		t.Errorf("application method Apply to Oldest: %v", err)
	}
	err = c.TestField("blocked", true)
	if !asAppError(err, &appErr) || appErr.Message("en-US") != "Blocked must be Yes" {
		t.Errorf("blocked: %v", err)
	}
	if err := c.TestField("credit_limit_lcy", types.NewDecimal(0)); err != nil {
		t.Errorf("decimal 0: %v", err)
	}
	if err := c.TestField("bogus"); err == nil {
		t.Error("unknown field accepted")
	}
	err = c.FieldError("City", "is not allowed for this customer")
	if !asAppError(err, &appErr) || appErr.Message("en-US") != "City is not allowed for this customer" {
		t.Errorf("FieldError: %v", err)
	}
}

func asAppError(err error, target **apperrors.AppError) bool {
	e, ok := err.(*apperrors.AppError)
	*target = e
	return ok
}

func TestCalcSumsChangeCompanyInit(t *testing.T) {
	db := newTestDB(t)
	for _, company := range []string{"c1", "c2"} {
		if err := (&gtables.CustomerLedgerEntry{}).CreateTableWithDBType(db, company, database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	if err := (&gtables.Company{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO "Company" (name) VALUES ('c1'), ('c2')`); err != nil {
		t.Fatal(err)
	}
	add := func(company string, no int, customer, amount string) {
		e := &gtables.CustomerLedgerEntry{}
		e.InitWithDBType(db, company, database.DBTypeSQLite)
		e.Entry_no, e.Customer_no, e.Amount_lcy = no, types.NewCode(customer), types.MustDecimal(amount)
		if !e.Insert(false) {
			t.Fatalf("insert %d", no)
		}
	}
	add("c1", 1, "C1", "100.50")
	add("c1", 2, "C1", "-20.25")
	add("c1", 3, "C2", "1000")
	add("c2", 1, "C1", "7")

	var e gtables.CustomerLedgerEntry
	e.InitWithDBType(db, "c1", database.DBTypeSQLite)
	e.SetRange("customer_no", "C1")
	if err := e.CalcSums("amount_lcy", "entry_no"); err != nil {
		t.Fatal(err)
	}
	if e.Amount_lcy.String() != "80.25" || e.Entry_no != 3 {
		t.Errorf("sums: amount %s, entry_no %d", e.Amount_lcy, e.Entry_no)
	}
	if err := e.CalcSums("description"); err == nil {
		t.Error("CalcSums on a text field accepted")
	}

	// ChangeCompany: the same filters in another company
	if e.CompanyName() != "c1" || e.ChangeCompany("nope") || e.CompanyName() != "c1" {
		t.Error("ChangeCompany to a company that does not exist")
	}
	if !e.ChangeCompany("c2") || e.CompanyName() != "c2" || e.Count() != 1 {
		t.Errorf("ChangeCompany c2: %s, %d entries", e.CompanyName(), e.Count())
	}

	// Init keeps the primary key
	e.Entry_no, e.Description = 5, types.NewText("x")
	e.Init()
	if e.Entry_no != 5 || e.Description != "" {
		t.Errorf("Init: entry %d, description %q", e.Entry_no, e.Description)
	}
}

func TestCaptionsAndValidate(t *testing.T) {
	var c gtables.Customer
	if c.TableCaption() != "Customer" || c.FieldCaption("name") != "Name" || !strings.Contains(c.TableCaption("nb-NO"), "Kunde") {
		t.Errorf("captions %q %q %q", c.TableCaption(), c.FieldCaption("name"), c.TableCaption("nb-NO"))
	}
	if err := c.Validate("name", "Acme"); err != nil || c.Search_name != "ACME" {
		t.Errorf("Validate must run OnValidate_Name (Search Name follows): %v, %q", err, c.Search_name)
	}
}

// recordingDB records the SQL a record sends (no database behind it)
type recordingDB struct{ queries []string }

func (r *recordingDB) Exec(q string, _ ...interface{}) (sql.Result, error) {
	r.queries = append(r.queries, q)
	return nil, errors.New("no database")
}
func (r *recordingDB) Query(q string, _ ...interface{}) (*sql.Rows, error) {
	r.queries = append(r.queries, q)
	return nil, errors.New("no database")
}
func (r *recordingDB) QueryRow(q string, _ ...interface{}) *sql.Row {
	r.queries = append(r.queries, q)
	return &sql.Row{}
}
func (r *recordingDB) Prepare(q string) (*sql.Stmt, error) { return nil, errors.New("no database") }

func TestLockTableReadsForUpdate(t *testing.T) {
	db := &recordingDB{}
	var c gtables.Customer
	c.InitWithDBType(db, "c1", database.DBTypePostgres)
	c.FindSet()
	c.LockTable()
	c.FindSet()
	if len(db.queries) != 2 || strings.HasSuffix(db.queries[0], "FOR UPDATE") || !strings.HasSuffix(db.queries[1], " FOR UPDATE") {
		t.Errorf("queries %q", db.queries)
	}
	// SQLite has no FOR UPDATE
	lite := &recordingDB{}
	c.InitWithDBType(lite, "c1", database.DBTypeSQLite)
	c.LockTable()
	c.FindSet()
	if len(lite.queries) != 1 || strings.Contains(lite.queries[0], "FOR UPDATE") {
		t.Errorf("SQLite queries %q", lite.queries)
	}
}

func TestMoreRecordFunctions(t *testing.T) {
	db := newTestDB(t)
	// Rename carries the new number over to every table that refers to customers
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	for _, no := range []string{"C1", "C2", "C3"} {
		c := &gtables.Customer{}
		c.InitWithDBType(db, "c1", database.DBTypeSQLite)
		c.No, c.Name = types.NewCode(no), types.NewText("Name "+no)
		if !c.Insert(false) {
			t.Fatalf("insert %s", no)
		}
	}
	var c gtables.Customer
	c.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if c.MaxStrLen("name") != 50 || c.MaxStrLen("credit_limit_lcy") != 0 {
		t.Errorf("MaxStrLen %d", c.MaxStrLen("name"))
	}

	// Filters
	c.SetRange("no", "C1", "C2")
	if c.GetFilter("no") != "C1..C2" || c.GetRangeMin("no") != "C1" || c.GetRangeMax("no") != "C2" || c.GetFilter("name") != "" {
		t.Errorf("GetFilter %q", c.GetFilter("no"))
	}
	c.SetFilter("city", "Oslo|Bergen")
	if c.GetFilter("city") != "Oslo|Bergen" {
		t.Errorf("GetFilter expression %q", c.GetFilter("city"))
	}
	c.Reset()

	// Find, SetRecFilter, RecordId
	if !c.Find("+") || c.No != "C3" || !c.Find("-") || c.No != "C1" || c.RecordId() != "Customer: C1" {
		t.Errorf("Find -/+: %s", c.No)
	}
	c.No = types.NewCode("C2")
	if !c.Find("=") || c.Name != "Name C2" || c.GetFilter("no") != "" {
		t.Errorf("Find =: %s, filter %q", c.Name, c.GetFilter("no"))
	}
	c.SetRecFilter()
	if c.Count() != 1 || c.CountApprox() != 1 {
		t.Errorf("SetRecFilter: %d", c.Count())
	}
	c.Reset()

	// Mark / MarkedOnly
	c.Get(types.NewCode("C1"))
	c.Mark(true)
	c.Get(types.NewCode("C3"))
	if !c.Mark(true) {
		t.Error("Mark(true) must report marked")
	}
	c.MarkedOnly(true)
	if c.Count() != 2 {
		t.Errorf("MarkedOnly: %d records", c.Count())
	}
	c.Get(types.NewCode("C3"))
	c.Mark(false)
	if c.Count() != 1 {
		t.Errorf("after unmarking: %d", c.Count())
	}
	c.ClearMarks()
	if c.Count() != 0 {
		t.Errorf("MarkedOnly without marks: %d", c.Count())
	}
	c.MarkedOnly(false)

	// Copy and Rename
	var other gtables.Customer
	other.InitWithDBType(db, "c1", database.DBTypeSQLite)
	c.SetRange("name", "Name C2")
	c.Get(types.NewCode("C2"))
	other.Copy(&c)
	if other.Name != "Name C2" || other.GetFilter("name") != "Name C2" {
		t.Errorf("Copy: %s, %q", other.Name, other.GetFilter("name"))
	}
	if !c.Rename("C9") {
		t.Fatalf("Rename: %v", c.TriggerError())
	}
	var check gtables.Customer
	check.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if check.Get(types.NewCode("C2")) || !check.Get(types.NewCode("C9")) || check.Name != "Name C2" {
		t.Error("Rename did not move C2 to C9")
	}
	if c.Rename(42) || c.Rename("A", "B") {
		t.Error("Rename with a wrong type or count accepted")
	}
}

// A date formula field (date_formula: true) only takes date formulas: in ValidateField and in
// the field checks of Insert/Modify
func TestDateFormulaField(t *testing.T) {
	var pt gtables.PaymentTerms
	pt.Code = types.NewCode("14D")
	if err := pt.ValidateField("due_date_calculation", "14d"); err != nil || pt.Due_date_calculation != "14D" {
		t.Errorf("14D: %v (%q)", err, pt.Due_date_calculation)
	}
	err := pt.ValidateField("due_date_calculation", "14 days")
	var appErr *apperrors.AppError
	if !asAppError(err, &appErr) || appErr.Code != apperrors.ErrDateFormulaInvalid || appErr.Field != "due_date_calculation" {
		t.Errorf("14 days: %v", err)
	}
	pt.Due_date_calculation, pt.Discount_date_calculation = types.NewCode("1M+CM"), types.NewCode("8X")
	if err := pt.CheckFields(); !asAppError(err, &appErr) || appErr.Field != "discount_date_calculation" {
		t.Errorf("CheckFields with 8X: %v", err)
	}
	pt.Discount_date_calculation = ""
	if err := pt.CheckFields(); err != nil {
		t.Errorf("CheckFields with a blank formula: %v", err)
	}
}

// CopyFilter copies one field's filter to a field of another table; no filter removes it.
// SetRange without values removes a filter. Every table has its number as a constant.
func TestCopyFilter(t *testing.T) {
	var cust gtables.Customer
	var entry gtables.CustomerLedgerEntry
	cust.SetFilter("no", "C1..C5|C9")
	cust.CopyFilter("no", &entry, "customer_no")
	if got := entry.GetFilter("customer_no"); got != "C1..C5|C9" {
		t.Errorf("expression: %q", got)
	}
	cust.SetRange("no", "C2", "C3")
	cust.CopyFilter("no", &entry, "customer_no")
	if entry.GetRangeMin("customer_no") != "C2" || entry.GetFilter("customer_no") != "C2..C3" {
		t.Errorf("range: %q", entry.GetFilter("customer_no"))
	}
	cust.SetRange("no")
	if cust.GetFilter("no") != "" {
		t.Errorf("SetRange without values: %q", cust.GetFilter("no"))
	}
	cust.CopyFilter("no", &entry, "customer_no")
	if entry.GetFilter("customer_no") != "" {
		t.Errorf("no filter copied: %q", entry.GetFilter("customer_no"))
	}
	if gtables.PaymentTermsTableID != 3 || gtables.CustomerTableID != 18 || cust.GetTableID() != gtables.CustomerTableID {
		t.Error("table number constants")
	}
}
