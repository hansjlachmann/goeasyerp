package appaudit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
)

// RuleVulnerability: the app's code calls a function with a known vulnerability in a module the
// app brings in (govulncheck); the key is the vulnerability's ID (GO-2026-1234)
const RuleVulnerability = "vulnerability"

// Known vulnerabilities (S55 rule 5). The check runs govulncheck on the app in a merged build
// tree (go.work joining the base and the app, the table code generated), because the app's code
// compiles only there. It is needed only for an app whose go.mod requires modules besides the
// base (App Build Setup's Allowed Go Modules): every other module the app's code reaches is the
// base's or the standard library's, which the base's own security scan covers in every release —
// and govulncheck on the whole program takes a minute and gigabytes. Vulnerabilities govulncheck
// finds in the base's modules (at the version the base requires) are the base's: counted in a
// note, never a finding of the app; those in the standard library are left out (the base's
// release scan checks the Go version its images are built with).

// VulnRunner runs govulncheck in the app's folder and returns its JSON output (-format json)
type VulnRunner func(ctx context.Context, dir string) ([]byte, error)

// Govulncheck runs the govulncheck program on PATH (golang.org/x/vuln/cmd/govulncheck)
func Govulncheck(ctx context.Context, dir string) ([]byte, error) {
	if _, err := exec.LookPath("govulncheck"); err != nil {
		return nil, fmt.Errorf("govulncheck is not installed (go install golang.org/x/vuln/cmd/govulncheck@v1.10.0): %w", err)
	}
	cmd := exec.CommandContext(ctx, "govulncheck", "-format", "json", "./...")
	cmd.Dir = dir
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 2000 {
			msg = msg[:2000] + " …"
		}
		return nil, fmt.Errorf("govulncheck: %w: %s", err, msg)
	}
	return out.Bytes(), nil
}

// ExtraModules: the modules an app's go.mod requires besides the base (sorted)
func ExtraModules(goMod []byte) []string {
	requires, _ := parseGoMod(string(goMod))
	var out []string
	for _, r := range requires {
		if r.path != BaseModule {
			out = append(out, r.path)
		}
	}
	sort.Strings(out)
	return out
}

// AuditVulnerabilities checks the app in appDir (in a merged build tree) for known
// vulnerabilities its code calls in the modules it brings in. baseGoMod is the base's go.mod.
// An app whose go.mod requires only the base is not scanned (nil, nil). Exceptions apply as in
// Audit (rule vulnerability, key the vulnerability's ID).
func AuditVulnerabilities(ctx context.Context, app, appDir string, appGoMod, baseGoMod []byte, opts Options, run VulnRunner) (*Report, error) {
	if len(ExtraModules(appGoMod)) == 0 {
		return nil, nil
	}
	out, err := run(ctx, appDir)
	if err != nil {
		return nil, err
	}
	r, err := VulnReport(app, out, baseGoMod)
	if err != nil {
		return nil, err
	}
	r.applyExceptions(opts.Exceptions)
	return r, nil
}

// govulncheck's JSON stream: one object per message; only findings matter here
type vulnMessage struct {
	Finding *struct {
		OSV          string `json:"osv"`
		FixedVersion string `json:"fixed_version"`
		Trace        []struct {
			Module   string `json:"module"`
			Version  string `json:"version"`
			Package  string `json:"package"`
			Function string `json:"function"`
			Receiver string `json:"receiver"`
		} `json:"trace"`
	} `json:"finding"`
}

// VulnReport turns govulncheck's JSON output into a report: a finding per vulnerability the
// app's code calls (the first frame of the trace names a function) in a module that is not the
// base's — the base's modules at the version the base requires only counted in a note, the
// standard library left out
func VulnReport(app string, out, baseGoMod []byte) (*Report, error) {
	baseVersions := map[string]string{}
	requires, _ := parseGoMod(string(baseGoMod))
	for _, req := range requires {
		baseVersions[req.path] = req.version
	}
	r := &Report{App: app + " (known vulnerabilities)"}
	seen := map[string]bool{}
	baseCount := map[string]bool{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var m vulnMessage
		err := dec.Decode(&m)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading govulncheck's output: %w", err)
		}
		f := m.Finding
		if f == nil || len(f.Trace) == 0 || f.Trace[0].Function == "" {
			continue // not called by the app's code (an imported package or a required module only)
		}
		top := f.Trace[0]
		if top.Module == "stdlib" || top.Module == "toolchain" {
			// the Go the images are built with (the latest patch of the release's Go) is the
			// base's: its release scan checks it; govulncheck here may see another patch level
			continue
		}
		base := top.Module == BaseModule
		if v, ok := baseVersions[top.Module]; ok && v == top.Version {
			base = true
		}
		if base {
			baseCount[f.OSV] = true
			continue
		}
		key := f.OSV + "\x00" + top.Module
		if seen[key] {
			continue
		}
		seen[key] = true
		function := top.Function
		if top.Receiver != "" {
			function = strings.TrimPrefix(top.Receiver, "*") + "." + function
		}
		fixed := "no fixed version yet"
		if f.FixedVersion != "" {
			fixed = "fixed in " + f.FixedVersion
		}
		r.add(Finding{Rule: RuleVulnerability, File: top.Module + "@" + top.Version, Key: f.OSV,
			Detail: fmt.Sprintf("the app calls %s.%s, which has a known vulnerability (%s; https://pkg.go.dev/vuln/%s)", top.Package, function, fixed, f.OSV)})
	}
	if len(baseCount) > 0 {
		what := fmt.Sprintf("%d known vulnerabilities", len(baseCount))
		if len(baseCount) == 1 {
			what = "1 known vulnerability"
		}
		r.Notes = append(r.Notes, what+" the app's code reaches in the base's modules: the base's to fix (its own security scan) — keep GoEasyERP up to date")
	}
	return r, nil
}
