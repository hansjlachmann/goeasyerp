// Package fajnl is the fixed asset journal: journal management (codeunit 5638 FAJnlManagement), the
// check and posting codeunits (5631 FA Jnl.-Check Line, 5632 FA Jnl.-Post Line with 5600 FA
// Insert Ledger Entry, 5604/5647 Make FA/Maintenance Ledger Entry and 5606 FA Check Consistency,
// 5633 FA Jnl.-Post Batch).
package fajnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// TemplateListPageID is page 5631 FA Journal Template List (the template to open a journal
// with, when there are several)
const TemplateListPageID = 5631

// OpenJournalPage opens the FA journal page (OnOpenPage of the Fixed Asset Journal): the
// template (TemplateSelection) and its batch (OpenJnl), as the page's filters. It returns early
// when the user must pick a template.
func OpenJournalPage(rec *gtables.FAJournalLine, page *pages.Open, recurringJnl bool) error {
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

// TemplateSelection finds the FA journal template — the template in the page's
// parameters, the only (non-recurring) template, a new FA template when there is none —
// or asks the user to pick one (page 5631); the template's name becomes the page's filter and
// parameter. It reports whether a template was selected.
func TemplateSelection(rec *gtables.FAJournalLine, page *pages.Open, recurringJnl bool) (bool, error) {
	faJnlTemplate := gtables.NewFAJournalTemplate(rec)
	if page.Param("template") != "" {
		err := faJnlTemplate.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return false, err
		}
	} else {
		faJnlTemplate.SetRange("recurring", recurringJnl)
		switch faJnlTemplate.Count() {
		case 0:
			faJnlTemplate.InitRecord()
			faJnlTemplate.Recurring = recurringJnl
			if recurringJnl {
				faJnlTemplate.Name = "RECURRING"
				faJnlTemplate.Description = types.Text(i18n.GetInstance().Message("RECURRING_FA_JNL_TEMPLATE_DESCRIPTION", fcodeunits.CurrentLanguage()))
			} else {
				faJnlTemplate.Name = "ASSETS"
				faJnlTemplate.Description = types.Text(i18n.GetInstance().Message("FA_JNL_TEMPLATE_DESCRIPTION", fcodeunits.CurrentLanguage()))
			}
			err := faJnlTemplate.Validate("page_id", 0)
			if err != nil {
				return false, err
			}
			err = faJnlTemplate.InsertOrError(false)
			if err != nil {
				return false, err
			}
		case 1:
			faJnlTemplate.FindFirst()
		default:
			page.SelectRecord(TemplateListPageID, "template", "name", pages.FixedFilter{Field: "recurring", Value: recurringJnl})
			return false, nil
		}
	}
	page.SetParam("template", faJnlTemplate.Name.String())
	page.SetFilter("journal_template_name", faJnlTemplate.Name.String())
	return true, nil
}

// OpenJnl opens the journal on a batch of the template: the given one when it exists, else the
// first batch of the template — a new DEFAULT batch when it has none; the batch becomes the
// page's filter and parameter
func OpenJnl(rec *gtables.FAJournalLine, page *pages.Open, currentJnlBatchName *types.Code) error {
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
func checkTemplateName(rec *gtables.FAJournalLine, currentJnlTemplateName types.Code, currentJnlBatchName *types.Code) error {
	faJnlBatch := gtables.NewFAJournalBatch(rec)
	faJnlBatch.SetRange("journal_template_name", currentJnlTemplateName)
	if !faJnlBatch.Get(map[string]interface{}{"journal_template_name": currentJnlTemplateName, "name": *currentJnlBatchName}) {
		if !faJnlBatch.FindFirst() {
			faJnlBatch.InitRecord()
			faJnlBatch.Journal_template_name = currentJnlTemplateName
			err := tables.SetupNewFAJnlBatch(faJnlBatch)
			if err != nil {
				return err
			}
			faJnlBatch.Name = "DEFAULT"
			faJnlBatch.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_DEFAULT_BATCH", fcodeunits.CurrentLanguage()))
			err = faJnlBatch.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		*currentJnlBatchName = faJnlBatch.Name
	}
	return nil
}

// GetFADescription returns the description of the line's fixed asset (the journal page's FA
// Description)
func GetFADescription(faJnlLine *gtables.FAJournalLine) string {
	if faJnlLine.Fa_no != "" {
		fa := gtables.NewFixedAsset(faJnlLine)
		if fa.Get(faJnlLine.Fa_no) {
			return fa.Description.String()
		}
	}
	return ""
}
