// Package appaudit checks an extension app's code before the installation builds it into the
// server (S55, the safeguard for S8): an app is Go code compiled into the server, so without the
// audit it could do whatever the server can — read every company, the secrets in the
// environment, files, the network, the database behind the base's back. The audit refuses what
// would give it those powers. It reads the source only (no type checking, no toolchain), so it
// runs the same way in Upload App, the installation's builder and the CI pipeline: because
// unsafe, reflect and //go:linkname are refused, the app can reach a function only by writing its
// name, and a forbidden name is found in the source.
//
// It does not make an app harmless: an app that passes can still post wrong amounts or change
// the records of the companies where it runs — the audit closes the server-wide powers, not
// dishonest business logic.
package appaudit

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"
)

// BaseModule is the base application's Go module
const BaseModule = "github.com/hansjlachmann/goeasyerp"

// Rules (a finding's Rule; an exception names the rule and the finding's Key)
const (
	RuleUnsafe      = "unsafe"       // import "unsafe"
	RuleCgo         = "cgo"          // import "C", #cgo
	RuleLinkname    = "linkname"     // //go:linkname
	RuleDirective   = "directive"    // //go:generate and other compiler directives that reach outside Go's rules
	RuleSourceFile  = "source-file"  // assembly, C and other non-Go sources, object files
	RuleImport      = "import"       // a package outside the allowlists
	RuleForbidden   = "forbidden"    // a call or use of a function the app may not use
	RuleModule      = "module"       // go.mod: a module that is not allowed, replace/exclude
	RuleNoGoMod     = "go-mod"       // Go code without a go.mod, or a go.mod that cannot be read
	RuleParse       = "parse"        // a Go file that does not parse
	RuleSize        = "size"         // too many or too large files
	RuleBuildIgnore = "build-ignore" // a file the build tags would leave out (it is still audited)
	RuleAssign      = "assign"       // an assignment to another package's variable (taking over the base's behaviour)
	RuleFile        = "file"         // a link or other file that is not a regular file
)

// Finding is one thing the audit refuses
type Finding struct {
	Rule   string `json:"rule"`
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Key    string `json:"key"`    // what was found: the import path, the function name … (exceptions match it)
	Detail string `json:"detail"` // why it is refused, for people
	// Excepted: a SUPER user approved this finding for this version of the app
	Excepted bool `json:"excepted,omitempty"`
}

// Report is the result of an audit
type Report struct {
	App      string    `json:"app"`
	Findings []Finding `json:"findings"`
	// Notes: what the audit saw that is not the app's to answer for (printed, never refused)
	Notes []string `json:"notes,omitempty"`
}

// Passed: nothing refused (findings that are excepted do not count)
func (r *Report) Passed() bool {
	for _, f := range r.Findings {
		if !f.Excepted {
			return false
		}
	}
	return true
}

// Excepted: the number of findings passed by an exception
func (r *Report) Excepted() int {
	n := 0
	for _, f := range r.Findings {
		if f.Excepted {
			n++
		}
	}
	return n
}

// Text is the report for people: a line per finding
func (r *Report) Text() string {
	var b strings.Builder
	switch {
	case len(r.Findings) == 0:
		fmt.Fprintf(&b, "App audit of %s: passed.\n", r.App)
	case r.Passed():
		fmt.Fprintf(&b, "App audit of %s: passed with %d approved exception(s).\n", r.App, r.Excepted())
	default:
		fmt.Fprintf(&b, "App audit of %s: FAILED — %d finding(s) refused.\n", r.App, len(r.Findings)-r.Excepted())
	}
	for _, f := range r.Findings {
		where := f.File
		if f.Line > 0 {
			where += ":" + strconv.Itoa(f.Line)
		}
		mark := "REFUSED "
		if f.Excepted {
			mark = "approved"
		}
		fmt.Fprintf(&b, "  %s [%s] %s: %s (%s)\n", mark, f.Rule, where, f.Detail, f.Key)
	}
	for _, n := range r.Notes {
		fmt.Fprintf(&b, "  note: %s\n", n)
	}
	return b.String()
}

// Exception is a SUPER user's approval of one finding (rule and key) for one version of an app
type Exception struct {
	Rule string
	Key  string
}

// Options are what an audit may allow beyond the rules
type Options struct {
	// AllowedModules: Go modules (path, or path@version) the app's go.mod may require besides
	// the base (App Build Setup)
	AllowedModules []string
	// Exceptions approved for this app version
	Exceptions []Exception
}

// limits of an app's code
const (
	maxGoFiles   = 500
	maxGoFileLen = 1 << 20
)

// Audit checks an app's files (path in the app's folder, slash-separated → content)
func Audit(app string, files map[string][]byte, opts Options) *Report {
	r := &Report{App: app}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	goFiles := 0
	hasGo := false
	for _, name := range names {
		base := path.Base(name)
		ext := strings.ToLower(path.Ext(base))
		if reason, ok := forbiddenSourceExt[ext]; ok {
			r.add(Finding{Rule: RuleSourceFile, File: name, Key: ext, Detail: reason})
			continue
		}
		if ext != ".go" {
			continue
		}
		if strings.HasSuffix(base, "_test.go") {
			continue // tests are not built into the server
		}
		hasGo = true
		goFiles++
		if goFiles > maxGoFiles || len(files[name]) > maxGoFileLen {
			r.add(Finding{Rule: RuleSize, File: name, Key: "size", Detail: "the app has too much Go code (at most 500 files of 1 MB)"})
			continue
		}
		auditGoFile(r, name, files[name])
	}
	if hasGo {
		auditGoMod(r, files, opts.AllowedModules)
	}
	r.applyExceptions(opts.Exceptions)
	return r
}

func (r *Report) add(f Finding) {
	r.Findings = append(r.Findings, f)
}

func (r *Report) applyExceptions(exceptions []Exception) {
	for i := range r.Findings {
		for _, e := range exceptions {
			if e.Rule == r.Findings[i].Rule && e.Key == r.Findings[i].Key {
				r.Findings[i].Excepted = true
			}
		}
	}
}

// forbiddenSourceExt: files the Go tool would compile or link besides Go code
var forbiddenSourceExt = map[string]string{
	".s": "assembly is compiled into the server unchecked", ".S": "assembly is compiled into the server unchecked",
	".sx": "assembly is compiled into the server unchecked",
	".c":  "C code (cgo) runs outside Go's rules", ".cc": "C++ code (cgo) runs outside Go's rules",
	".cpp": "C++ code (cgo) runs outside Go's rules", ".cxx": "C++ code (cgo) runs outside Go's rules",
	".h": "C headers (cgo)", ".hh": "C headers (cgo)", ".hpp": "C headers (cgo)", ".hxx": "C headers (cgo)",
	".m": "Objective-C (cgo)", ".f": "Fortran (cgo)", ".f90": "Fortran (cgo)", ".for": "Fortran (cgo)",
	".syso": "an object file is linked into the server unchecked", ".swig": "SWIG (cgo)", ".swigcxx": "SWIG (cgo)",
}

// auditGoFile checks one Go file's directives, imports and the names it uses
func auditGoFile(r *Report, name string, src []byte) {
	auditDirectives(r, name, src)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		r.add(Finding{Rule: RuleParse, File: name, Key: "parse", Detail: "the file does not parse: " + err.Error()})
		return
	}
	if constraint := buildConstraint(file); constraint != "" {
		r.add(Finding{Rule: RuleBuildIgnore, File: name, Line: 1, Key: constraint,
			Detail: "build constraints are not allowed: every file of an app is built (and audited)"})
	}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		line := fset.Position(spec.Pos()).Line
		switch {
		case importPath == "unsafe":
			r.add(Finding{Rule: RuleUnsafe, File: name, Line: line, Key: "unsafe", Detail: "unsafe gets around Go's type and memory safety"})
		case importPath == "C":
			r.add(Finding{Rule: RuleCgo, File: name, Line: line, Key: "C", Detail: "cgo runs C code outside Go's rules"})
		default:
			if reason := importRefused(importPath); reason != "" {
				r.add(Finding{Rule: RuleImport, File: name, Line: line, Key: importPath, Detail: reason})
			}
		}
	}
	// the names the file gives its imports (an assignment to pkg.X changes another package)
	imported := map[string]bool{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		local := path.Base(importPath)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		imported[local] = true
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				target, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				pkg, ok := target.X.(*ast.Ident)
				if ok && imported[pkg.Name] {
					r.add(Finding{Rule: RuleAssign, File: name, Line: fset.Position(target.Pos()).Line, Key: pkg.Name + "." + target.Sel.Name,
						Detail: "an app may not change another package's variables (it would take over the base's behaviour)"})
				}
			}
		}
		if inc, ok := n.(*ast.IncDecStmt); ok {
			if target, ok := inc.X.(*ast.SelectorExpr); ok {
				if pkg, ok := target.X.(*ast.Ident); ok && imported[pkg.Name] {
					r.add(Finding{Rule: RuleAssign, File: name, Line: fset.Position(target.Pos()).Line, Key: pkg.Name + "." + target.Sel.Name,
						Detail: "an app may not change another package's variables (it would take over the base's behaviour)"})
				}
			}
		}
		if unary, ok := n.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			if target, ok := unary.X.(*ast.SelectorExpr); ok {
				if pkg, ok := target.X.(*ast.Ident); ok && imported[pkg.Name] {
					r.add(Finding{Rule: RuleAssign, File: name, Line: fset.Position(target.Pos()).Line, Key: pkg.Name + "." + target.Sel.Name,
						Detail: "an app may not take the address of another package's variable (it could change it)"})
				}
			}
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if reason, forbidden := forbiddenNames[sel.Sel.Name]; forbidden {
			r.add(Finding{Rule: RuleForbidden, File: name, Line: fset.Position(sel.Sel.Pos()).Line, Key: sel.Sel.Name, Detail: reason})
		}
		return true
	})
}

// buildConstraint: the file's //go:build or // +build line ("" none)
func buildConstraint(file *ast.File) string {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, c := range group.List {
			text := strings.TrimSpace(c.Text)
			if strings.HasPrefix(text, "//go:build") || strings.HasPrefix(text, "// +build") {
				return text
			}
		}
	}
	return ""
}

// auditDirectives finds the compiler directives in the file's lines (also in a file that does
// not parse)
func auditDirectives(r *Report, name string, src []byte) {
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), maxGoFileLen)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(text, "//go:linkname"):
			r.add(Finding{Rule: RuleLinkname, File: name, Line: line, Key: "go:linkname", Detail: "//go:linkname reaches the base's private functions"})
		case strings.HasPrefix(text, "#cgo") || strings.HasPrefix(text, "//go:cgo_"):
			r.add(Finding{Rule: RuleCgo, File: name, Line: line, Key: "cgo", Detail: "cgo runs C code outside Go's rules"})
		case strings.HasPrefix(text, "//go:"):
			directive := strings.Fields(text)[0]
			if !allowedDirectives[directive] {
				r.add(Finding{Rule: RuleDirective, File: name, Line: line, Key: strings.TrimPrefix(directive, "//"),
					Detail: "this compiler directive is not allowed in an app (go:generate runs commands; others get around Go's rules)"})
			}
		}
	}
}

// allowedDirectives: compiler directives an app may use (//go:build is reported as
// build-ignore, //go:embed reads only the app's own files)
var allowedDirectives = map[string]bool{"//go:build": true, "//go:embed": true, "//go:noinline": true}

// importRefused: why an app may not import the package ("" allowed)
func importRefused(importPath string) string {
	if strings.HasPrefix(importPath, BaseModule+"/") {
		inBase := strings.TrimPrefix(importPath, BaseModule+"/")
		for _, prefix := range baseDenied {
			if inBase == prefix || strings.HasPrefix(inBase, prefix+"/") {
				return "this part of GoEasyERP is not part of the app API (it reaches the database, secrets or the server's internals)"
			}
		}
		for _, prefix := range baseAllowed {
			if inBase == prefix || strings.HasPrefix(inBase, prefix+"/") {
				return ""
			}
		}
		return "this part of GoEasyERP is not part of the app API"
	}
	if importPath == BaseModule {
		return "this part of GoEasyERP is not part of the app API"
	}
	if isStandardLibrary(importPath) {
		if stdAllowed[importPath] {
			return ""
		}
		if reason, ok := stdDenied[importPath]; ok {
			return reason
		}
		for prefix, reason := range stdDeniedPrefixes {
			if strings.HasPrefix(importPath, prefix) {
				return reason
			}
		}
		return "this standard library package is not on the app allowlist"
	}
	return "" // another module: allowed only when go.mod may require it (checked there)
}

// isStandardLibrary: a standard library path has no dot in its first element
func isStandardLibrary(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}
