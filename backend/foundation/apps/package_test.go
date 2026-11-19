package apps

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zipEntry struct {
	name    string
	content string
	mode    os.FileMode
}

func makeZip(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			h.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testManifest(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "apps", "loyalty-points", "application.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A package holds the app's files at its root or in one top folder; ReadPackage gives the app
// and its files relative to the app's folder, and Extract writes them
func TestReadPackage(t *testing.T) {
	m := testManifest(t)
	for _, prefix := range []string{"", "extensions/"} {
		data := makeZip(t,
			zipEntry{name: prefix + "application.yaml", content: m},
			zipEntry{name: prefix + "tables/x.extend.yaml", content: "table: {}"},
			zipEntry{name: prefix + "go.mod", content: "module x"},
			zipEntry{name: prefix + "LICENSE", content: "MIT"},
		)
		p, err := ReadPackage(data)
		if err != nil {
			t.Fatalf("prefix %q: %v", prefix, err)
		}
		if p.App.Prefix == "" || len(p.Files) != 4 || p.Files["tables/x.extend.yaml"] == nil {
			t.Fatalf("prefix %q: app %+v files %v", prefix, p.App, len(p.Files))
		}
		dir := filepath.Join(t.TempDir(), "app")
		if err := p.Extract(dir); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(dir, "tables", "x.extend.yaml"))
		if err != nil || string(got) != "table: {}" {
			t.Fatalf("extracted: %q %v", got, err)
		}
	}
}

// Untrusted packages: paths outside the app, links, other file types, no application.yaml,
// too large, not a zip
func TestReadPackageRefuses(t *testing.T) {
	m := testManifest(t)
	app := zipEntry{name: "application.yaml", content: m}
	cases := []struct {
		what string
		data []byte
		want string
	}{
		{"zip slip", makeZip(t, app, zipEntry{name: "../evil.go", content: "x"}), "not a path within the app"},
		{"nested zip slip", makeZip(t, app, zipEntry{name: "a/../../evil.go", content: "x"}), "not a path within the app"},
		{"absolute", makeZip(t, app, zipEntry{name: "/etc/evil.go", content: "x"}), "not a path within the app"},
		{"backslash", makeZip(t, app, zipEntry{name: "..\\evil.go", content: "x"}), "not a path within the app"},
		{"symlink", makeZip(t, app, zipEntry{name: "link.go", content: "/etc/passwd", mode: os.ModeSymlink | 0o777}), "no links"},
		{"binary", makeZip(t, app, zipEntry{name: "tool.exe", content: "MZ"}), "not part of an app"},
		{"script", makeZip(t, app, zipEntry{name: "build.sh", content: "rm -rf /"}), "not part of an app"},
		{"hidden", makeZip(t, app, zipEntry{name: ".github/x.yaml", content: "x"}), "not part of an app"},
		{"no manifest", makeZip(t, zipEntry{name: "go.mod", content: "module x"}), "no application.yaml"},
		{"two tops", makeZip(t, zipEntry{name: "a/application.yaml", content: m}, zipEntry{name: "b/go.mod", content: "x"}), "no application.yaml"},
		{"bad manifest", makeZip(t, zipEntry{name: "application.yaml", content: "application: {}"}), ""},
		{"too large unpacked", makeZip(t, app, zipEntry{name: "big.txt", content: strings.Repeat("a", packageMaxFile+1)}), "too large"},
		{"not a zip", []byte("PK nonsense"), "not a .zip"},
		{"empty", makeZip(t), "empty"},
	}
	for _, c := range cases {
		_, err := ReadPackage(c.data)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", c.what, err, c.want)
		}
	}
	big := make([]byte, PackageMaxSize+1)
	if _, err := ReadPackage(big); err == nil || !strings.Contains(err.Error(), "larger") {
		t.Errorf("oversized package: %v", err)
	}
}
