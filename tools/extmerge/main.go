package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
)

// PageDef represents a page definition from YAML
type PageDef struct {
	Page Page `yaml:"page"`
}

// Page represents the page configuration
type Page struct {
	ID               int      `yaml:"id"`
	Type             string   `yaml:"type"`
	Name             string   `yaml:"name"`
	SourceTable      string   `yaml:"source_table"`
	Caption          string   `yaml:"caption"`
	ListPageID       int      `yaml:"list_page_id,omitempty"`
	CardPageID       int      `yaml:"card_page_id,omitempty"`
	EnableNavigation bool     `yaml:"enable_navigation,omitempty"`
	Editable         bool     `yaml:"editable,omitempty"`
	FocusField       string   `yaml:"focus_field,omitempty"`
	Layout           Layout   `yaml:"layout,omitempty"`
	Actions          []Action `yaml:"actions,omitempty"`
	Columns          []Column `yaml:"columns,omitempty"`
}

// Layout represents the page layout
type Layout struct {
	Sections []Section `yaml:"sections,omitempty"`
}

// Section represents a layout section
type Section struct {
	Name    string  `yaml:"name"`
	Caption string  `yaml:"caption,omitempty"`
	Fields  []Field `yaml:"fields,omitempty"`
}

// Field represents a field in a section
type Field struct {
	Source        string `yaml:"source"`
	Caption       string `yaml:"caption,omitempty"`
	Editable      bool   `yaml:"editable,omitempty"`
	Visible       *bool  `yaml:"visible,omitempty"`
	Importance    string `yaml:"importance,omitempty"`
	TableRelation string `yaml:"table_relation,omitempty"`
}

// Column represents a column in a list page
type Column struct {
	Source        string `yaml:"source"`
	Caption       string `yaml:"caption,omitempty"`
	Width         int    `yaml:"width,omitempty"`
	Sortable      bool   `yaml:"sortable,omitempty"`
	Filterable    bool   `yaml:"filterable,omitempty"`
	Importance    string `yaml:"importance,omitempty"`
	TableRelation string `yaml:"table_relation,omitempty"`
}

// Action represents a page action
type Action struct {
	Name      string `yaml:"name"`
	Caption   string `yaml:"caption,omitempty"`
	Shortcut  string `yaml:"shortcut,omitempty"`
	Promoted  bool   `yaml:"promoted,omitempty"`
	Enabled   *bool  `yaml:"enabled,omitempty"`
	RunPage   int    `yaml:"run_page,omitempty"`
	RunObject string `yaml:"run_object,omitempty"`
}

// PageExtension represents a page extension definition
type PageExtension struct {
	Extends int      `yaml:"extends"`
	Version string   `yaml:"version,omitempty"`
	Actions []Action `yaml:"actions,omitempty"`
	Layout  Layout   `yaml:"layout,omitempty"`
}

// TableExtension represents a table extension definition
type TableExtension struct {
	ExtendsTable int                      `yaml:"extends_table"`
	Fields       []map[string]interface{} `yaml:"fields,omitempty"` // as in a table definition, every key kept
}

func main() {
	corePath := flag.String("core", "", "Path to the core definitions directory (backend/definitions)")
	extensionsPath := flag.String("extensions", "", "Path to extensions directory")
	outputPath := flag.String("output", "", "Path to output merged directory")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *corePath == "" || *extensionsPath == "" || *outputPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: extmerge --core <path> --extensions <path> --output <path>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --core        Path to the core definitions directory (backend/definitions)")
		fmt.Fprintln(os.Stderr, "  --extensions  Path to extensions directory")
		fmt.Fprintln(os.Stderr, "  --output      Path to output merged directory")
		fmt.Fprintln(os.Stderr, "  --verbose     Enable verbose output")
		os.Exit(1)
	}

	if err := run(*corePath, *extensionsPath, *outputPath, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(corePath, extensionsPath, outputPath string, verbose bool) error {
	// The base application and the extension app: each has its own number series, which must
	// not overlap (backend/foundation/apps)
	base, err := apps.Load(filepath.Join(corePath, "application.yaml"))
	if err != nil {
		return fmt.Errorf("base application: %w", err)
	}
	ext, err := apps.Load(filepath.Join(extensionsPath, "application.yaml"))
	if err != nil {
		return fmt.Errorf("the extension needs an application.yaml with its own id and number series: %w", err)
	}
	if err := apps.CheckOverlap([]*apps.App{base, ext}); err != nil {
		return err
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Copy core files to output
	if err := copyDir(corePath, outputPath, verbose); err != nil {
		return fmt.Errorf("failed to copy core files: %w", err)
	}

	// Find and process page extensions
	pageExtensions, err := findExtensions(extensionsPath, "pages", ".extend.yaml")
	if err != nil {
		return fmt.Errorf("failed to find page extensions: %w", err)
	}

	for _, extPath := range pageExtensions {
		if err := mergePageExtension(extPath, outputPath, verbose); err != nil {
			return fmt.Errorf("failed to merge page extension %s: %w", extPath, err)
		}
	}

	// Find and process table extensions
	tableExtensions, err := findExtensions(extensionsPath, "tables", ".extend.yaml")
	if err != nil {
		return fmt.Errorf("failed to find table extensions: %w", err)
	}

	for _, extPath := range tableExtensions {
		if err := mergeTableExtension(extPath, outputPath, ext, verbose); err != nil {
			return fmt.Errorf("failed to merge table extension %s: %w", extPath, err)
		}
	}

	// Copy new objects (non-extend files)
	if err := copyNewObjects(extensionsPath, outputPath, ext, verbose); err != nil {
		return fmt.Errorf("failed to copy new objects: %w", err)
	}

	if verbose {
		fmt.Printf("Merge completed successfully\n")
		fmt.Printf("  Page extensions merged: %d\n", len(pageExtensions))
		fmt.Printf("  Table extensions merged: %d\n", len(tableExtensions))
	}

	return nil
}

func copyDir(src, dst string, verbose bool) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		if verbose {
			fmt.Printf("Copying: %s\n", relPath)
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func findExtensions(basePath, subdir, suffix string) ([]string, error) {
	var extensions []string
	searchPath := filepath.Join(basePath, subdir)

	if _, err := os.Stat(searchPath); os.IsNotExist(err) {
		return extensions, nil
	}

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), suffix) {
			extensions = append(extensions, path)
		}
		return nil
	})

	return extensions, err
}

func mergePageExtension(extPath, outputPath string, verbose bool) error {
	// Read extension file
	extData, err := os.ReadFile(extPath)
	if err != nil {
		return fmt.Errorf("failed to read extension: %w", err)
	}

	var ext PageExtension
	if err := yaml.Unmarshal(extData, &ext); err != nil {
		return fmt.Errorf("failed to parse extension: %w", err)
	}

	if ext.Extends == 0 {
		return fmt.Errorf("extension must specify 'extends' field with page ID")
	}

	// Find core page file
	corePagePath, err := findCorePageFile(outputPath, ext.Extends)
	if err != nil {
		return fmt.Errorf("failed to find core page %d: %w", ext.Extends, err)
	}

	// Read core page
	coreData, err := os.ReadFile(corePagePath)
	if err != nil {
		return fmt.Errorf("failed to read core page: %w", err)
	}

	var pageDef PageDef
	if err := yaml.Unmarshal(coreData, &pageDef); err != nil {
		return fmt.Errorf("failed to parse core page: %w", err)
	}

	// Merge extension into page
	mergePage(&pageDef.Page, &ext)

	// Write merged page
	mergedData, err := yaml.Marshal(&pageDef)
	if err != nil {
		return fmt.Errorf("failed to marshal merged page: %w", err)
	}

	if err := os.WriteFile(corePagePath, mergedData, 0644); err != nil {
		return fmt.Errorf("failed to write merged page: %w", err)
	}

	if verbose {
		fmt.Printf("Merged page extension: %s -> page %d\n", filepath.Base(extPath), ext.Extends)
	}

	return nil
}

func findCorePageFile(outputPath string, pageID int) (string, error) {
	pagesPath := filepath.Join(outputPath, "pages")

	var found string
	err := filepath.Walk(pagesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Match files like "21-customer-card.yaml"
		name := info.Name()
		if strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".extend.yaml") {
			// Parse the ID from filename
			parts := strings.SplitN(name, "-", 2)
			if len(parts) >= 1 {
				var id int
				if _, err := fmt.Sscanf(parts[0], "%d", &id); err == nil && id == pageID {
					found = path
					return filepath.SkipAll
				}
			}
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", err
	}

	if found == "" {
		return "", fmt.Errorf("page with ID %d not found", pageID)
	}

	return found, nil
}

func mergePage(page *Page, ext *PageExtension) {
	// Append actions
	page.Actions = append(page.Actions, ext.Actions...)

	// Merge layout sections
	for _, extSection := range ext.Layout.Sections {
		merged := false
		for i, pageSection := range page.Layout.Sections {
			if pageSection.Name == extSection.Name {
				// Merge fields into existing section
				page.Layout.Sections[i].Fields = mergeFields(pageSection.Fields, extSection.Fields)
				merged = true
				break
			}
		}
		if !merged {
			// Add as new section
			page.Layout.Sections = append(page.Layout.Sections, extSection)
		}
	}
}

func mergeFields(existing, new []Field) []Field {
	result := make([]Field, len(existing))
	copy(result, existing)

	for _, newField := range new {
		found := false
		for i, existingField := range result {
			if existingField.Source == newField.Source {
				// Override existing field
				result[i] = newField
				found = true
				break
			}
		}
		if !found {
			// Add new field
			result = append(result, newField)
		}
	}

	return result
}

func mergeTableExtension(extPath, outputPath string, ext *apps.App, verbose bool) error {
	// Read extension file
	extData, err := os.ReadFile(extPath)
	if err != nil {
		return fmt.Errorf("failed to read extension: %w", err)
	}

	var extension TableExtension
	if err := yaml.Unmarshal(extData, &extension); err != nil {
		return fmt.Errorf("failed to parse extension: %w", err)
	}

	if extension.ExtendsTable == 0 {
		return fmt.Errorf("extension must specify 'extends_table' field with table ID")
	}

	// Find core table file
	coreTablePath, err := findCoreTableFile(outputPath, extension.ExtendsTable)
	if err != nil {
		return fmt.Errorf("failed to find core table %d: %w", extension.ExtendsTable, err)
	}

	// Read core table as raw YAML to preserve structure
	coreData, err := os.ReadFile(coreTablePath)
	if err != nil {
		return fmt.Errorf("failed to read core table: %w", err)
	}

	var tableDef map[string]interface{}
	if err := yaml.Unmarshal(coreData, &tableDef); err != nil {
		return fmt.Errorf("failed to parse core table: %w", err)
	}

	// Merge table extension: the new fields are added as written (every key kept)
	table, ok := tableDef["table"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("core table %d has no table section", extension.ExtendsTable)
	}
	fields, _ := table["fields"].([]interface{})
	if err := checkExtensionFields(fields, extension.Fields, ext); err != nil {
		return fmt.Errorf("table %d: %w", extension.ExtendsTable, err)
	}
	for _, f := range extension.Fields {
		f["app"] = ext.ID // tablegen: checked here against the extension's series
		fields = append(fields, f)
	}
	table["fields"] = fields

	// Write merged table
	mergedData, err := yaml.Marshal(&tableDef)
	if err != nil {
		return fmt.Errorf("failed to marshal merged table: %w", err)
	}

	if err := os.WriteFile(coreTablePath, mergedData, 0644); err != nil {
		return fmt.Errorf("failed to write merged table: %w", err)
	}

	if verbose {
		fmt.Printf("Merged table extension: %s -> table %d\n", filepath.Base(extPath), extension.ExtendsTable)
	}

	return nil
}

func findCoreTableFile(outputPath string, tableID int) (string, error) {
	tablesPath := filepath.Join(outputPath, "tables")

	var found string
	err := filepath.Walk(tablesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".extend.yaml") {
			// Read file to check table ID
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			var def struct {
				Table struct {
					ID int `yaml:"id"`
				} `yaml:"table"`
			}
			if err := yaml.Unmarshal(data, &def); err != nil {
				return nil
			}
			if def.Table.ID == tableID {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return "", err
	}

	if found == "" {
		return "", fmt.Errorf("table with ID %d not found", tableID)
	}

	return found, nil
}

func copyNewObjects(extensionsPath, outputPath string, ext *apps.App, verbose bool) error {
	// New tables: numbers in the extension's series; the table and its fields are marked with
	// the app (tablegen checks only the base application's tables against its series)
	newTablesPath := filepath.Join(extensionsPath, "custom", "tables")
	if _, err := os.Stat(newTablesPath); err == nil {
		err := eachNewYAML(newTablesPath, func(name string, data []byte) error {
			var def map[string]interface{}
			if err := yaml.Unmarshal(data, &def); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			table, _ := def["table"].(map[string]interface{})
			id, _ := table["id"].(int)
			if table == nil || !ext.ObjectAllowed(apps.Table, id) {
				return fmt.Errorf("%s: table number %d is outside the object series of %s", name, id, ext.Name)
			}
			table["app"] = ext.ID
			fields, _ := table["fields"].([]interface{})
			added := make([]map[string]interface{}, 0, len(fields))
			for _, f := range fields {
				m, _ := f.(map[string]interface{})
				if m == nil {
					return fmt.Errorf("%s: invalid field", name)
				}
				added = append(added, m)
			}
			if err := checkExtensionFields(nil, added, ext); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			for _, m := range added {
				m["app"] = ext.ID
			}
			out, err := yaml.Marshal(def)
			if err != nil {
				return err
			}
			if verbose {
				fmt.Printf("Copying new table: %s\n", name)
			}
			return os.WriteFile(filepath.Join(outputPath, "tables", name), out, 0644)
		})
		if err != nil {
			return err
		}
	}

	// New pages: numbers in the extension's series
	newPagesPath := filepath.Join(extensionsPath, "custom", "pages")
	if _, err := os.Stat(newPagesPath); err == nil {
		err := eachNewYAML(newPagesPath, func(name string, data []byte) error {
			var def struct {
				Page struct {
					ID int `yaml:"id"`
				} `yaml:"page"`
			}
			if err := yaml.Unmarshal(data, &def); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			if !ext.ObjectAllowed(apps.Page, def.Page.ID) {
				return fmt.Errorf("%s: page number %d is outside the object series of %s", name, def.Page.ID, ext.Name)
			}
			if verbose {
				fmt.Printf("Copying new page: %s\n", name)
			}
			return os.WriteFile(filepath.Join(outputPath, "pages", name), data, 0644)
		})
		if err != nil {
			return err
		}
	}

	// Go code of an extension (table logic, codeunits) is not copied: it is a Go module of
	// its own that registers its business logic (EXTENSIONS.md, "Extension apps")
	return nil
}

// eachNewYAML calls fn for every new object definition (*.yaml, not *.extend.yaml) in dir
func eachNewYAML(dir string, fn func(name string, data []byte) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".extend.yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		if err := fn(name, data); err != nil {
			return err
		}
	}
	return nil
}

// checkExtensionFields checks the fields an extension adds to a table: each has a field number
// in the extension's field series (its application.yaml), and neither its number nor its name
// (or db_name) is used by the table or another extension field already. Field numbers
// identify fields (field IDs): two apps must never use the same number.
func checkExtensionFields(existing []interface{}, added []map[string]interface{}, ext *apps.App) error {
	numbers, names := map[int]string{}, map[string]bool{}
	for _, f := range existing {
		m, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		name := fieldString(m, "name")
		if no, ok := m["no"].(int); ok {
			numbers[no] = name
		}
		names[strings.ToLower(name)] = true
		if db := fieldString(m, "db_name"); db != "" {
			names[strings.ToLower(db)] = true
		}
	}
	for _, f := range added {
		name := fieldString(f, "name")
		if name == "" {
			return fmt.Errorf("an extension field has no name")
		}
		no, ok := f["no"].(int)
		if !ok || !ext.FieldAllowed(no) {
			return fmt.Errorf("extension field %s needs a field number (no:) in the field series of %s", name, ext.Name)
		}
		if other, used := numbers[no]; used {
			return fmt.Errorf("extension field %s: number %d is already used by field %s", name, no, other)
		}
		for _, n := range []string{name, fieldString(f, "db_name")} {
			if n != "" && names[strings.ToLower(n)] {
				return fmt.Errorf("extension field %s: the table already has a field named %s", name, n)
			}
		}
		numbers[no] = name
		names[strings.ToLower(name)] = true
		if db := fieldString(f, "db_name"); db != "" {
			names[strings.ToLower(db)] = true
		}
	}
	return nil
}

func fieldString(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}
