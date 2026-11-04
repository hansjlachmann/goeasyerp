package insurancejnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// TemplateListPageID is page 5653 Insurance Jnl. Template List (the template to open a journal
// with, when there are several)
const TemplateListPageID = 5653

// OpenJournalPage opens the insurance journal page (codeunit 5656 TemplateSelection and
// OpenJournal): the template in the page's parameters, the only template, a new INSURANCE
// template when there is none, or the user picks one; then the batch — the given one, else the
// template's first, a new DEFAULT batch when it has none — as the page's filters
func OpenJournalPage(rec *gtables.InsuranceJournalLine, page *pages.Open) error {
	insuranceJnlTemplate := gtables.NewInsuranceJournalTemplate(rec)
	if page.Param("template") != "" {
		err := insuranceJnlTemplate.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return err
		}
	} else {
		switch insuranceJnlTemplate.Count() {
		case 0:
			insuranceJnlTemplate.Init()
			insuranceJnlTemplate.Name = "INSURANCE"
			insuranceJnlTemplate.Description = types.Text(i18n.GetInstance().Message("INSURANCE_JNL_TEMPLATE_DESCRIPTION", fcodeunits.CurrentLanguage()))
			err := insuranceJnlTemplate.Validate("page_id", 0)
			if err != nil {
				return err
			}
			err = insuranceJnlTemplate.InsertOrError(false)
			if err != nil {
				return err
			}
		case 1:
			insuranceJnlTemplate.FindFirst()
		default:
			page.SelectRecord(TemplateListPageID, "template", "name")
			return nil
		}
	}
	page.SetParam("template", insuranceJnlTemplate.Name.String())
	page.SetFilter("journal_template_name", insuranceJnlTemplate.Name.String())

	currentJnlBatchName := types.Code(page.Param("batch"))
	err := checkTemplateName(rec, insuranceJnlTemplate.Name, &currentJnlBatchName)
	if err != nil {
		return err
	}
	page.SetParam("batch", currentJnlBatchName.String())
	page.SetFilter("journal_batch_name", currentJnlBatchName.String())
	return nil
}

// checkTemplateName (CheckTemplateName): the batch exists in the template, or the template's
// first batch is taken — a new DEFAULT batch when the template has none
func checkTemplateName(rec *gtables.InsuranceJournalLine, currentJnlTemplateName types.Code, currentJnlBatchName *types.Code) error {
	insuranceJnlBatch := gtables.NewInsuranceJournalBatch(rec)
	if !insuranceJnlBatch.Get(map[string]interface{}{"journal_template_name": currentJnlTemplateName, "name": *currentJnlBatchName}) {
		insuranceJnlBatch.SetRange("journal_template_name", currentJnlTemplateName)
		if !insuranceJnlBatch.FindFirst() {
			insuranceJnlBatch.Init()
			insuranceJnlBatch.Journal_template_name = currentJnlTemplateName
			err := tables.SetupNewInsuranceJnlBatch(insuranceJnlBatch)
			if err != nil {
				return err
			}
			insuranceJnlBatch.Name = "DEFAULT"
			insuranceJnlBatch.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_DEFAULT_BATCH", fcodeunits.CurrentLanguage()))
			err = insuranceJnlBatch.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		*currentJnlBatchName = insuranceJnlBatch.Name
	}
	return nil
}

// GetDescriptions (GetDescriptions): the descriptions of the line's insurance and fixed asset
// (the journal's footer)
func GetDescriptions(insuranceJnlLine *gtables.InsuranceJournalLine) (insuranceDescription, faDescription string) {
	if insuranceJnlLine.Insurance_no != "" {
		insurance := gtables.NewInsurance(insuranceJnlLine)
		if insurance.Get(insuranceJnlLine.Insurance_no) {
			insuranceDescription = insurance.Description.String()
		}
	}
	if insuranceJnlLine.Fa_no != "" {
		fa := gtables.NewFixedAsset(insuranceJnlLine)
		if fa.Get(insuranceJnlLine.Fa_no) {
			faDescription = fa.Description.String()
		}
	}
	return insuranceDescription, faDescription
}
