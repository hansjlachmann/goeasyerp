package pages

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 376 sums the quantities of the current line's resource in its batch per unit of measure
// and work type, a total per unit of measure and the total; other resources are left out, and
// a line of another type cannot be reconciled
func TestProjectJournalReconcile(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := tables.CreateCompanyTables(db, "demo", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if _, err := demodata.Create(db, "demo", database.DBTypeSQLite, demodata.Options{Today: time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)}); err != nil {
		t.Fatal(err)
	}
	src := ftables.DBSource(db, "demo", database.DBTypeSQLite)
	project := gtables.NewProject(src)
	project.SetRange("status", gtables.Project_Status.Order)
	if !project.FindFirst() {
		t.Fatal("no demo project on Order")
	}
	add := func(lineNo int, lineType gtables.ProjectJournalLineType, no, workType, qty string) {
		t.Helper()
		line := gtables.NewProjectJournalLine(src)
		line.InitRecord()
		line.Journal_template_name, line.Journal_batch_name, line.Line_no = "PROJECT", "DEFAULT", lineNo
		err := tables.SetUpNewProjectJnlLine(line, gtables.NewProjectJournalLine(src))
		for _, step := range []struct {
			field string
			value interface{}
		}{{"posting_date", types.NewDate(2026, 10, 5)}, {"project_no", project.No}, {"type", lineType}, {"no", types.Code(no)}, {"work_type_code", types.Code(workType)}, {"quantity", types.MustDecimal(qty)}} {
			if err == nil && (step.field != "work_type_code" || workType != "") {
				err = line.Validate(step.field, step.value)
			}
		}
		if err == nil {
			err = line.InsertOrError(true)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	add(10000, gtables.ProjectJournalLine_Type.Resource, "R0010", "NORMAL", "8")
	add(20000, gtables.ProjectJournalLine_Type.Resource, "R0010", "OVERTIME", "2")
	add(30000, gtables.ProjectJournalLine_Type.Resource, "R0040", "NORMAL", "5")
	add(40000, gtables.ProjectJournalLine_Type.Resource, "R0010", "NORMAL", "3")
	add(50000, gtables.ProjectJournalLine_Type.Item, "1310", "", "1")

	projectJnlQty := gtables.NewProjectJournalQuantity(src)
	if err := projectJnlQty.InitTemporary(db, "demo", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	if err := ProjectJnlReconcile(projectJnlQty, "PROJECT", "DEFAULT", "40000"); err != nil {
		t.Fatal(err)
	}
	var rows []string
	for found := projectJnlQty.FindSet(); found; found = projectJnlQty.Next() {
		rows = append(rows, fmt.Sprintf("%s/%s/%s", projectJnlQty.Unit_of_measure_text, projectJnlQty.Work_type_code, projectJnlQty.Quantity))
	}
	want := "HOUR/NORMAL/11|HOUR/OVERTIME/2|Total HOUR//13|Total//13"
	if strings.Join(rows, "|") != want {
		t.Errorf("rows %s, want %s", strings.Join(rows, "|"), want)
	}

	if err := ProjectJnlReconcile(projectJnlQty, "PROJECT", "DEFAULT", "50000"); err == nil {
		t.Error("an item line was reconciled")
	}
}
