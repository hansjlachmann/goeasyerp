package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"
)

// TestServerRolePostgres: a superuser DB_USER makes AppRole, hands it its objects, and the
// server works as AppRole; BuilderRole gets its tables only (TEST_POSTGRES_DSN: a superuser's
// connection to a database of its own)
func TestServerRolePostgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	cluster, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cluster.Close() })
	for _, role := range []string{AppRole, BuilderRole} {
		exists, err := roleExists(cluster, role)
		if err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Skipf("the role %s exists in this cluster already", role)
		}
	}
	exec := func(db *sql.DB, query string) {
		t.Helper()
		_, err := db.Exec(query)
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	// a database and a superuser of its own: the handover looks at every object of the database
	stamp := time.Now().UnixNano()
	role := fmt.Sprintf("lp_test_%d", stamp)
	dbName := fmt.Sprintf("lp_test_db_%d", stamp)
	exec(cluster, "CREATE ROLE "+role+" SUPERUSER LOGIN PASSWORD 'pw'")
	t.Cleanup(func() {
		for _, r := range []string{BuilderRole, AppRole, role} {
			_, err := cluster.Exec("DROP ROLE IF EXISTS " + r)
			if err != nil {
				t.Errorf("cleanup: role %s: %v", r, err)
			}
		}
	})
	exec(cluster, "CREATE DATABASE "+dbName+" OWNER "+role)
	t.Cleanup(func() {
		_, err := cluster.Exec("DROP DATABASE IF EXISTS " + dbName + " WITH (FORCE)")
		if err != nil {
			t.Errorf("cleanup: database %s: %v", dbName, err)
		}
	})

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config := &PostgresConfig{Host: u.Hostname(), Port: u.Port(), User: role, Password: "pw", DBName: dbName, SSLMode: "disable"}
	if config.Port == "" {
		config.Port = "5432"
	}
	owner, err := connect(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Close() }()
	exec(owner, `CREATE TABLE "App_Build" (entry_no SERIAL PRIMARY KEY, status INT)`)
	exec(owner, `CREATE TABLE "Thing" (id INT PRIMARY KEY)`)
	exec(owner, `CREATE SEQUENCE free_seq`)
	exec(owner, `CREATE FUNCTION thing_trigger() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$`)
	exec(owner, `CREATE TRIGGER thing_t BEFORE INSERT ON "Thing" FOR EACH ROW EXECUTE FUNCTION thing_trigger()`)

	t.Setenv(EnvBuilderPassword, "builder-pw")
	resolved, done, err := serverRole(config)
	if err != nil {
		t.Fatalf("serverRole: %v (%v)", err, done)
	}
	t.Log(done)
	if resolved.User != AppRole {
		t.Fatalf("the server works as %s, want %s", resolved.User, AppRole)
	}
	app, err := connect(resolved)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = app.Close() }()
	left, err := ownedByOthers(app)
	if err != nil || left != 0 {
		t.Fatalf("objects not handed over: %d %v", left, err)
	}
	// the owner works on its tables and makes new ones, but is no superuser
	exec(app, `INSERT INTO "App_Build" (status) VALUES (0)`)
	exec(app, `INSERT INTO "Thing" VALUES (1)`)
	exec(app, `SELECT nextval('free_seq')`)
	exec(app, `ALTER TABLE "Thing" ADD COLUMN name TEXT`)
	exec(app, `CREATE TABLE "New_Thing" (id INT)`)
	exec(app, `DROP TABLE "New_Thing"`)
	_, err = app.Exec(`COPY (SELECT 1) TO PROGRAM 'true'`)
	if err == nil {
		t.Fatal("COPY TO PROGRAM allowed for the server's role")
	}
	_, err = app.Exec(`CREATE DATABASE lp_test_other`)
	if err == nil {
		_, _ = cluster.Exec(`DROP DATABASE lp_test_other`)
		t.Fatal("the server's role may create databases")
	}

	// the builder: its tables, nothing else
	err = GrantBuilder(app)
	if err != nil {
		t.Fatal(err)
	}
	builderConfig := withUser(config, BuilderRole)
	builderConfig.Password = "builder-pw"
	builder, err := connect(builderConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = builder.Close() }()
	exec(builder, `UPDATE "App_Build" SET status = 1`)
	for _, query := range []string{`DELETE FROM "App_Build"`, `DROP TABLE "App_Build"`, `SELECT * FROM "Thing"`} {
		_, err = builder.Exec(query)
		if err == nil {
			t.Fatalf("the builder may: %s", query)
		}
	}

	// the builder's settings: its own role, or the server's instead of a superuser DB_USER
	for _, c := range []*PostgresConfig{builderConfig, config} {
		got, err := builderRole(c)
		if err != nil {
			t.Fatalf("builderRole(%s): %v", c.User, err)
		}
		want := BuilderRole
		if c == config {
			want = AppRole
		}
		if got.User != want {
			t.Fatalf("builderRole(%s) = %s, want %s", c.User, got.User, want)
		}
	}

	// the next start: nothing to do (the test's DB_USER is no bootstrap superuser: it keeps its
	// password, and the server takes AppRole)
	resolved, done, err = serverRole(config)
	if err != nil || resolved.User != AppRole || len(done) != 0 {
		t.Fatalf("second start: %s %v %v", resolved.User, done, err)
	}
}
