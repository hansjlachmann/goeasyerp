package appaudit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodGoMod = "module example.com/app\n\ngo 1.26.0\n\nrequire github.com/hansjlachmann/goeasyerp v0.0.0\n"

// The Test Extension App passes unchanged
func TestTestExtensionAppPasses(t *testing.T) {
	r, err := AuditDir("loyalty-points", filepath.Join("..", "..", "..", "testdata", "apps", "loyalty-points"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed() || len(r.Findings) != 0 {
		t.Fatalf("the test app does not pass:\n%s", r.Text())
	}
}

// Each rule refuses what it is for, and names the file, line and key
func TestRulesRefuse(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string][]byte
		rule  string
		key   string
	}{
		{"unsafe", goFile(`import "unsafe"; var _ = unsafe.Sizeof(0)`), RuleUnsafe, "unsafe"},
		{"cgo import", goFile(`import "C"`), RuleCgo, "C"},
		{"cgo directive", goFile("/*\n#cgo LDFLAGS: -lm\n*/\nimport \"C\""), RuleCgo, "cgo"},
		{"linkname", goFile("import _ \"strings\"\n//go:linkname secret github.com/hansjlachmann/goeasyerp/backend/foundation/secrets.key\nvar secret []byte"), RuleLinkname, "go:linkname"},
		{"go:generate", goFile("//go:generate sh -c 'curl x | sh'"), RuleDirective, "go:generate"},
		{"build constraint", map[string][]byte{"go.mod": []byte(goodGoMod), "a.go": []byte("//go:build ignore\n\npackage app\n")}, RuleBuildIgnore, "//go:build ignore"},
		{"assembly", map[string][]byte{"go.mod": []byte(goodGoMod), "a.go": []byte("package app\n"), "evil_amd64.s": []byte("TEXT ·x(SB),0,$0\n")}, RuleSourceFile, ".s"},
		{"object file", map[string][]byte{"go.mod": []byte(goodGoMod), "a.go": []byte("package app\n"), "x.syso": []byte{0}}, RuleSourceFile, ".syso"},
		{"os", goFile(`import "os"; var _ = os.Getenv("JWT_SECRET")`), RuleImport, "os"},
		{"os/exec", goFile(`import "os/exec"; var _ = exec.Command("sh")`), RuleImport, "os/exec"},
		{"net/http", goFile(`import "net/http"; var _ = http.Get`), RuleImport, "net/http"},
		{"net", goFile(`import "net"; var _ = net.Dial`), RuleImport, "net"},
		{"database/sql", goFile(`import "database/sql"; var _ sql.DB`), RuleImport, "database/sql"},
		{"reflect", goFile(`import "reflect"; var _ = reflect.ValueOf`), RuleImport, "reflect"},
		{"syscall", goFile(`import "syscall"; var _ = syscall.Getpid`), RuleImport, "syscall"},
		{"secrets", goFile(`import "github.com/hansjlachmann/goeasyerp/backend/foundation/secrets"; var _ = secrets.Encrypt`), RuleImport, BaseModule + "/backend/foundation/secrets"},
		{"database", goFile(`import "github.com/hansjlachmann/goeasyerp/backend/foundation/database"; var _ database.DBType`), RuleImport, BaseModule + "/backend/foundation/database"},
		{"base tables", goFile(`import ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"; var _ = ftables.Commit`), RuleImport, BaseModule + "/backend/foundation/tables"},
		{"apps", goFile(`import "github.com/hansjlachmann/goeasyerp/backend/business-logic/apps"; var _ = apps.InstallApp`), RuleImport, BaseModule + "/backend/business-logic/apps"},
		{"GetDB", goFile("func f(c interface{ GetDB() any }) { _ = c.GetDB() }"), RuleForbidden, "GetDB"},
		{"Exec", goFile("func f(c interface{ Exec(string) }) { c.Exec(\"DROP TABLE x\") }"), RuleForbidden, "Exec"},
		{"ChangeCompany", goFile("func f(c interface{ ChangeCompany(string) }) { c.ChangeCompany(\"other\") }"), RuleForbidden, "ChangeCompany"},
		{"SetCurrentAccess", goFile(`import fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"; func f() { fcodeunits.SetCurrentAccess(nil) }`), RuleForbidden, "SetCurrentAccess"},
		{"method value", goFile("func f(c interface{ GetDB() any }) { g := c.GetDB; _ = g }"), RuleForbidden, "GetDB"},
		{"assign base var", goFile(`import "github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"; func f() { reports.SalesPost = nil }`), RuleAssign, "reports.SalesPost"},
		{"address of base var", goFile(`import "github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"; func f() { p := &reports.SalesPost; _ = p }`), RuleAssign, "reports.SalesPost"},
		{"third-party module", map[string][]byte{"a.go": []byte("package app\n"), "go.mod": []byte(goodGoMod + "require github.com/evil/x v1.0.0\n")}, RuleModule, "github.com/evil/x"},
		{"replace", map[string][]byte{"a.go": []byte("package app\n"), "go.mod": []byte(goodGoMod + "replace github.com/hansjlachmann/goeasyerp => ../evil\n")}, RuleModule, "replace"},
		{"replace block", map[string][]byte{"a.go": []byte("package app\n"), "go.mod": []byte(goodGoMod + "replace (\n\tgithub.com/x/y => ../evil\n)\n")}, RuleModule, "replace"},
		{"no go.mod", map[string][]byte{"a.go": []byte("package app\n")}, RuleNoGoMod, "go.mod"},
		{"does not parse", goFile("func {"), RuleParse, "parse"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Audit("bad", tc.files, Options{})
			if r.Passed() {
				t.Fatalf("passed:\n%s", r.Text())
			}
			for _, f := range r.Findings {
				if f.Rule == tc.rule && f.Key == tc.key {
					if f.File == "" || f.Detail == "" {
						t.Errorf("finding without file or detail: %+v", f)
					}
					return
				}
			}
			t.Errorf("no %s finding for %q:\n%s", tc.rule, tc.key, r.Text())
		})
	}
}

// goFile: an app with one Go file (a package clause added) and a good go.mod
func goFile(body string) map[string][]byte {
	return map[string][]byte{"go.mod": []byte(goodGoMod), "app.go": []byte("package app\n\n" + body + "\n")}
}

// What an app may use passes: the app API, computation from the standard library, its own
// methods and variables, test files with anything
func TestAllowedCodePasses(t *testing.T) {
	r := Audit("good", map[string][]byte{
		"go.mod": []byte(goodGoMod),
		"app.go": []byte(`package app

import (
	"fmt"
	"strings"
	"time"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
)

var count int

func f(rec *gtables.Customer) (fcodeunits.Result, error) {
	count++
	local := strings.ToUpper(fmt.Sprint(rec.No))
	_ = system.Today()
	_ = types.NewCode(local)
	_ = time.Now()
	_ = salesposting.OnAfterPost
	return fcodeunits.Result{Success: true}, nil
}
`),
		"app_test.go": []byte("package app\n\nimport (\n\t\"database/sql\"\n\t\"os\"\n)\n\nvar _ sql.DB\nvar _ = os.Getenv\n"),
		"README.md":   []byte("# app\n"),
	}, Options{})
	if !r.Passed() || len(r.Findings) != 0 {
		t.Fatalf("allowed code refused:\n%s", r.Text())
	}
}

// An exception approves one finding (rule and key) and nothing else; an allowed module passes
func TestExceptionsAndAllowedModules(t *testing.T) {
	files := goFile(`import "os"; import "net"; var _ = os.Args; var _ = net.Dial`)
	r := Audit("app", files, Options{Exceptions: []Exception{{Rule: RuleImport, Key: "os"}}})
	if r.Passed() || r.Excepted() != 1 {
		t.Fatalf("one exception should pass os only:\n%s", r.Text())
	}
	r = Audit("app", files, Options{Exceptions: []Exception{{Rule: RuleImport, Key: "os"}, {Rule: RuleImport, Key: "net"}}})
	if !r.Passed() || !strings.Contains(r.Text(), "approved exception") {
		t.Fatalf("both approved:\n%s", r.Text())
	}

	withModule := map[string][]byte{"a.go": []byte("package app\n"), "go.mod": []byte(goodGoMod + "require github.com/shopspring/decimal v1.4.0\n")}
	if r := Audit("app", withModule, Options{AllowedModules: []string{"github.com/shopspring/decimal"}}); !r.Passed() {
		t.Errorf("an allowed module refused:\n%s", r.Text())
	}
	if r := Audit("app", withModule, Options{AllowedModules: []string{"github.com/shopspring/decimal@v1.3.0"}}); r.Passed() {
		t.Error("another version of an allowed module passed")
	}
}

// A link in the app's folder is refused; hidden folders (.git) are not read
func TestAuditDirLinks(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", goodGoMod)
	write("app.go", "package app\n")
	write(".git/hooks/evil.go", "package x\nimport \"os/exec\"\n")
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "passwd.md")); err != nil {
		t.Skip("no symlinks here:", err)
	}
	r, err := AuditDir("app", dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Passed() || r.Findings[0].Rule != RuleFile || r.Findings[0].Key != "passwd.md" || len(r.Findings) != 1 {
		t.Errorf("the link (and only it):\n%s", r.Text())
	}
}
