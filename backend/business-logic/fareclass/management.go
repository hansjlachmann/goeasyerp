package fareclass

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// TemplateListPageID is page 5638 FA Reclass. Jnl. Template List (the template to open a
// journal with, when there are several)
const TemplateListPageID = 5638

// OpenJournalPage opens the FA reclass. journal page (codeunit 5646 TemplateSelection and
// OpenJournal): the template in the page's parameters, the only template, a new RECLASSIFY
// template when there is none, or the user picks one; then the batch — the given one, else the
// template's first, a new DEFAULT batch when it has none — as the page's filters
func OpenJournalPage(rec *gtables.FAReclassJournalLine, page *pages.Open) error {
	faReclassJnlTemplate := gtables.NewFAReclassJournalTemplate(rec)
	if page.Param("template") != "" {
		err := faReclassJnlTemplate.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return err
		}
	} else {
		switch faReclassJnlTemplate.Count() {
		case 0:
			faReclassJnlTemplate.Init()
			faReclassJnlTemplate.Name = "RECLASSIFY"
			faReclassJnlTemplate.Description = types.Text(i18n.GetInstance().Message("FA_RECLASS_JNL_TEMPLATE_DESCRIPTION", fcodeunits.CurrentLanguage()))
			err := faReclassJnlTemplate.Validate("page_id", 0)
			if err != nil {
				return err
			}
			err = faReclassJnlTemplate.InsertOrError(false)
			if err != nil {
				return err
			}
		case 1:
			faReclassJnlTemplate.FindFirst()
		default:
			page.SelectRecord(TemplateListPageID, "template", "name")
			return nil
		}
	}
	page.SetParam("template", faReclassJnlTemplate.Name.String())
	page.SetFilter("journal_template_name", faReclassJnlTemplate.Name.String())

	currentJnlBatchName := types.Code(page.Param("batch"))
	err := checkTemplateName(rec, faReclassJnlTemplate.Name, &currentJnlBatchName)
	if err != nil {
		return err
	}
	page.SetParam("batch", currentJnlBatchName.String())
	page.SetFilter("journal_batch_name", currentJnlBatchName.String())
	return nil
}

// checkTemplateName (CheckTemplateName): the batch exists in the template, or the template's
// first batch is taken — a new DEFAULT batch when the template has none
func checkTemplateName(rec *gtables.FAReclassJournalLine, currentJnlTemplateName types.Code, currentJnlBatchName *types.Code) error {
	faReclassJnlBatch := gtables.NewFAReclassJournalBatch(rec)
	faReclassJnlBatch.SetRange("journal_template_name", currentJnlTemplateName)
	if !faReclassJnlBatch.Get(map[string]interface{}{"journal_template_name": currentJnlTemplateName, "name": *currentJnlBatchName}) {
		if !faReclassJnlBatch.FindFirst() {
			faReclassJnlBatch.Init()
			faReclassJnlBatch.Journal_template_name = currentJnlTemplateName
			faReclassJnlBatch.Name = "DEFAULT"
			faReclassJnlBatch.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_DEFAULT_BATCH", fcodeunits.CurrentLanguage()))
			err := faReclassJnlBatch.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		*currentJnlBatchName = faReclassJnlBatch.Name
	}
	return nil
}

// GetFAS (GetFAS): the descriptions of the line's two fixed assets (the journal's footer)
func GetFAS(faReclassJnlLine *gtables.FAReclassJournalLine) (faDescription, faDescription2 string) {
	fa := gtables.NewFixedAsset(faReclassJnlLine)
	if faReclassJnlLine.Fa_no != "" {
		if fa.Get(faReclassJnlLine.Fa_no) {
			faDescription = fa.Description.String()
		}
	}
	if faReclassJnlLine.New_fa_no != "" {
		if fa.Get(faReclassJnlLine.New_fa_no) {
			faDescription2 = fa.Description.String()
		}
	}
	return faDescription, faDescription2
}
