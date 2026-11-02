// Package intrastatjnl is codeunit 350 IntraJnlManagement: the Intrastat journal's template and
// batch selection and the statistical value totals of its footer.
package intrastatjnl

import (
	"strconv"
	"time"

	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// OpenJournalPage opens the Intrastat journal page (TemplateSelection and OpenJnl): the template
// in the page's parameters, the only template, a new INTRASTAT template when there is none, or
// the user picks one; then the batch — the given one, else the template's first, a new DEFAULT
// batch when it has none — as the page's filters
func OpenJournalPage(rec *gtables.IntrastatJnlLine, page *pages.Open) error {
	language := fcodeunits.CurrentLanguage()
	intraJnlTemplate := gtables.NewIntrastatJnlTemplate(rec)
	if page.Param("template") != "" {
		err := intraJnlTemplate.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return err
		}
	} else {
		switch intraJnlTemplate.Count() {
		case 0:
			intraJnlTemplate.Init()
			intraJnlTemplate.Name = "INTRASTAT"
			intraJnlTemplate.Description = types.Text(i18n.GetInstance().Message("INTRASTAT_JNL_TEMPLATE_DESCRIPTION", language))
			err := intraJnlTemplate.InsertOrError(true)
			if err != nil {
				return err
			}
		case 1:
			intraJnlTemplate.FindFirst()
		default:
			page.SelectRecord(tables.IntrastatJnlTemplateListPage, "template", "name")
			return nil
		}
	}
	page.SetParam("template", intraJnlTemplate.Name.String())
	page.SetFilter("journal_template_name", intraJnlTemplate.Name.String())

	currentJnlBatchName := types.Code(page.Param("batch"))
	err := CheckTemplateName(rec, intraJnlTemplate.Name, &currentJnlBatchName)
	if err != nil {
		return err
	}
	page.SetParam("batch", currentJnlBatchName.String())
	page.SetFilter("journal_batch_name", currentJnlBatchName.String())
	return nil
}

// CheckTemplateName (CheckTemplateName): the batch exists in the template, or the template's first
// batch is taken — a new DEFAULT batch when the template has none
func CheckTemplateName(rec *gtables.IntrastatJnlLine, currentJnlTemplateName types.Code, currentJnlBatchName *types.Code) error {
	intraJnlBatch := gtables.NewIntrastatJnlBatch(rec)
	intraJnlBatch.SetRange("journal_template_name", currentJnlTemplateName)
	if !intraJnlBatch.Get(map[string]interface{}{"journal_template_name": currentJnlTemplateName, "name": *currentJnlBatchName}) {
		if !intraJnlBatch.FindFirst() {
			intraJnlBatch.Init()
			intraJnlBatch.Journal_template_name = currentJnlTemplateName
			intraJnlBatch.Name = "DEFAULT"
			intraJnlBatch.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_DEFAULT_BATCH", fcodeunits.CurrentLanguage()))
			err := intraJnlBatch.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		*currentJnlBatchName = intraJnlBatch.Name
	}
	return nil
}

// CalcStatisticalValue (CalcStatisticalValue): the statistical value of the batch up to the line,
// and of the whole batch (the journal's footer)
func CalcStatisticalValue(intraJnlLine *gtables.IntrastatJnlLine) (statisticalValue, totalStatisticalValue types.Decimal) {
	tempIntraJnlLine := gtables.NewIntrastatJnlLine(intraJnlLine)
	tempIntraJnlLine.SetRange("journal_template_name", intraJnlLine.Journal_template_name)
	tempIntraJnlLine.SetRange("journal_batch_name", intraJnlLine.Journal_batch_name)
	if tempIntraJnlLine.CalcSums("statistical_value") == nil {
		totalStatisticalValue = tempIntraJnlLine.Statistical_value
	}
	tempIntraJnlLine.SetFilter("line_no", ".."+strconv.Itoa(intraJnlLine.Line_no))
	if tempIntraJnlLine.CalcSums("statistical_value") == nil {
		statisticalValue = tempIntraJnlLine.Statistical_value
	}
	return statisticalValue, totalStatisticalValue
}

// PeriodDates: the first and last date of a batch's Statistics Period (YYMM, the century of the
// work date)
func PeriodDates(statisticsPeriod types.Code, workDate types.Date) (startDate, endDate types.Date, ok bool) {
	if len(statisticsPeriod) != 4 {
		return startDate, endDate, false
	}
	year, err1 := strconv.Atoi(string(statisticsPeriod[0:2]))
	month, err2 := strconv.Atoi(string(statisticsPeriod[2:4]))
	if err1 != nil || err2 != nil || month < 1 || month > 12 {
		return startDate, endDate, false
	}
	century := workDate.Year() / 100
	startDate = types.NewDate(century*100+year, time.Month(month), 1)
	endDate = startDate.AddMonths(1).AddDays(-1)
	return startDate, endDate, true
}
