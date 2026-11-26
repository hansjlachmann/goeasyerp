package appaudit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// maxDirFiles and maxDirSize: an app folder read for the audit (an app is source code and
// definitions)
const (
	maxDirFiles = 5000
	maxDirSize  = 50 << 20
)

// AuditDir audits the app in a folder (the builder's fetched app, the CI pipeline's checkout).
// Hidden folders (.git) are skipped; a link or other file that is not a regular file is a
// finding, as it would be refused in a package.
func AuditDir(app, dir string, opts Options) (*Report, error) {
	files, odd, err := readDir(dir)
	if err != nil {
		return nil, err
	}
	r := Audit(app, files, opts)
	for _, name := range odd {
		r.Findings = append([]Finding{{Rule: RuleFile, File: name, Key: name,
			Detail: "only regular files can be part of an app (no links, devices or pipes)"}}, r.Findings...)
	}
	r.applyExceptions(opts.Exceptions)
	return r, nil
}

// readDir reads the regular files of dir (slash-separated paths) and lists the others
func readDir(dir string) (map[string][]byte, []string, error) {
	files := map[string][]byte{}
	var odd []string
	var total int64
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			odd = append(odd, rel)
			return nil
		}
		if len(files) >= maxDirFiles {
			return fmt.Errorf("the app has more than %d files", maxDirFiles)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		total += int64(len(data))
		if total > maxDirSize {
			return fmt.Errorf("the app's files are larger than %d MB", maxDirSize>>20)
		}
		files[rel] = data
		return nil
	})
	return files, odd, err
}
