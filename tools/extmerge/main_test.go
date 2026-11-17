package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const coreCustomer = `table:
  id: 18
  name: "Customer"
  fields:
    - name: no
      no: 1
      type: types.Code
      db_name: no
      primary_key: true
      length: 20
    - name: name
      no: 2
      type: types.Text
      length: 100
`

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

const baseApp = `application:
  id: 760e20ec-7a66-4e3f-b6e5-83c94db5a4bd
  name: Base Application
  number_series:
    objects: [{from: 1, to: 999}]
    fields: [{from: 1, to: 999}]
`

const extApp = `application:
  id: 11111111-1111-4111-8111-111111111111
  name: Loyalty
  prefix: lp
  number_series:
    objects: [{from: 50000, to: 50999}]
    fields: [{from: 50000, to: 50999}]
  dependencies:
    - id: 760e20ec-7a66-4e3f-b6e5-83c94db5a4bd
`

func mergeCustomer(t *testing.T, extension string) (map[string]interface{}, error) {
	t.Helper()
	return mergeWith(t, extApp, map[string]string{"tables/customer.extend.yaml": extension})
}

// mergeWith merges extension files (path below the extension directory → content) with an
// extension application.yaml into the core Customer table and returns the merged Customer
func mergeWith(t *testing.T, app string, files map[string]string) (map[string]interface{}, error) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "core", "application.yaml"), baseApp)
	writeFile(t, filepath.Join(dir, "core", "tables", "customer.yaml"), coreCustomer)
	writeFile(t, filepath.Join(dir, "core", "pages", "21-customer-card.yaml"), "page:\n  id: 21\n  type: Card\n")
	writeFile(t, filepath.Join(dir, "ext", "application.yaml"), app)
	for name, content := range files {
		writeFile(t, filepath.Join(dir, "ext", name), content)
	}
	out := filepath.Join(dir, "out")
	if err := run(filepath.Join(dir, "core"), filepath.Join(dir, "ext"), out, false); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(out, "tables", "customer.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var def map[string]interface{}
	if err := yaml.Unmarshal(data, &def); err != nil {
		t.Fatal(err)
	}
	return def, nil
}

// An extension field is added with every key it has (not only name/type/length/caption).
func TestTableExtensionKeepsFieldKeys(t *testing.T) {
	def, err := mergeCustomer(t, `extends_table: 18
fields:
  - name: lp_loyalty_points
    no: 50000
    type: int
    db_name: lp_loyalty_points
    default: 100
    validation: {min: 0, max: 10000}
`)
	if err != nil {
		t.Fatal(err)
	}
	fields := def["table"].(map[string]interface{})["fields"].([]interface{})
	if len(fields) != 3 {
		t.Fatalf("%d fields, want 3", len(fields))
	}
	added := fields[2].(map[string]interface{})
	if added["no"] != 50000 || added["default"] != 100 || added["validation"] == nil || added["db_name"] != "lp_loyalty_points" ||
		added["app"] != "11111111-1111-4111-8111-111111111111" {
		t.Errorf("added field %v", added)
	}
}

func TestTableExtensionFieldRules(t *testing.T) {
	cases := map[string]struct{ ext, want string }{
		"no number": {`extends_table: 18
fields:
  - name: lp_loyalty_points
    type: int
`, "needs a field number"},
		"outside the extension's series": {`extends_table: 18
fields:
  - name: lp_loyalty_points
    no: 999
    type: int
`, "needs a field number"},
		"number used by the table": {`extends_table: 18
fields:
  - name: lp_loyalty_points
    no: 50000
    type: int
  - name: lp_tier
    no: 50000
    type: types.Code
`, "number 50000 is already used by field lp_loyalty_points"},
		"name used by the table": {`extends_table: 18
fields:
  - name: lp_x
    db_name: Name
    no: 50001
    type: types.Text
`, "already has a field named Name"},
		"without the prefix": {`extends_table: 18
fields:
  - name: loyalty_points
    no: 50001
    type: int
`, "must start with the app's prefix"},
	}
	for name, c := range cases {
		if _, err := mergeCustomer(t, c.ext); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}

func TestExtensionApplication(t *testing.T) {
	field := "tables/customer.extend.yaml"
	ext := "extends_table: 18\nfields:\n  - name: lp_tier\n    no: 50001\n    type: int\n"
	if _, err := mergeWith(t, strings.Replace(extApp, "id: 11111111-1111-4111-8111-111111111111\n", "", 1), map[string]string{field: ext}); err == nil ||
		!strings.Contains(err.Error(), "needs an application.yaml") {
		t.Errorf("without an id: %v", err)
	}
	overlapping := strings.Replace(extApp, "objects: [{from: 50000, to: 50999}]", "objects: [{from: 900, to: 1999}]", 1)
	if _, err := mergeWith(t, overlapping, map[string]string{field: ext}); err == nil || !strings.Contains(err.Error(), "overlaps") {
		t.Errorf("overlapping series: %v", err)
	}
	newTable := "table:\n  id: 50000\n  name: LP_Loyalty_Tier\n  fields:\n    - name: code\n      no: 50000\n      type: types.Code\n"
	if _, err := mergeWith(t, extApp, map[string]string{"custom/tables/loyalty_tier.yaml": newTable}); err != nil {
		t.Errorf("new table in the series: %v", err)
	}
	if _, err := mergeWith(t, extApp, map[string]string{"custom/tables/loyalty_tier.yaml": strings.Replace(newTable, "id: 50000", "id: 51000", 1)}); err == nil {
		t.Error("new table outside the series accepted")
	}
	if _, err := mergeWith(t, extApp, map[string]string{"custom/pages/50000-tiers.yaml": "page:\n  id: 18\n  name: LP Tiers\n"}); err == nil {
		t.Error("new page outside the series accepted")
	}
	if _, err := mergeWith(t, extApp, map[string]string{"custom/pages/50000-tiers.yaml": "page:\n  id: 50000\n  name: Tiers\n"}); err == nil ||
		!strings.Contains(err.Error(), "prefix") {
		t.Errorf("new page without the prefix: %v", err)
	}
	if _, err := mergeWith(t, extApp, map[string]string{"custom/tables/t.yaml": strings.Replace(newTable, "LP_Loyalty_Tier", "Loyalty_Tier", 1)}); err == nil ||
		!strings.Contains(err.Error(), "prefix") {
		t.Errorf("new table without the prefix: %v", err)
	}
}
