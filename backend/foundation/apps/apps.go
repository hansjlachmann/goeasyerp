// Package apps reads application.yaml: an app's identity (app.json) and the number series
// its objects and fields may use. The base application has one
// (backend/definitions/application.yaml); every extension app has its own.
package apps

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Object types that have numbers. A number is unique per type (table 18 and page 18 can both
// exist).
const (
	Table    = "table"
	Page     = "page"
	Codeunit = "codeunit"
	Report   = "report"
)

// Range is an inclusive number range
type Range struct {
	From int `yaml:"from"`
	To   int `yaml:"to"`
}

// Contains reports whether n is in the range
func (r Range) Contains(n int) bool { return n >= r.From && n <= r.To }

func (r Range) String() string { return fmt.Sprintf("%d–%d", r.From, r.To) }

// Dependency is another app this app needs (an extension depends on the base application)
type Dependency struct {
	ID         string `yaml:"id"`
	Name       string `yaml:"name"`
	MinVersion string `yaml:"min_version"`
}

// App is an application.yaml
type App struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	Publisher    string `yaml:"publisher"`
	Contact      string `yaml:"contact"`
	Version      string `yaml:"version"`
	NumberSeries struct {
		Objects []Range `yaml:"objects"`
		Fields  []Range `yaml:"fields"`
	} `yaml:"number_series"`
	// SystemTables are the system table numbers (2000000000 range), a section of its own
	SystemTables []Range      `yaml:"system_tables"`
	Dependencies []Dependency `yaml:"dependencies"`
	// Prefix: an extension app's own names start with it — the fields it adds to base tables
	// (lp_loyalty_points), its tables (LP_Loyalty_Tier) and pages (LP Loyalty Tiers) — so two
	// apps cannot clash by name. Required for an extension app; the base application has none.
	Prefix string `yaml:"prefix"`

	path string // the file it was read from
}

var guidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Load reads and checks an application.yaml
func Load(path string) (*App, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	app, err := Parse(data)
	if err != nil {
		return nil, err
	}
	app.path = path
	return app, nil
}

// Parse reads and checks the content of an application.yaml
func Parse(data []byte) (*App, error) {
	var file struct {
		Application App `yaml:"application"`
	}
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil {
		return nil, fmt.Errorf("application.yaml: %w", err)
	}
	app := &file.Application
	if err := app.check(); err != nil {
		return nil, fmt.Errorf("application.yaml: %w", err)
	}
	return app, nil
}

func (a *App) check() error {
	if !guidPattern.MatchString(a.ID) {
		return fmt.Errorf("id %q is not a GUID (lower case)", a.ID)
	}
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("name is missing")
	}
	if len(a.NumberSeries.Objects) == 0 || len(a.NumberSeries.Fields) == 0 {
		return fmt.Errorf("number_series needs objects and fields")
	}
	all := map[string][]Range{"objects": a.NumberSeries.Objects, "fields": a.NumberSeries.Fields, "system_tables": a.SystemTables}
	for name, ranges := range all {
		for i, r := range ranges {
			if r.From < 1 || r.To < r.From {
				return fmt.Errorf("%s: range %s is invalid", name, r)
			}
			for _, o := range ranges[:i] {
				if r.From <= o.To && o.From <= r.To {
					return fmt.Errorf("%s: ranges %s and %s overlap", name, o, r)
				}
			}
		}
	}
	return nil
}

var prefixPattern = regexp.MustCompile(`^[a-z][a-z0-9]{1,9}$`)

// CheckExtension checks what an extension app needs beyond any app: a prefix (2–10 lower case
// letters and digits, starting with a letter) and a dependency on the base application
func (a *App) CheckExtension(base *App) error {
	if !prefixPattern.MatchString(a.Prefix) {
		return fmt.Errorf("%s: prefix %q must be 2–10 lower case letters and digits, starting with a letter", a.Name, a.Prefix)
	}
	for _, d := range a.Dependencies {
		if d.ID == base.ID {
			return nil
		}
	}
	return fmt.Errorf("%s: an extension app depends on %s (id %s): add it to its dependencies", a.Name, base.Name, base.ID)
}

// HasPrefix reports whether a name an extension app gives (a field, table or page name) starts
// with its prefix and a separator: lp_points, LP_Loyalty_Tier, LP Loyalty Tiers
func (a *App) HasPrefix(name string) bool {
	if len(name) <= len(a.Prefix)+1 {
		return false
	}
	sep := name[len(a.Prefix)]
	return strings.EqualFold(name[:len(a.Prefix)], a.Prefix) && (sep == '_' || sep == ' ')
}

func inRanges(ranges []Range, n int) bool {
	for _, r := range ranges {
		if r.Contains(n) {
			return true
		}
	}
	return false
}

// ObjectAllowed reports whether the app may use number id for an object of the type: its
// object series, and for tables also its system tables
func (a *App) ObjectAllowed(objectType string, id int) bool {
	if objectType == Table && inRanges(a.SystemTables, id) {
		return true
	}
	return inRanges(a.NumberSeries.Objects, id)
}

// FieldAllowed reports whether the app may use field number no
func (a *App) FieldAllowed(no int) bool {
	return inRanges(a.NumberSeries.Fields, no)
}

// CheckObject returns an error naming the object when its number is outside the app's series
func (a *App) CheckObject(objectType string, id int, name string) error {
	if a.ObjectAllowed(objectType, id) {
		return nil
	}
	series := a.NumberSeries.Objects
	if objectType == Table {
		series = append(append([]Range{}, series...), a.SystemTables...)
	}
	return fmt.Errorf("%s %d %s is outside the number series of %s (%s)", objectType, id, name, a.Name, rangesText(series))
}

// CheckOverlap returns an error when two apps share numbers: their object, field or system
// table series overlap (every installed app has numbers of its own)
func CheckOverlap(list []*App) error {
	ids := map[string]string{}
	for i, a := range list {
		if other, dup := ids[a.ID]; dup {
			return fmt.Errorf("apps %s and %s have the same id %s", other, a.Name, a.ID)
		}
		ids[a.ID] = a.Name
		for _, b := range list[:i] {
			pairs := []struct {
				what string
				x, y []Range
			}{
				{"object", a.NumberSeries.Objects, b.NumberSeries.Objects},
				{"field", a.NumberSeries.Fields, b.NumberSeries.Fields},
				{"system table", a.SystemTables, b.SystemTables},
			}
			for _, p := range pairs {
				for _, r := range p.x {
					for _, o := range p.y {
						if r.From <= o.To && o.From <= r.To {
							return fmt.Errorf("the %s series %s of %s overlaps %s of %s", p.what, r, a.Name, o, b.Name)
						}
					}
				}
			}
		}
	}
	return nil
}

func rangesText(ranges []Range) string {
	parts := make([]string, len(ranges))
	for i, r := range ranges {
		parts[i] = r.String()
	}
	return strings.Join(parts, ", ")
}

// Order checks the installed apps' dependencies and returns them in the order they are merged
// and loaded: the base application first, every app after the apps it depends on (apps without
// a dependency between them in the order given). Refused: a dependency that is not installed,
// installed in a version below its min_version, or installed under another name than the
// dependency names, and dependencies in a circle.
func Order(base *App, extensions []*App) ([]*App, error) {
	byID := map[string]*App{base.ID: base}
	for _, a := range extensions {
		byID[a.ID] = a
	}
	for _, a := range extensions {
		for _, d := range a.Dependencies {
			dep, ok := byID[d.ID]
			if !ok {
				return nil, fmt.Errorf("%s needs %s (id %s%s), which is not installed", a.Name, d.Name, d.ID, versionText(d.MinVersion))
			}
			if d.Name != "" && d.Name != dep.Name {
				return nil, fmt.Errorf("%s depends on %s (id %s), but the app with that id is %s", a.Name, d.Name, d.ID, dep.Name)
			}
			if d.MinVersion != "" && CompareVersions(dep.Version, d.MinVersion) < 0 {
				return nil, fmt.Errorf("%s needs %s version %s or later, %s is installed", a.Name, dep.Name, d.MinVersion, dep.Version)
			}
		}
	}

	ordered := []*App{base}
	done := map[string]bool{base.ID: true}
	visiting := map[string]bool{}
	var visit func(a *App, path []string) error
	visit = func(a *App, path []string) error {
		if done[a.ID] {
			return nil
		}
		path = append(path, a.Name)
		if visiting[a.ID] {
			return fmt.Errorf("the apps depend on each other in a circle: %s", strings.Join(path, " → "))
		}
		visiting[a.ID] = true
		for _, d := range a.Dependencies {
			err := visit(byID[d.ID], path)
			if err != nil {
				return err
			}
		}
		visiting[a.ID] = false
		done[a.ID] = true
		ordered = append(ordered, a)
		return nil
	}
	for _, a := range extensions {
		err := visit(a, nil)
		if err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

func versionText(v string) string {
	if v == "" {
		return ""
	}
	return ", version " + v + " or later"
}

// CompareVersions compares two versions major.minor.patch (a missing part counts as 0, anything
// after a - or + is left out): -1, 0 or 1
func CompareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func versionParts(v string) [3]int {
	var parts [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v, _, _ = strings.Cut(v, "-")
	v, _, _ = strings.Cut(v, "+")
	for i, p := range strings.SplitN(v, ".", 3) {
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		parts[i] = n
	}
	return parts
}

// AppsDir is where an installation's build puts the application.yaml of every extension app it
// merged (<id>.yaml), relative to the definitions directory: the server reads them at startup
const AppsDir = "apps"

// LoadInstalled reads the base application and the extension apps merged into the definitions
// (backend/definitions/apps/<id>.yaml) and returns them in dependency order (Order)
func LoadInstalled() ([]*App, error) {
	base, err := LoadBase()
	if err != nil {
		return nil, err
	}
	definitions := filepath.Dir(base.path)
	files, err := filepath.Glob(filepath.Join(definitions, AppsDir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	var extensions []*App
	for _, f := range files {
		a, err := Load(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
		err = a.CheckExtension(base)
		if err != nil {
			return nil, err
		}
		extensions = append(extensions, a)
	}
	err = CheckOverlap(append([]*App{base}, extensions...))
	if err != nil {
		return nil, err
	}
	return Order(base, extensions)
}

// BasePath is the base application's application.yaml, relative to the project root
const BasePath = "backend/definitions/application.yaml"

// LoadBase reads the base application's application.yaml (the project root is the directory
// with go.mod, searched upwards from the working directory)
func LoadBase() (*App, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return Load(filepath.Join(dir, BasePath))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("project root not found (no go.mod)")
		}
		dir = parent
	}
}
