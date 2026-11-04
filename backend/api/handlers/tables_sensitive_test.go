package handlers

import (
	"database/sql"
	"encoding/json"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"golang.org/x/crypto/bcrypt"
)

// newUserTestApp serves the table API over a database holding one user (HANS) with a
// known password.
func newUserTestApp(t *testing.T) (*fiber.App, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.User{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatalf("create table: %v", err)
	}
	var u gtables.User
	u.InitWithDBType(db, "TEST", database.DBTypeSQLite)
	u.User_id.Set("HANS")
	u.User_name.Set("Hans")
	if err := tables.SetPassword(&u, "secret-123"); err != nil {
		t.Fatal(err)
	}
	if !u.Insert(false) {
		t.Fatal("insert user failed")
	}

	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/tables/:table/list", h.ListRecords)
	app.Get("/api/tables/:table/ids", h.GetRecordIDs)
	app.Get("/api/tables/:table/card/:id", h.GetRecord)
	app.Post("/api/tables/:table/insert", h.InsertRecord)
	app.Put("/api/tables/:table/modify/:id", h.ModifyRecord)
	app.Post("/api/tables/:table/validate", h.ValidateField)
	app.Post("/api/tables/:table/init", h.InitRecord)
	return app, db
}

func storedHash(t *testing.T, db *sql.DB, userID string) string {
	t.Helper()
	var hash string
	if err := db.QueryRow(`SELECT password_hash FROM "User" WHERE user_id = ?`, userID).Scan(&hash); err != nil {
		t.Fatalf("read hash of %s: %v", userID, err)
	}
	return hash
}

// sendJSON sends a JSON body without app.Test's 1 s timeout: hashing a password with
// bcrypt takes longer than that under the race detector.
func sendJSON(t *testing.T, app *fiber.App, method, target, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v (%s)", target, err, raw)
	}
	return resp.StatusCode, out
}

// The password hash never leaves the server: no record the table API returns contains it
// (nor its caption).
func TestSensitiveFieldNotReturned(t *testing.T) {
	app, _ := newUserTestApp(t)

	responses := map[string]map[string]interface{}{}
	_, responses["list"] = getJSON(t, app, "/api/tables/User/list")
	_, responses["card"] = getJSON(t, app, "/api/tables/User/card/HANS")
	_, responses["modify"] = sendJSON(t, app, "PUT", "/api/tables/User/modify/HANS", `{"user_name":"Hans L"}`)
	_, responses["insert"] = sendJSON(t, app, "POST", "/api/tables/User/insert", `{"user_id":"KARI","user_name":"Kari","password":"other-456"}`)
	_, responses["validate"] = sendJSON(t, app, "POST", "/api/tables/User/validate", `{"field":"user_name","value":"X","record":{"user_id":"HANS"}}`)
	_, responses["init"] = sendJSON(t, app, "POST", "/api/tables/User/init", `{}`)

	for name, out := range responses {
		if out["success"] != true {
			t.Errorf("%s failed: %v", name, out["error"])
			continue
		}
		raw, _ := json.Marshal(out)
		body := string(raw)
		if strings.Contains(body, "password_hash") || strings.Contains(body, "$2a$") {
			t.Errorf("%s response contains the password hash: %s", name, body)
		}
	}
}

// Filters, sorting and search on the hash would reveal it piece by piece
// (e.g. password_hash=$2a$10$a* → one row or none), so they are rejected.
func TestSensitiveFieldNotQueryable(t *testing.T) {
	app, _ := newUserTestApp(t)

	for _, target := range []string{
		"/api/tables/User/list?filters=" + url.QueryEscape(`[{"field":"password_hash","expression":"$2a$*"}]`),
		"/api/tables/User/list?sort_by=password_hash",
		"/api/tables/User/list?search=2a&search_fields=" + url.QueryEscape(`["password_hash"]`),
		"/api/tables/User/ids?sort_by=password_hash",
	} {
		if status, out := getJSON(t, app, target); status != 400 || out["success"] != false {
			t.Errorf("%s: status %d, %v — want 400", target, status, out)
		}
	}
	// A normal field still works
	if status, _ := getJSON(t, app, "/api/tables/User/list?sort_by=user_name"); status != 200 {
		t.Errorf("sort by user_name: status %d", status)
	}
}

// The hash is set by the server only (from the virtual "password" field), never directly.
func TestSensitiveFieldNotWritable(t *testing.T) {
	app, db := newUserTestApp(t)
	before := storedHash(t, db, "HANS")
	known, _ := bcrypt.GenerateFromPassword([]byte("attacker"), bcrypt.MinCost)

	if status, _ := sendJSON(t, app, "PUT", "/api/tables/User/modify/HANS", `{"password_hash":"`+string(known)+`"}`); status != 400 {
		t.Errorf("modify password_hash: status %d, want 400", status)
	}
	if storedHash(t, db, "HANS") != before {
		t.Fatal("password_hash was overwritten through modify")
	}
	if _, out := sendJSON(t, app, "POST", "/api/tables/User/insert", `{"user_id":"EVE","password_hash":"`+string(known)+`"}`); out["success"] != false {
		t.Error("insert with password_hash accepted")
	}
	if _, out := sendJSON(t, app, "POST", "/api/tables/User/validate", `{"field":"password_hash","value":"x"}`); out["success"] != false {
		t.Error("validate of password_hash accepted")
	}

	// The password field still sets a new hash
	if status, out := sendJSON(t, app, "PUT", "/api/tables/User/modify/HANS", `{"password":"new-secret-789"}`); status != 200 {
		t.Fatalf("modify password: status %d, %v", status, out)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash(t, db, "HANS")), []byte("new-secret-789")); err != nil {
		t.Errorf("new password not stored: %v", err)
	}
}

// A masked field (SMTP password) is write-only: set through the API, sent back only as
// MaskedValue; the placeholder sent back keeps it, "" clears it; it cannot be queried.
func TestMaskedFieldWriteOnly(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.SMTPSetup{}).CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/tables/:table/list", h.ListRecords)
	app.Put("/api/tables/:table/modify", h.ModifyRecord)
	app.Post("/api/tables/:table/validate", h.ValidateField)

	stored := func() string {
		var p string
		if err := db.QueryRow(`SELECT password FROM "SMTP_Setup"`).Scan(&p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	listed := func() interface{} {
		_, out := getJSON(t, app, "/api/tables/SMTP_Setup/list") // creates the setup record
		recs := out["data"].(map[string]interface{})["records"].([]interface{})
		return recs[0].(map[string]interface{})["password"]
	}

	if got := listed(); got != "" {
		t.Errorf("empty password listed as %q", got)
	}
	if status, out := sendJSON(t, app, "PUT", "/api/tables/SMTP_Setup/modify", `{"password":"s3cret!","smtp_server":"mail.example.com"}`); status != 200 {
		t.Fatalf("set password: %d %v", status, out["error"])
	} else if out["data"].(map[string]interface{})["password"] != ftables.MaskedValue {
		t.Errorf("modify response password = %v, want the placeholder", out["data"].(map[string]interface{})["password"])
	}
	if stored() != "s3cret!" {
		t.Fatalf("stored password = %q", stored())
	}
	if got := listed(); got != ftables.MaskedValue {
		t.Errorf("listed password = %q, want the placeholder", got)
	}

	// The whole record sent back (placeholder included) keeps the password
	if status, _ := sendJSON(t, app, "PUT", "/api/tables/SMTP_Setup/modify", `{"password":"`+ftables.MaskedValue+`","smtp_server":"smtp.example.com"}`); status != 200 || stored() != "s3cret!" {
		t.Errorf("placeholder sent back: status %d, stored %q — want the old password kept", status, stored())
	}
	if _, out := sendJSON(t, app, "POST", "/api/tables/SMTP_Setup/validate", `{"field":"password","value":"`+ftables.MaskedValue+`","record":{"password":"`+ftables.MaskedValue+`"}}`); out["success"] != true {
		t.Errorf("validate placeholder: %v", out["error"])
	}

	// Not queryable: a filter or sort would reveal it
	for _, target := range []string{
		"/api/tables/SMTP_Setup/list?sort_by=password",
		"/api/tables/SMTP_Setup/list?filters=" + url.QueryEscape(`[{"field":"password","expression":"s*"}]`),
		"/api/tables/SMTP_Setup/list?search=s3&search_fields=" + url.QueryEscape(`["password"]`),
	} {
		if status, _ := getJSON(t, app, target); status != 400 {
			t.Errorf("%s: status %d, want 400", target, status)
		}
	}

	// "" clears it
	if status, _ := sendJSON(t, app, "PUT", "/api/tables/SMTP_Setup/modify", `{"password":""}`); status != 200 || stored() != "" {
		t.Errorf("clear: status %d, stored %q", status, stored())
	}
}
