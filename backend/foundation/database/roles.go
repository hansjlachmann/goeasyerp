package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
)

// Least privilege on PostgreSQL. The official image makes POSTGRES_USER (openerp) the cluster's
// superuser — it can drop every database, read and write the database server's files and run
// programs there, so any bug, injection or harmful app code would be total loss. The server
// therefore works as a role of its own, AppRole: the owner of the database and its tables,
// without SUPERUSER, CREATEDB or CREATEROLE. On the first start that finds DB_USER a superuser
// the server makes the role (with DB_PASSWORD), hands it everything DB_USER owns in the
// database, and — DB_USER being the image's superuser — takes DB_USER's password away: the
// superuser is then reached only inside the database container (psql -U openerp, which the
// image trusts there), and the server's environment opens nothing more than its own tables. The
// app builder works as BuilderRole with the tables it needs only.

// AppRole is the server's database role (password: DB_PASSWORD)
const AppRole = "goeasyerp"

// BuilderRole is the app builder's database role (password: EnvBuilderPassword)
const BuilderRole = "goeasyerp_builder"

// EnvBuilderPassword: the app builder's database password; set, the server makes BuilderRole
// with it (while it holds the superuser) and grants it the builder's tables
const EnvBuilderPassword = "BUILDER_DB_PASSWORD"

// EnvKeepSuperuser: "true" — the server connects as DB_USER as it is, even a superuser
// (not recommended)
const EnvKeepSuperuser = "DB_KEEP_SUPERUSER"

// transferBatch: objects handed over per transaction (each takes locks until its commit)
const transferBatch = 200

// builderGrants: what the app builder may do (claim and finish builds; read the packages and
// the setup)
var builderGrants = []struct{ privileges, table string }{
	{"SELECT, UPDATE", "App_Build"},
	{"SELECT", "App_Package"},
	{"SELECT", "Installed_App"},
	{"SELECT", "App_Build_Setup"},
}

var serverConfig struct {
	once   sync.Once
	config *PostgresConfig
	ok     bool
}

// ServerConfigFromEnv: the server's PostgreSQL settings — PostgresConfigFromEnv with the role
// the server works as (AppRole, made on the first start; see above). Done once per process.
func ServerConfigFromEnv() (*PostgresConfig, bool) {
	serverConfig.once.Do(func() {
		config, ok := PostgresConfigFromEnv()
		serverConfig.ok = ok
		if !ok {
			return
		}
		resolved, done, err := serverRole(config)
		for _, line := range done {
			log.Print(line)
		}
		if err != nil {
			log.Printf("WARNING: database roles: %v", err)
		}
		serverConfig.config = resolved
	})
	return serverConfig.config, serverConfig.ok
}

// BuilderConfigFromEnv: the app builder's PostgreSQL settings — DB_USER (BuilderRole), or
// AppRole when DB_USER is the superuser the server takes the password from (an installation
// without BuilderRole). It waits while the server has not made the role yet: the builder never
// works as a superuser.
func BuilderConfigFromEnv() (*PostgresConfig, bool) {
	config, ok := PostgresConfigFromEnv()
	if !ok || strings.EqualFold(os.Getenv(EnvKeepSuperuser), "true") {
		return config, ok
	}
	waiting := false
	for {
		resolved, err := builderRole(config)
		if err == nil {
			return resolved, true
		}
		if !waiting {
			log.Printf("database: %v — waiting for the server to set up its roles", err)
			waiting = true
		}
		time.Sleep(5 * time.Second)
	}
}

// builderRole: the role the builder can work as, or why there is none yet
func builderRole(config *PostgresConfig) (*PostgresConfig, error) {
	candidates := []*PostgresConfig{config}
	if config.User != BuilderRole && config.User != AppRole {
		candidates = []*PostgresConfig{withUser(config, AppRole), config}
	}
	var lastErr error
	for _, c := range candidates {
		db, err := connect(c)
		if err != nil {
			lastErr = err
			continue
		}
		var super bool
		err = db.QueryRow(`SELECT rolsuper FROM pg_roles WHERE rolname = current_user`).Scan(&super)
		_ = db.Close()
		if err != nil {
			return nil, err
		}
		if super {
			return nil, fmt.Errorf("the role %s is a superuser", c.User)
		}
		return c, nil
	}
	return nil, lastErr
}

// serverRole: the settings the server connects with, and what was done for the log
func serverRole(config *PostgresConfig) (*PostgresConfig, []string, error) {
	if strings.EqualFold(os.Getenv(EnvKeepSuperuser), "true") || config.User == AppRole {
		return config, nil, nil
	}
	app := withUser(config, AppRole)
	appDB, err := connect(app)
	if err == nil {
		defer func() { _ = appDB.Close() }()
		left, err := ownedByOthers(appDB)
		if err != nil || left == 0 {
			return app, nil, err
		}
		// a handover that stopped half-way: finish it with DB_USER, while it can still log in
		admin, err := connect(config)
		if err != nil {
			return app, []string{fmt.Sprintf("WARNING: %d database object(s) are not owned by %s; give them to it as the superuser (inside the database container): REASSIGN OWNED BY … TO %s", left, AppRole, AppRole)}, nil
		}
		defer func() { _ = admin.Close() }()
		done, err := setUp(admin, config)
		return app, done, err
	}
	if !authFailed(err) {
		return config, nil, err
	}
	admin, err := connect(config)
	if err != nil {
		return config, nil, err
	}
	defer func() { _ = admin.Close() }()
	var super, createDB, createRole bool
	err = admin.QueryRow(`SELECT rolsuper, rolcreatedb, rolcreaterole FROM pg_roles WHERE rolname = current_user`).Scan(&super, &createDB, &createRole)
	if err != nil {
		return config, nil, err
	}
	if !super {
		if createDB || createRole {
			return config, []string{fmt.Sprintf("WARNING: the database role %s can create databases or roles; remove that: ALTER ROLE %s NOCREATEDB NOCREATEROLE", config.User, config.User)}, nil
		}
		return config, nil, nil // an owner role set up by the installation
	}
	done, err := setUp(admin, config)
	if err != nil {
		return config, done, err
	}
	return app, done, nil
}

// setUp, as the superuser: AppRole (and BuilderRole), everything the superuser owns in the
// database handed to AppRole, then the superuser's password taken away when it is the image's
func setUp(admin *sql.DB, config *PostgresConfig) ([]string, error) {
	var done []string
	exists, err := roleExists(admin, AppRole)
	if err != nil {
		return nil, err
	}
	verb := "CREATE"
	if exists {
		verb = "ALTER"
	}
	_, err = admin.Exec(verb + " ROLE " + pq.QuoteIdentifier(AppRole) + " LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD " + pq.QuoteLiteral(config.Password))
	if err != nil {
		return nil, fmt.Errorf("making the role %s: %w", AppRole, err)
	}
	if !exists {
		done = append(done, fmt.Sprintf("database: role %s for the server (owner of its tables, no superuser)", AppRole))
	}
	builderPassword := os.Getenv(EnvBuilderPassword)
	if builderPassword != "" {
		exists, err := roleExists(admin, BuilderRole)
		if err != nil {
			return done, err
		}
		verb := "CREATE"
		if exists {
			verb = "ALTER"
		}
		_, err = admin.Exec(verb + " ROLE " + pq.QuoteIdentifier(BuilderRole) + " LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD " + pq.QuoteLiteral(builderPassword))
		if err != nil {
			return done, fmt.Errorf("making the role %s: %w", BuilderRole, err)
		}
		if !exists {
			done = append(done, fmt.Sprintf("database: role %s for the app builder", BuilderRole))
		}
	}
	_, err = admin.Exec("ALTER DATABASE " + pq.QuoteIdentifier(config.DBName) + " OWNER TO " + pq.QuoteIdentifier(AppRole))
	if err != nil {
		return done, fmt.Errorf("database owner: %w", err)
	}
	n, err := handOver(admin)
	if err != nil {
		return done, err
	}
	if n > 0 {
		done = append(done, fmt.Sprintf("database: %d object(s) handed over to %s", n, AppRole))
	}

	var bootstrap bool
	err = admin.QueryRow(`SELECT oid = 10 FROM pg_roles WHERE rolname = current_user`).Scan(&bootstrap)
	if err != nil {
		return done, err
	}
	if !bootstrap || config.User == "postgres" {
		return append(done, fmt.Sprintf("WARNING: DB_USER %s is a superuser and keeps its password: connect the server as %s (DB_USER) and remove %s's rights or password", config.User, AppRole, config.User)), nil
	}
	_, err = admin.Exec("ALTER ROLE " + pq.QuoteIdentifier(config.User) + " PASSWORD NULL")
	if err != nil {
		return done, fmt.Errorf("taking the password of %s: %w", config.User, err)
	}
	return append(done, fmt.Sprintf("database: the superuser %s has no password any more (psql -U %s inside the database container); the server works as %s", config.User, config.User, AppRole)), nil
}

// handOver gives AppRole the schemas, relations, functions and types the connection's role owns
// in the database, a batch per transaction (a table's indexes and sequences go with it)
func handOver(admin *sql.DB) (int, error) {
	total := 0
	for {
		statements, err := ownedStatements(admin, transferBatch)
		if err != nil || len(statements) == 0 {
			return total, err
		}
		tx, err := admin.Begin()
		if err != nil {
			return total, err
		}
		for _, statement := range statements {
			_, err = tx.Exec(statement)
			if err != nil {
				_ = tx.Rollback()
				return total, fmt.Errorf("%s: %w", statement, err)
			}
		}
		err = tx.Commit()
		if err != nil {
			return total, err
		}
		total += len(statements)
	}
}

// userObjects: the objects of the database's own schemas (not the system's), with their owner
// and the statement that gives one to AppRole
const userObjects = `
WITH ns AS (
	SELECT oid, nspname, nspowner FROM pg_namespace
	WHERE nspname NOT LIKE 'pg\_%' AND nspname <> 'information_schema'
)
SELECT nspowner, 'ALTER SCHEMA ' || quote_ident(nspname) FROM ns
UNION ALL
SELECT c.relowner, CASE c.relkind
		WHEN 'v' THEN 'ALTER VIEW '
		WHEN 'm' THEN 'ALTER MATERIALIZED VIEW '
		WHEN 'S' THEN 'ALTER SEQUENCE '
		WHEN 'f' THEN 'ALTER FOREIGN TABLE '
		ELSE 'ALTER TABLE ' END || quote_ident(ns.nspname) || '.' || quote_ident(c.relname)
FROM pg_class c JOIN ns ON ns.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f')
	OR (c.relkind = 'S' AND NOT EXISTS (
		SELECT 1 FROM pg_depend d WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype IN ('a', 'i')))
UNION ALL
SELECT p.proowner, CASE p.prokind WHEN 'p' THEN 'ALTER PROCEDURE ' ELSE 'ALTER FUNCTION ' END || p.oid::regprocedure::text
FROM pg_proc p JOIN ns ON ns.oid = p.pronamespace
WHERE p.prokind IN ('f', 'p')
UNION ALL
SELECT t.typowner, 'ALTER TYPE ' || quote_ident(ns.nspname) || '.' || quote_ident(t.typname)
FROM pg_type t JOIN ns ON ns.oid = t.typnamespace
WHERE t.typtype IN ('d', 'e', 'r') OR (t.typtype = 'c' AND (SELECT relkind FROM pg_class WHERE oid = t.typrelid) = 'c')
`

// ownedStatements: up to limit statements handing the connection's role's objects to AppRole
func ownedStatements(admin *sql.DB, limit int) ([]string, error) {
	rows, err := admin.Query(`SELECT statement || ' OWNER TO ' || quote_ident($1) FROM (`+userObjects+`) o(owner, statement)
		WHERE owner = (SELECT oid FROM pg_roles WHERE rolname = current_user) LIMIT $2`, AppRole, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var statements []string
	for rows.Next() {
		var statement string
		err = rows.Scan(&statement)
		if err != nil {
			return nil, err
		}
		statements = append(statements, statement)
	}
	return statements, rows.Err()
}

// ownedByOthers: objects of the database's own schemas the connection's role does not own
// (the public schema of PostgreSQL 15+ belongs to pg_database_owner: its own)
func ownedByOthers(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM (` + userObjects + `) o(owner, statement)
		WHERE owner NOT IN (SELECT oid FROM pg_roles WHERE rolname IN (current_user, 'pg_database_owner'))`).Scan(&n)
	return n, err
}

// GrantBuilder grants BuilderRole its tables when the role exists — by their owner, the
// server's role, on every start (a table made new gets them again). Call it after the startup
// table sync.
func GrantBuilder(db *sql.DB) error {
	exists, err := roleExists(db, BuilderRole)
	if err != nil {
		return err
	}
	if !exists {
		if os.Getenv(EnvBuilderPassword) != "" {
			log.Printf("WARNING: %s is set but the role %s does not exist; make it as the superuser (inside the database container): CREATE ROLE %s LOGIN PASSWORD '…'", EnvBuilderPassword, BuilderRole, BuilderRole)
		}
		return nil
	}
	for _, g := range builderGrants {
		var table sql.NullString
		err = db.QueryRow(`SELECT to_regclass($1)::text`, pq.QuoteIdentifier(g.table)).Scan(&table)
		if err != nil {
			return err
		}
		if !table.Valid {
			continue
		}
		_, err = db.Exec("GRANT " + g.privileges + " ON " + pq.QuoteIdentifier(g.table) + " TO " + pq.QuoteIdentifier(BuilderRole))
		if err != nil {
			return fmt.Errorf("granting %s on %s to %s: %w", g.privileges, g.table, BuilderRole, err)
		}
	}
	return nil
}

func roleExists(db *sql.DB, role string) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, role).Scan(&exists)
	return exists, err
}

func withUser(config *PostgresConfig, user string) *PostgresConfig {
	c := *config
	c.User = user
	return &c
}

func connect(config *PostgresConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", config.ConnectionString())
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// authFailed: the login was refused (no such role, wrong or no password)
func authFailed(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "28P01" || pqErr.Code == "28000"
	}
	return false
}
