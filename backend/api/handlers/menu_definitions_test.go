package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"

	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
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
	inventory := find(modules, "Lager")
	inventorySetup := find(inventory["items"].([]interface{}), "Oppsett")
	if reservation := find(inventorySetup["items"].([]interface{}), "Reservasjonsoppsett"); reservation["enabled"] != false {
		t.Errorf("Inventory › Setup › Reservation Setup (no page yet) enabled: %v", reservation)
	}
	jobs := find(modules, "Jobber")
	if jobsSetup := find(jobs["items"].([]interface{}), "Oppsett"); jobsSetup["enabled"] != false {
		t.Errorf("a sub-menu without anything built is enabled: %v", jobsSetup)
	}
}
