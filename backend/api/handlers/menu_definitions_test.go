package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
)

// Every menu file: each entry is a separator, or has a name and opens a page, runs a report or
// codeunit ("report:93") or holds a sub-menu (one of them, or none for an entry still to be
// built); every name has a caption in each language
func TestMenuDefinitions(t *testing.T) {
	files, err := filepath.Glob("../../definitions/menus/*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("menu files: %v", err)
	}
	ts := i18n.GetInstance()
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var def pages.MenuDefinition
		if err := yaml.Unmarshal(data, &def); err != nil {
			t.Errorf("%s: %v", filepath.Base(file), err)
			continue
		}
		var check func(path string, items []pages.MenuItem)
		check = func(path string, items []pages.MenuItem) {
			for _, item := range items {
				if item.Separator {
					if item.Name != "" || item.PageID != 0 || item.RunObject != "" || len(item.Items) > 0 {
						t.Errorf("%s %s: a separator with more", filepath.Base(file), path)
					}
					continue
				}
				where := path + " › " + item.Name
				if item.Name == "" {
					t.Errorf("%s %s: an entry without a name", filepath.Base(file), path)
				}
				kinds := 0
				for _, set := range []bool{item.PageID != 0, item.RunObject != "", len(item.Items) > 0} {
					if set {
						kinds++
					}
				}
				if kinds > 1 {
					t.Errorf("%s %s: opens a page, runs an object or holds a sub-menu — not several", filepath.Base(file), where)
				}
				if item.RunObject != "" && !strings.HasPrefix(item.RunObject, "report:") && !strings.HasPrefix(item.RunObject, "codeunit:") {
					t.Errorf("%s %s: run_object %q is not report:<id> or codeunit:<id>", filepath.Base(file), where, item.RunObject)
				}
				for _, lang := range []string{"en-US", "nb-NO", "da-DK"} {
					key := "common.menu." + i18n.Key(item.Name)
					if ts.Translate(key, lang) == key {
						t.Errorf("%s %s: no caption %s in %s", filepath.Base(file), where, key, lang)
					}
				}
				check(where, item.Items)
			}
		}
		for _, group := range def.Menu {
			for _, lang := range []string{"en-US", "nb-NO", "da-DK"} {
				key := "common.menu." + i18n.Key(group.Name)
				if ts.Translate(key, lang) == key {
					t.Errorf("%s: no caption %s in %s", filepath.Base(file), key, lang)
				}
			}
			check(group.Name, group.Items)
		}
	}
}

// The menu sent to the browser: translated, entries whose page or report exists enabled, those
// still to be built greyed out, a sub-menu enabled when one of its entries is
func TestGetMenuMarksAvailableEntries(t *testing.T) {
	if err := pages.GetRegistry().LoadPages(); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "nb-NO", Menu: "admin"})
		return c.Next()
	})
	app.Get("/api/menu", (&PagesHandler{}).GetMenu)
	status, out := getJSON(t, app, "/api/menu")
	if status != 200 {
		t.Fatalf("menu: %d %v", status, out)
	}
	data, _ := out["data"].(map[string]interface{})
	modules, _ := data["menu"].([]interface{})
	find := func(items []interface{}, name string) map[string]interface{} {
		for _, it := range items {
			m := it.(map[string]interface{})
			if m["name"] == name {
				return m
			}
		}
		t.Fatalf("%q not in %v", name, items)
		return nil
	}
	gl := find(modules, "Finans")
	items := gl["items"].([]interface{})
	if chart := find(items, "Kontoplan"); chart["enabled"] != true {
		t.Errorf("Chart of Accounts not enabled: %v", chart)
	}
	setup := find(items, "Oppsett")
	if checklist := find(setup["items"].([]interface{}), "Sjekkliste for oppsett"); checklist["enabled"] != true {
		t.Errorf("Setup › Setup Checklist not enabled: %v", checklist)
	}
	activities := find(items, "Aktiviteter")
	fiscal := find(activities["items"].([]interface{}), "Regnskapsår")
	if fiscal["enabled"] != true || find(fiscal["items"].([]interface{}), "Opprett år")["enabled"] != true ||
		find(fiscal["items"].([]interface{}), "Avslutt resultatregnskap")["enabled"] != true {
		t.Errorf("Activities › Fiscal Year: %v", fiscal)
	}
	inventory := find(modules, "Varer og lager")
	inventorySetup := find(inventory["items"].([]interface{}), "Oppsett")
	if reservation := find(inventorySetup["items"].([]interface{}), "Reservasjonsoppsett"); reservation["enabled"] != true {
		t.Errorf("Inventory › Setup › Reservation Setup not enabled: %v", reservation)
	}
	projects := find(modules, "Prosjekter")
	if projectsSetup := find(projects["items"].([]interface{}), "Oppsett"); projectsSetup["enabled"] != true {
		t.Errorf("Projects › Setup not enabled: %v", projectsSetup)
	}
	fixedAssets := find(modules, "Anleggsmidler og avskrivninger")
	faActivities := find(fixedAssets["items"].([]interface{}), "Aktiviteter")
	if indexing := find(faActivities["items"].([]interface{}), "Indeksering"); indexing["enabled"] != true {
		t.Errorf("Activities › Index not enabled: %v", indexing)
	}

	// An entry whose object is not built, and a sub-menu holding only such entries, are greyed out
	notBuilt := menuEntries([]pages.MenuItem{{Name: "Index", Items: []pages.MenuItem{{Name: "Fixed Assets", RunObject: "report:99999"}}}},
		i18n.GetInstance(), "en-US", nil)
	if notBuilt[0].Enabled || notBuilt[0].Items[0].Enabled {
		t.Errorf("a sub-menu without anything built is enabled: %+v", notBuilt)
	}
}

// Entries and actions the user may not run are greyed out and marked denied: a page whose table
// the user may not read, a report without Execute, a sub-menu with nothing allowed; New and
// Delete without Insert/Delete. SUPER, and a user without roles, may run everything.
func TestDeniedMenuEntriesAndActions(t *testing.T) {
	clerk := &session.Session{}
	clerk.SetPermissions(true, false, map[string]session.TablePermission{
		"CUSTOMER": {Read: true, Modify: true},
		session.ObjectKey(session.ObjectReport, 101): {Execute: true},
	})
	entries := []pages.MenuItem{
		{Name: "Customers", PageID: 22},
		{Name: "Items", PageID: 31},
		{Name: "Reports", Items: []pages.MenuItem{{Name: "Customer - List", RunObject: "report:101"}, {Name: "Chart of Accounts", RunObject: "report:1"}}},
		{Name: "Activities", Items: []pages.MenuItem{{Name: "Chart of Accounts", RunObject: "report:1"}}},
	}
	got := menuEntries(entries, i18n.GetInstance(), "en-US", clerk)
	if !got[0].Enabled || got[0].Denied || got[1].Enabled || !got[1].Denied ||
		!got[2].Enabled || !got[2].Items[0].Enabled || got[2].Items[1].Enabled || !got[2].Items[1].Denied || got[3].Enabled {
		t.Errorf("clerk's menu %+v", got)
	}
	for _, sess := range []*session.Session{{IsSuper: true, HasRoles: true}, {}} {
		for _, e := range menuEntries(entries, i18n.GetInstance(), "en-US", sess) {
			if !e.Enabled || e.Denied {
				t.Errorf("%+v: %+v greyed out", sess, e)
			}
		}
	}

	actions := []pages.Action{
		{Name: "New"}, {Name: "Delete"}, {Name: "Edit"},
		{Name: "Ledger Entries", RunPage: 25},
		{Name: "Statistics", RunPage: 151},
		{Name: "Print", RunObject: "report:1"},
		{Name: "Functions", Items: []pages.Action{{Name: "Post", RunObject: "codeunit:231"}}},
		{Name: "Customer", Items: []pages.Action{{Name: "Card", RunPage: 21}, {Name: "Run", RunObject: "codeunit:231"}}},
	}
	markDeniedActions(clerk, actions, "Customer")
	denied := map[string]bool{}
	for _, a := range actions {
		denied[a.Name] = a.Denied
		if a.Denied != (a.Enabled != nil && !*a.Enabled) {
			t.Errorf("%s: denied %v, enabled %v", a.Name, a.Denied, a.Enabled)
		}
	}
	want := map[string]bool{"New": true, "Delete": true, "Edit": false, "Ledger Entries": true, "Statistics": false,
		"Print": true, "Functions": true, "Customer": false}
	for name, d := range want {
		if denied[name] != d {
			t.Errorf("%s denied %v, want %v", name, denied[name], d)
		}
	}
	if !actions[7].Items[1].Denied || actions[7].Items[0].Denied {
		t.Errorf("menu items %+v", actions[7].Items)
	}
}
