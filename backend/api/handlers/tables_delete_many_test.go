package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// deleteGuardCustomer is a customer whose business logic (deleteGuard) refuses to delete
// a customer named "Keep" (an OnDelete business rule)
type deleteGuardCustomer struct {
	gtables.Customer
}

func (t *deleteGuardCustomer) InitWithDBType(db database.Executor, company string, dbType database.DBType) {
	t.Customer.InitWithDBType(db, company, dbType)
	t.SetLogic(deleteGuard{})
}

type deleteGuard struct{}

func (deleteGuard) OnDelete(rec *gtables.Customer) error {
	if rec.Name.String() == "Keep" {
		return errors.New("customers named Keep stay")
	}
	return nil
}

func init() {
	gtables.RegisterTableFactory("Test_DeleteGuard", 99002, func() ftables.Table {
		return &deleteGuardCustomer{}
	})
}

func deleteJSON(t *testing.T, app *fiber.App, target, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest("DELETE", target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", target, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v (%s)", target, err, raw)
	}
	return resp.StatusCode, out
}

// Bulk delete (list multi-select): selected keys or every record the list shows, in one
// transaction — a record that cannot be deleted rolls back all and is named.
func TestDeleteRecords(t *testing.T) {
	app := newTablesTestApp(t)
	for i, name := range []string{"Anna", "Bjørn", "Carl", "Keep", "Dina", "Erik"} {
		postJSON(t, app, "/api/tables/Customer/insert", fmt.Sprintf(`{"no":"C%02d","name":"%s"}`, i+1, name))
	}
	remaining := func() string {
		t.Helper()
		_, out := getJSON(t, app, "/api/tables/Customer/list")
		nos, _ := listNos(t, out)
		return fmt.Sprint(nos)
	}

	// Selected keys
	status, out := deleteJSON(t, app, "/api/tables/Customer/delete-many", `{"keys":["C01","C02"]}`)
	if status != 200 || out["data"].(map[string]interface{})["deleted"] != 2.0 || remaining() != "[C03 C04 C05 C06]" {
		t.Fatalf("delete keys = %d %v, remaining %s", status, out, remaining())
	}

	// A key that does not exist: nothing is deleted
	status, _ = deleteJSON(t, app, "/api/tables/Customer/delete-many", `{"keys":["C03","C99"]}`)
	if status != 404 || remaining() != "[C03 C04 C05 C06]" {
		t.Errorf("unknown key = %d, remaining %s (want 404 and nothing deleted)", status, remaining())
	}

	// An OnDelete trigger refuses one record: nothing is deleted, the record is named
	status, out = deleteJSON(t, app, "/api/tables/Test_DeleteGuard/delete-many", `{"keys":["C03","C04","C05"]}`)
	msg, _ := out["error"].(string)
	if status != 400 || !strings.Contains(msg, "C04") || !strings.Contains(msg, "customers named Keep stay") || remaining() != "[C03 C04 C05 C06]" {
		t.Errorf("refused = %d %q, remaining %s", status, msg, remaining())
	}

	// Every record the list shows (filter + search as /list)
	filters := url.QueryEscape(`[{"field":"name","expression":"*i*"}]`)
	status, out = deleteJSON(t, app, "/api/tables/Customer/delete-many?filters="+filters, `{"all":true}`)
	if status != 200 || out["data"].(map[string]interface{})["deleted"] != 2.0 || remaining() != "[C03 C04]" {
		t.Errorf("delete all *i* = %d %v, remaining %s (want Dina and Erik gone)", status, out, remaining())
	}
	fields := url.QueryEscape(`["name"]`)
	status, _ = deleteJSON(t, app, "/api/tables/Customer/delete-many?search=car&search_fields="+fields, `{"all":true}`)
	if status != 200 || remaining() != "[C04]" {
		t.Errorf("delete all search car = %d, remaining %s", status, remaining())
	}

	// Bad requests
	if status, _ := deleteJSON(t, app, "/api/tables/Customer/delete-many", `not json`); status != 400 {
		t.Errorf("bad body = %d, want 400", status)
	}
	if status, _ := deleteJSON(t, app, "/api/tables/Customer/delete-many?filters="+url.QueryEscape(`[{"field":"nope","expression":"x"}]`), `{"all":true}`); status != 400 || remaining() != "[C04]" {
		t.Errorf("bad filter = %d, remaining %s", status, remaining())
	}
}

// More than bulkDeleteTypeCount records at once: only with their number typed (confirm_count),
// and only while the list still has that many; every bulk delete is logged with the user, the
// company, the table, the count, the list's filters and the keys
func TestDeleteRecordsConfirmCountAndLog(t *testing.T) {
	app := newTablesTestApp(t)
	n := bulkDeleteTypeCount + 2
	for i := 1; i <= n; i++ {
		postJSON(t, app, "/api/tables/Customer/insert", fmt.Sprintf(`{"no":"K%02d","name":"Bulk %d"}`, i, i))
	}
	count := func() int {
		t.Helper()
		_, out := getJSON(t, app, "/api/tables/Customer/list")
		nos, _ := listNos(t, out)
		return len(nos)
	}
	filters := "?filters=" + url.QueryEscape(`[{"field":"name","expression":"Bulk*"}]`)
	if status, out := deleteJSON(t, app, "/api/tables/Customer/delete-many"+filters, `{"all":true}`); status != 400 || count() != n {
		t.Fatalf("without the count: %d %v, %d left", status, out["error"], count())
	}
	if status, out := deleteJSON(t, app, "/api/tables/Customer/delete-many"+filters, fmt.Sprintf(`{"all":true,"confirm_count":%d}`, n-1)); status != 409 || !strings.Contains(fmt.Sprint(out["error"]), fmt.Sprint(n)) || count() != n {
		t.Fatalf("another count: %d %v, %d left", status, out["error"], count())
	}
	// a small selection needs no count
	if status, out := deleteJSON(t, app, "/api/tables/Customer/delete-many", `{"keys":["K01","K02"]}`); status != 200 {
		t.Fatalf("two keys: %d %v", status, out["error"])
	}
	if status, out := deleteJSON(t, app, "/api/tables/Customer/delete-many"+filters, fmt.Sprintf(`{"all":true,"confirm_count":%d}`, n-2)); status != 200 || count() != 0 {
		t.Fatalf("the count typed: %d %v, %d left", status, out["error"], count())
	}

	_, out := getJSON(t, app, "/api/tables/Bulk_Delete_Log/list")
	records, _ := out["data"].(map[string]interface{})["records"].([]interface{})
	if len(records) != 2 {
		t.Fatalf("log: %d entries, want 2 (%v)", len(records), out)
	}
	last := records[1].(map[string]interface{})
	if last["no_of_records"] != float64(n-2) || last["table_name"] != "Customer" || last["company"] != "TEST" || !strings.Contains(fmt.Sprint(last["filters"]), "Bulk*") {
		t.Errorf("log entry %v", last)
	}
}
