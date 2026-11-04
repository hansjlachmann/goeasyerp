package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func pictureApp(t *testing.T) (*fiber.App, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := (&gtables.Item{}).CreateTableWithDBType(db, "TEST", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	var item gtables.Item
	item.InitWithDBType(db, "TEST", database.DBTypeSQLite)
	item.InitRecord()
	item.No, item.Description = "1000", types.Text("Copy paper")
	if !item.Insert(false) {
		t.Fatal("insert item")
	}
	h := NewTablesHandler(db)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("session", &session.Session{Company: "TEST", Language: "en-US"})
		return c.Next()
	})
	app.Get("/api/tables/:table/card/:id", h.GetRecord)
	app.Put("/api/tables/:table/modify/:id", h.ModifyRecord)
	app.Get("/api/tables/:table/blob/:field/:id", h.GetBlob)
	app.Put("/api/tables/:table/blob/:field/:id", h.PutBlob)
	app.Delete("/api/tables/:table/blob/:field/:id", h.DeleteBlob)
	return app, db
}

func pngImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.RGBA{R: 200, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func do(t *testing.T, app *fiber.App, method, url string, body []byte) (int, []byte, map[string]string) {
	t.Helper()
	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	resp, err := app.Test(req, -1) // no timeout: re-encoding is slow under -race
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	headers := map[string]string{}
	for _, k := range []string{"Content-Type", "X-Content-Type-Options", "Content-Security-Policy"} {
		headers[k] = resp.Header.Get(k)
	}
	return resp.StatusCode, raw, headers
}

// An item picture: uploaded (checked and re-encoded), served with safe headers, only a Yes/No in
// the record's JSON, not written by a modify, deleted
func TestItemPicture(t *testing.T) {
	app, _ := pictureApp(t)
	url := "/api/tables/Item/blob/picture/1000"

	if status, _, _ := do(t, app, "GET", url, nil); status != 404 {
		t.Errorf("no picture yet: %d", status)
	}
	// not an image: refused
	if status, _, _ := do(t, app, "PUT", url, []byte("<svg onload=alert(1)>")); status != 400 {
		t.Errorf("an SVG was accepted: %d", status)
	}
	upload := append(pngImage(t, 40, 30), []byte("<script>appended</script>")...)
	if status, raw, _ := do(t, app, "PUT", url, upload); status != 200 {
		t.Fatalf("upload: %d %s", status, raw)
	}
	status, raw, headers := do(t, app, "GET", url, nil)
	if status != 200 || headers["X-Content-Type-Options"] != "nosniff" || headers["Content-Security-Policy"] == "" ||
		(headers["Content-Type"] != "image/jpeg" && headers["Content-Type"] != "image/png") || bytes.Contains(raw, []byte("script")) {
		t.Fatalf("served picture: %d %v (%d bytes)", status, headers, len(raw))
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(raw)); err != nil {
		t.Errorf("served picture is not an image: %v", err)
	}
	// the record says Yes, and a modify sending the field does not change the picture
	_, body, _ := do(t, app, "GET", "/api/tables/Item/card/1000", nil)
	var card struct {
		Data map[string]interface{} `json:"data"`
	}
	_ = json.Unmarshal(body, &card)
	if card.Data["picture"] != true {
		t.Errorf("record picture %v", card.Data["picture"])
	}
	req := httptest.NewRequest("PUT", "/api/tables/Item/modify/1000", bytes.NewReader([]byte(`{"description":"Copy paper A4","picture":false}`)))
	req.Header.Set("Content-Type", "application/json")
	if _, err := app.Test(req, -1); err != nil {
		t.Fatal(err)
	}
	if status, _, _ := do(t, app, "GET", url, nil); status != 200 {
		t.Errorf("a modify removed the picture: %d", status)
	}
	if status, _, _ := do(t, app, "DELETE", url, nil); status != 200 {
		t.Errorf("delete: %d", status)
	}
	if status, _, _ := do(t, app, "GET", url, nil); status != 404 {
		t.Errorf("deleted picture still served: %d", status)
	}
	if status, _, _ := do(t, app, "GET", "/api/tables/Item/blob/description/1000", nil); status != 404 {
		t.Errorf("a text field served as a picture: %d", status)
	}
}
