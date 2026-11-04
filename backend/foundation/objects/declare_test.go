package objects

import (
	"strings"
	"testing"
)

// A declaration records the declaring package; the same object again with its name is fine,
// with another name it panics
func TestDeclare(t *testing.T) {
	Codeunit(99365, "Format Address Test")
	Codeunit(99365, "Format Address Test")
	var found *Declaration
	for _, d := range Declared() {
		if d.Type == TypeCodeunit && d.ID == 99365 {
			d := d
			found = &d
		}
	}
	if found == nil || found.Name != "Format Address Test" || !strings.HasSuffix(found.Package, "/foundation/objects") {
		t.Fatalf("declaration: %+v", found)
	}
	if DeclaredName(TypeCodeunit, 99365) != "Format Address Test" {
		t.Error("DeclaredName")
	}
	defer func() {
		if recover() == nil {
			t.Error("a second name for the same object did not panic")
		}
	}()
	Codeunit(99365, "Another Name")
}
