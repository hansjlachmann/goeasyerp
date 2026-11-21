package migrations

import (
	"database/sql"
	"sort"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	fmigrations "github.com/hansjlachmann/goeasyerp/backend/foundation/migrations"
)

// Users without roles become SUPER only while nobody is SUPER (they could do everything before;
// with an administrator they get nothing)
func TestSuperForUsersWithoutRoles(t *testing.T) {
	for _, c := range []struct {
		name    string
		members string // user:role pairs before
		want    string // the SUPER members after
	}{
		{"no administrator", "KARI:READER", "HANS,OLA"},
		{"an administrator", "KARI:READER,OLA:SUPER", "OLA"},
		{"no roles at all", "", "HANS,KARI,OLA"},
	} {
		db, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		for _, q := range []string{
			`CREATE TABLE "User" (user_id VARCHAR(50) PRIMARY KEY)`,
			`CREATE TABLE "User_Member" (user_id VARCHAR(50), role_id VARCHAR(20), company VARCHAR(100), PRIMARY KEY (user_id, role_id, company))`,
			`INSERT INTO "User" VALUES ('HANS'), ('KARI'), ('OLA')`,
		} {
			if _, err := db.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
		if c.members != "" {
			for _, m := range strings.Split(c.members, ",") {
				p := strings.Split(m, ":")
				if _, err := db.Exec(`INSERT INTO "User_Member" VALUES (?, ?, 'cronus')`, p[0], p[1]); err != nil {
					t.Fatal(err)
				}
			}
		}
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		ctx := &fmigrations.Context{DB: db, DBType: database.DBTypeSQLite}
		ctx.SetTx(tx)
		if err := (&Migration059SuperForUsersWithoutRoles{}).Up(ctx); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		rows, err := db.Query(`SELECT user_id FROM "User_Member" WHERE role_id = 'SUPER'`)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for rows.Next() {
			var u string
			_ = rows.Scan(&u)
			got = append(got, u)
		}
		_ = rows.Close()
		sort.Strings(got)
		if strings.Join(got, ",") != c.want {
			t.Errorf("%s: SUPER %v, want %s", c.name, got, c.want)
		}
		_ = db.Close()
	}
}
