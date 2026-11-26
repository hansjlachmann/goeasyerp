// Command schemagen writes the JSON schemas of GoEasyERP's YAML definitions (backend/schemas):
// application.yaml, tables, pages, menus, an app's extension files and its translations. Editors
// validate and complete the YAML with them (the GoEasyERP VS Code extension; the server serves
// them at /api/dev/schemas).
//
// The schemas come from the Go types that read the YAML — parsed from their source, not by
// reflection, so the table definition types of tools/tablegen (a module and main package of its
// own) are reached as well: every field with a yaml tag is a property, its Go comment the
// description; a key no type knows is refused (additionalProperties false), so a typo is shown
// in the editor. The extension files are the definitions' parts with the keys the merge adds
// (tools/extmerge: after, before, in, extends).
//
//	go run ./tools/schemagen           (from the repository's root)
//
// A test checks that the committed schemas are up to date, and that every definition in the
// repository is valid against them.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// OutDir: where the schemas are written, from the repository's root
const OutDir = "backend/schemas"

func main() {
	schemas, err := Generate(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "schemagen: %v\n", err)
		os.Exit(1)
	}
	for name, data := range schemas {
		err = os.WriteFile(filepath.Join(OutDir, name), data, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "schemagen: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("✓ %d schema(s) written to %s\n", len(schemas), OutDir)
}

// schema is a JSON schema object (keys sorted when written)
type schema map[string]interface{}

// source: the Go package whose types read a definition
type source struct {
	dir    string // the package's directory, from the repository's root
	prefix string // the prefix of its types' names in $defs
}

var (
	pagesPkg = source{"backend/foundation/pages", "Page"}
	appsPkg  = source{"backend/foundation/apps", "App"}
	tablePkg = source{"tools/tablegen", "Table"}
)

// skipped: keys the merge sets, never written by hand (an app's parts are marked with app)
var skipped = map[string]bool{"app": true}

// enums: the values a property takes (definition name . property)
var enums = map[string][]interface{}{
	"TableField.type": {"types.Code", "types.Text", "types.Decimal", "types.Date", "types.DateTime", "int", "int64",
		"bool", "Option", "[]byte", "float64", "string", "time.Time"},
	"TableField.calc_formula":                     {"Sum", "Count", "Lookup", "Exist", "Average", "Min", "Max"},
	"TableFlowFilter.type":                        {"const", "field", "filter", "field_filter", "language"},
	"TableTableDef_table.refuse_delete_when_used": {"all", "current"},
	"PagePageMetadata.type":                       {"Card", "List"},
}

// overrides: a property's schema replaced (definition name . property) — a value YAML reads
// into a Go string from any scalar (value: 8)
var overrides = map[string]schema{
	"TableFlowFilter.value": {"description": "A constant value, or the field / FlowFilter field of this table the filter takes its value from.",
		"anyOf": []schema{{"type": "string"}, {"type": "integer"}, {"type": "number"}, {"type": "boolean"}}},
}

// extraProps: keys of a definition read by another reader than its Go type, and keys kept for
// people only (definition name → key → schema)
var extraProps = map[string]map[string]schema{
	// the server's own reader of the table definitions (foundation/pages, TableMetadata)
	"TableTableDef_table": {
		"lookup_page_id": {"type": "integer", "description": "The page a lookup of this table opens and its New uses (classic LookupFormID); else its list page with the lowest number."},
		"auto_key":       {"type": "boolean", "description": "The table's key is given by its OnInsert trigger (a number series): a new record is inserted with a blank key."},
		"type_to_delete": {"type": "boolean", "description": "A delete asks the user to type the record's key (Company)."},
		"caption":        {"type": "string", "description": "The table's English caption, for people reading the file (the captions shown come from the translations)."},
		"description":    {"type": "string", "description": "What the table is for, for people reading the file."},
	},
	"TableField": {
		"caption":     {"type": "string", "description": "The field's English caption, for people reading the file (the captions shown come from the translations)."},
		"description": {"type": "string", "description": "What the field holds, for people reading the file."},
	},
}

// Generate builds the schemas from the repository at root: file name → JSON
func Generate(root string) (map[string][]byte, error) {
	g := &generator{root: root, defs: map[string]schema{}, pkgs: map[string]*pkgTypes{}}
	out := map[string][]byte{}
	add := func(name, title, description string, rootSchema schema, defs ...string) error {
		s := schema{
			"$schema":     "http://json-schema.org/draft-07/schema#",
			"$id":         "https://github.com/hansjlachmann/goeasyerp/backend/schemas/" + name,
			"title":       title,
			"description": description,
		}
		for k, v := range rootSchema {
			s[k] = v
		}
		used := g.usedDefs(rootSchema, defs)
		if len(used) > 0 {
			s["$defs"] = used
		}
		data, err := marshal(s)
		if err != nil {
			return err
		}
		out[name] = data
		return nil
	}

	app, err := g.ref(appsPkg, "App")
	if err != nil {
		return nil, err
	}
	err = add("application.schema.json", "GoEasyERP application.yaml",
		"An app's identity, number series, prefix and dependencies (EXTENSIONS.md, section 2).",
		object(map[string]schema{"application": app}, "application"))
	if err != nil {
		return nil, err
	}

	table, err := g.ref(tablePkg, "TableDef")
	if err != nil {
		return nil, err
	}
	err = add("table.schema.json", "GoEasyERP table definition",
		"A table: its fields with their numbers, keys and properties (backend/definitions/tables, an app's custom/tables).",
		g.deref(table))
	if err != nil {
		return nil, err
	}

	page, err := g.ref(pagesPkg, "PageDefinition")
	if err != nil {
		return nil, err
	}
	err = add("page.schema.json", "GoEasyERP page definition",
		"A list or card page (backend/definitions/pages, an app's custom/pages).", g.deref(page))
	if err != nil {
		return nil, err
	}

	menu, err := g.ref(pagesPkg, "MenuDefinition")
	if err != nil {
		return nil, err
	}
	err = add("menu.schema.json", "GoEasyERP main menu", "A main menu (backend/definitions/menus).", g.deref(menu))
	if err != nil {
		return nil, err
	}

	// the extension files: the definitions' parts, with the keys the merge adds
	field, err := g.ref(tablePkg, "Field")
	if err != nil {
		return nil, err
	}
	err = add("table-extension.schema.json", "GoEasyERP table extension",
		"Fields an app adds to a table of the base or of another app (tables/<id>-<name>.extend.yaml; EXTENSIONS.md, section 4).",
		object(map[string]schema{
			"extends_table": {"type": "integer", "description": "The number of the table the fields are added to."},
			"fields":        {"type": "array", "items": field, "description": "The fields, written as a table definition's fields, numbered in the app's field series and named with its prefix."},
		}, "extends_table"))
	if err != nil {
		return nil, err
	}

	placement := map[string]schema{
		"after":  {"type": "string", "description": "Placed after this one (a field by its source, the others by name); else at the end."},
		"before": {"type": "string", "description": "Placed before this one (a field by its source, the others by name)."},
	}
	fieldExt, err := g.extended(pagesPkg, "Field", "PageFieldExtension", placement)
	if err != nil {
		return nil, err
	}
	sectionExt, err := g.extended(pagesPkg, "Section", "PageSectionExtension", placement)
	if err != nil {
		return nil, err
	}
	g.defs["PageSectionExtension"]["properties"].(map[string]schema)["fields"] = schema{"type": "array", "items": fieldExt}
	actionPlacement := map[string]schema{
		"in": {"type": "string", "description": "Added to the items of this menu button (at any depth) instead of the page's actions."},
	}
	for k, v := range placement {
		actionPlacement[k] = v
	}
	actionExt, err := g.extended(pagesPkg, "Action", "PageActionExtension", actionPlacement)
	if err != nil {
		return nil, err
	}
	err = add("page-extension.schema.json", "GoEasyERP page extension",
		"Card fields and sections, list columns and buttons an app adds to a page (pages/<id>-<name>.extend.yaml; EXTENSIONS.md, section 5).",
		object(map[string]schema{
			"extends": {"type": "integer", "description": "The number of the page that is extended."},
			"version": {"type": "integer", "description": "The extension file's format version."},
			"layout": object(map[string]schema{
				"sections": {"type": "array", "items": sectionExt, "description": "A card's sections: an existing one (by name) only gets fields; a new one is placed by after/before."},
				"repeater": object(map[string]schema{"fields": {"type": "array", "items": fieldExt}}),
			}),
			"actions": {"type": "array", "items": actionExt},
		}, "extends"))
	if err != nil {
		return nil, err
	}

	menuItemExt, err := g.extended(pagesPkg, "MenuItem", "MenuItemExtension", map[string]schema{
		"in": {"description": "The sub-menu it is added to: a name, or the path of names from the top. Without in: a new module.",
			"anyOf": []schema{{"type": "string"}, {"type": "array", "items": schema{"type": "string"}}}},
		"after":  placement["after"],
		"before": placement["before"],
	})
	if err != nil {
		return nil, err
	}
	err = add("menu-extension.schema.json", "GoEasyERP main menu extension",
		"Entries an app adds to the main menu (menus/<file>.extend.yaml; EXTENSIONS.md, section 6).",
		object(map[string]schema{
			"extends": {"type": "string", "description": "The menu file extended (e.g. admin.yaml)."},
			"items":   {"type": "array", "items": menuItemExt},
		}, "extends"))
	if err != nil {
		return nil, err
	}

	err = add("translations.schema.json", "GoEasyERP translations",
		"Texts of a language (translations/<language>/*.yaml): keys in lower case (\"Loyalty Tiers\" → loyalty_tiers), nested, each text a string (EXTENSIONS.md, section 7).",
		schema{"$ref": "#/$defs/TranslationNode"}, "TranslationNode")
	if err != nil {
		return nil, err
	}
	return out, nil
}

func init() {
	// the translations' tree: objects of texts and objects
	builtinDefs["TranslationNode"] = schema{
		"type": "object",
		"additionalProperties": schema{"anyOf": []schema{
			{"type": "string"}, {"type": "number"}, {"type": "boolean"}, {"$ref": "#/$defs/TranslationNode"},
		}},
	}
}

// builtinDefs: definitions written here, not from a Go type
var builtinDefs = map[string]schema{}

// object: a closed object schema with the properties (required: the keys that must be there)
func object(props map[string]schema, required ...string) schema {
	s := schema{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// generator turns Go struct types into schema definitions
type generator struct {
	root string
	defs map[string]schema
	pkgs map[string]*pkgTypes
}

// pkgTypes: a package's type declarations by name
type pkgTypes struct {
	types map[string]*ast.TypeSpec
	docs  map[string]string // the type's doc comment
}

func (g *generator) load(src source) (*pkgTypes, error) {
	if p, ok := g.pkgs[src.dir]; ok {
		return p, nil
	}
	fset := token.NewFileSet()
	dir := filepath.Join(g.root, src.dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	p := &pkgTypes{types: map[string]*ast.TypeSpec{}, docs: map[string]string{}}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				p.types[ts.Name.Name] = ts
				doc := ts.Doc
				if doc == nil && len(gd.Specs) == 1 {
					doc = gd.Doc
				}
				p.docs[ts.Name.Name] = commentText(doc)
			}
		}
	}
	g.pkgs[src.dir] = p
	return p, nil
}

// ref: a reference to the definition of a named type of the package (made when first used)
func (g *generator) ref(src source, typeName string) (schema, error) {
	defName := src.prefix + typeName
	if _, ok := g.defs[defName]; !ok {
		p, err := g.load(src)
		if err != nil {
			return nil, err
		}
		ts, ok := p.types[typeName]
		if !ok {
			return nil, fmt.Errorf("%s: no type %s", src.dir, typeName)
		}
		g.defs[defName] = schema{} // a placeholder: the type may refer to itself
		s, err := g.typeSchema(src, defName, ts.Type)
		if err != nil {
			return nil, err
		}
		if doc := p.docs[typeName]; doc != "" {
			s["description"] = doc
		}
		g.defs[defName] = s
	}
	return schema{"$ref": "#/$defs/" + defName}, nil
}

// deref: the definition a reference names (for a schema's root)
func (g *generator) deref(ref schema) schema {
	name := strings.TrimPrefix(ref["$ref"].(string), "#/$defs/")
	out := schema{}
	for k, v := range g.defs[name] {
		out[k] = v
	}
	return out
}

// extended: a copy of a struct type's definition with more properties, as a definition of its
// own
func (g *generator) extended(src source, typeName, defName string, extra map[string]schema) (schema, error) {
	base, err := g.ref(src, typeName)
	if err != nil {
		return nil, err
	}
	orig := g.defs[strings.TrimPrefix(base["$ref"].(string), "#/$defs/")]
	copied := schema{}
	for k, v := range orig {
		copied[k] = v
	}
	props := map[string]schema{}
	for k, v := range orig["properties"].(map[string]schema) {
		props[k] = v
	}
	for k, v := range extra {
		props[k] = v
	}
	copied["properties"] = props
	g.defs[defName] = copied
	return schema{"$ref": "#/$defs/" + defName}, nil
}

// typeSchema: the schema of a Go type expression of the package (defName: the definition being
// built, for nested anonymous structs and enums)
func (g *generator) typeSchema(src source, defName string, expr ast.Expr) (schema, error) {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return schema{"type": "string"}, nil
		case "int", "int64", "int32":
			return schema{"type": "integer"}, nil
		case "float64", "float32":
			return schema{"type": "number"}, nil
		case "bool":
			return schema{"type": "boolean"}, nil
		case "any":
			return schema{}, nil
		}
		return g.ref(src, t.Name)
	case *ast.StarExpr:
		return g.typeSchema(src, defName, t.X)
	case *ast.ArrayType:
		if id, ok := t.Elt.(*ast.Ident); ok && id.Name == "byte" {
			return schema{"type": "string"}, nil
		}
		items, err := g.typeSchema(src, defName, t.Elt)
		if err != nil {
			return nil, err
		}
		return schema{"type": "array", "items": items}, nil
	case *ast.MapType:
		values, err := g.typeSchema(src, defName, t.Value)
		if err != nil {
			return nil, err
		}
		return schema{"type": "object", "additionalProperties": values}, nil
	case *ast.InterfaceType:
		return schema{}, nil
	case *ast.SelectorExpr:
		return schema{}, nil // a type of another package: any value
	case *ast.StructType:
		return g.structSchema(src, defName, t)
	}
	return nil, fmt.Errorf("%s: type %T not handled", defName, expr)
}

// structSchema: a closed object of the struct's yaml fields (inline ones merged in)
func (g *generator) structSchema(src source, defName string, st *ast.StructType) (schema, error) {
	props := map[string]schema{}
	for _, f := range st.Fields.List {
		key, inline, ok := yamlKey(f)
		if !ok {
			continue
		}
		if inline {
			inner, err := g.inlined(src, defName, f.Type)
			if err != nil {
				return nil, err
			}
			innerProps, _ := inner["properties"].(map[string]schema)
			for k, v := range innerProps {
				props[k] = v
			}
			continue
		}
		if skipped[key] {
			continue
		}
		childDef := defName
		if _, anonymous := f.Type.(*ast.StructType); anonymous {
			childDef = defName + "_" + key
		}
		s, err := g.typeSchema(src, childDef, f.Type)
		if err != nil {
			return nil, err
		}
		if _, anonymous := f.Type.(*ast.StructType); anonymous {
			g.defs[childDef] = s
			s = schema{"$ref": "#/$defs/" + childDef}
		}
		prop := schema{}
		for k, v := range s {
			prop[k] = v
		}
		if doc := fieldDoc(f); doc != "" {
			prop["description"] = doc
		}
		if values, ok := enums[defName+"."+key]; ok {
			prop["enum"] = values
		}
		if o, ok := overrides[defName+"."+key]; ok {
			prop = o
		}
		props[key] = prop
	}
	for key, extra := range extraProps[defName] {
		if _, known := props[key]; !known {
			props[key] = extra
		}
	}
	return schema{"type": "object", "properties": props, "additionalProperties": false}, nil
}

// inlined: the object schema of an inline field's type — a named struct built from its
// declaration, not through its definition (which may be the placeholder of a type being built:
// a table relation's conditions inline the table relation)
func (g *generator) inlined(src source, defName string, expr ast.Expr) (schema, error) {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	id, ok := expr.(*ast.Ident)
	if !ok {
		return g.typeSchema(src, defName, expr)
	}
	p, err := g.load(src)
	if err != nil {
		return nil, err
	}
	ts, ok := p.types[id.Name]
	if !ok {
		return nil, fmt.Errorf("%s: no type %s", src.dir, id.Name)
	}
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return nil, fmt.Errorf("%s: %s inlined but not a struct", src.dir, id.Name)
	}
	return g.structSchema(src, src.prefix+id.Name, st)
}

// yamlKey: a struct field's YAML key (ok false: not read from YAML)
func yamlKey(f *ast.Field) (key string, inline, ok bool) {
	if f.Tag == nil {
		return "", false, false
	}
	tag := reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("yaml")
	if tag == "" || tag == "-" {
		return "", false, false
	}
	parts := strings.Split(tag, ",")
	for _, p := range parts[1:] {
		if p == "inline" {
			return "", true, true
		}
	}
	return parts[0], false, parts[0] != ""
}

// fieldDoc: a field's comment, as one paragraph
func fieldDoc(f *ast.Field) string {
	text := commentText(f.Doc)
	if text == "" {
		text = commentText(f.Comment)
	}
	return text
}

func commentText(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	return strings.Join(strings.Fields(cg.Text()), " ")
}

// usedDefs: the definitions a schema refers to, directly or through others (and the named ones)
func (g *generator) usedDefs(root schema, named []string) map[string]schema {
	used := map[string]schema{}
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch t := v.(type) {
		case schema:
			if ref, ok := t["$ref"].(string); ok {
				name := strings.TrimPrefix(ref, "#/$defs/")
				if _, done := used[name]; !done {
					def := g.defs[name]
					if def == nil {
						def = builtinDefs[name]
					}
					used[name] = def
					walk(def)
				}
			}
			for k, child := range t {
				if k != "$ref" {
					walk(child)
				}
			}
		case map[string]schema:
			for _, child := range t {
				walk(child)
			}
		case []schema:
			for _, child := range t {
				walk(child)
			}
		}
	}
	walk(root)
	for _, n := range named {
		if _, ok := used[n]; !ok {
			def := g.defs[n]
			if def == nil {
				def = builtinDefs[n]
			}
			used[n] = def
			walk(def)
		}
	}
	return used
}

// marshal: indented JSON with sorted keys and a final newline (stable output)
func marshal(s schema) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	err := enc.Encode(normalize(s))
	return buf.Bytes(), err
}

// normalize turns the schema's maps into ordinary maps (encoding/json sorts their keys)
func normalize(v interface{}) interface{} {
	switch t := v.(type) {
	case schema:
		out := map[string]interface{}{}
		for k, child := range t {
			out[k] = normalize(child)
		}
		return out
	case map[string]schema:
		out := map[string]interface{}{}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out[k] = normalize(t[k])
		}
		return out
	case []schema:
		out := make([]interface{}, len(t))
		for i, child := range t {
			out[i] = normalize(child)
		}
		return out
	case []string:
		return t
	}
	return v
}
