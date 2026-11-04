package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The Gen. posting groups tell whether their default tax group is filled in automatically
func TestValidateTaxPostingGroups(t *testing.T) {
	db := newTestDB(t)
	for _, tbl := range []interface {
		CreateTableWithDBType(database.Executor, string, database.DBType) error
	}{&gtables.GenBusinessPostingGroup{}, &gtables.GenProductPostingGroup{}} {
		if err := tbl.CreateTableWithDBType(db, "c1", database.DBTypeSQLite); err != nil {
			t.Fatal(err)
		}
	}
	var bus gtables.GenBusinessPostingGroup
	bus.InitWithDBType(db, "c1", database.DBTypeSQLite)
	bus.InitRecord()
	bus.Code = "EXPORT"
	bus.Auto_insert_default = false
	if !bus.Insert(false) {
		t.Fatal("insert EXPORT")
	}

	var g gtables.GenBusinessPostingGroup
	g.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if auto, err := ValidateTaxBusPostingGroup(&g, "EXPORT"); err != nil || auto {
		t.Errorf("EXPORT: %v %v, want no auto insert", auto, err)
	}
	if auto, err := ValidateTaxBusPostingGroup(&g, ""); err != nil || !auto {
		t.Errorf("blank: %v %v, want the default (auto insert)", auto, err)
	}
	if _, err := ValidateTaxBusPostingGroup(&g, "NONE"); err == nil || message(t, err, "en-US") != "Gen. Business Posting Group NONE not found" {
		t.Errorf("unknown group: %v", err)
	}

	var p gtables.GenProductPostingGroup
	p.InitWithDBType(db, "c1", database.DBTypeSQLite)
	if auto, err := ValidateTaxProdPostingGroup(&p, ""); err != nil || !auto {
		t.Errorf("product blank: %v %v", auto, err)
	}
}
