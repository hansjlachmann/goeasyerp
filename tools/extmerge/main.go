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

// TableExtension represents a table extension definition
type TableExtension struct {
	ExtendsTable int                      `yaml:"extends_table"`
	Fields       []map[string]interface{} `yaml:"fields,omitempty"` // as in a table definition, every key kept
}

// stringList is a flag that can be given more than once
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, ",") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

// options of a merge: the base definitions, the apps' directories, where the merged definitions
// go, and — when an app has translations — the base translations and where the merged ones go
type options struct {
	core               string
	apps               []string
	output             string
	translations       string
	translationsOutput string
	goRoot             string // the base repository: go.work and the apps' Go code installation are written there
	verbose            bool
}

func main() {
	var opts options
	var extensions stringList
	appsDir := flag.String("apps", "", "Directory whose subdirectories are extension apps (each with an application.yaml)")
	flag.StringVar(&opts.core, "core", "", "Path to the core definitions directory (backend/definitions)")
	flag.Var(&extensions, "extensions", "Path to an extension app's directory (repeat for several apps)")
	flag.StringVar(&opts.output, "output", "", "Path to output merged directory")
	flag.StringVar(&opts.translations, "translations", "", "Path to the core translations directory (needed when an app has translations)")
	flag.StringVar(&opts.translationsOutput, "translations-output", "", "Path to output merged translations directory")
	flag.StringVar(&opts.goRoot, "go-root", "", "The base repository being built: link the apps' Go code (go.work, cmd/api-server/apps_gen.go)")
	flag.BoolVar(&opts.verbose, "verbose", false, "Enable verbose output")
	flag.Parse()

	opts.apps = extensions
	if *appsDir != "" {
		found, err := appDirectories(*appsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		opts.apps = append(opts.apps, found...)
	}
	if opts.core == "" || len(opts.apps) == 0 || opts.output == "" {
		fmt.Fprintln(os.Stderr, "Usage: extmerge --core <path> (--extensions <app dir> ... | --apps <dir of apps>) --output <path> [--translations <path> --translations-output <path>]")
		fmt.Fprintln(os.Stderr, "")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if err := merge(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// appDirectories: the subdirectories of dir that hold an application.yaml
func appDirectories(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if _, err := os.Stat(filepath.Join(path, "application.yaml")); e.IsDir() && err == nil {
			found = append(found, path)
		}
	}
	return found, nil
}

// run merges one extension app (the tests' short form)
func run(corePath, extensionsPath, outputPath string, verbose bool) error {
	return merge(options{core: corePath, apps: []string{extensionsPath}, output: outputPath, verbose: verbose})
}

// merge merges the extension apps into the base definitions: each app is checked (its own id,
// number series that overlap no other app's, a prefix, a dependency on the base application),
// the apps' dependencies are checked and the apps merged in their order (an app after the apps
// it depends on, so it can extend what they added), and each app's application.yaml is written
// to apps/<id>.yaml of the merged definitions, where the server reads the installed apps
func merge(opts options) error {
	base, err := apps.Load(filepath.Join(opts.core, "application.yaml"))
	if err != nil {
		return fmt.Errorf("base application: %w", err)
	}
	dirOf := map[string]string{}
	var extensions []*apps.App
	for _, dir := range opts.apps {
		ext, err := apps.Load(filepath.Join(dir, "application.yaml"))
		if err != nil {
			return fmt.Errorf("the extension %s needs an application.yaml with its own id and number series: %w", dir, err)
		}
		err = ext.CheckExtension(base)
		if err != nil {
			return err
		}
		if other, dup := dirOf[ext.ID]; dup {
			return fmt.Errorf("%s and %s are the same app (id %s)", other, dir, ext.ID)
		}
		dirOf[ext.ID] = dir
		extensions = append(extensions, ext)
	}
	err = apps.CheckOverlap(append([]*apps.App{base}, extensions...))
	if err != nil {
		return err
	}
	ordered, err := apps.Order(base, extensions)
	if err != nil {
		return err
	}

	// the outputs start empty (a merge run again starts afresh); they are never the inputs
	for _, out := range [][2]string{{opts.output, opts.core}, {opts.translationsOutput, opts.translations}} {
		if out[0] == "" {
			continue
		}
		if filepath.Clean(out[0]) == filepath.Clean(out[1]) {
			return fmt.Errorf("the output %s is the input: give another directory", out[0])
		}
		err = os.RemoveAll(out[0])
		if err != nil {
			return err
		}
	}
	err = os.MkdirAll(opts.output, 0755)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	err = copyDir(opts.core, opts.output, opts.verbose)
	if err != nil {
		return fmt.Errorf("failed to copy core files: %w", err)
	}
	for _, ext := range ordered[1:] {
		err = mergeApp(ext, dirOf[ext.ID], opts.output, opts.verbose)
		if err != nil {
			return fmt.Errorf("%s: %w", ext.Name, err)
		}
	}
	err = mergeAppTranslations(ordered[1:], dirOf, opts)
	if err != nil {
		return err
	}
	if opts.goRoot == "" {
		return nil
	}
	return writeGoWiring(ordered[1:], dirOf, opts.goRoot, opts.verbose)
}

// mergeApp merges one app: its new tables and pages, then what it adds to existing tables,
// pages and menus; its application.yaml goes to apps/<id>.yaml
func mergeApp(ext *apps.App, dir, outputPath string, verbose bool) error {
	err := copyNewObjects(dir, outputPath, ext, verbose)
	if err != nil {
		return fmt.Errorf("failed to copy new objects: %w", err)
	}
	steps := []struct {
		subdir string
		merge  func(path, out string, ext *apps.App, verbose bool) error
	}{
		{"tables", mergeTableExtension},
		{"pages", mergePageExtension},
		{"menus", mergeMenuExtension},
	}
	for _, step := range steps {
		files, err := findExtensions(dir, step.subdir, ".extend.yaml")
		if err != nil {
			return fmt.Errorf("failed to find %s extensions: %w", step.subdir, err)
		}
		for _, f := range files {
			err = step.merge(f, outputPath, ext, verbose)
			if err != nil {
				return fmt.Errorf("failed to merge %s: %w", f, err)
			}
		}
	}
	err = os.MkdirAll(filepath.Join(outputPath, apps.AppsDir), 0755)
	if err != nil {
		return err
	}
	if verbose {
		fmt.Printf("Merged app %s %s\n", ext.Name, ext.Version)
	}
	return copyFile(filepath.Join(dir, "application.yaml"), filepath.Join(outputPath, apps.AppsDir, ext.ID+".yaml"))
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
	if err := checkExtensionFields(fields, extension.Fields, ext, true); err != nil {
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
			if tableName := fieldString(table, "name"); !ext.HasPrefix(tableName) {
				return fmt.Errorf("%s: table %q must start with the app's prefix (%s_…)", name, tableName, strings.ToUpper(ext.Prefix))
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
			if err := checkExtensionFields(nil, added, ext, false); err != nil {
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
					ID   int    `yaml:"id"`
					Name string `yaml:"name"`
				} `yaml:"page"`
			}
			if err := yaml.Unmarshal(data, &def); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			if !ext.ObjectAllowed(apps.Page, def.Page.ID) {
				return fmt.Errorf("%s: page number %d is outside the object series of %s", name, def.Page.ID, ext.Name)
			}
			if !ext.HasPrefix(def.Page.Name) {
				return fmt.Errorf("%s: page %q must start with the app's prefix (%s …)", name, def.Page.Name, strings.ToUpper(ext.Prefix))
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
// identify fields (field IDs): two apps must never use the same number. A field an app adds to
// a table of another app (prefixed) starts with the app's prefix; the fields of its own tables
// need none (the table's name has it).
func checkExtensionFields(existing []interface{}, added []map[string]interface{}, ext *apps.App, prefixed bool) error {
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
		if prefixed && !ext.HasPrefix(name) {
			return fmt.Errorf("extension field %s must start with the app's prefix (%s_…)", name, ext.Prefix)
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
