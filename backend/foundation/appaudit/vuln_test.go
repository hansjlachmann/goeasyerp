package appaudit

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

const appGoModWithText = `module example.com/vulnapp

go 1.26.0

require (
	github.com/hansjlachmann/goeasyerp v0.0.0
	golang.org/x/text v0.3.5
)
`

const baseGoMod = `module github.com/hansjlachmann/goeasyerp

go 1.26.0

require (
	github.com/shopspring/decimal v1.4.0
	golang.org/x/text v0.31.0 // indirect
)
`

// govulncheck's output (testdata/govulncheck.json, a real run trimmed): the app calls
// language.Parse of golang.org/x/text v0.3.5 (GO-2021-0113); GO-2022-1059 is in a package it
// imports but not called; GO-2026-4602 is in the standard library
func TestVulnReport(t *testing.T) {
	out, err := os.ReadFile("testdata/govulncheck.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := VulnReport("Vuln App 1.0.0", out, []byte(baseGoMod))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Findings) != 1 || r.Findings[0].Key != "GO-2021-0113" || r.Findings[0].Rule != RuleVulnerability ||
		r.Findings[0].File != "golang.org/x/text@v0.3.5" || !strings.Contains(r.Findings[0].Detail, "golang.org/x/text/language.Parse") ||
		!strings.Contains(r.Findings[0].Detail, "fixed in v0.3.7") {
		t.Fatalf("findings: %+v", r.Findings)
	}
	if r.Passed() || !strings.Contains(r.Text(), "FAILED") {
		t.Errorf("passed: %s", r.Text())
	}

	// the same module at the version the base requires is the base's
	r, err = VulnReport("Vuln App 1.0.0", out, []byte(strings.Replace(baseGoMod, "v0.31.0", "v0.3.5", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Findings) != 0 || len(r.Notes) != 1 || !strings.Contains(r.Notes[0], "1 known vulnerability") {
		t.Errorf("a vulnerability of the base's module version: %+v %v", r.Findings, r.Notes)
	}
}

// A vulnerability the app calls in the standard library is left out, one in the base is the base's
func TestVulnReportStandardLibrary(t *testing.T) {
	out := `{"finding":{"osv":"GO-2026-1","trace":[{"module":"stdlib","version":"v1.26.0","package":"net/url","function":"Parse"},{"module":"example.com/vulnapp","function":"F"}]}}
{"finding":{"osv":"GO-2026-2","trace":[{"module":"github.com/hansjlachmann/goeasyerp","package":"github.com/hansjlachmann/goeasyerp/backend/foundation/types","function":"NewCode"}]}}
{"finding":{"osv":"GO-2026-2","trace":[{"module":"github.com/hansjlachmann/goeasyerp","package":"github.com/hansjlachmann/goeasyerp/backend/foundation/types","function":"NewText"}]}}`
	r, err := VulnReport("App", []byte(out), []byte(baseGoMod))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed() || len(r.Findings) != 0 || len(r.Notes) != 1 || !strings.Contains(r.Notes[0], "1 known vulnerability") {
		t.Errorf("report: %s", r.Text())
	}
	if _, err := VulnReport("App", []byte("{not json"), nil); err == nil {
		t.Error("broken output accepted")
	}
}

// An app that requires only the base is not scanned; one with another module is, and an
// approved exception passes the finding
func TestAuditVulnerabilities(t *testing.T) {
	out, err := os.ReadFile("testdata/govulncheck.json")
	if err != nil {
		t.Fatal(err)
	}
	ran := 0
	run := func(ctx context.Context, dir string) ([]byte, error) {
		ran++
		return out, nil
	}
	onlyBase := "module example.com/a\n\ngo 1.26.0\n\nrequire github.com/hansjlachmann/goeasyerp v0.0.0\n"
	r, err := AuditVulnerabilities(context.Background(), "App", "dir", []byte(onlyBase), []byte(baseGoMod), Options{}, run)
	if err != nil || r != nil || ran != 0 {
		t.Fatalf("an app of the base alone: %v, %v, ran %d", r, err, ran)
	}
	if got := ExtraModules([]byte(appGoModWithText)); len(got) != 1 || got[0] != "golang.org/x/text" {
		t.Errorf("ExtraModules = %v", got)
	}
	r, err = AuditVulnerabilities(context.Background(), "App", "dir", []byte(appGoModWithText), []byte(baseGoMod), Options{}, run)
	if err != nil || r == nil || r.Passed() || ran != 1 {
		t.Fatalf("an app with golang.org/x/text: %v, %v", r, err)
	}
	r, err = AuditVulnerabilities(context.Background(), "App", "dir", []byte(appGoModWithText), []byte(baseGoMod),
		Options{Exceptions: []Exception{{Rule: RuleVulnerability, Key: "GO-2021-0113"}}}, run)
	if err != nil || !r.Passed() || r.Excepted() != 1 {
		t.Errorf("with the exception: %v, %v", r, err)
	}
	failing := func(ctx context.Context, dir string) ([]byte, error) { return nil, errors.New("no network") }
	if _, err := AuditVulnerabilities(context.Background(), "App", "dir", []byte(appGoModWithText), []byte(baseGoMod), Options{}, failing); err == nil {
		t.Error("a failed govulncheck run passed")
	}
}
