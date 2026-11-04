package handlers

import (
	"net/url"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Every subpage of a card is a list page, linked by stored fields of both tables
func TestSubpageDefinitions(t *testing.T) {
	if err := pages.GetRegistry().LoadPages(); err != nil {
		t.Fatal(err)
	}
	all := pages.GetRegistry().GetAllPages()
	found := 0
	for id, card := range all {
		for _, sec := range card.Page.Layout.Sections {
			sub := sec.Subpage
			if sub == nil {
				continue
			}
			found++
			list, ok := all[sub.PageID]
			if !ok || list.Page.Type != "List" {
				t.Errorf("page %d section %s: subpage %d is not a list page", id, sec.Name, sub.PageID)
				continue
			}
			if card.Page.Type != "Card" || len(sec.Fields) > 0 || len(sub.Link) == 0 {
				t.Errorf("page %d section %s: a subpage is a card section without fields, with a link", id, sec.Name)
			}
			cardFactory, ok1 := gtables.GetTableFactory(card.Page.SourceTable)
			listFactory, ok2 := gtables.GetTableFactory(list.Page.SourceTable)
			if !ok1 || !ok2 {
				t.Errorf("page %d section %s: tables %s / %s", id, sec.Name, card.Page.SourceTable, list.Page.SourceTable)
				continue
			}
			for subField, cardField := range sub.Link {
				if !listFactory().HasColumn(subField) || !cardFactory().HasColumn(cardField) {
					t.Errorf("page %d section %s: link %s = %s is not on stored fields", id, sec.Name, subField, cardField)
				}
			}
		}
	}
	if found == 0 {
		t.Error("no subpage found (Tax Area 464 has one)")
	}
}

// fixed_filters (a subpage's link) are kept when the user filters the same field
func TestFixedFiltersCannotBeReplaced(t *testing.T) {
	app := newTablesTestApp(t)
	for _, c := range []string{`{"no":"C01","name":"Anna","city":"Oslo"}`, `{"no":"C02","name":"Bjørn","city":"Bergen"}`, `{"no":"C03","name":"Carl","city":"Oslo"}`} {
		postJSON(t, app, "/api/tables/Customer/insert", c)
	}
	list := func(query string) string {
		t.Helper()
		_, out := getJSON(t, app, "/api/tables/Customer/list?"+query)
		nos, _ := listNos(t, out)
		return strings.Join(nos, " ")
	}
	fixed := "fixed_filters=" + url.QueryEscape(`[{"field":"city","value":"Oslo"}]`)
	if got := list(fixed); got != "C01 C03" {
		t.Errorf("fixed city Oslo: %s", got)
	}
	if got := list(fixed + "&filters=" + url.QueryEscape(`[{"field":"city","expression":"Bergen"}]`)); got != "" {
		t.Errorf("the user's filter on the same field replaced the fixed one: %q", got)
	}
	if got := list(fixed + "&filters=" + url.QueryEscape(`[{"field":"name","expression":"C*"}]`)); got != "C03" {
		t.Errorf("fixed and the user's filter: %s", got)
	}
}
