package appbundle

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// The launcher: the standard server program (the image's) runs the newest bundle built for its
// version when the installation builds its apps itself; with no bundle while apps are
// installed — a new version of the server, before its apps are built — it must not run without
// them, so it waits in maintenance until the bundle is there.

// EnvHome: set by Exec for a bundle's program — the standard program's folder, to start it
// again when a newer bundle is there
const EnvHome = "GOEASYERP_HOME"

// Option numbers of the tables read here
const (
	buildsByLocalBuilder     = 0 // App Build Setup Builds By (1 before migration 058)
	installedStatusInstalled = 0 // Installed App Status
	installedStatusRemoval   = 2 // … Pending Removal
)

// Decision is what the launcher does
type Decision struct {
	Bundle      *Build // run this bundle
	Dir         string // … in this folder
	Maintenance bool   // no bundle, but apps installed: wait for their build
	Reason      string // why a bundle was not taken (logged)
}

// Decide: with the local builder (App Build Setup), the newest verified bundle of the version;
// none while extension apps are installed: maintenance
func Decide(db *sql.DB, dbType database.DBType, buildsDir, version, baseAppID string) (Decision, error) {
	local, err := LocalBuilder(db, dbType)
	if err != nil || !local {
		return Decision{}, err
	}
	var d Decision
	b, err := Latest(db, dbType, version)
	if err != nil {
		return d, err
	}
	if b != nil {
		dir := Dir(buildsDir, b.No)
		err = Verify(dir, version, b.SHA256)
		if err == nil {
			return Decision{Bundle: b, Dir: dir}, nil
		}
		d.Reason = err.Error()
	}
	needed, err := appsInstalled(db, dbType, baseAppID)
	if err != nil {
		return d, err
	}
	d.Maintenance = needed
	return d, nil
}

// BuildsByLocalFirstMigration: the migration that swapped Builds By's options (Local Builder
// first). The launcher runs before migrations: on the first start of a version that brings it,
// the stored value still has the old meaning.
const BuildsByLocalFirstMigration = 58

// LocalBuilder: the installation builds its apps itself (App Build Setup: Builds By = Local
// Builder); false before the table exists. A setup never saved has the default, as the page and
// Build Apps see it: Local Builder since migration 058 (before it the default was CI Pipeline).
func LocalBuilder(db *sql.DB, dbType database.DBType) (bool, error) {
	var buildsBy int
	err := db.QueryRow(`SELECT builds_by FROM "App_Build_Setup" WHERE primary_key = ''`).Scan(&buildsBy)
	if err == sql.ErrNoRows {
		return migrated(db, BuildsByLocalFirstMigration), nil
	}
	if err != nil && missingTable(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	local := buildsByLocalBuilder
	if !migrated(db, BuildsByLocalFirstMigration) {
		local = 1 // the value before the swap
	}
	return buildsBy == local, nil
}

// migrated: the migration has run (a database without the migrations table has none to run:
// its tables are made new)
func migrated(db *sql.DB, version int) bool {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM "_migrations" WHERE version = ` + strconv.Itoa(version) + ` AND status = 'completed'`).Scan(&n)
	if err != nil {
		return missingTable(err)
	}
	return n > 0
}

// appsInstalled: extension apps are installed (Installed, or Pending Removal: still built in)
func appsInstalled(db *sql.DB, dbType database.DBType, baseAppID string) (bool, error) {
	query := `SELECT COUNT(*) FROM "Installed_App" WHERE app_id <> ? AND status IN (?, ?)`
	if dbType == database.DBTypePostgres {
		query = `SELECT COUNT(*) FROM "Installed_App" WHERE app_id <> $1 AND status IN ($2, $3)`
	}
	var n int
	err := db.QueryRow(query, baseAppID, installedStatusInstalled, installedStatusRemoval).Scan(&n)
	if err != nil && missingTable(err) {
		return false, nil
	}
	return n > 0, err
}

// Current is the build number of the bundle this program is (0: the standard program)
func Current() int {
	no, _ := strconv.Atoi(os.Getenv(EnvBundle))
	return no
}

// Exec replaces this process with the bundle's program, run in the bundle's folder (it reads
// its definitions and translations there); home is the standard program's folder
func Exec(dir string, no int, home string) error {
	err := os.Chdir(dir)
	if err != nil {
		return err
	}
	env := withEnv(os.Environ(), EnvBundle, strconv.Itoa(no))
	env = withEnv(env, EnvHome, home)
	return syscall.Exec(filepath.Join(dir, BinaryName), os.Args, env)
}

// Restart replaces this process with the standard program, whose launcher takes the newest
// bundle (a bundle's program: the standard program in its home folder)
func Restart() error {
	home := os.Getenv(EnvHome)
	program, err := os.Executable()
	if err != nil {
		return err
	}
	if home != "" {
		program = filepath.Join(home, BinaryName)
		err = os.Chdir(home)
		if err != nil {
			return err
		}
	}
	env := withoutEnv(withoutEnv(os.Environ(), EnvBundle), EnvHome)
	return syscall.Exec(program, os.Args, env)
}

// Home is the standard program's folder: where this program runs from, or its home when it is a
// bundle's
func Home() (string, error) {
	home := os.Getenv(EnvHome)
	if home != "" {
		return home, nil
	}
	return os.Getwd()
}

func withEnv(env []string, key, value string) []string {
	return append(withoutEnv(env, key), key+"="+value)
}

func withoutEnv(env []string, key string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return out
}

// String describes a decision for the log
func (d Decision) String() string {
	switch {
	case d.Bundle != nil:
		return fmt.Sprintf("app build %d (%s)", d.Bundle.No, d.Dir)
	case d.Maintenance:
		return "maintenance: the apps are not built for this version yet"
	}
	return "the standard program"
}
