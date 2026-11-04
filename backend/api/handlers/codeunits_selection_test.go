package handlers

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// selectionCodeunit returns the customer numbers of rec.SetSelectionFilter
type selectionCodeunit struct{}

func (selectionCodeunit) ID() int             { return 99990 }
func (selectionCodeunit) Name() string        { return "selection-test" }
func (selectionCodeunit) SourceTable() string { return "Customer" }
func (selectionCodeunit) UsesProgress() bool  { return false }
func (selectionCodeunit) Run(record interface{}) (fcodeunits.Result, error) {
	rec := record.(*gtables.Customer)
	rec.SetSelectionFilter()
	var nos []string
	for ok := rec.FindSet(); ok; ok = rec.Next() {
		nos = append(nos, string(rec.No))
	}
	return fcodeunits.Result{Success: true, Message: strings.Join(nos, ",")}, nil
}

// A codeunit run from a list gets the list's selection: keys, or all records of the list's query
func TestCodeunitRunsOnListSelection(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.Customer{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"Anna", "Bjørn", "Carl", "Dora"} {
		c := &gtables.Customer{}
		c.InitWithDBType(db, "TEST", database.DBTypeSQLite)
		c.No, c.Name = types.NewCode(fmt.Sprintf("C%02d", i+1)), types.NewText(name)
		if !c.Insert(false) {
			t.Fatalf("insert %s", name)
		}
	}
	codeunits.Register(99990, "selection-test", func(database.Executor, string, database.DBType) fcodeunits.Codeunit {
		return selectionCodeunit{}
	})

	h := NewCodeunitsHandlerWithDBType(db, database.DBTypeSQLite)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Post("/api/codeunits/run", h.RunCodeunit)

	run := func(url, body string) string {
		t.Helper()
		out := postJSON(t, app, url, body)
		data, _ := out["data"].(map[string]interface{})
		if data == nil {
			t.Fatalf("%s %s: %v", url, body, out)
		}
		return fmt.Sprint(data["message"])
	}
	if got := run("/api/codeunits/run", `{"codeunit_id":99990,"record":{"no":"C02"}}`); got != "C02" {
		t.Errorf("no selection: the current record, got %q", got)
	}
	if got := run("/api/codeunits/run", `{"codeunit_id":99990,"record":{"no":"C02"},"selection":{"keys":["C01","C03","C04"]}}`); got != "C01,C03,C04" {
		t.Errorf("selected keys: %q", got)
	}
	filters := url.QueryEscape(`[{"field":"name","expression":"*r*"}]`)
	if got := run("/api/codeunits/run?filters="+filters, `{"codeunit_id":99990,"record":{"no":"C01"},"selection":{"all":true}}`); got != "C02,C03,C04" {
		t.Errorf("all of a filtered list: %q", got)
	}
}
