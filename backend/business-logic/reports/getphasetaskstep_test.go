package reports_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Get Phase/Task/Step (codeunit 215): page 216 opens on the budget lines of the line's project;
// the line takes the phase, task and step of the budget line picked — Cancel leaves it
func TestGetPhaseTaskStep(t *testing.T) {
	src := listCompany(t)
	budgetLine := gtables.NewProjectBudgetLine(src)
	budgetLine.SetFilter("phase_code", "<>''")
	budgetLine.SetFilter("task_code", "<>''")
	if !budgetLine.FindFirst() {
		t.Fatal("no demo budget line with a phase and task")
	}
	line := gtables.NewProjectJournalLine(src)
	line.Journal_template_name = "PROJECT"
	line.Journal_batch_name = "TEST"
	line.Line_no = 10000
	line.Project_no = budgetLine.Project_no
	mustOK(t, line.InsertOrError(false))

	// Cancel: nothing changes
	opened := pageAnswers(t, map[string]string{"action": "LookupCancel"})
	_, err := tables.ProjectJnlLineGetPhaseTaskStep(line)
	mustOK(t, err)
	if line.Phase_code != "" || len(*opened) != 1 {
		t.Fatalf("after Cancel: phase %q, pages %d", line.Phase_code, len(*opened))
	}
	ev := (*opened)[0]
	if ev.Data["page_id"] != tables.ProjectBudgetPhaseTaskStepPageID || ev.Data["lookup_mode"] != true ||
		!strings.Contains(fmt.Sprint(ev.Data["filters"]), budgetLine.Project_no.String()) {
		t.Errorf("page %v lookup %v filters %v", ev.Data["page_id"], ev.Data["lookup_mode"], ev.Data["filters"])
	}

	// OK on the budget line: its codes
	_ = pageAnswers(t, map[string]string{"action": "LookupOK", "key": ftables.KeyString(budgetLine)})
	_, err = tables.ProjectJnlLineGetPhaseTaskStep(line)
	mustOK(t, err)
	stored := gtables.NewProjectJournalLine(src)
	mustOK(t, stored.GetOrError(map[string]interface{}{"journal_template_name": "PROJECT", "journal_batch_name": "TEST", "line_no": 10000}))
	if stored.Phase_code != budgetLine.Phase_code || stored.Task_code != budgetLine.Task_code || stored.Step_code != budgetLine.Step_code {
		t.Errorf("line %s/%s/%s, want %s/%s/%s", stored.Phase_code, stored.Task_code, stored.Step_code,
			budgetLine.Phase_code, budgetLine.Task_code, budgetLine.Step_code)
	}
}
