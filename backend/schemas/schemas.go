// Package schemas holds the JSON schemas of GoEasyERP's YAML definitions — application.yaml,
// tables, pages, menus, an app's extension files and translations — written by tools/schemagen
// from the Go types that read them. Editors validate and complete the YAML with them; the
// development endpoint serves them (GET /api/dev/schemas/<name>), and the base's source has
// them in this folder.
package schemas

import (
	"embed"
	"sort"
	"strings"
)

//go:embed *.schema.json
var files embed.FS

// Names lists the schemas ("table.schema.json" …)
func Names() []string {
	entries, _ := files.ReadDir(".")
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".schema.json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// Get returns a schema by its name (ok false: there is none)
func Get(name string) ([]byte, bool) {
	if strings.ContainsAny(name, "/\\") {
		return nil, false
	}
	data, err := files.ReadFile(name)
	return data, err == nil
}
