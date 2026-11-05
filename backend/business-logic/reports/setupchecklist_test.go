package reports_test

import (
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/apps"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/setupchecklist"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 26 prints the checklist under its areas and the comments of the tables that have them,
// numbered
func TestSetupChecklistReport(t *testing.T) {
	src := listCompany(t)
	mustOK(t, (&gtables.Object{}).CreateTableWithDBType(src.GetDB(), "", src.GetDBType()))
	mustOK(t, (&gtables.ObjectTranslation{}).CreateTableWithDBType(src.GetDB(), "", src.GetDBType()))
	mustOK(t, apps.SyncObjects(src.GetDB(), src.GetDBType()))
	mustOK(t, setupchecklist.UpdateChecklist(src))
	comment := gtables.NewSetupChecklistComment(src)
	comment.Table_id = gtables.CurrencyTableID
	comment.Line_no = 10000
	comment.Comment = "Rates from the central bank"
	mustOK(t, comment.InsertOrError(false))

	result, err := reports.Execute(reports.SetupChecklistID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{Output: reports.OutputPDF})
	mustOK(t, err)
	texts := printedTexts(result, 0)
	for _, want := range []string{"Setup Checklist", "General Ledger", "Company Information", "Fixed Assets", "Comments", "Rates from the central bank"} {
		if !strings.Contains(texts, want) {
			t.Errorf("%q not printed: %s", want, texts)
		}
	}
	if strings.Index(texts, "General Ledger") > strings.Index(texts, "Sales & Receivables") {
		t.Error("the areas are not in their order")
	}
}
