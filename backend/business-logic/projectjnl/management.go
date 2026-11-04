// Package projectjnl is the project journal: journal management (codeunit 280 ProjectJnlManagement)
// and the check and posting codeunits (201 Project Jnl.-Check Line, 202 Project Jnl.-Post Line, 203
// Project Jnl.-Post Batch).
package projectjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// TemplateListPageID is page 275 Project Journal Template List (the template to open a journal
// with, when there are several)
const TemplateListPageID = 275

// OpenJournalPage opens the project journal page (OnOpenPage of the Project Journal): the
// template (TemplateSelection) and its batch (OpenJnl), as the page's filters. It returns early
// when the user must pick a template.
func OpenJournalPage(rec *gtables.ProjectJournalLine, page *pages.Open, recurringJnl bool) error {
	jnlSelected, err := TemplateSelection(rec, page, recurringJnl)
	if err != nil {
		return err
	}
	if !jnlSelected {
		return nil
	}
	currentJnlBatchName := types.Code(page.Param("batch"))
	return OpenJnl(rec, page, &currentJnlBatchName)
}

// TemplateSelection finds the project journal template — the template in the page's
// parameters, the only (non-recurring) template, a new PROJECT template when there is none —
// or asks the user to pick one (page 275); the template's name becomes the page's filter and
// parameter. It reports whether a template was selected.
func TemplateSelection(rec *gtables.ProjectJournalLine, page *pages.Open, recurringJnl bool) (bool, error) {
	projectJnlTemplate := gtables.NewProjectJournalTemplate(rec)
	if page.Param("template") != "" {
		err := projectJnlTemplate.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return false, err
		}
	} else {
		projectJnlTemplate.SetRange("recurring", recurringJnl)
		switch projectJnlTemplate.Count() {
		case 0:
			projectJnlTemplate.InitRecord()
			projectJnlTemplate.Recurring = recurringJnl
			if recurringJnl {
				projectJnlTemplate.Name = "RECURRING"
				projectJnlTemplate.Description = types.Text(i18n.GetInstance().Message("RECURRING_PROJECT_JNL_TEMPLATE_DESCRIPTION", fcodeunits.CurrentLanguage()))
			} else {
				projectJnlTemplate.Name = "PROJECT"
				projectJnlTemplate.Description = types.Text(i18n.GetInstance().Message("PROJECT_JNL_TEMPLATE_DESCRIPTION", fcodeunits.CurrentLanguage()))
			}
			err := projectJnlTemplate.Validate("page_id", 0)
			if err != nil {
				return false, err
			}
			err = projectJnlTemplate.InsertOrError(false)
			if err != nil {
				return false, err
			}
		case 1:
			projectJnlTemplate.FindFirst()
		default:
			page.SelectRecord(TemplateListPageID, "template", "name", pages.FixedFilter{Field: "recurring", Value: recurringJnl})
			return false, nil
		}
	}
	page.SetParam("template", projectJnlTemplate.Name.String())
	page.SetFilter("journal_template_name", projectJnlTemplate.Name.String())
	return true, nil
}

// OpenJnl opens the journal on a batch of the template: the given one when it exists, else the
// first batch of the template — a new DEFAULT batch when it has none; the batch becomes the
// page's filter and parameter
func OpenJnl(rec *gtables.ProjectJournalLine, page *pages.Open, currentJnlBatchName *types.Code) error {
	err := checkTemplateName(rec, types.Code(page.Param("template")), currentJnlBatchName)
	if err != nil {
		return err
	}
	page.SetParam("batch", currentJnlBatchName.String())
	page.SetFilter("journal_batch_name", currentJnlBatchName.String())
	return nil
}

// checkTemplateName: the batch exists in the template, or the template's first batch is taken —
// a new DEFAULT batch when the template has none
func checkTemplateName(rec *gtables.ProjectJournalLine, currentJnlTemplateName types.Code, currentJnlBatchName *types.Code) error {
	projectJnlBatch := gtables.NewProjectJournalBatch(rec)
	projectJnlBatch.SetRange("journal_template_name", currentJnlTemplateName)
	if !projectJnlBatch.Get(map[string]interface{}{"journal_template_name": currentJnlTemplateName, "name": *currentJnlBatchName}) {
		if !projectJnlBatch.FindFirst() {
			projectJnlBatch.InitRecord()
			projectJnlBatch.Journal_template_name = currentJnlTemplateName
			err := tables.SetupNewProjectJnlBatch(projectJnlBatch)
			if err != nil {
				return err
			}
			projectJnlBatch.Name = "DEFAULT"
			projectJnlBatch.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_DEFAULT_BATCH", fcodeunits.CurrentLanguage()))
			err = projectJnlBatch.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		*currentJnlBatchName = projectJnlBatch.Name
	}
	return nil
}

// GetNames returns the line's project description and the name of its resource, item or G/L
// account (the journal page's footer, classic JobJnlManagement GetNames)
func GetNames(projectJnlLine *gtables.ProjectJournalLine) (string, string) {
	projectDescription, accName := "", ""
	if projectJnlLine.Project_no != "" {
		project := gtables.NewProject(projectJnlLine)
		if project.Get(projectJnlLine.Project_no) {
			projectDescription = project.Description.String()
		}
	}
	if projectJnlLine.No != "" {
		switch projectJnlLine.Type {
		case gtables.ProjectJournalLine_Type.Resource:
			res := gtables.NewResource(projectJnlLine)
			if res.Get(projectJnlLine.No) {
				accName = res.Name.String()
			}
		case gtables.ProjectJournalLine_Type.Item:
			item := gtables.NewItem(projectJnlLine)
			if item.Get(projectJnlLine.No) {
				accName = item.Description.String()
			}
		case gtables.ProjectJournalLine_Type.AccountGL:
			glAcc := gtables.NewGLAccount(projectJnlLine)
			if glAcc.Get(projectJnlLine.No) {
				accName = glAcc.Name.String()
			}
		}
	}
	return projectDescription, accName
}
