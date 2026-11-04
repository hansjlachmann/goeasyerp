package apps

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	fobjects "github.com/hansjlachmann/goeasyerp/backend/foundation/objects"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// objectRuleExempt: the platform packages of business-logic, which implement no object (the
// ABSOLUTE RULE "Object identity" names them); every other package declares its objects
var objectRuleExempt = map[string]string{
	"apps":       "installed apps and the Object table itself",
	"migrations": "schema migrations",
	"scheduler":  "the Job Queue poller (runs the jobs' objects)",
	"themerepo":  "theme installation",
	"demodata":   "the demo data loader (codeunit 901 Create Demo Data runs it)",
	"pages":      "triggers of pages, which are objects by their YAML",
}

// declaration is an object a package's source declares: objects.Codeunit/Page/Report/Table
type declaration struct {
	objectType string
	id         int
	name       string
	pkg        string
}

// packageObjects reads a package's (non-test) Go files: its objects.X(id, "name") declarations
// and whether it registers codeunits or reports (codeunits.Register, reports.Register, Register)
func packageObjects(t *testing.T, dir string) ([]declaration, bool) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Base(dir)
	var decls []declaration
	registers := false
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				x, ok := fn.X.(*ast.Ident)
				if !ok {
					return true
				}
				if x.Name == "objects" && len(call.Args) == 2 {
					id, idOK := call.Args[0].(*ast.BasicLit)
					name, nameOK := call.Args[1].(*ast.BasicLit)
					if idOK && nameOK {
						n, _ := strconv.Atoi(id.Value)
						s, _ := strconv.Unquote(name.Value)
						decls = append(decls, declaration{fn.Sel.Name, n, s, pkg})
					}
				}
				if (x.Name == "codeunits" || x.Name == "reports") && fn.Sel.Name == "Register" {
					registers = true
				}
			case *ast.Ident:
				if fn.Name == "Register" && (pkg == "codeunits" || pkg == "reports") {
					registers = true
				}
			}
			return true
		})
	}
	return decls, registers
}

// ABSOLUTE RULE "Object identity": every business-logic package (but the platform packages)
// declares the objects it implements — type, number and classic name —, a number is one object
// (the same name wherever it is declared), declared pages and tables exist, and every
// registered codeunit has its classic name declared
func TestEveryBusinessPackageDeclaresItsObjects(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "*"))
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]declaration{}
	var missing []string
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		pkg := filepath.Base(dir)
		decls, registers := packageObjects(t, dir)
		if _, exempt := objectRuleExempt[pkg]; exempt {
			continue
		}
		if len(decls) == 0 && !registers {
			missing = append(missing, pkg)
		}
		for _, d := range decls {
			key := fmt.Sprintf("%s %d", d.objectType, d.id)
			if old, ok := byKey[key]; ok && old.name != d.name {
				t.Errorf("%s declared as %q (%s) and %q (%s)", key, old.name, old.pkg, d.name, d.pkg)
			}
			byKey[key] = d
			switch d.objectType {
			case "Table":
				if _, ok := gtables.NewRecords()[d.id]; !ok {
					t.Errorf("%s declares table %d, which does not exist", d.pkg, d.id)
				}
			case "Page":
				pages, _ := filepath.Glob(filepath.Join("..", "..", "definitions", "pages", fmt.Sprintf("%d-*.yaml", d.id)))
				if len(pages) == 0 {
					t.Errorf("%s declares page %d, which does not exist", d.pkg, d.id)
				}
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("packages without object declarations (objects.Codeunit(id, \"Classic Name\") in their init): %s", strings.Join(missing, ", "))
	}
	for id, name := range codeunits.List() {
		if fobjects.DeclaredName(fobjects.TypeCodeunit, id) == "" {
			t.Errorf("registered codeunit %d %s has no declared name (codeunits/objects.go)", id, name)
		}
	}
}

// The Object table's rows: a declared library codeunit with its classic name, not runnable; a
// registered codeunit with its classic name (not the registry's), runnable; reports runnable
func TestObjectsDeclaredAndRunnable(t *testing.T) {
	objects := Objects()
	formatAddress := objects[objectKey{gtables.Object_Type.Codeunit, 365}]
	if formatAddress.name != "Format Address" || formatAddress.runnable {
		t.Errorf("codeunit 365: %+v", formatAddress)
	}
	genJnlPost := objects[objectKey{gtables.Object_Type.Codeunit, 231}]
	if genJnlPost.name != "Gen. Jnl.-Post" || !genJnlPost.runnable {
		t.Errorf("codeunit 231: %+v", genJnlPost)
	}
	if report := objects[objectKey{gtables.Object_Type.Report, 101}]; !report.runnable || report.name == "" {
		t.Errorf("report 101: %+v", report)
	}
}
