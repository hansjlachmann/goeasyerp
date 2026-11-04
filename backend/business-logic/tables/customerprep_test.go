package tables

import (
	"database/sql"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func createTables(t *testing.T, db *sql.DB, tables ...interface {
	CreateTableWithDBType(database.Executor, string, database.DBType) error
}) {
	t.Helper()
	for _, tbl := range tables {
		if err := tbl.CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
}

func errCode(t *testing.T, err error) apperrors.ErrorCode {
	t.Helper()
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		t.Fatalf("error %v is not an AppError", err)
	}
	return appErr.Code
}

// addGLAccount inserts an account (type, blocked, Gen. Prod. Posting Group, direct posting)
func addGLAccount(t *testing.T, db *sql.DB, no string, accType gtables.GLAccountAccount_type, blocked bool, prodGroup string, direct bool) {
	t.Helper()
	var a gtables.GLAccount
	a.InitWithDBType(db, "c1", database.DBTypeSQLite)
	a.InitRecord()
	a.No, a.Account_type, a.Blocked = types.Code(no), accType, blocked
	a.Gen_prod_posting_group, a.Direct_posting = types.Code(prodGroup), direct
	if !a.Insert(false) {
		t.Fatalf("insert account %s", no)
	}
}

// Customer Posting Group and Payment Method accounts: posting accounts, not blocked, with a
// Gen. Prod. Posting Group / Direct Posting where the field needs it
func TestCustomerPostingGroupAndPaymentMethodAccounts(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.GLAccount{}, &gtables.CustomerPostingGroup{}, &gtables.PaymentMethod{})
	posting := gtables.GLAccount_Account_type.Posting
	addGLAccount(t, db, "1000", gtables.GLAccount_Account_type.Heading, false, "", false)
	addGLAccount(t, db, "1100", posting, true, "", false)
	addGLAccount(t, db, "1200", posting, false, "", false)
	addGLAccount(t, db, "1300", posting, false, "SERVICES", false)
	addGLAccount(t, db, "1400", posting, false, "SERVICES", true)

	var g gtables.CustomerPostingGroup
	g.InitWithDBType(db, "c1", database.DBTypeSQLite)
	g.InitRecord()
	for _, c := range []struct {
		field, acc string
		want       apperrors.ErrorCode
	}{
		{"receivables_account", "1000", apperrors.ErrRelatedNotFound}, // not a posting account: the relation is to posting accounts only
		{"receivables_account", "1100", apperrors.ErrFieldMustBe},     // blocked
		{"receivables_account", "9999", apperrors.ErrRelatedNotFound},
		{"receivables_account", "1200", ""},                      // no further checks
		{"interest_account", "1200", apperrors.ErrRequiredField}, // Gen. Prod. Posting Group
		{"interest_account", "1300", ""},                         // no Direct Posting needed
		{"service_charge_acc", "1300", apperrors.ErrFieldMustBe}, // Direct Posting
		{"service_charge_acc", "1400", ""},
		{"receivables_account", "", ""},
	} {
		err := g.Validate(c.field, types.Code(c.acc))
		if c.want == "" && err != nil || c.want != "" && (err == nil || errCode(t, err) != c.want) {
			t.Errorf("%s = %s: %v, want %s", c.field, c.acc, err, c.want)
		}
	}
	err := g.Validate("service_charge_acc", types.Code("1300"))
	if msg := err.(*apperrors.AppError).Message("en-US"); msg != "Direct Posting must be Yes" {
		t.Errorf("message = %q", msg)
	}
	if msg := err.(*apperrors.AppError).Message("nb-NO"); msg == "Direct Posting must be Yes" {
		t.Errorf("nb-NO message not translated: %q", msg)
	}

	var pm gtables.PaymentMethod
	pm.InitWithDBType(db, "c1", database.DBTypeSQLite)
	pm.InitRecord()
	if err := pm.Validate("bal_account_no", types.Code("1300")); err == nil {
		t.Error("payment method account without direct posting: no error")
	}
	if err := pm.Validate("bal_account_no", types.Code("1400")); err != nil || pm.Bal_account_no != "1400" {
		t.Errorf("payment method account 1400: %v", err)
	}
	if err := pm.Validate("bal_account_type", gtables.PaymentMethod_Bal_account_type.BankAccount); err != nil || pm.Bal_account_no != "" {
		t.Errorf("type change keeps the account %q: %v", pm.Bal_account_no, err)
	}
	if msg := apperrors.FieldMustBeValue(gtables.GLAccountTableName, "account_type", gtables.GLAccount_Account_type.Posting).Message("en-US"); msg != "Account Type must be Posting" {
		t.Errorf("option value message = %q", msg)
	}
}

// Reminder levels: the next No. of the terms, texts deleted with their level and checked
// against it on insert
func TestReminderLevelsAndTexts(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.ReminderLevel{}, &gtables.ReminderText{})
	for _, l := range []struct {
		terms string
		no    int
	}{{"DOM", 1}, {"DOM", 2}, {"FOR", 1}} {
		var lv gtables.ReminderLevel
		lv.InitWithDBType(db, "c1", database.DBTypeSQLite)
		lv.InitRecord()
		lv.Reminder_terms_code, lv.No = types.Code(l.terms), l.no
		if !lv.Insert(true) {
			t.Fatalf("insert level %v", l)
		}
	}
	var lv gtables.ReminderLevel
	lv.InitWithDBType(db, "c1", database.DBTypeSQLite)
	lv.InitRecord()
	lv.Reminder_terms_code = "DOM"
	if NewReminderLevel(&lv); lv.No != 3 {
		t.Errorf("next DOM level = %d, want 3", lv.No)
	}
	lv.Reminder_terms_code = "NEW"
	if NewReminderLevel(&lv); lv.No != 1 {
		t.Errorf("first level of new terms = %d, want 1", lv.No)
	}

	addText := func(terms string, level, line int) bool {
		var tx gtables.ReminderText
		tx.InitWithDBType(db, "c1", database.DBTypeSQLite)
		tx.InitRecord()
		tx.Reminder_terms_code, tx.Reminder_level, tx.Line_no = types.Code(terms), level, line
		tx.Text = "Text"
		ok := tx.Insert(true)
		if !ok && (tx.TriggerError() == nil || errCode(t, tx.TriggerError()) != apperrors.ErrRecordNotFound) {
			t.Errorf("insert text %s %d: %v", terms, level, tx.TriggerError())
		}
		return ok
	}
	if !addText("DOM", 2, 10000) || !addText("DOM", 2, 20000) || !addText("DOM", 1, 10000) {
		t.Fatal("texts of existing levels not inserted")
	}
	if addText("DOM", 7, 10000) {
		t.Error("text of a missing level inserted")
	}

	var del gtables.ReminderLevel
	del.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if !del.GetByPK("DOM", 2) || !del.Delete(true) {
		t.Fatal("delete level DOM 2")
	}
	var tx gtables.ReminderText
	tx.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if n := tx.Count(); n != 1 {
		t.Errorf("texts after deleting level 2 = %d, want 1", n)
	}
}

// Deleting a number series deletes its lines and its relationships either way
func TestNoSeriesDelete(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.NoSeries{}, &gtables.NoSeriesLine{}, &gtables.NoSeriesRelationship{})
	for _, c := range []string{"A", "B"} {
		var s gtables.NoSeries
		s.InitWithDBType(db, "c1", database.DBTypeSQLite)
		s.Code = types.Code(c)
		var l gtables.NoSeriesLine
		l.InitWithDBType(db, "c1", database.DBTypeSQLite)
		l.InitRecord()
		l.Series_code, l.Line_no = types.Code(c), 10000
		if !s.Insert(true) || !l.Insert(false) {
			t.Fatal("insert series")
		}
	}
	for _, r := range [][2]string{{"A", "B"}, {"B", "A"}} {
		var rel gtables.NoSeriesRelationship
		rel.InitWithDBType(db, "c1", database.DBTypeSQLite)
		rel.Code, rel.Series_code = types.Code(r[0]), types.Code(r[1])
		if !rel.Insert(true) {
			t.Fatal("insert relationship")
		}
	}
	var s gtables.NoSeries
	s.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if !s.Get(types.Code("A")) || !s.Delete(true) {
		t.Fatal("delete A")
	}
	var l gtables.NoSeriesLine
	l.InitWithDBType(db, "c1", database.DBTypeSQLite)
	var rel gtables.NoSeriesRelationship
	rel.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if l.Count() != 1 || rel.Count() != 0 {
		t.Errorf("after deleting A: %d lines (want 1), %d relationships (want 0)", l.Count(), rel.Count())
	}
}

// Find("=") keeps the filters, also those on key fields; a date range from the blank date
// matches blank dates
func TestFindKeepsKeyFiltersAndBlankDateRanges(t *testing.T) {
	db := newTestDB(t)
	createTables(t, db, &gtables.NoSeries{}, &gtables.NoSeriesLine{})
	for _, c := range []string{"A", "B", "C"} {
		var s gtables.NoSeries
		s.InitWithDBType(db, "c1", database.DBTypeSQLite)
		s.Code = types.Code(c)
		if !s.Insert(false) {
			t.Fatal("insert")
		}
	}
	var s gtables.NoSeries
	s.InitWithDBType(db, "c1", database.DBTypeSQLite)
	s.SetFilter("code", "A|B")
	if s.Code = "C"; s.Find() {
		t.Error("Find found C outside the filter A|B")
	}
	if s.Code = "B"; !s.Find() || s.Code != "B" {
		t.Error("Find did not find B")
	}
	if s.GetFilter("code") != "A|B" {
		t.Errorf("filter after Find = %q", s.GetFilter("code"))
	}

	for i, d := range []types.Date{{}, types.MustDate("2026-01-10"), types.MustDate("2026-02-10")} {
		var l gtables.NoSeriesLine
		l.InitWithDBType(db, "c1", database.DBTypeSQLite)
		l.InitRecord()
		l.Series_code, l.Line_no, l.Starting_date = "A", (i+1)*10000, d
		if !l.Insert(false) {
			t.Fatal("insert line")
		}
	}
	var l gtables.NoSeriesLine
	l.InitWithDBType(db, "c1", database.DBTypeSQLite)
	l.SetRange("starting_date", types.Date{}, types.MustDate("2026-01-31"))
	if n := l.Count(); n != 2 {
		t.Errorf("0D..2026-01-31 = %d lines, want 2 (the blank date too)", n)
	}
	l.SetRange("starting_date", types.Date{})
	if n := l.Count(); n != 1 {
		t.Errorf("SetRange(blank date) = %d lines, want 1", n)
	}
}

// setupCustomerNos creates the Sales & Receivables Setup with Customer Nos. CUST (C00001 …,
// manual numbers allowed when manual)
func setupCustomerNos(t *testing.T, db *sql.DB, company string, manual bool) {
	t.Helper()
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.SalesReceivablesSetup{}, &gtables.NoSeries{}, &gtables.NoSeriesLine{}, &gtables.NoSeriesRelationship{}} {
		if err := tbl.CreateTableWithDBType(db, company, database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	var s gtables.NoSeries
	s.InitWithDBType(db, company, database.DBTypeSQLite)
	s.Code, s.Default_nos, s.Manual_nos = "CUST", true, manual
	var l gtables.NoSeriesLine
	l.InitWithDBType(db, company, database.DBTypeSQLite)
	l.InitRecord()
	l.Series_code, l.Line_no, l.Starting_no = "CUST", 10000, "C00001"
	var setup gtables.SalesReceivablesSetup
	setup.InitWithDBType(db, company, database.DBTypeSQLite)
	setup.InitRecord()
	setup.Customer_nos = "CUST"
	if !s.Insert(false) || !l.Insert(false) || !setup.Insert(false) {
		t.Fatal("insert customer number setup")
	}
}

// A relation to posting accounts only (WHERE Account Type=CONST(Posting)) still follows a G/L
// account's rename
func TestPostingAccountRelationRename(t *testing.T) {
	db := newTestDB(t)
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	addGLAccount(t, db, "1200", gtables.GLAccount_Account_type.Posting, false, "", false)
	var g gtables.CustomerPostingGroup
	g.InitWithDBType(db, "c1", database.DBTypeSQLite)
	g.InitRecord()
	g.Code = "DOM"
	if err := g.Validate("receivables_account", types.Code("1200")); err != nil {
		t.Fatal(err)
	}
	if !g.Insert(false) {
		t.Fatal("insert posting group")
	}
	var glAcc gtables.GLAccount
	glAcc.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if !glAcc.Get(types.Code("1200")) || !glAcc.Rename(types.Code("1210")) {
		t.Fatalf("rename: %v", glAcc.TriggerError())
	}
	if !g.Get(types.Code("DOM")) || g.Receivables_account != "1210" {
		t.Errorf("receivables account after the rename: %q", g.Receivables_account)
	}
}
