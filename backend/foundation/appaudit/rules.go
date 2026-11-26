package appaudit

import (
	"fmt"
	"strings"
)

// The app API: what an app's Go code may use. The lists grow with the app API — a package or
// function an app legitimately needs is added here (in the base's repository, reviewed), or
// approved for one app version as an exception.

// baseAllowed: the parts of GoEasyERP an app may import (path prefixes in the base module)
var baseAllowed = []string{
	"backend/generated/tables",       // the records: Get, Find, Insert, Modify, triggers …
	"backend/business-logic",         // business functions and events (salesposting.OnAfterPost …), but see baseDenied
	"backend/foundation/types",       // Code, Text, Decimal, Date …
	"backend/foundation/system",      // the classic functions (CalcDate, Round, StrSubstNo …)
	"backend/foundation/functions",   // page functions
	"backend/foundation/events",      // events
	"backend/foundation/codeunits",   // Result, dialogs (Message, Confirm, RequestInput), the run's user and language
	"backend/foundation/errors",      // translated errors
	"backend/foundation/i18n",        // translated texts
	"backend/foundation/format",      // dates and amounts for people
	"backend/foundation/dateformula", // date formulas
	"backend/foundation/flowfilter",  // FlowFilter expressions
}

// baseDenied: parts of GoEasyERP that would give an app the server's powers (checked before
// baseAllowed)
var baseDenied = []string{
	"backend/business-logic/apps",           // installing, enabling apps
	"backend/business-logic/migrations",     // schema changes
	"backend/business-logic/scheduler",      // runs jobs as other users
	"backend/business-logic/themerepo",      // downloads from GitHub
	"backend/business-logic/demodata",       // replaces a company's data
	"backend/business-logic/jobqueue",       // the mailer and SMTP settings
	"backend/business-logic/docmail",        // sends e-mail
	"backend/business-logic/refusedposting", // writes outside the transaction
	"backend/foundation/database",
	"backend/foundation/tables",
	"backend/foundation/secrets",
	"backend/foundation/session",
	"backend/foundation/mail",
	"backend/foundation/securitylog",
	"backend/foundation/themes",
	"backend/foundation/apps",
	"backend/foundation/appbundle",
	"backend/foundation/appaudit",
	"backend/foundation/extensions",
	"backend/foundation/company",
	"backend/foundation/sift",
	"backend/foundation/migrations",
	"backend/foundation/objects",
	"backend/foundation/pdf",
	"backend/api",
	"cmd",
	"tools",
}

// stdAllowed: standard library packages an app may import — computation only, no files,
// environment, processes or network
var stdAllowed = map[string]bool{
	"bufio": true, "bytes": true, "cmp": true, "context": true, "errors": true, "fmt": true,
	"io": true, "iter": true, "maps": true, "math": true, "math/big": true, "math/bits": true,
	"math/rand": true, "math/rand/v2": true, "regexp": true, "slices": true, "sort": true,
	"strconv": true, "strings": true, "sync": true, "sync/atomic": true, "time": true,
	"unicode": true, "unicode/utf8": true, "unicode/utf16": true, "html": true,
	"encoding/json": true, "encoding/csv": true, "encoding/base64": true, "encoding/hex": true,
	"encoding/xml": true, "encoding/binary": true,
	"hash": true, "hash/crc32": true, "hash/fnv": true,
	"crypto/sha256": true, "crypto/sha512": true, "crypto/sha1": true, "crypto/md5": true,
	"net/url": true, // parsing only
}

// stdDenied: refused standard library packages, with the reason
var stdDenied = map[string]string{
	"os":            "os reads the environment (the server's secrets) and files",
	"os/exec":       "os/exec runs programs on the server",
	"os/signal":     "os/signal reaches the server process",
	"os/user":       "os/user reads the server's users",
	"syscall":       "syscall calls the operating system directly",
	"plugin":        "plugin loads code at run time",
	"reflect":       "reflect gets around Go's type rules (private fields and methods)",
	"runtime":       "runtime reaches the server process's internals",
	"database/sql":  "database/sql reaches the database behind the base's checks",
	"io/ioutil":     "io/ioutil reads and writes files",
	"io/fs":         "io/fs reads files",
	"path/filepath": "path/filepath works on the server's files",
	"embed":         "embed: use //go:embed only through the base's API (not needed by apps)",
	"log":           "log writes to the server's log (use errors and messages)",
	"text/template": "templates can call functions given to them: not on the app allowlist yet",
	"html/template": "templates can call functions given to them: not on the app allowlist yet",
	"crypto/rand":   "not on the app allowlist yet",
	"crypto/tls":    "crypto/tls opens network connections",
}

// stdDeniedPrefixes: refused standard library trees
var stdDeniedPrefixes = map[string]string{
	"net/":      "the network: an app does not open connections",
	"net":       "the network: an app does not open connections",
	"runtime/":  "runtime reaches the server process's internals",
	"debug/":    "debug reads binaries and the process",
	"internal/": "internal packages of the standard library",
	"go/":       "the Go toolchain's packages",
	"os/":       "the operating system",
	"syscall/":  "syscall calls the operating system directly",
	"archive/":  "not on the app allowlist yet",
	"compress/": "not on the app allowlist yet",
	"crypto/":   "not on the app allowlist yet",
	"database/": "the database behind the base's checks",
	"testing":   "testing belongs in _test.go files",
	"expvar":    "expvar publishes the process's variables",
	"image":     "not on the app allowlist yet",
	"mime":      "not on the app allowlist yet",
	"log/":      "writes to the server's log",
}

// forbiddenNames: functions and methods an app may not call or use, whatever their package —
// the selector name is enough, as an app cannot reach a function without naming it (unsafe,
// reflect and linkname being refused)
var forbiddenNames = map[string]string{
	"GetDB":               "the record's database connection: SQL behind the base's checks, every company",
	"SetDB":               "puts a record on another database connection",
	"Exec":                "runs SQL on the database",
	"ExecContext":         "runs SQL on the database",
	"Query":               "runs SQL on the database",
	"QueryContext":        "runs SQL on the database",
	"QueryRow":            "runs SQL on the database",
	"QueryRowContext":     "runs SQL on the database",
	"Begin":               "opens a database transaction of its own",
	"BeginTx":             "opens a database transaction of its own",
	"Conn":                "a raw database connection",
	"ChangeCompany":       "reads and writes another company's data",
	"InitWithDBType":      "opens a table on a database the app chooses",
	"InitTemporary":       "opens a table on a database the app chooses",
	"SetCurrentAccess":    "takes another user's rights",
	"SetCurrentContext":   "takes another user's identity",
	"ClearCurrentContext": "drops the run's user",
	"SetCurrentWorkDate":  "changes the run's work date behind the user",
	"SetCurrentDialog":    "takes over the run's dialog",
	"Consistent":          "marks a table consistent: gets around the G/L balance check",
	"DBSource":            "a record source on a database the app chooses",
	"Getenv":              "reads the server's environment (secrets)",
	"LookupEnv":           "reads the server's environment (secrets)",
	"Environ":             "reads the server's environment (secrets)",
}

// auditGoMod checks the app's go.mod: it requires only the base (and modules App Build Setup
// allows), with no replace or exclude directives
func auditGoMod(r *Report, files map[string][]byte, allowed []string) {
	data, ok := files["go.mod"]
	if !ok {
		r.add(Finding{Rule: RuleNoGoMod, File: "go.mod", Key: "go.mod", Detail: "an app with Go code needs a go.mod at its root"})
		return
	}
	requires, directives := parseGoMod(string(data))
	for _, d := range directives {
		r.add(Finding{Rule: RuleModule, File: "go.mod", Line: d.line, Key: d.name,
			Detail: d.name + " directives are not allowed: the build decides where the base comes from"})
	}
	for _, req := range requires {
		if req.path == BaseModule || moduleAllowed(req.path, req.version, allowed) {
			continue
		}
		r.add(Finding{Rule: RuleModule, File: "go.mod", Line: req.line, Key: req.path,
			Detail: "the module is not allowed (App Build Setup: Allowed Go Modules): its code would be built into the server unaudited"})
	}
}

// moduleAllowed: path (at version) is on the list ("path" any version, "path@version" that one)
func moduleAllowed(path, version string, allowed []string) bool {
	for _, a := range allowed {
		a = strings.TrimSpace(a)
		if a == path || a == path+"@"+version {
			return true
		}
	}
	return false
}

type goModRequire struct {
	path, version string
	line          int
}

type goModDirective struct {
	name string
	line int
}

// parseGoMod reads the require lines and the replace/exclude/retract/tool/godebug directives of
// a go.mod (single lines and blocks; comments ignored)
func parseGoMod(text string) ([]goModRequire, []goModDirective) {
	var requires []goModRequire
	var directives []goModDirective
	block := ""
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if c := strings.Index(line, "//"); c >= 0 {
			line = strings.TrimSpace(line[:c])
		}
		if line == "" {
			continue
		}
		if block != "" {
			if line == ")" {
				block = ""
				continue
			}
			handleGoModEntry(block, line, i+1, &requires, &directives)
			continue
		}
		fields := strings.Fields(line)
		verb := fields[0]
		rest := strings.TrimSpace(strings.TrimPrefix(line, verb))
		if rest == "(" {
			block = verb
			continue
		}
		handleGoModEntry(verb, rest, i+1, &requires, &directives)
	}
	return requires, directives
}

func handleGoModEntry(verb, entry string, line int, requires *[]goModRequire, directives *[]goModDirective) {
	switch verb {
	case "require":
		fields := strings.Fields(entry)
		if len(fields) >= 1 {
			req := goModRequire{path: strings.Trim(fields[0], `"`), line: line}
			if len(fields) >= 2 {
				req.version = fields[1]
			}
			*requires = append(*requires, req)
		}
	case "replace", "exclude", "retract", "tool", "godebug", "ignore":
		*directives = append(*directives, goModDirective{name: verb, line: line})
	case "module", "go", "toolchain":
	default:
		*directives = append(*directives, goModDirective{name: verb, line: line})
	}
}

// String is a finding as one line
func (f Finding) String() string {
	return fmt.Sprintf("[%s] %s:%d %s (%s)", f.Rule, f.File, f.Line, f.Detail, f.Key)
}
