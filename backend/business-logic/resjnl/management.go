// Package resjnl is the resource journal: journal management (codeunit 270 ResJnlManagement)
// and the check and posting codeunits (211 Res. Jnl.-Check Line, 212 Res. Jnl.-Post Line, 213
// Res. Jnl.-Post Batch).
package resjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// TemplateListPageID is page 271 Res. Journal Template List (the template to open a journal
// with, when there are several)
const TemplateListPageID = 271

// OpenJournalPage opens the resource journal page (OnOpenPage of the Resource Journal): the
// template (TemplateSelection) and its batch (OpenJnl), as the page's filters. It returns early
// when the user must pick a template.
func OpenJournalPage(rec *gtables.ResJournalLine, page *pages.Open, recurringJnl bool) error {
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

// TemplateSelection finds the resource journal template — the template in the page's
// parameters, the only (non-recurring) template, a new RESOURCES template when there is none —
// or asks the user to pick one (page 271); the template's name becomes the page's filter and
// parameter. It reports whether a template was selected.
func TemplateSelection(rec *gtables.ResJournalLine, page *pages.Open, recurringJnl bool) (bool, error) {
	resJnlTemplate := gtables.NewResJournalTemplate(rec)
	if page.Param("template") != "" {
		err := resJnlTemplate.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return false, err
		}
	} else {
		resJnlTemplate.SetRange("recurring", recurringJnl)
		switch resJnlTemplate.Count() {
		case 0:
			resJnlTemplate.InitRecord()
			resJnlTemplate.Recurring = recurringJnl
			if recurringJnl {
				resJnlTemplate.Name = "RECURRING"
				resJnlTemplate.Description = types.Text(i18n.GetInstance().Message("RECURRING_RES_JNL_TEMPLATE_DESCRIPTION", fcodeunits.CurrentLanguage()))
			} else {
				resJnlTemplate.Name = "RESOURCES"
				resJnlTemplate.Description = types.Text(i18n.GetInstance().Message("RES_JNL_TEMPLATE_DESCRIPTION", fcodeunits.CurrentLanguage()))
			}
			err := resJnlTemplate.Validate("page_id", 0)
			if err != nil {
				return false, err
			}
			err = resJnlTemplate.InsertOrError(false)
			if err != nil {
				return false, err
			}
		case 1:
			resJnlTemplate.FindFirst()
		default:
			page.SelectRecord(TemplateListPageID, "template", "name", pages.FixedFilter{Field: "recurring", Value: recurringJnl})
			return false, nil
		}
	}
	page.SetParam("template", resJnlTemplate.Name.String())
	page.SetFilter("journal_template_name", resJnlTemplate.Name.String())
	return true, nil
}

// OpenJnl opens the journal on a batch of the template: the given one when it exists, else the
// first batch of the template — a new DEFAULT batch when it has none; the batch becomes the
// page's filter and parameter
func OpenJnl(rec *gtables.ResJournalLine, page *pages.Open, currentJnlBatchName *types.Code) error {
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
func checkTemplateName(rec *gtables.ResJournalLine, currentJnlTemplateName types.Code, currentJnlBatchName *types.Code) error {
	resJnlBatch := gtables.NewResJournalBatch(rec)
	resJnlBatch.SetRange("journal_template_name", currentJnlTemplateName)
	if !resJnlBatch.Get(map[string]interface{}{"journal_template_name": currentJnlTemplateName, "name": *currentJnlBatchName}) {
		if !resJnlBatch.FindFirst() {
			resJnlBatch.InitRecord()
			resJnlBatch.Journal_template_name = currentJnlTemplateName
			err := tables.SetupNewResJnlBatch(resJnlBatch)
			if err != nil {
				return err
			}
			resJnlBatch.Name = "DEFAULT"
			resJnlBatch.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_DEFAULT_BATCH", fcodeunits.CurrentLanguage()))
			err = resJnlBatch.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		*currentJnlBatchName = resJnlBatch.Name
	}
	return nil
}

// GetRes returns the name of the line's resource (the journal page's Resource Name)
func GetRes(resJnlLine *gtables.ResJournalLine) string {
	if resJnlLine.Resource_no != "" {
		res := gtables.NewResource(resJnlLine)
		if res.Get(resJnlLine.Resource_no) {
			return res.Name.String()
		}
	}
	return ""
}
