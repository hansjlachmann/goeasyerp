package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The committed schemas are the ones the Go types give now — after changing a type that reads a
// definition (a new key), run: go run ./tools/schemagen
func TestSchemasUpToDate(t *testing.T) {
	schemas, err := Generate("../..")
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range schemas {
		committed, err := os.ReadFile(filepath.Join("../..", OutDir, name))
		if err != nil {
			t.Errorf("%s: %v (run go run ./tools/schemagen)", name, err)
			continue
		}
		if !bytes.Equal(committed, data) {
			t.Errorf("%s is out of date: run go run ./tools/schemagen", name)
		}
	}
	entries, err := os.ReadDir(filepath.Join("../..", OutDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			if _, ok := schemas[e.Name()]; !ok {
				t.Errorf("%s is not generated any more: delete it", e.Name())
			}
		}
	}
}
