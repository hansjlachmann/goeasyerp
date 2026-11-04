package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/openerp/backend/foundation/migrations"
)

// Users on the former default themes move to the GoEasyERP themes; another theme stays
func TestUsersMoveToGoEasyERPThemes(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE "User_Personalization" (user_id VARCHAR(50) PRIMARY KEY, light_theme VARCHAR(50), dark_theme VARCHAR(50))`,
		`INSERT INTO "User_Personalization" VALUES ('HANS', 'CLASSIC-LIGHT', 'CLASSIC-DARK'), ('ANNE', 'NORD', 'CLASSIC-DARK')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite}
	ctx.SetTx(tx)
	if err := (&Migration054GoEasyERPThemes{}).Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var light, dark string
	if err := db.QueryRow(`SELECT light_theme || ',' || dark_theme, (SELECT light_theme || ',' || dark_theme FROM "User_Personalization" WHERE user_id = 'ANNE') FROM "User_Personalization" WHERE user_id = 'HANS'`).Scan(&light, &dark); err != nil {
		t.Fatal(err)
	}
	if light != "GOEASYERP-LIGHT,GOEASYERP-DARK" || dark != "NORD,GOEASYERP-DARK" {
		t.Errorf("HANS %s, ANNE %s", light, dark)
	}
}
