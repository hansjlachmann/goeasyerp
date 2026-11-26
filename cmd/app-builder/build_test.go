package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/appbundle"
	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// folderFetcher gives every app from a folder (a fake Git repository)
type folderFetcher struct{ dir string }

func (f folderFetcher) Fetch(_ context.Context, _ fapps.BuildApp, dest string, _ io.Writer) error {
	return copyTree(f.dir, dest, nil)
}

const repoRoot = "../.."

// A build merges the test app into a copy of the base source and builds a bundle: the program
// (verified by its SHA-256), the merged definitions with the app's fields and its
// application.yaml, and build.json
func TestBuildBundle(t *testing.T) {
	if testing.Short() || os.Getenv("GOEASYERP_SKIP_APP_BUILD") != "" {
		t.Skip("builds the server program (CI: in the nightly run; every run builds with the test app in scripts/check-extension-fixture.sh)")
	}
	base, err := fapps.Load(filepath.Join(repoRoot, "backend", "definitions", "application.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(repoRoot, "testdata", "apps", "loyalty-points")
	app, err := fapps.Load(filepath.Join(appDir, "application.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	builds := t.TempDir()
	b := &builder{baseSource: repoRoot, buildsDir: builds, workDir: t.TempDir(), fetch: folderFetcher{dir: appDir}}
	var list fapps.BuildList
	list.Base.Version = base.Version
	list.Apps = []fapps.BuildApp{{ID: app.ID, Name: app.Name, Version: app.Version, Source: fapps.SourceGit, Repository: "https://example.com/x", Ref: "v1"}}
	blog := &buildLog{}
	sha, err := b.build(context.Background(), 7, list, blog)
	if err != nil {
		t.Fatalf("%v\n%s", err, blog.Bytes())
	}
	lines := strings.Split(strings.TrimSpace(string(blog.Bytes())), "\n")
	t.Log(lines[len(lines)-1]) // the steps' times
	dir := appbundle.Dir(builds, 7)
	if err := appbundle.Verify(dir, base.Version, sha); err != nil {
		t.Fatal(err)
	}
	customer, err := os.ReadFile(filepath.Join(dir, "backend", "definitions", "tables", "customer.yaml"))
	if err != nil || !strings.Contains(string(customer), "lp_loyalty_points") {
		t.Errorf("the merged Customer table: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backend", "definitions", "apps", app.ID+".yaml")); err != nil {
		t.Errorf("the app's application.yaml in the bundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "translations", "en-US", "messages.yaml")); err != nil {
		t.Errorf("translations: %v", err)
	}
	if _, err := os.Stat(dir + ".tmp"); !os.IsNotExist(err) {
		t.Error("the temporary bundle folder is left")
	}
	// a changed program is refused
	if err := os.WriteFile(filepath.Join(dir, appbundle.BinaryName), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := appbundle.Verify(dir, base.Version, sha); err == nil {
		t.Error("a changed program was accepted")
	}
}

// A build whose source holds another app or version is refused
func TestBuildChecksTheSource(t *testing.T) {
	appDir := filepath.Join(repoRoot, "testdata", "apps", "loyalty-points")
	app, err := fapps.Load(filepath.Join(appDir, "application.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	b := &builder{baseSource: filepath.Join(repoRoot, "testdata", "apps"), buildsDir: t.TempDir(), workDir: t.TempDir(), fetch: folderFetcher{dir: appDir}}
	var list fapps.BuildList
	list.Apps = []fapps.BuildApp{{ID: app.ID, Name: app.Name, Version: "9.9.9", Source: fapps.SourceGit}}
	_, err = b.build(context.Background(), 1, list, &buildLog{})
	if err == nil || !strings.Contains(err.Error(), "not version 9.9.9") {
		t.Errorf("err = %v", err)
	}
}

// An app that fails the app audit is not merged or compiled: the build fails with the report in
// its log; an exception approved for the finding lets it through to the next step (S55)
func TestBuildAuditsTheApp(t *testing.T) {
	appDir := filepath.Join(repoRoot, "testdata", "apps", "loyalty-points")
	app, err := fapps.Load(filepath.Join(appDir, "application.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	bad := t.TempDir()
	if err := copyTree(appDir, bad, nil); err != nil {
		t.Fatal(err)
	}
	evil := "package loyaltypoints\n\nimport \"os\"\n\nvar secret = os.Getenv(\"JWT_SECRET\")\n"
	if err := os.WriteFile(filepath.Join(bad, "evil.go"), []byte(evil), 0o644); err != nil {
		t.Fatal(err)
	}
	// the base source is not a real one: a build that got past the audit fails at the merge
	b := &builder{baseSource: filepath.Join(repoRoot, "testdata", "apps"), buildsDir: t.TempDir(), workDir: t.TempDir(), fetch: folderFetcher{dir: bad}}
	var list fapps.BuildList
	list.Apps = []fapps.BuildApp{{ID: app.ID, Name: app.Name, Version: app.Version, Source: fapps.SourceGit, Commit: "abc123"}}
	log := &buildLog{}
	_, err = b.build(context.Background(), 1, list, log)
	if err == nil || !strings.Contains(err.Error(), "did not pass the app audit") {
		t.Fatalf("err = %v", err)
	}
	text := string(log.Bytes())
	if !strings.Contains(text, "REFUSED") || !strings.Contains(text, "evil.go:3") || !strings.Contains(text, "source abc123") {
		t.Errorf("the report in the log:\n%s", text)
	}

	list.Apps[0].AuditExceptions = []fapps.AuditException{{Rule: "import", Key: "os"}, {Rule: "forbidden", Key: "Getenv"}}
	_, err = b.build(context.Background(), 2, list, &buildLog{})
	if err == nil || strings.Contains(err.Error(), "app audit") {
		t.Errorf("with the findings approved the audit passes (the build then fails at the merge of this fake base): %v", err)
	}
}

// An uploaded package is unpacked only when it is the one the build was requested with
func TestUnpack(t *testing.T) {
	manifest, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "apps", "loyalty-points", "application.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("application.yaml")
	_, _ = w.Write(manifest)
	_ = zw.Close()
	data := buf.Bytes()
	sum := sha256.Sum256(data)
	dest := filepath.Join(t.TempDir(), "app")
	if err := unpack(data, "0000", dest); err == nil {
		t.Error("a package with another SHA-256 was unpacked")
	}
	if err := unpack(data, hex.EncodeToString(sum[:]), dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "application.yaml")); err != nil {
		t.Error(err)
	}
}

// The log keeps its end when it grows too long and hides secrets
func TestBuildLog(t *testing.T) {
	l := &buildLog{secrets: []string{"s3cret"}}
	l.Printf("cloning https://x-access-token:s3cret@github.com/a/b")
	if strings.Contains(string(l.Bytes()), "s3cret") {
		t.Error("the token is in the log")
	}
	_, _ = l.Write(bytes.Repeat([]byte("a"), 3*logMax))
	l.Printf("the end")
	out := l.Bytes()
	if len(out) > logMax+16 || !strings.HasSuffix(string(out), "the end\n") {
		t.Errorf("log of %d bytes, ends %q", len(out), out[len(out)-10:])
	}
}

// Two builders never take the same request, and a builder takes only the requests for its own
// version: one for another version waits for a builder of that version (PostgreSQL:
// TEST_POSTGRES_DSN)
func TestClaimPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, _ = db.Exec(`DROP TABLE IF EXISTS "App_Build"`)
	if err := (&gtables.AppBuild{}).CreateTableWithDBType(db, "", database.DBTypePostgres); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DROP TABLE IF EXISTS "App_Build"`) })
	src := ftables.DBSource(db, "", database.DBTypePostgres)
	for no, version := range map[int]string{1: "1.0.0", 2: "1.0.1", 3: "1.0.0"} {
		rec := gtables.NewAppBuild(src)
		rec.Init()
		rec.Entry_no = no
		rec.Status = gtables.AppBuild_Status.Requested
		rec.Base_version = types.NewText(version)
		if err := rec.InsertOrError(false); err != nil {
			t.Fatal(err)
		}
	}
	w := &worker{db: db, b: &builder{buildsDir: t.TempDir()}, version: "1.0.0"}
	a, err := w.claim()
	if err != nil {
		t.Fatal(err)
	}
	b, err := w.claim()
	if err != nil {
		t.Fatal(err)
	}
	c, _ := w.claim()
	if a != 1 || b != 3 || c != 0 {
		t.Errorf("claims %d %d %d, want 1 3 0", a, b, c)
	}
	rec := gtables.NewAppBuild(src)
	if !rec.Get(2) || rec.Status != gtables.AppBuild_Status.Requested {
		t.Errorf("the request for another version was taken: %+v", rec)
	}
	other := &worker{db: db, b: w.b, version: "1.0.1"}
	if d, _ := other.claim(); d != 2 {
		t.Errorf("the builder of 1.0.1 claims %d, want 2", d)
	}
	if !rec.Get(1) || rec.Status != gtables.AppBuild_Status.Running {
		t.Errorf("claimed build %+v", rec)
	}
	w.recover()
	if !rec.Get(1) || rec.Status != gtables.AppBuild_Status.Failed {
		t.Errorf("after recover: %+v", rec)
	}
}

// The compile of the generated table code is kept small (one thread, GOGC=25, few parallel
// compiles) unless the operator's environment says otherwise; the package path is the module's
func TestLowMemoryBuild(t *testing.T) {
	envOf := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	args, env := lowMemoryBuild(envOf(nil))
	if strings.Join(args, " ") != "-gcflags="+tablesPackage+"=-c=1 -p=2" || strings.Join(env, " ") != "GOGC=25" {
		t.Errorf("defaults: %v %v", args, env)
	}
	args, env = lowMemoryBuild(envOf(map[string]string{"GOFLAGS": "-p=4 -gcflags=all=-c=2", "GOGC": "100"}))
	if len(args) != 0 || len(env) != 0 {
		t.Errorf("operator settings: %v %v", args, env)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "backend", "generated", "tables")); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tablesPackage, "github.com/hansjlachmann/goeasyerp/") {
		t.Errorf("package %s", tablesPackage)
	}
}

// git ls-remote's output: an annotated tag's commit is its peeled line, a lightweight tag's its
// own line; a missing tag gives ""
func TestTagCommit(t *testing.T) {
	const ref = "refs/tags/v1.0.0"
	annotated := "21d5557e4793918bab97857044982e9def703cbf\trefs/tags/v1.0.0\n69250db6b66b1a974fd9c4d45d34dd5e53b6e4dc\trefs/tags/v1.0.0^{}\n"
	if got := tagCommit(annotated, ref); got != "69250db6b66b1a974fd9c4d45d34dd5e53b6e4dc" {
		t.Errorf("annotated: %q", got)
	}
	if got := tagCommit("69250db6b66b1a974fd9c4d45d34dd5e53b6e4dc\trefs/tags/v1.0.0\n", ref); got != "69250db6b66b1a974fd9c4d45d34dd5e53b6e4dc" {
		t.Errorf("lightweight: %q", got)
	}
	if got := tagCommit("69250db6b66b1a974fd9c4d45d34dd5e53b6e4dc\trefs/tags/v1.0.10\n", ref); got != "" {
		t.Errorf("another tag: %q", got)
	}
}
