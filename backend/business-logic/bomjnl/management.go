// Package bomjnl is the BOM journal: journal management (codeunit 250 BOMJnlManagement), the check
// and posting codeunits (31 BOM Jnl.-Check Line, 32 BOM Jnl.-Post Line, 33 BOM Jnl.-Post Batch)
// and the bill of materials functions (51 BOM-Explode BOM, 54 BOM-Calculate Cost, 246 Item
// Jnl.-Explode BOM).
package bomjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// TemplateListPageID is page 264 BOM Journal Template List (the template to open a journal
// with, when there are several)
const TemplateListPageID = 264

// OpenJournalPage opens the BOM journal page (OnOpenPage of the BOM Journal): the
// template (TemplateSelection) and its batch (OpenJnl), as the page's filters. It returns early
// when the user must pick a template.
func OpenJournalPage(rec *gtables.BOMJournalLine, page *pages.Open, recurringJnl bool) error {
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

// TemplateSelection finds the BOM journal template — the template in the page's
// parameters, the only (non-recurring) template, a new BOM template when there is none —
// or asks the user to pick one (page 264); the template's name becomes the page's filter and
// parameter. It reports whether a template was selected.
func TemplateSelection(rec *gtables.BOMJournalLine, page *pages.Open, recurringJnl bool) (bool, error) {
	bomJnlTemplate := gtables.NewBOMJournalTemplate(rec)
	if page.Param("template") != "" {
		err := bomJnlTemplate.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return false, err
		}
	} else {
		bomJnlTemplate.SetRange("recurring", recurringJnl)
		switch bomJnlTemplate.Count() {
		case 0:
			bomJnlTemplate.InitRecord()
			bomJnlTemplate.Recurring = recurringJnl
			if recurringJnl {
				bomJnlTemplate.Name = "RECURRING"
				bomJnlTemplate.Description = types.Text(i18n.GetInstance().Message("RECURRING_BOM_JNL_TEMPLATE_DESCRIPTION", fcodeunits.CurrentLanguage()))
			} else {
				bomJnlTemplate.Name = "BOM"
				bomJnlTemplate.Description = types.Text(i18n.GetInstance().Message("BOM_JNL_TEMPLATE_DESCRIPTION", fcodeunits.CurrentLanguage()))
			}
			err := bomJnlTemplate.Validate("page_id", 0)
			if err != nil {
				return false, err
			}
			err = bomJnlTemplate.InsertOrError(false)
			if err != nil {
				return false, err
			}
		case 1:
			bomJnlTemplate.FindFirst()
		default:
			page.SelectRecord(TemplateListPageID, "template", "name", pages.FixedFilter{Field: "recurring", Value: recurringJnl})
			return false, nil
		}
	}
	page.SetParam("template", bomJnlTemplate.Name.String())
	page.SetFilter("journal_template_name", bomJnlTemplate.Name.String())
	return true, nil
}

// OpenJnl opens the journal on a batch of the template: the given one when it exists, else the
// first batch of the template — a new DEFAULT batch when it has none; the batch becomes the
// page's filter and parameter
func OpenJnl(rec *gtables.BOMJournalLine, page *pages.Open, currentJnlBatchName *types.Code) error {
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
func checkTemplateName(rec *gtables.BOMJournalLine, currentJnlTemplateName types.Code, currentJnlBatchName *types.Code) error {
	bomJnlBatch := gtables.NewBOMJournalBatch(rec)
	bomJnlBatch.SetRange("journal_template_name", currentJnlTemplateName)
	if !bomJnlBatch.Get(map[string]interface{}{"journal_template_name": currentJnlTemplateName, "name": *currentJnlBatchName}) {
		if !bomJnlBatch.FindFirst() {
			bomJnlBatch.InitRecord()
			bomJnlBatch.Journal_template_name = currentJnlTemplateName
			err := tables.SetupNewBOMJnlBatch(bomJnlBatch)
			if err != nil {
				return err
			}
			bomJnlBatch.Name = "DEFAULT"
			bomJnlBatch.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_DEFAULT_BATCH", fcodeunits.CurrentLanguage()))
			err = bomJnlBatch.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		*currentJnlBatchName = bomJnlBatch.Name
	}
	return nil
}

// GetItem returns the description of the line's item (the journal page's Item Description)
func GetItem(bomJnlLine *gtables.BOMJournalLine) string {
	if bomJnlLine.Item_no != "" {
		item := gtables.NewItem(bomJnlLine)
		if item.Get(bomJnlLine.Item_no) {
			return item.Description.String()
		}
	}
	return ""
}
