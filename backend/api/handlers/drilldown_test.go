package handlers

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// The Chart of Accounts' Net Change / Balance at Date drill down with the FlowField's filters:
// a posting account its own No., a total account its Totaling, and the list's Date Filter
// (Balance at Date up to its end)
func TestFlowFieldDrilldown(t *testing.T) {
	if err := pages.GetRegistry().LoadPages(); err != nil {
		t.Skipf("pages: %v", err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/pages/:id", NewPagesHandler().GetPage)
	app.Post("/api/pages/:id/drilldown", PageDrilldown)

	// the page marks the FlowFields whose drilldown takes the FlowField's filters
	_, out := getJSON(t, app, "/api/pages/16")
	raw, _ := json.Marshal(out["data"])
	var def struct {
		Page pages.PageMetadata `json:"page"`
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		t.Fatal(err)
	}
	marked := map[string]bool{}
	for _, f := range def.Page.Layout.Repeater.Fields {
		marked[f.Source] = f.DrilldownFlowFilters
	}
	if !marked["net_change"] || !marked["balance_at_date"] || marked["no"] {
		t.Errorf("drilldown_flow_filters: %v", marked)
	}

	filtersOf := func(pageID, target int, body string) map[string]string {
		t.Helper()
		out := postJSON(t, app, fmt.Sprintf("/api/pages/%d/drilldown", pageID), body)
		data, _ := out["data"].(map[string]interface{})
		if data == nil || data["page_id"] != float64(target) {
			t.Fatalf("drilldown: %v", out)
		}
		got := map[string]string{}
		list, _ := data["filters"].([]interface{})
		for _, f := range list {
			m := f.(map[string]interface{})
			got[m["field"].(string)] = m["expression"].(string)
		}
		return got
	}
	filters := func(body string) map[string]string {
		t.Helper()
		return filtersOf(16, 20, body)
	}
	got := filters(`{"field":"net_change","record":{"no":"6100","totaling":""}}`)
	if len(got) != 1 || got["gl_account_no"] != "6100" {
		t.Errorf("posting account: %v", got)
	}
	got = filters(`{"field":"net_change","record":{"no":"6999","totaling":"6000..6990"},
		"flow_filters":[{"field":"date_filter","expression":"2026-01-01..2026-03-31"},{"field":"dimension_1_filter","expression":"SALES"}]}`)
	if got["gl_account_no"] != "6000..6990" || got["posting_date"] != "2026-01-01..2026-03-31" || got["dimension_1_code"] != "SALES" || len(got) != 3 {
		t.Errorf("total account: %v", got)
	}
	got = filters(`{"field":"balance_at_date","record":{"no":"6999","totaling":"6000..6990"},
		"flow_filters":[{"field":"date_filter","expression":"2026-01-01..2026-03-31"}]}`)
	if got["gl_account_no"] != "6000..6990" || got["posting_date"] != "..2026-03-31" {
		t.Errorf("balance at date: %v", got)
	}
	// Balance Due (LCY) on the Customer List: the open entries, the Date Filter on the Due Date
	got = filtersOf(22, 25, `{"field":"balance_due_lcy","record":{"no":"C00010"},
		"flow_filters":[{"field":"date_filter","expression":"..2026-03-31"}]}`)
	if got["customer_no"] != "C00010" || got["open"] != "true" || got["due_date"] != "..2026-03-31" || len(got) != 3 {
		t.Errorf("balance due: %v", got)
	}
	// a fixed asset's Depreciation: its book and the posting type and category by number
	got = filtersOf(5602, 5604, `{"field":"depreciation","record":{"fa_no":"FA0010","depreciation_book_code":"COMPANY"}}`)
	if got["fa_no"] != "FA0010" || got["depreciation_book_code"] != "COMPANY" || got["fa_posting_type"] != "1" || got["fa_posting_category"] != "0" || len(got) != 4 {
		t.Errorf("FA depreciation: %v", got)
	}
	// a field without a drilldown is refused
	out = postJSON(t, app, "/api/pages/16/drilldown", `{"field":"name","record":{}}`)
	if out["success"] == true {
		t.Errorf("no drilldown accepted: %v", out)
	}
}
