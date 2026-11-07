package tables

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/company"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	_ "github.com/lib/pq"
)

func refused(t *testing.T, what string, deleted bool, err error, code apperrors.ErrorCode) {
	t.Helper()
	if deleted {
		t.Fatalf("%s: deleted, want %s", what, code)
	}
	if errCode(t, err) != code {
		t.Fatalf("%s: %v, want %s", what, err, code)
	}
}

// checkCodeInUse: a code is refused while any record uses it, posted documents too, by the
// delete with triggers only; a used value of the same text in a conditional relation's other
// type does not count
func checkCodeInUse(t *testing.T, src ftables.Source) {
	terms := gtables.NewPaymentTerms(src)
	terms.Code = "30D"
	if err := terms.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	cust := gtables.NewCustomer(src)
	cust.No, cust.Payment_terms_code = "C1", "30D"
	if err := cust.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	refused(t, "terms used by a customer", terms.Delete(true), terms.TriggerError(), apperrors.ErrDeleteInUse)
	if !gtables.NewPaymentTerms(src).Get(types.Code("30D")) {
		t.Fatal("refused, but the terms are gone")
	}

	cust.Payment_terms_code = ""
	if err := cust.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	invoice := gtables.NewSalesInvoiceHeader(src)
	invoice.No, invoice.Payment_terms_code = "I1", "30D"
	if err := invoice.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	refused(t, "terms used by a posted invoice", terms.Delete(true), terms.TriggerError(), apperrors.ErrDeleteInUse)
	if !invoice.Delete(false) {
		t.Fatal("delete invoice")
	}
	if err := terms.DeleteOrError(true); err != nil {
		t.Fatalf("unused terms: %v", err)
	}

	// Delete without triggers is what code uses on its own records: not checked
	used := gtables.NewPaymentTerms(src)
	used.Code = "14D"
	if err := used.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	cust.Payment_terms_code = "14D"
	if err := cust.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	if !used.Delete(false) {
		t.Fatal("delete without triggers")
	}

	// A sales line of type Resource with the same number does not use the item
	item := gtables.NewItem(src)
	item.No = "X1"
	if err := item.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	line := gtables.NewSalesLine(src)
	line.Document_no, line.Line_no, line.Type, line.No = "S1", 10000, gtables.SalesLine_Type.Resource, "X1"
	if err := line.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := item.DeleteOrError(true); err != nil {
		t.Fatalf("item with a resource line of its number: %v", err)
	}
	item.No = "X1"
	if err := item.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	line.Type = gtables.SalesLine_Type.Item
	if err := line.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	refused(t, "item on a sales line", item.Delete(true), item.TriggerError(), apperrors.ErrDeleteInUse)
}

func TestDeleteRefusedWhileCodeInUse(t *testing.T) {
	db := newTestDB(t)
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	checkCodeInUse(t, ftables.DBSource(db, "c1", database.DBTypeSQLite))
}

// The same on PostgreSQL (TEST_POSTGRES_DSN), where the check's own SQL differs; a global
// table's references are looked for in every company, and a company without the table is
// no error
func TestDeleteRefusedWhileCodeInUsePostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	name := fmt.Sprintf("deltest%d", time.Now().UnixNano()%1000000)
	dbType := database.DBTypePostgres
	language := &gtables.Language{}
	comp := &gtables.Company{}
	if err := language.CreateTableWithDBType(db, "", dbType); err != nil {
		t.Fatal(err)
	}
	if err := comp.CreateTableWithDBType(db, "", dbType); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		tx, err := db.Begin()
		if err == nil {
			_ = company.DropObjects(tx, dbType, name)
			_ = tx.Commit()
		}
		_, _ = db.Exec(`DELETE FROM "Company" WHERE name = $1`, name)
		_, _ = db.Exec(`DELETE FROM "Language" WHERE code = 'XT'`)
	})
	if err := CreateCompanyTables(db, name, dbType); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, name, dbType)
	checkCodeInUse(t, src)

	if _, err := db.Exec(`INSERT INTO "Company" (name) VALUES ($1)`, name); err != nil {
		t.Fatal(err)
	}
	lang := gtables.NewLanguage(src)
	lang.Code = "XT"
	if err := lang.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	cust := gtables.NewCustomer(src)
	cust.No, cust.Language_code = "C2", "XT"
	if err := cust.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	lang.SetDB(tx)
	refused(t, "language used in a company", lang.Delete(true), lang.TriggerError(), apperrors.ErrDeleteInUse)
	if err := ftables.Rollback(tx); err != nil {
		t.Fatal(err)
	}
	lang.SetDB(db)
	if !cust.Delete(false) {
		t.Fatal("delete customer")
	}
	if err := lang.DeleteOrError(true); err != nil {
		t.Fatalf("unused language: %v", err)
	}
}

// Master data: open documents refuse the delete, its posted documents do not; a customer
// billed to itself does not refuse its own delete, another customer billed to it does
func TestDeleteCustomerWithDocuments(t *testing.T) {
	db := newTestDB(t)
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	cust := gtables.NewCustomer(src)
	cust.No, cust.Bill_to_customer_no = "C1", "C1"
	if err := cust.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	order := gtables.NewSalesHeader(src)
	order.Document_type, order.No, order.Sell_to_customer_no = gtables.SalesHeader_Document_type.Order, "S1", "C1"
	if err := order.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	refused(t, "customer with an order", cust.Delete(true), cust.TriggerError(), apperrors.ErrDeleteInUse)
	if !order.Delete(false) {
		t.Fatal("delete order")
	}
	other := gtables.NewCustomer(src)
	other.No, other.Bill_to_customer_no = "C2", "C1"
	if err := other.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	refused(t, "customer another bills to", cust.Delete(true), cust.TriggerError(), apperrors.ErrDeleteInUse)
	if !other.Delete(false) {
		t.Fatal("delete other")
	}
	invoice := gtables.NewSalesInvoiceHeader(src)
	invoice.No, invoice.Sell_to_customer_no = "I1", "C1"
	if err := invoice.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := cust.DeleteOrError(true); err != nil {
		t.Fatalf("customer with a posted invoice only: %v", err)
	}
}

// An item: refused with entries in an open year or open entries; otherwise they lose its No.
func TestDeleteItemWithEntries(t *testing.T) {
	db := newTestDB(t)
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	addPeriods(t, db, 2025, 2, types.NewDate(2025, 12, 1))
	item := gtables.NewItem(src)
	item.No = "1000"
	if err := item.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	entry := gtables.NewItemLedgerEntry(src)
	entry.Entry_no, entry.Item_no, entry.Posting_date, entry.Open = 1, "1000", types.NewDate(2026, 3, 1), false
	if err := entry.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	refused(t, "entry in an open year", item.Delete(true), item.TriggerError(), apperrors.ErrDeleteHasEntries)
	entry.Posting_date, entry.Open = types.NewDate(2025, 3, 1), true
	if err := entry.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	refused(t, "open entry", item.Delete(true), item.TriggerError(), apperrors.ErrDeleteOpenEntries)
	entry.Open = false
	if err := entry.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := item.DeleteOrError(true); err != nil {
		t.Fatalf("closed entry in a closed year: %v", err)
	}
	if !entry.Get(1) || entry.Item_no != "" {
		t.Errorf("entry item no %q", entry.Item_no)
	}
}

// A G/L account: refused with entries in an open year, a balance, or a posting group using
// it; otherwise the entries lose its No.
func TestDeleteGLAccountWithEntries(t *testing.T) {
	db := newTestDB(t)
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	addPeriods(t, db, 2025, 2, types.NewDate(2025, 12, 1))
	glAcc := gtables.NewGLAccount(src)
	glAcc.No = "1000"
	if err := glAcc.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	entry := gtables.NewGLEntry(src)
	entry.Entry_no, entry.Gl_account_no, entry.Posting_date, entry.Amount = 1, "1000", types.NewDate(2026, 3, 1), types.MustDecimal("100")
	if err := entry.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	refused(t, "entry in an open year", glAcc.Delete(true), glAcc.TriggerError(), apperrors.ErrDeleteHasEntries)
	entry.Posting_date = types.NewDate(2025, 3, 1)
	if err := entry.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	refused(t, "balance", glAcc.Delete(true), glAcc.TriggerError(), apperrors.ErrDeleteBalanceNotZero)
	second := gtables.NewGLEntry(src)
	second.Entry_no, second.Gl_account_no, second.Posting_date, second.Amount = 2, "1000", types.NewDate(2025, 4, 1), types.MustDecimal("-100")
	if err := second.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	group := gtables.NewCustomerPostingGroup(src)
	group.Code, group.Receivables_account = "DOM", "1000"
	if err := group.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	refused(t, "posting group", glAcc.Delete(true), glAcc.TriggerError(), apperrors.ErrDeleteInUse)
	group.Receivables_account = ""
	if err := group.ModifyOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := glAcc.DeleteOrError(true); err != nil {
		t.Fatalf("no balance in a closed year: %v", err)
	}
	if !entry.Get(1) || entry.Gl_account_no != "" {
		t.Errorf("entry account no %q", entry.Gl_account_no)
	}
}

// What belongs to a record goes with it: a role's permissions, terms' levels and texts, a tax
// area's lines; a role with a member stays
func TestDeleteTakesItsOwnRecords(t *testing.T) {
	db := newTestDB(t)
	if err := CreateCompanyTables(db, "c1", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.UserRole{}, &gtables.Permission{}, &gtables.UserMember{}, &gtables.Company{}} {
		if err := tbl.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	src := ftables.DBSource(db, "c1", database.DBTypeSQLite)
	role := gtables.NewUserRole(src)
	role.Code = "CLERK"
	if err := role.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	permission := gtables.NewPermission(src)
	permission.Role_id, permission.Table_name = "CLERK", "Customer"
	if err := permission.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	member := gtables.NewUserMember(src)
	member.User_id, member.Role_id = "HANS", "CLERK"
	if err := member.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	role.SetDB(tx)
	refused(t, "role with a member", role.Delete(true), role.TriggerError(), apperrors.ErrDeleteInUse)
	if err := ftables.Rollback(tx); err != nil {
		t.Fatal(err)
	}
	role.SetDB(db)
	if gtables.NewPermission(src).Count() != 1 {
		t.Fatal("the refused delete took the permissions along")
	}
	if !member.Delete(false) {
		t.Fatal("delete member")
	}
	if err := role.DeleteOrError(true); err != nil {
		t.Fatal(err)
	}
	if gtables.NewPermission(src).Count() != 0 {
		t.Error("the role's permissions are left")
	}

	terms := gtables.NewReminderTerms(src)
	terms.Code = "STD"
	if err := terms.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	level := gtables.NewReminderLevel(src)
	level.Reminder_terms_code, level.No = "STD", 1
	if err := level.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := terms.DeleteOrError(true); err != nil {
		t.Fatal(err)
	}
	if gtables.NewReminderLevel(src).Count() != 0 {
		t.Error("the terms' levels are left")
	}

	area := gtables.NewTaxArea(src)
	area.Code = "OSLO"
	if err := area.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	areaLine := gtables.NewTaxAreaLine(src)
	areaLine.Tax_area, areaLine.Tax_jurisdiction_code = "OSLO", "NO"
	if err := areaLine.InsertOrError(false); err != nil {
		t.Fatal(err)
	}
	if err := area.DeleteOrError(true); err != nil {
		t.Fatal(err)
	}
	if gtables.NewTaxAreaLine(src).Count() != 0 {
		t.Error("the area's lines are left")
	}
}
