package pages

import (
	"strconv"
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 376 Project Journal Reconcile (Reconcile, Ctrl+F9 on the project journal): the quantities
// of the current line's resource in its journal batch per unit of measure and work type, a
// total per unit of measure and the total (OnOpenForm into a temporary Project Journal
// Quantity). The page parameters journal_template_name, journal_batch_name and line_no are the
// line it was opened from (SetJobJnlLine: a resource line).
const projectJournalReconcilePageID = 376

func init() {
	pages.OnFindRecords(projectJournalReconcilePageID, func(rec *gtables.ProjectJournalQuantity, page *pages.Open) error {
		return ProjectJnlReconcile(rec, page.Param("journal_template_name"), page.Param("journal_batch_name"), page.Param("line_no"))
	})
}

// ProjectJnlReconcile fills projectJnlQty, a temporary Project Journal Quantity, with the
// quantities of the resource of a journal line in its batch (form 376 OnOpenForm)
func ProjectJnlReconcile(projectJnlQty *gtables.ProjectJournalQuantity, templateName, batchName, lineNo string) error {
	projectJnlQty.DeleteAll()
	line := gtables.NewProjectJournalLine(projectJnlQty)
	no, err := strconv.Atoi(lineNo)
	if err != nil || !line.Get(map[string]interface{}{"journal_template_name": templateName, "journal_batch_name": batchName, "line_no": no}) {
		return nil
	}
	err = line.TestField("type", gtables.ProjectJournalLine_Type.Resource)
	if err != nil {
		return err
	}
	projectJnlLine := gtables.NewProjectJournalLine(projectJnlQty)
	projectJnlLine.SetCurrentKey("journal_template_name", "journal_batch_name", "type", "no", "unit_of_measure_code", "work_type_code")
	projectJnlLine.SetRange("journal_template_name", line.Journal_template_name)
	projectJnlLine.SetRange("journal_batch_name", line.Journal_batch_name)
	projectJnlLine.SetRange("type", gtables.ProjectJournalLine_Type.Resource)
	projectJnlLine.SetRange("no", line.No)
	if !projectJnlLine.FindSetBuffered() {
		return nil
	}
	language := fcodeunits.CurrentLanguage()
	totalText := func(code types.Code) string {
		return strings.TrimSpace(system.StrSubstNo(i18n.GetInstance().Message("PROJECT_JNL_RECONCILE_TOTAL", language), code))
	}
	var unitOfMeasureQty, workTypeQty, totalQty types.Decimal
	oldUnitOfMeasureCode := projectJnlLine.Unit_of_measure_code
	oldWorkTypeCode := projectJnlLine.Work_type_code
	insert := func(isTotal bool, unitOfMeasureCode types.Code, lineType gtables.ProjectJournalQuantityLine_type, workTypeCode types.Code, qty types.Decimal) error {
		projectJnlQty.Init()
		projectJnlQty.Is_total = isTotal
		projectJnlQty.Unit_of_measure_code = unitOfMeasureCode
		projectJnlQty.Line_type = lineType
		projectJnlQty.Work_type_code = workTypeCode
		projectJnlQty.Quantity = qty
		projectJnlQty.Unit_of_measure_text = types.Text(unitOfMeasureCode)
		if lineType == gtables.ProjectJournalQuantity_Line_type.Total {
			projectJnlQty.Unit_of_measure_text = types.Text(totalText(unitOfMeasureCode))
		}
		return projectJnlQty.InsertOrError(false)
	}
	// InsertUnitOfMeasureQty, InsertWorkTypeQty
	insertUnitOfMeasureQty := func() error {
		err := insert(false, oldUnitOfMeasureCode, gtables.ProjectJournalQuantity_Line_type.Total, "", unitOfMeasureQty)
		unitOfMeasureQty = types.Decimal{}
		return err
	}
	insertWorkTypeQty := func() error {
		err := insert(false, oldUnitOfMeasureCode, gtables.ProjectJournalQuantity_Line_type.Blank, oldWorkTypeCode, workTypeQty)
		workTypeQty = types.Decimal{}
		return err
	}
	for found := true; found; found = projectJnlLine.Next() {
		if oldUnitOfMeasureCode != projectJnlLine.Unit_of_measure_code {
			err = insertUnitOfMeasureQty()
			if err == nil {
				err = insertWorkTypeQty()
			}
		} else if oldWorkTypeCode != projectJnlLine.Work_type_code {
			err = insertWorkTypeQty()
		}
		if err != nil {
			return err
		}
		oldUnitOfMeasureCode = projectJnlLine.Unit_of_measure_code
		oldWorkTypeCode = projectJnlLine.Work_type_code
		unitOfMeasureQty = unitOfMeasureQty.Add(projectJnlLine.Quantity)
		workTypeQty = workTypeQty.Add(projectJnlLine.Quantity)
		totalQty = totalQty.Add(projectJnlLine.Quantity)
	}
	err = insertUnitOfMeasureQty()
	if err == nil {
		err = insertWorkTypeQty()
	}
	if err != nil {
		return err
	}
	return insert(true, "", gtables.ProjectJournalQuantity_Line_type.Total, "", totalQty)
}
