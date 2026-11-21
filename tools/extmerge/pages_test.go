package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const appID = "11111111-1111-4111-8111-111111111111"

// A card with keys the merge tool knows nothing about (a subpage, a drilldown, a menu button)
const coreCard = `page:
  id: 21
  type: Card
  name: Customer Card
  source_table: Customer
  layout:
    sections:
      - name: General
        caption: General
        fields:
          - source: no
          - source: name
          - source: credit_limit_lcy
            drilldown: 25
            drilldown_flow_filters: true
          - source: blocked
      - name: Lines
        subpage: {page_id: 95, link: {document_no: no}}
      - name: Payments
        fields:
          - source: payment_terms_code
  actions:
    - name: Back to List
      shortcut: Esc
    - name: Customer
      items:
        - name: Ledger Entries
          run_page: 25
        - name: Functions
          items:
            - name: Copy
              run_object: "report:1"
    - name: Print
      image: Print
`

const coreList = `page:
  id: 22
  type: List
  name: Customer List
  source_table: Customer
  card_page_id: 21
  layout:
    repeater:
      fields:
        - source: no
          width: 120
        - source: name
          width: 300
        - source: balance_lcy
          drilldown_flow_filters: true
  actions:
    - name: New
`

// mergePages merges page extension files into the card 21 and list 22 and returns the merged
// page definitions (id → page)
func mergePages(t *testing.T, files map[string]string) (map[int]map[string]interface{}, error) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "core", "application.yaml"), baseApp)
	writeFile(t, filepath.Join(dir, "core", "tables", "customer.yaml"), coreCustomer)
	writeFile(t, filepath.Join(dir, "core", "pages", "21-customer-card.yaml"), coreCard)
	writeFile(t, filepath.Join(dir, "core", "pages", "22-customer-list.yaml"), coreList)
	writeFile(t, filepath.Join(dir, "ext", "application.yaml"), extApp)
	for name, content := range files {
		writeFile(t, filepath.Join(dir, "ext", name), content)
	}
	out := filepath.Join(dir, "out")
	if err := run(filepath.Join(dir, "core"), filepath.Join(dir, "ext"), out, false); err != nil {
		return nil, err
	}
	pages := map[int]map[string]interface{}{}
	for id, file := range map[int]string{21: "21-customer-card.yaml", 22: "22-customer-list.yaml"} {
		data, err := os.ReadFile(filepath.Join(out, "pages", file))
		if err != nil {
			t.Fatal(err)
		}
		var def map[string]interface{}
		if err := yaml.Unmarshal(data, &def); err != nil {
			t.Fatal(err)
		}
		pages[id] = def["page"].(map[string]interface{})
	}
	return pages, nil
}

func list(m map[string]interface{}, path ...string) []interface{} {
	var cur interface{} = m
	for _, p := range path {
		cur = cur.(map[string]interface{})[p]
	}
	l, _ := cur.([]interface{})
	return l
}

func keys(entries []interface{}, keyField string) string {
	var out []string
	for _, e := range entries {
		out = append(out, e.(map[string]interface{})[keyField].(string))
	}
	return strings.Join(out, ",")
}

func entry(entries []interface{}, keyField, key string) map[string]interface{} {
	for _, e := range entries {
		m := e.(map[string]interface{})
		if m[keyField] == key {
			return m
		}
	}
	return nil
}

// An extension adds card fields and sections, list columns, buttons and menu items where it
// places them; the base definition keeps every key; what was added carries the app
func TestPageExtension(t *testing.T) {
	pages, err := mergePages(t, map[string]string{
		"pages/21-customer-card.extend.yaml": `extends: 21
layout:
  sections:
    - name: General
      fields:
        - source: lp_loyalty_points
          after: credit_limit_lcy
        - source: lp_tier
          before: no
    - name: Loyalty
      caption: Loyalty
      after: General
      fields:
        - source: lp_joined
actions:
  - name: Loyalty Tiers
    after: Customer
    run_page: 50000
  - name: Recalculate Points
    in: Functions
    before: Copy
    run_object: "codeunit:50000"
`,
		"pages/22-customer-list.extend.yaml": `extends: 22
layout:
  repeater:
    fields:
      - source: lp_loyalty_points
        after: name
        width: 100
actions:
  - name: Loyalty Tiers
    run_page: 50000
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	card, cardList := pages[21], pages[22]

	sections := list(card, "layout", "sections")
	if got := keys(sections, "name"); got != "General,Loyalty,Lines,Payments" {
		t.Errorf("sections %s", got)
	}
	general := entry(sections, "name", "General")
	if got := keys(general["fields"].([]interface{}), "source"); got != "lp_tier,no,name,credit_limit_lcy,lp_loyalty_points,blocked" {
		t.Errorf("General fields %s", got)
	}
	if general["caption"] != "General" || general["app"] != nil {
		t.Errorf("the existing section changed: %v", general)
	}
	added := entry(general["fields"].([]interface{}), "source", "lp_loyalty_points")
	if added["app"] != appID || added["after"] != nil {
		t.Errorf("added field %v", added)
	}
	if f := entry(general["fields"].([]interface{}), "source", "credit_limit_lcy"); f["drilldown"] != 25 || f["drilldown_flow_filters"] != true || f["app"] != nil {
		t.Errorf("a base field lost keys or got the app: %v", f)
	}
	loyalty := entry(sections, "name", "Loyalty")
	if loyalty["app"] != appID || entry(loyalty["fields"].([]interface{}), "source", "lp_joined")["app"] != appID {
		t.Errorf("new section not marked: %v", loyalty)
	}
	if lines := entry(sections, "name", "Lines"); lines["subpage"] == nil {
		t.Error("the subpage was lost")
	}

	actions := list(card, "actions")
	if got := keys(actions, "name"); got != "Back to List,Customer,Loyalty Tiers,Print" {
		t.Errorf("actions %s", got)
	}
	if print := entry(actions, "name", "Print"); print["image"] != "Print" {
		t.Error("an action lost its image")
	}
	functions := entry(list(entry(actions, "name", "Customer"), "items"), "name", "Functions")
	if got := keys(functions["items"].([]interface{}), "name"); got != "Recalculate Points,Copy" {
		t.Errorf("Functions items %s", got)
	}
	if item := entry(functions["items"].([]interface{}), "name", "Recalculate Points"); item["app"] != appID || item["in"] != nil {
		t.Errorf("menu item %v", item)
	}

	columns := list(cardList, "layout", "repeater", "fields")
	if got := keys(columns, "source"); got != "no,name,lp_loyalty_points,balance_lcy" {
		t.Errorf("list columns %s", got)
	}
	if c := entry(columns, "source", "balance_lcy"); c["drilldown_flow_filters"] != true {
		t.Error("a list column lost its keys")
	}
	if got := keys(list(cardList, "actions"), "name"); got != "New,Loyalty Tiers" {
		t.Errorf("list actions %s", got)
	}
	if cardList["card_page_id"] != 21 {
		t.Error("a page key was lost")
	}
}

func TestPageExtensionRefusals(t *testing.T) {
	cases := map[string]struct{ file, ext, want string }{
		"replacing a field":   {"21", "extends: 21\nlayout:\n  sections:\n    - name: General\n      fields:\n        - source: name\n", "already there"},
		"replacing an action": {"21", "extends: 21\nactions:\n  - name: Ledger Entries\n", "already on the page"},
		"unknown anchor":      {"21", "extends: 21\nlayout:\n  sections:\n    - name: General\n      fields:\n        - source: lp_x\n          after: nothing\n", `no field "nothing"`},
		"after and before":    {"21", "extends: 21\nactions:\n  - name: X\n    after: Print\n    before: Print\n", "not both"},
		"unknown menu":        {"21", "extends: 21\nactions:\n  - name: X\n    in: Nothing\n", `no menu button "Nothing"`},
		"in a button":         {"21", "extends: 21\nactions:\n  - name: X\n    in: Print\n", "not a menu button"},
		"changing a section":  {"21", "extends: 21\nlayout:\n  sections:\n    - name: General\n      caption: Mine\n", `unknown key "caption"`},
		"unknown key":         {"21", "extends: 21\nlayot: {}\n", `unknown key "layot"`},
		"columns on a card":   {"21", "extends: 21\nlayout:\n  repeater:\n    fields:\n      - source: lp_x\n", "has no list"},
		"sections on a list":  {"22", "extends: 22\nlayout:\n  sections:\n    - name: General\n      fields:\n        - source: lp_x\n", "is a list"},
	}
	for name, c := range cases {
		file := "pages/" + c.file + "-customer.extend.yaml"
		if _, err := mergePages(t, map[string]string{file: c.ext}); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}

const coreMenu = `menu:
  - name: Sales
    items:
      - name: Customers
        page_id: 22
      - name: Reports
        items:
          - name: Customer - List
            run_object: "report:101"
  - name: Administration
    items:
      - name: Users
        page_id: 59
`

func mergeMenu(t *testing.T, ext string) (map[string]interface{}, error) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "core", "application.yaml"), baseApp)
	writeFile(t, filepath.Join(dir, "core", "menus", "admin.yaml"), coreMenu)
	writeFile(t, filepath.Join(dir, "ext", "application.yaml"), extApp)
	writeFile(t, filepath.Join(dir, "ext", "menus", "admin.extend.yaml"), ext)
	out := filepath.Join(dir, "out")
	if err := run(filepath.Join(dir, "core"), filepath.Join(dir, "ext"), out, false); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(out, "menus", "admin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var def map[string]interface{}
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatal(err)
	}
	return def, nil
}

// A menu extension adds entries into a sub-menu (by its path) or new modules, marked with the app
func TestMenuExtension(t *testing.T) {
	def, err := mergeMenu(t, `extends: admin.yaml
items:
  - in: [Sales, Reports]
    before: Customer - List
    name: Loyalty Points
    run_object: "report:50000"
  - in: Sales
    after: Customers
    name: Loyalty Tiers
    page_id: 50000
  - name: Loyalty
    before: Administration
    items:
      - name: Tiers
        page_id: 50000
`)
	if err != nil {
		t.Fatal(err)
	}
	menu := def["menu"].([]interface{})
	if got := keys(menu, "name"); got != "Sales,Loyalty,Administration" {
		t.Errorf("modules %s", got)
	}
	sales := entry(menu, "name", "Sales")["items"].([]interface{})
	if got := keys(sales, "name"); got != "Customers,Loyalty Tiers,Reports" {
		t.Errorf("Sales %s", got)
	}
	reports := entry(sales, "name", "Reports")["items"].([]interface{})
	if got := keys(reports, "name"); got != "Loyalty Points,Customer - List" {
		t.Errorf("Sales/Reports %s", got)
	}
	module := entry(menu, "name", "Loyalty")
	if module["app"] != appID || entry(module["items"].([]interface{}), "name", "Tiers")["app"] != appID {
		t.Errorf("new module not marked: %v", module)
	}
	if entry(sales, "name", "Customers")["app"] != nil {
		t.Error("a base entry got the app")
	}

	for name, c := range map[string]struct{ ext, want string }{
		"no such menu": {"extends: admin.yaml\nitems:\n  - in: Purchases\n    name: X\n", `no menu "Purchases"`},
		"not a menu":   {"extends: admin.yaml\nitems:\n  - in: [Sales, Customers]\n    name: X\n", "not a sub-menu"},
		"replacing":    {"extends: admin.yaml\nitems:\n  - in: Sales\n    name: Customers\n", "already there"},
		"no menu file": {"extends: other.yaml\nitems: []\n", "other.yaml"},
		"a path":       {"extends: ../admin.yaml\nitems: []\n", "the menu file"},
	} {
		if _, err := mergeMenu(t, c.ext); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}

// Every page of the base application keeps every key through a merge: an extension adds an
// action and a field (a card's first section, a list's columns); after a round trip through the
// written YAML, with those two taken out again, the page is exactly the base definition
// (a merge through a partial struct used to drop the keys it did not know)
func TestEveryPageKeepsItsKeys(t *testing.T) {
	dir := filepath.Join("..", "..", "backend", "definitions", "pages")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".extend.yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		var original, def map[string]interface{}
		if err := yaml.Unmarshal(data, &original); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := yaml.Unmarshal(data, &def); err != nil {
			t.Fatal(err)
		}
		page, _ := def["page"].(map[string]interface{})
		if page == nil {
			t.Fatalf("%s: no page section", name)
		}
		layout, _ := page["layout"].(map[string]interface{})
		ext := map[string]interface{}{
			"extends": page["id"],
			"actions": []interface{}{map[string]interface{}{"name": "Zz Merge Check", "run_page": 50000}},
		}
		field := map[string]interface{}{"source": "zz_merge_check"}
		sections, _ := layout["sections"].([]interface{})
		switch {
		case layout["repeater"] != nil:
			ext["layout"] = map[string]interface{}{"repeater": map[string]interface{}{"fields": []interface{}{field}}}
		case len(sections) > 0:
			first, _ := sections[0].(map[string]interface{})
			ext["layout"] = map[string]interface{}{"sections": []interface{}{
				map[string]interface{}{"name": first["name"], "fields": []interface{}{field}},
			}}
		}
		if err := mergePage(page, ext, "zz-app"); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		out, err := yaml.Marshal(def)
		if err != nil {
			t.Fatal(err)
		}
		var merged map[string]interface{}
		if err := yaml.Unmarshal(out, &merged); err != nil {
			t.Fatal(err)
		}
		// take the two additions out again
		mp := merged["page"].(map[string]interface{})
		mp["actions"] = withoutItem(t, name, mp["actions"], "name", "Zz Merge Check")
		if _, had := original["page"].(map[string]interface{})["actions"]; !had && len(mp["actions"].([]interface{})) == 0 {
			delete(mp, "actions")
		}
		if ml, _ := mp["layout"].(map[string]interface{}); ml != nil {
			var target map[string]interface{}
			if r, _ := ml["repeater"].(map[string]interface{}); r != nil {
				target = r
			} else if s, _ := ml["sections"].([]interface{}); len(s) > 0 {
				target = s[0].(map[string]interface{})
			}
			if target != nil {
				_, had := target["fields"]
				had = had && !addedFieldsKey(original, target["name"])
				target["fields"] = withoutItem(t, name, target["fields"], "source", "zz_merge_check")
				if !had && len(target["fields"].([]interface{})) == 0 {
					delete(target, "fields") // a section without fields (a period grid): none added
				}
			}
		}
		if !reflect.DeepEqual(original, merged) {
			want, _ := yaml.Marshal(original)
			t.Errorf("%s: the merge changed the page\n--- base\n%s\n--- merged\n%s", name, want, out)
		}
		checked++
	}
	if checked < 100 {
		t.Fatalf("only %d pages checked", checked)
	}
}

// withoutItem: the list without the one entry whose key is value (it must be there once)
func withoutItem(t *testing.T, page string, list interface{}, key, value string) []interface{} {
	t.Helper()
	items, _ := list.([]interface{})
	out := make([]interface{}, 0, len(items))
	found := 0
	for _, item := range items {
		if m, _ := item.(map[string]interface{}); m != nil && m[key] == value {
			found++
			continue
		}
		out = append(out, item)
	}
	if found != 1 {
		t.Errorf("%s: %s %q added %d times", page, key, value, found)
	}
	return out
}

// addedFieldsKey: the base page's list or first section had no fields list (the merge made one)
func addedFieldsKey(original map[string]interface{}, sectionName interface{}) bool {
	layout, _ := original["page"].(map[string]interface{})["layout"].(map[string]interface{})
	if r, _ := layout["repeater"].(map[string]interface{}); r != nil {
		_, has := r["fields"]
		return !has
	}
	sections, _ := layout["sections"].([]interface{})
	for _, s := range sections {
		if m, _ := s.(map[string]interface{}); m != nil && m["name"] == sectionName {
			_, has := m["fields"]
			return !has
		}
	}
	return false
}
