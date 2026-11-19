package apps

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// An app package is an extension app as one file: a .zip of the app's folder — its
// application.yaml, definitions, Go code and translations — uploaded in Installed Apps (Upload
// App) and built by the installation's builder. The zip holds the folder's files at its root,
// or inside one top folder (a folder zipped as it is). It is untrusted input: ReadPackage reads
// it within limits and refuses paths outside the app, links, and files an app does not have.

// Package limits: an app is source code and definitions, so a package is small; it comes
// base64-encoded in the upload dialog's answer, which must stay under the server's request limit
// (4 MB)
const (
	PackageMaxSize      = 2 << 20  // the .zip
	packageMaxUnpacked  = 20 << 20 // all files unpacked
	packageMaxFile      = 5 << 20  // one file unpacked
	packageMaxFileCount = 2000
)

// packageExtensions: the file types an app consists of
var packageExtensions = map[string]bool{
	".yaml": true, ".yml": true, ".go": true, ".mod": true, ".sum": true,
	".json": true, ".md": true, ".txt": true,
}

// packageNames: files without one of those extensions that an app may have
var packageNames = map[string]bool{"LICENSE": true, "NOTICE": true, "README": true}

// Package is a read app package: the app's application.yaml (parsed and as bytes) and its
// files by their path in the app's folder (slash-separated)
type Package struct {
	App      *App
	Manifest []byte
	Files    map[string][]byte
}

// ReadPackage reads and checks an app package
func ReadPackage(data []byte) (*Package, error) {
	if len(data) > PackageMaxSize {
		return nil, fmt.Errorf("the package is larger than %d MB", PackageMaxSize>>20)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("the file is not a .zip file")
	}
	if len(zr.File) > packageMaxFileCount {
		return nil, fmt.Errorf("the package has more than %d files", packageMaxFileCount)
	}
	files := map[string][]byte{}
	var total int64
	for _, f := range zr.File {
		name, err := packagePath(f.Name)
		if err != nil {
			return nil, err
		}
		mode := f.Mode()
		if mode.IsDir() {
			continue
		}
		if !mode.IsRegular() {
			return nil, fmt.Errorf("%s: only files can be in a package (no links)", f.Name)
		}
		if !packageFileAllowed(name) {
			return nil, fmt.Errorf("%s: this file type is not part of an app", f.Name)
		}
		if _, ok := files[name]; ok {
			return nil, fmt.Errorf("%s: twice in the package", f.Name)
		}
		content, err := readZipFile(f, packageMaxUnpacked-total)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		total += int64(len(content))
		files[name] = content
	}
	files, err = withoutTopFolder(files)
	if err != nil {
		return nil, err
	}
	manifest, ok := files["application.yaml"]
	if !ok {
		return nil, errors.New("the package has no application.yaml (at its root, or in one top folder)")
	}
	app, err := Parse(manifest)
	if err != nil {
		return nil, err
	}
	return &Package{App: app, Manifest: manifest, Files: files}, nil
}

// packagePath: a zip entry's path, refused when it could reach outside the app's folder
func packagePath(name string) (string, error) {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("%q: not a path within the app", name)
	}
	clean := path.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", fmt.Errorf("%q: not a path within the app", name)
	}
	for _, part := range strings.Split(clean, "/") {
		if part == ".." {
			return "", fmt.Errorf("%q: not a path within the app", name)
		}
	}
	return clean, nil
}

func packageFileAllowed(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	base := path.Base(name)
	return packageExtensions[strings.ToLower(path.Ext(base))] || packageNames[base]
}

// readZipFile reads one entry within the limits (the size the zip claims is not trusted)
func readZipFile(f *zip.File, left int64) ([]byte, error) {
	limit := int64(packageMaxFile)
	if left < limit {
		limit = left
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	content, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, errors.New("too large unpacked")
	}
	return content, nil
}

// withoutTopFolder: the files of a package whose files are all in one top folder (a folder
// zipped as it is), relative to that folder
func withoutTopFolder(files map[string][]byte) (map[string][]byte, error) {
	if len(files) == 0 {
		return nil, errors.New("the package is empty")
	}
	if _, ok := files["application.yaml"]; ok {
		return files, nil
	}
	top := ""
	for name := range files {
		first, _, found := strings.Cut(name, "/")
		if !found {
			return files, nil
		}
		if top != "" && first != top {
			return files, nil
		}
		top = first
	}
	out := make(map[string][]byte, len(files))
	for name, content := range files {
		out[strings.TrimPrefix(name, top+"/")] = content
	}
	return out, nil
}

// Extract writes the package's files into dir (created; must not exist or be empty)
func (p *Package) Extract(dir string) error {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(p.Files))
	for name := range p.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target := filepath.Join(dir, filepath.FromSlash(name))
		rel, err := filepath.Rel(dir, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%s: not a path within the app", name)
		}
		err = os.MkdirAll(filepath.Dir(target), 0o755)
		if err != nil {
			return err
		}
		err = os.WriteFile(target, p.Files[name], 0o644)
		if err != nil {
			return err
		}
	}
	return nil
}
