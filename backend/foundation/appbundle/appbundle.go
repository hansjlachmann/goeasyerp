// Package appbundle is the program with the extension apps that the installation's own builder
// builds (cmd/app-builder; App Build Setup: Builds By = Local Builder): a bundle — a folder in
// the builds volume (APP_BUILDS_DIR) with the built server program, the merged definitions and
// translations it reads at run time, and build.json. The standard server program runs the
// newest bundle built for its own version (Launch), so an installation runs its apps without an
// image of its own; the builder and the server share the volume and the database (App Build).
package appbundle

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	fapps "github.com/hansjlachmann/goeasyerp/backend/foundation/apps"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
)

// Names in a bundle
const (
	BinaryName   = "api-server"
	ManifestName = "build.json"
)

// Environment
const (
	// EnvBuildsDir: the builds volume (unset: no bundles — the standard program only)
	EnvBuildsDir = "APP_BUILDS_DIR"
	// EnvBundle: set by Launch for the bundle's program — the build number it runs
	EnvBundle = "GOEASYERP_BUNDLE"
)

// statusSucceeded: App Build Status Succeeded (the option's number)
const statusSucceeded = 2

// Manifest is a bundle's build.json
type Manifest struct {
	BuildNo      int              `json:"build_no"`
	BaseVersion  string           `json:"base_version"`
	Apps         []fapps.BuildApp `json:"apps"`
	BinarySHA256 string           `json:"binary_sha256"`
	BuiltAt      string           `json:"built_at"`
}

// Dir is the folder of build no in the builds volume
func Dir(buildsDir string, no int) string {
	return filepath.Join(buildsDir, strconv.Itoa(no))
}

// FileSHA256 is a file's SHA-256 (hex)
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ReadManifest reads a bundle's build.json
func ReadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		return nil, err
	}
	var m Manifest
	err = json.Unmarshal(data, &m)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ManifestName, err)
	}
	return &m, nil
}

// WriteManifest writes a bundle's build.json
func WriteManifest(dir string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ManifestName), data, 0o644)
}

// Verify checks that the bundle in dir is the one recorded: built for the version, its program
// the one with the SHA-256 the builder recorded (a partly written or changed program refused)
func Verify(dir, version, sha string) error {
	m, err := ReadManifest(dir)
	if err != nil {
		return err
	}
	if m.BaseVersion != version {
		return fmt.Errorf("bundle %s is built for version %s, not %s", dir, m.BaseVersion, version)
	}
	got, err := FileSHA256(filepath.Join(dir, BinaryName))
	if err != nil {
		return err
	}
	if sha == "" || got != sha || m.BinarySHA256 != sha {
		return fmt.Errorf("bundle %s: its program is not the one built", dir)
	}
	return nil
}

// Build is a successful build recorded in App Build
type Build struct {
	No     int
	SHA256 string
	Path   string // the bundle's folder in the builds volume
}

// Latest is the newest successful build for the version (nil when there is none, also before
// the App Build table exists)
func Latest(db *sql.DB, dbType database.DBType, version string) (*Build, error) {
	query := `SELECT entry_no, binary_sha256, bundle_path FROM "App_Build" WHERE status = ? AND base_version = ? ORDER BY entry_no DESC LIMIT 1`
	if dbType == database.DBTypePostgres {
		query = strings.Replace(strings.Replace(query, "?", "$1", 1), "?", "$2", 1)
	}
	var b Build
	var sha, path sql.NullString
	err := db.QueryRow(query, statusSucceeded, version).Scan(&b.No, &sha, &path)
	if errors.Is(err, sql.ErrNoRows) || err != nil && missingTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.SHA256 = sha.String
	b.Path = path.String
	return &b, nil
}

// missingTable: the error of a query on a table that does not exist yet
func missingTable(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no such table") || strings.Contains(text, "does not exist")
}
