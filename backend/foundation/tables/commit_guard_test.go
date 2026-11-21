package tables

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every transaction of the API and the business logic ends with Commit / Rollback of this
// package, so a table left inconsistent (Consistent(false): G/L entries that do not balance)
// is never committed. Only the framework itself (this package, SIFT, migrations, the database
// roles — no records) uses the
// database/sql methods directly.
func TestTransactionsUseCheckedCommit(t *testing.T) {
	plain := regexp.MustCompile(`\b(tx|transaction)\.(Commit|Rollback)\(\)`)
	allowed := map[string]bool{
		"foundation/tables/consistent.go": true,
		"foundation/sift/sift.go":         true,
		"foundation/migrations/runner.go": true,
		"foundation/database/roles.go":    true,
	}
	root := filepath.Join("..", "..")
	for _, dir := range []string{"api", "business-logic", "foundation"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if allowed[rel] {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(data), "\n") {
				if plain.MatchString(line) {
					t.Errorf("%s:%d: use ftables.Commit(tx) / ftables.Rollback(tx) (the balance check): %s", rel, i+1, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
