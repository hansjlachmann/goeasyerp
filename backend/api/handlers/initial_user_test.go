package handlers

import (
	"database/sql"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The first user is made once: calls at the same moment on a fresh installation cannot both
// make a SUPER user (S24)
func TestInitialUserOnlyOnce(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	sqlite := database.DBTypeSQLite
	if err := (&gtables.User{}).CreateTableWithDBType(db, "", sqlite); err != nil {
		t.Fatal(err)
	}
	if err := (&gtables.UserMember{}).CreateTableWithDBType(db, "", sqlite); err != nil {
		t.Fatal(err)
	}
	SetSetupToken("the-setup-token")
	t.Cleanup(func() { SetSetupToken("") })
	app := fiber.New()
	app.Post("/api/auth/init", NewAuthHandlerWithDBType(db, sqlite).CreateInitialUser)

	const calls = 6
	var wg sync.WaitGroup
	statuses := make([]int, calls)
	for i := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := fmt.Sprintf(`{"user_id":"ADMIN%d","user_name":"Admin","password":"first-secret-%d","setup_token":"the-setup-token"}`, i, i)
			req := httptest.NewRequest("POST", "/api/auth/init", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, 10000)
			if err != nil {
				t.Error(err)
				return
			}
			statuses[i] = resp.StatusCode
		}()
	}
	wg.Wait()
	created := 0
	for _, s := range statuses {
		if s == 200 {
			created++
		}
	}
	var users, supers int
	_ = db.QueryRow(`SELECT COUNT(*) FROM "User"`).Scan(&users)
	_ = db.QueryRow(`SELECT COUNT(*) FROM "User_Member" WHERE role_id = 'SUPER'`).Scan(&supers)
	if created != 1 || users != 1 || supers != 1 {
		t.Errorf("%d calls at once: %d answered 200 (%v), %d users, %d SUPER members; want 1 each", calls, created, statuses, users, supers)
	}
}

// The first user needs the setup token (S37); the login page's empty question still learns
// whether users exist (403), and wrong tokens lock the address like failed logins
func TestInitialUserNeedsSetupToken(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	sqlite := database.DBTypeSQLite
	if err := (&gtables.User{}).CreateTableWithDBType(db, "", sqlite); err != nil {
		t.Fatal(err)
	}
	if err := (&gtables.UserMember{}).CreateTableWithDBType(db, "", sqlite); err != nil {
		t.Fatal(err)
	}
	logins = newLoginGuard()
	t.Cleanup(func() { logins = newLoginGuard() })
	SetSetupToken("the-setup-token")
	t.Cleanup(func() { SetSetupToken("") })
	app := fiber.New()
	app.Post("/api/auth/init", NewAuthHandlerWithDBType(db, sqlite).CreateInitialUser)
	post := func(body string) int {
		req := httptest.NewRequest("POST", "/api/auth/init", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, 10000)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	const user = `"user_id":"ADMIN","user_name":"Admin","password":"first-secret-1"`

	// the login page's question (empty body) never counts as a guess
	for range 12 {
		if got := post(`{}`); got != 401 {
			t.Fatalf("empty question: %d, want 401 (no users yet)", got)
		}
	}
	if got := post(`{` + user + `}`); got != 401 {
		t.Errorf("without token: %d, want 401", got)
	}
	if got := post(`{` + user + `,"setup_token":"guess"}`); got != 401 {
		t.Errorf("wrong token: %d, want 401", got)
	}
	if got := post(`{"user_id":"ADMIN","user_name":"Admin","password":"short","setup_token":"the-setup-token"}`); got != 400 {
		t.Errorf("right token, short password: %d, want 400", got)
	}
	if got := post(`{` + user + `,"setup_token":"the-setup-token"}`); got != 200 {
		t.Fatalf("right token: %d, want 200", got)
	}
	if got := post(`{}`); got != 403 {
		t.Errorf("after the first user: %d, want 403", got)
	}

	// wrong tokens lock the address: then not even the right one gets in
	SetSetupToken("other-token")
	_, _ = db.Exec(`DELETE FROM "User"`)
	for range 10 {
		post(`{` + user + `,"setup_token":"guess"}`)
	}
	if got := post(`{` + user + `,"setup_token":"other-token"}`); got != 429 {
		t.Errorf("after 10 wrong tokens: %d, want 429", got)
	}
}
