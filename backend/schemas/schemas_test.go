package schemas

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// root: the repository's root, from this package
const root = "../.."

// The files of the repository and of the test app, by the schema they follow
var definitionFiles = map[string][]string{
	"application.schema.json":     {"backend/definitions/application.yaml", "testdata/apps/*/application.yaml"},
	"table.schema.json":           {"backend/definitions/tables/*.yaml", "testdata/apps/*/custom/tables/*.yaml"},
	"page.schema.json":            {"backend/definitions/pages/*.yaml", "testdata/apps/*/custom/pages/*.yaml"},
	"menu.schema.json":            {"backend/definitions/menus/*.yaml"},
	"table-extension.schema.json": {"testdata/apps/*/tables/*.extend.yaml"},
	"page-extension.schema.json":  {"testdata/apps/*/pages/*.extend.yaml"},
	"menu-extension.schema.json":  {"testdata/apps/*/menus/*.extend.yaml"},
	"translations.schema.json":    {"translations/*/*.yaml", "testdata/apps/*/translations/*/*.yaml"},
}

// Every definition in the repository is valid against its schema: the schemas know every key
// the definitions use (a key they do not know would be marked as an error in the editor)
func TestDefinitionsMatchTheSchemas(t *testing.T) {
	for name, patterns := range definitionFiles {
		data, ok := Get(name)
		if !ok {
			t.Fatalf("no schema %s", name)
		}
		var s map[string]interface{}
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		v := &validator{root: s}
		checked := 0
		var problems []string
		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(root, pattern))
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range matches {
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				var doc interface{}
				if err := yaml.Unmarshal(raw, &doc); err != nil {
					t.Fatalf("%s: %v", file, err)
				}
				checked++
				rel, _ := filepath.Rel(root, file)
				for _, p := range v.check(s, doc, "") {
					problems = append(problems, rel+": "+p)
				}
			}
		}
		if checked == 0 {
			t.Errorf("%s: no files checked", name)
		}
		if len(problems) > 0 {
			sort.Strings(problems)
			if len(problems) > 40 {
				problems = append(problems[:40], fmt.Sprintf("… %d more", len(problems)-40))
			}
			t.Errorf("%s: %d file(s), problems:\n%s", name, checked, strings.Join(problems, "\n"))
		}
	}
}

// No YAML file has a quoted text continued on the next lines: the server's YAML reader takes
// one whose continuation lines are not indented, but editors' YAML parsers (and the YAML
// standard) refuse it — the file would show as broken in VS Code
func TestQuotedTextsOnOneLine(t *testing.T) {
	var files []string
	for _, pattern := range []string{"translations/*/*.yaml", "backend/definitions/*/*.yaml", "backend/definitions/*.yaml",
		"testdata/apps/*/*.yaml", "testdata/apps/*/*/*.yaml", "testdata/apps/*/*/*/*.yaml"} {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if openQuote(line) {
				rel, _ := filepath.Rel(root, file)
				t.Errorf("%s:%d: a quoted text continues on the next line — write it on one line", rel, i+1)
			}
		}
	}
}

// openQuote: the line's value starts a double-quoted text it does not close
func openQuote(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "#") {
		return false
	}
	i := strings.Index(line, ": \"")
	if i < 0 {
		if !strings.HasPrefix(strings.TrimLeft(trimmed, "- "), "\"") || strings.Contains(trimmed, "\": ") {
			return false
		}
		i = strings.Index(line, "\"") - 2
	}
	quotes := 0
	value := line[i+2:]
	for j := 0; j < len(value); j++ {
		switch value[j] {
		case '\\':
			j++
		case '"':
			quotes++
		case '#':
			if quotes%2 == 0 {
				return false // a comment after the closed text
			}
		}
	}
	return quotes%2 == 1
}

// A mistake an editor should mark: an unknown key (a typo), a value of the wrong type, a field
// type that does not exist, a page without its page key
func TestSchemasRefuseMistakes(t *testing.T) {
	cases := []struct{ schema, yaml, want string }{
		{"table.schema.json", "table:\n  id: 50000\n  name: X\n  fields:\n    - name: code\n      no: 1\n      type: types.Code\n      requried: true\n", ".table.fields[0].requried: unknown key"},
		{"table.schema.json", "table:\n  id: 50000\n  fields:\n    - name: code\n      no: one\n      type: types.Code\n", ".table.fields[0].no: a whole number expected"},
		{"table.schema.json", "table:\n  id: 50000\n  fields:\n    - name: code\n      no: 1\n      type: types.Kode\n", "types.Kode is not one of"},
		{"page-extension.schema.json", "layout:\n  sections:\n    - name: General\n      fields:\n        - source: x\n          after: y\n", ": extends is missing"},
		{"page-extension.schema.json", "extends: 21\nactions:\n  - name: X\n    in: Customer\n    run_page: 50000\n    colour: red\n", ".actions[0].colour: unknown key"},
		{"application.schema.json", "application:\n  id: x\n  number_series:\n    objects: [{from: 1, too: 2}]\n", ".application.number_series.objects[0].too: unknown key"},
	}
	for _, c := range cases {
		data, _ := Get(c.schema)
		var s map[string]interface{}
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatal(err)
		}
		var doc interface{}
		if err := yaml.Unmarshal([]byte(c.yaml), &doc); err != nil {
			t.Fatal(err)
		}
		problems := (&validator{root: s}).check(s, doc, "")
		if !strings.Contains(strings.Join(problems, "\n"), c.want) {
			t.Errorf("%s: want %q, got %v", c.schema, c.want, problems)
		}
	}
}

// The schemas are listed and served by name; nothing outside them
func TestNamesAndGet(t *testing.T) {
	names := Names()
	if len(names) != len(definitionFiles) {
		t.Errorf("schemas %v", names)
	}
	if _, ok := Get("../schemas.go"); ok {
		t.Error("a path outside the schemas was served")
	}
	if _, ok := Get("nothing.schema.json"); ok {
		t.Error("a schema that does not exist was served")
	}
}

// validator: the part of JSON Schema the generated schemas use
type validator struct {
	root map[string]interface{}
}

func (v *validator) resolve(s map[string]interface{}) map[string]interface{} {
	for {
		ref, ok := s["$ref"].(string)
		if !ok {
			return s
		}
		defs, _ := v.root["$defs"].(map[string]interface{})
		next, _ := defs[strings.TrimPrefix(ref, "#/$defs/")].(map[string]interface{})
		if next == nil {
			return map[string]interface{}{"not": "unresolved " + ref}
		}
		s = next
	}
}

// check: the problems of a value against a schema (path: where it is)
func (v *validator) check(s map[string]interface{}, value interface{}, path string) []string {
	s = v.resolve(s)
	if msg, ok := s["not"].(string); ok {
		return []string{path + ": " + msg}
	}
	if options, ok := s["anyOf"].([]interface{}); ok {
		for _, o := range options {
			if len(v.check(o.(map[string]interface{}), value, path)) == 0 {
				return nil
			}
		}
		return []string{path + ": matches none of the choices"}
	}
	if values, ok := s["enum"].([]interface{}); ok && value != nil {
		found := false
		for _, e := range values {
			if fmt.Sprint(e) == fmt.Sprint(value) {
				found = true
			}
		}
		if !found {
			return []string{fmt.Sprintf("%s: %v is not one of %v", path, value, values)}
		}
	}
	typ, _ := s["type"].(string)
	if value == nil {
		return nil // an empty value (key:) — YAML's null
	}
	switch typ {
	case "object":
		m, ok := value.(map[string]interface{})
		if !ok {
			return []string{fmt.Sprintf("%s: an object expected, not %T", path, value)}
		}
		var problems []string
		props, _ := s["properties"].(map[string]interface{})
		for _, req := range asStrings(s["required"]) {
			if _, ok := m[req]; !ok {
				problems = append(problems, path+": "+req+" is missing")
			}
		}
		for key, child := range m {
			childPath := path + "." + key
			if p, ok := props[key].(map[string]interface{}); ok {
				problems = append(problems, v.check(p, child, childPath)...)
				continue
			}
			switch extra := s["additionalProperties"].(type) {
			case bool:
				if !extra {
					problems = append(problems, childPath+": unknown key")
				}
			case map[string]interface{}:
				problems = append(problems, v.check(extra, child, childPath)...)
			}
		}
		return problems
	case "array":
		list, ok := value.([]interface{})
		if !ok {
			return []string{fmt.Sprintf("%s: a list expected, not %T", path, value)}
		}
		var problems []string
		items, _ := s["items"].(map[string]interface{})
		for i, child := range list {
			if items != nil {
				problems = append(problems, v.check(items, child, fmt.Sprintf("%s[%d]", path, i))...)
			}
		}
		return problems
	case "string":
		if _, ok := value.(string); !ok {
			return []string{fmt.Sprintf("%s: a text expected, not %v", path, value)}
		}
	case "integer":
		if _, ok := value.(int); !ok {
			return []string{fmt.Sprintf("%s: a whole number expected, not %v", path, value)}
		}
	case "number":
		switch value.(type) {
		case int, float64:
		default:
			return []string{fmt.Sprintf("%s: a number expected, not %v", path, value)}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return []string{fmt.Sprintf("%s: true or false expected, not %v", path, value)}
		}
	}
	return nil
}

func asStrings(v interface{}) []string {
	list, _ := v.([]interface{})
	out := make([]string, 0, len(list))
	for _, x := range list {
		out = append(out, fmt.Sprint(x))
	}
	return out
}
