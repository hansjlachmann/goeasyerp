package objects

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// Object types of a declaration (the Object table's Type)
const (
	TypeTable    = "Table"
	TypePage     = "Page"
	TypeReport   = "Report"
	TypeCodeunit = "Codeunit"
)

// Declaration is an object a business-logic package implements: its type, number and classic
// name, and the package that declared it
type Declaration struct {
	Type    string
	ID      int
	Name    string
	Package string
}

var (
	declaredMu sync.Mutex
	declared   = map[string]Declaration{} // "<type> <id>" → declaration
)

// Codeunit declares a codeunit the calling package implements (ABSOLUTE RULE: every
// business-logic package declares its objects in its init): a library codeunit such as 365
// Format Address, or one a registered codeunit runs. It appears in the Object table with its
// classic name.
//
//	func init() { objects.Codeunit(365, "Format Address") }
func Codeunit(id int, name string) { declare(TypeCodeunit, id, name, 1) }

// Page declares a page whose logic the calling package implements (Navigate's FindRecords)
func Page(id int, name string) { declare(TypePage, id, name, 1) }

// Report declares a report the calling package implements
func Report(id int, name string) { declare(TypeReport, id, name, 1) }

// Table declares a table whose logic the calling package implements
func Table(id int, name string) { declare(TypeTable, id, name, 1) }

// declare records a declaration with the package of the caller skip frames up; the same object
// declared again with another name panics (two packages claiming one number)
func declare(objectType string, id int, name string, skip int) {
	pkg := callerPackage(skip + 1)
	key := fmt.Sprintf("%s %d", objectType, id)
	declaredMu.Lock()
	defer declaredMu.Unlock()
	if old, ok := declared[key]; ok {
		if old.Name != name {
			panic(fmt.Sprintf("objects: %s declared as %q by %s and as %q by %s", key, old.Name, old.Package, name, pkg))
		}
		return
	}
	declared[key] = Declaration{Type: objectType, ID: id, Name: name, Package: pkg}
}

// Declared returns the declared objects, sorted by type and number
func Declared() []Declaration {
	declaredMu.Lock()
	defer declaredMu.Unlock()
	list := make([]Declaration, 0, len(declared))
	for _, d := range declared {
		list = append(list, d)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Type != list[j].Type {
			return list[i].Type < list[j].Type
		}
		return list[i].ID < list[j].ID
	})
	return list
}

// DeclaredName returns the classic name of a declared object ("" when not declared)
func DeclaredName(objectType string, id int) string {
	declaredMu.Lock()
	defer declaredMu.Unlock()
	return declared[fmt.Sprintf("%s %d", objectType, id)].Name
}

// callerPackage is the import path of the function skip frames up the stack
func callerPackage(skip int) string {
	pc, _, _, ok := runtime.Caller(skip + 1)
	if !ok {
		return ""
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}
	name := fn.Name() // github.com/…/business-logic/formataddress.init.0
	slash := strings.LastIndex(name, "/")
	dot := strings.Index(name[slash+1:], ".")
	if dot < 0 {
		return name
	}
	return name[:slash+1+dot]
}
