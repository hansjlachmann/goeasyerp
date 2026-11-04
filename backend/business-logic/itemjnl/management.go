// Package itemjnl is the item journal: journal management (codeunit 240 ItemJnlManagement) and
// the check and posting codeunits (21 Item Jnl.-Check Line, 22 Item Jnl.-Post Line, 23 Item
// Jnl.-Post Batch).
package itemjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// TemplateListPageID is page 261 Item Journal Template List (the template to open a journal
// with, when there are several of the type)
const TemplateListPageID = 261

// OpenJournalPage opens an item journal page (OnOpenPage of the Item, Item Transfer and Phys.
// Inventory Journal): the template of the page's type (TemplateSelection) and its batch
// (OpenJnl), as the page's filters. It returns early when the user must pick a template.
func OpenJournalPage(rec *gtables.ItemJournalLine, page *pages.Open, pageTemplate gtables.ItemJournalTemplateType, recurringJnl bool) error {
	jnlSelected, err := TemplateSelection(rec, page, pageTemplate, recurringJnl)
	if err != nil {
		return err
	}
	if !jnlSelected {
		return nil
	}
	currentJnlBatchName := types.Code(page.Param("batch"))
	return OpenJnl(rec, page, &currentJnlBatchName)
}

// TemplateSelection finds the item journal template of a page's type — the template in the
// page's parameters, the only template of the type, a new one when there is none — or asks
// the user to pick one (page 261); the template's name becomes the page's filter and parameter.
// It reports whether a template was selected.
func TemplateSelection(rec *gtables.ItemJournalLine, page *pages.Open, pageTemplate gtables.ItemJournalTemplateType, recurringJnl bool) (bool, error) {
	itemJnlTemplate := gtables.NewItemJournalTemplate(rec)
	if page.Param("template") != "" {
		err := itemJnlTemplate.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return false, err
		}
	} else {
		itemJnlTemplate.SetRange("recurring", recurringJnl)
		itemJnlTemplate.SetRange("type", pageTemplate)
		switch itemJnlTemplate.Count() {
		case 0:
			itemJnlTemplate.InitRecord()
			itemJnlTemplate.Recurring = recurringJnl
			err := itemJnlTemplate.Validate("type", pageTemplate)
			if err != nil {
				return false, err
			}
			err = itemJnlTemplate.Validate("page_id", 0)
			if err != nil {
				return false, err
			}
			itemJnlTemplate.Name = types.Code(system.UpperCase(system.CopyStr(pageTemplate.String(), 1, itemJnlTemplate.MaxStrLen("name"))))
			language := fcodeunits.CurrentLanguage()
			typeCaption := i18n.GetInstance().OptionCaption(gtables.ItemJournalTemplateTableName, "type", i18n.Key(pageTemplate.String()), language)
			itemJnlTemplate.Description = types.Text(i18n.GetInstance().MessageWithParams("GEN_JNL_TEMPLATE_DESCRIPTION", language, typeCaption))
			if recurringJnl {
				itemJnlTemplate.Name = "RECURRING"
				itemJnlTemplate.Description = types.Text(i18n.GetInstance().Message("RECURRING_ITEM_JNL_TEMPLATE_DESCRIPTION", language))
			}
			err = itemJnlTemplate.InsertOrError(false)
			if err != nil {
				return false, err
			}
		case 1:
			itemJnlTemplate.FindFirst()
		default:
			page.SelectRecord(TemplateListPageID, "template", "name", pages.FixedFilter{Field: "type", Value: int(pageTemplate)}, pages.FixedFilter{Field: "recurring", Value: recurringJnl})
			return false, nil
		}
	}
	page.SetParam("template", itemJnlTemplate.Name.String())
	page.SetFilter("journal_template_name", itemJnlTemplate.Name.String())
	return true, nil
}

// OpenJnl opens the journal on a batch of the template: the given one when it exists, else the
// first batch of the template — a new DEFAULT batch when it has none; the batch becomes the
// page's filter and parameter
func OpenJnl(rec *gtables.ItemJournalLine, page *pages.Open, currentJnlBatchName *types.Code) error {
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
func checkTemplateName(rec *gtables.ItemJournalLine, currentJnlTemplateName types.Code, currentJnlBatchName *types.Code) error {
	itemJnlBatch := gtables.NewItemJournalBatch(rec)
	itemJnlBatch.SetRange("journal_template_name", currentJnlTemplateName)
	if !itemJnlBatch.Get(map[string]interface{}{"journal_template_name": currentJnlTemplateName, "name": *currentJnlBatchName}) {
		if !itemJnlBatch.FindFirst() {
			itemJnlBatch.InitRecord()
			itemJnlBatch.Journal_template_name = currentJnlTemplateName
			err := tables.SetupNewItemJnlBatch(itemJnlBatch)
			if err != nil {
				return err
			}
			itemJnlBatch.Name = "DEFAULT"
			itemJnlBatch.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_DEFAULT_BATCH", fcodeunits.CurrentLanguage()))
			err = itemJnlBatch.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		*currentJnlBatchName = itemJnlBatch.Name
	}
	return nil
}

// GetItem returns the description of the line's item (the journal page's Item Description)
func GetItem(itemJnlLine *gtables.ItemJournalLine) string {
	if itemJnlLine.Item_no != "" {
		item := gtables.NewItem(itemJnlLine)
		if item.Get(itemJnlLine.Item_no) {
			return item.Description.String()
		}
	}
	return ""
}
