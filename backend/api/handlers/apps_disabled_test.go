package handlers

import (
	"testing"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/extensions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
)

// The parts a disabled extension app added are left out of the page (a copy: the registry's
// definition keeps them), its menu entries are greyed out and its objects are refused
func TestDisabledAppParts(t *testing.T) {
	const app = "aaaaaaaa-0000-4000-8000-000000000009"
	extensions.SetObjectApps(func(objectType string, id int) string {
		if id >= 50000 && id <= 50099 {
			return app
		}
		return ""
	})
	extensions.SetDisabled(app)
	t.Cleanup(func() { extensions.SetDisabled(); extensions.SetObjectApps(nil) })

	shared := pages.PageMetadata{
		Layout: pages.Layout{
			Sections: []pages.Section{
				{Name: "General", Fields: []pages.Field{{Source: "no"}, {Source: "lp_points", App: app}}},
				{Name: "Loyalty", App: app, Fields: []pages.Field{{Source: "lp_tier", App: app}}},
			},
			Repeater: &pages.Repeater{Fields: []pages.Field{{Source: "no"}, {Source: "lp_points", App: app}}},
		},
		Actions: []pages.Action{
			{Name: "Customer", Items: []pages.Action{{Name: "Ledger Entries"}, {Name: "Tiers", App: app}}},
			{Name: "Loyalty Tiers", App: app},
		},
	}
	page := shared
	withoutDisabledApps(&page)
	if len(page.Layout.Sections) != 1 || len(page.Layout.Sections[0].Fields) != 1 ||
		len(page.Layout.Repeater.Fields) != 1 || len(page.Actions) != 1 || len(page.Actions[0].Items) != 1 {
		t.Errorf("disabled parts left: %+v", page)
	}
	if len(shared.Layout.Sections[0].Fields) != 2 || len(shared.Layout.Repeater.Fields) != 2 || len(shared.Actions[0].Items) != 2 {
		t.Error("the shared definition was changed")
	}

	for name, c := range map[string]struct {
		item pages.MenuItem
		want bool
	}{
		"base entry":       {pages.MenuItem{Name: "Customers", PageID: 22}, true},
		"added by the app": {pages.MenuItem{Name: "Tiers", PageID: 22, App: app}, false},
		"the app's page":   {pages.MenuItem{Name: "Tiers", PageID: 50000}, false},
		"the app's report": {pages.MenuItem{Name: "Points", RunObject: "report:50001"}, false},
		"a base report":    {pages.MenuItem{Name: "List", RunObject: "report:101"}, true},
	} {
		if got := menuEntryEnabled(c.item); got != c.want {
			t.Errorf("%s: enabled %v", name, got)
		}
	}
	if runObjectEnabled(0, 50001, "") || runObjectEnabled(50002, 0, "") || !runObjectEnabled(0, 101, "") {
		t.Error("runObjectEnabled by number")
	}
	if !extensions.ObjectEnabled(fapps.Page, 21) {
		t.Error("a base page refused")
	}
}
