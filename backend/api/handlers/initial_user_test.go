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
	app := fiber.New()
	app.Post("/api/auth/init", NewAuthHandlerWithDBType(db, sqlite).CreateInitialUser)

	const calls = 6
	var wg sync.WaitGroup
	statuses := make([]int, calls)
	for i := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := fmt.Sprintf(`{"user_id":"ADMIN%d","user_name":"Admin","password":"secret%d"}`, i, i)
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
