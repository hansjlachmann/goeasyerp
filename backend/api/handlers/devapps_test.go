package handlers

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/api/middleware"
	blapps "github.com/hansjlachmann/goeasyerp/backend/business-logic/apps"
	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// devTestApp: the development endpoint on a database with the base installed, the client's
// address from X-Forwarded-For; super: the session is a SUPER user's (nil session: none)
func devTestApp(t *testing.T, sess *session.Session, remote bool) (*fiber.App, *fapps.App) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, rec := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.InstalledApp{}, &gtables.AppPackage{}, &gtables.AppBuildSetup{}, &gtables.AppAuditException{}, &gtables.AppBuild{}} {
		if err := rec.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	base, err := fapps.LoadBase()
	if err != nil {
		t.Fatal(err)
	}
	if err := blapps.SyncInstalledApps(db, database.DBTypeSQLite, []*fapps.App{base}); err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{ProxyHeader: fiber.HeaderXForwardedFor})
	api := app.Group("/api", func(c *fiber.Ctx) error {
		c.Locals("session", sess)
		return c.Next()
	}, middleware.UserContext())
	NewDevAppsHandler(db, database.DBTypeSQLite, remote).Register(api)
	return app, base
}

func devRequest(t *testing.T, app *fiber.App, method, path, address string, body []byte) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set(fiber.HeaderXForwardedFor, address)
	req.Header.Set(fiber.HeaderContentType, "application/zip")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	_ = json.Unmarshal(data, &out)
	return resp.StatusCode, out
}

func devPackage(t *testing.T, baseID, code string) []byte {
	t.Helper()
	files := map[string]string{
		"application.yaml": "application:\n  id: aaaaaaaa-0000-4000-8000-000000000061\n  name: Dev App\n  version: 1.0.0\n  prefix: dv\n" +
			"  number_series:\n    objects: [{from: 54000, to: 54099}]\n    fields: [{from: 54000, to: 54099}]\n  dependencies:\n    - {id: " + baseID + "}\n",
		"go.mod": "module example.com/devapp\n\nrequire github.com/hansjlachmann/goeasyerp v0.0.0\n",
		"dev.go": code,
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// F5 from VS Code: the package is published and its build requested, the build's status read;
// the audit's findings come back with file and line
func TestDevEndpointPublish(t *testing.T) {
	admin := &session.Session{UserID: "ADMIN", Company: "main", Language: "en-US", HasRoles: true, IsSuper: true}
	app, base := devTestApp(t, admin, false)

	status, out := devRequest(t, app, "POST", "/api/dev/apps", "127.0.0.1", devPackage(t, base.ID, "package devapp\n"))
	data, _ := out["data"].(map[string]interface{})
	if status != fiber.StatusOK || data["build_no"] != float64(1) || data["version"] != "1.0.0" {
		t.Fatalf("publish: %d %v", status, out)
	}
	status, out = devRequest(t, app, "GET", "/api/dev/builds/1", "127.0.0.1", nil)
	data, _ = out["data"].(map[string]interface{})
	if status != fiber.StatusOK || data["status"] != "Requested" || data["done"] != false || data["live"] != false {
		t.Errorf("build: %d %v", status, out)
	}
	if status, _ := devRequest(t, app, "GET", "/api/dev/builds/99", "127.0.0.1", nil); status != fiber.StatusNotFound {
		t.Errorf("an unknown build: %d", status)
	}
	// a build is requested: the next publish waits for it
	if status, out := devRequest(t, app, "POST", "/api/dev/apps", "127.0.0.1", devPackage(t, base.ID, "package devapp\n")); status != fiber.StatusBadRequest ||
		!strings.Contains(out["error"].(string), "1") {
		t.Errorf("publish while build 1 is requested: %d %v", status, out)
	}

	evil := devPackage(t, base.ID, "package devapp\n\nimport \"os\"\n\nvar _ = os.Getenv(\"JWT_SECRET\")\n")
	app2, _ := devTestApp(t, admin, false)
	status, out = devRequest(t, app2, "POST", "/api/dev/apps", "127.0.0.1", evil)
	data, _ = out["data"].(map[string]interface{})
	findings, _ := data["findings"].([]interface{})
	if status != fiber.StatusUnprocessableEntity || len(findings) == 0 {
		t.Fatalf("a package the audit refuses: %d %v", status, out)
	}
	first := findings[0].(map[string]interface{})
	if first["file"] != "dev.go" || first["line"] != float64(3) || first["rule"] != "import" {
		t.Errorf("finding %v", first)
	}
}

// The endpoint answers only this machine (unless remote publishing is allowed), a logged-in
// user only, and SUPER only
func TestDevEndpointGuards(t *testing.T) {
	admin := &session.Session{UserID: "ADMIN", Company: "main", Language: "en-US", HasRoles: true, IsSuper: true}
	app, base := devTestApp(t, admin, false)
	pkg := devPackage(t, base.ID, "package devapp\n")
	if status, _ := devRequest(t, app, "POST", "/api/dev/apps", "198.51.100.7", pkg); status != fiber.StatusForbidden {
		t.Errorf("from another machine: %d", status)
	}
	remote, _ := devTestApp(t, admin, true)
	if status, out := devRequest(t, remote, "POST", "/api/dev/apps", "198.51.100.7", pkg); status != fiber.StatusOK {
		t.Errorf("remote publishing allowed: %d %v", status, out)
	}
	none, _ := devTestApp(t, nil, false)
	if status, _ := devRequest(t, none, "POST", "/api/dev/apps", "127.0.0.1", pkg); status != fiber.StatusUnauthorized {
		t.Errorf("without a session: %d", status)
	}
	clerk, _ := devTestApp(t, &session.Session{UserID: "CLERK", Company: "main", Language: "en-US", HasRoles: true}, false)
	if status, out := devRequest(t, clerk, "POST", "/api/dev/apps", "127.0.0.1", pkg); status != fiber.StatusBadRequest ||
		!strings.Contains(strings.ToLower(out["error"].(string)), "super") {
		t.Errorf("a user without SUPER: %d %v", status, out)
	}
	if status, _ := devRequest(t, app, "POST", "/api/dev/apps", "127.0.0.1", nil); status != fiber.StatusBadRequest {
		t.Errorf("no package: %d", status)
	}
}

// The schemas are served without a login (they are public), from this machine
func TestDevEndpointSchemas(t *testing.T) {
	app, _ := devTestApp(t, nil, false)
	status, out := devRequest(t, app, "GET", "/api/dev/schemas", "127.0.0.1", nil)
	names, _ := out["data"].([]interface{})
	if status != fiber.StatusOK || len(names) < 8 {
		t.Fatalf("schema names: %d %v", status, out)
	}
	req := httptest.NewRequest("GET", "/api/dev/schemas/table.schema.json", nil)
	req.Header.Set(fiber.HeaderXForwardedFor, "127.0.0.1")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != fiber.StatusOK || resp.Header.Get(fiber.HeaderContentType) != "application/schema+json" || !strings.Contains(string(body), "\"extends_table\"") && !strings.Contains(string(body), "\"fields\"") {
		t.Errorf("table schema: %d %s", resp.StatusCode, resp.Header.Get(fiber.HeaderContentType))
	}
	if status, _ := devRequest(t, app, "GET", "/api/dev/schemas/nothing.json", "127.0.0.1", nil); status != fiber.StatusNotFound {
		t.Errorf("an unknown schema: %d", status)
	}
	if status, _ := devRequest(t, app, "GET", "/api/dev/schemas", "198.51.100.7", nil); status != fiber.StatusForbidden {
		t.Errorf("schemas from another machine: %d", status)
	}
}

func TestLocalAddress(t *testing.T) {
	for address, want := range map[string]bool{
		"127.0.0.1": true, "::1": true, "172.18.0.1": true, "192.168.1.5": true, "fd00::1": true,
		"198.51.100.7": false, "2001:db8::1": false, "": false,
	} {
		if got := localAddress(address); got != want {
			t.Errorf("localAddress(%q) = %v", address, got)
		}
	}
}
