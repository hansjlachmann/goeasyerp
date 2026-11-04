// Package genjnl is the general journal: journal management (codeunit 230 GenJnlManagement),
// the check and posting codeunits and the application of entries.
package genjnl

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// TemplateListPageID is page 250 General Journal Template List (the template to open a
// journal with, when there are several of the type)
const TemplateListPageID = 250

// OpenJournalPage opens a journal page (OnOpenPage of the General, Sales, Purchase, Cash
// Receipt and Payment Journal): the template of the page's type (TemplateSelection: the one
// in the page's parameters, the only one, a new one, or the user picks one) and its batch
// (OpenJnl), as the page's filters. It returns early when the user must pick a template.
func OpenJournalPage(rec *gtables.GenJournalLine, page *pages.Open, pageTemplate gtables.GenJournalTemplateType, recurringJnl bool) error {
	jnlSelected, err := TemplateSelection(rec, page, pageTemplate, recurringJnl)
	if err != nil {
		return err
	}
	if !jnlSelected {
		return nil
	}
	currentJnlBatchName := types.Code(page.Param("batch"))
	err = OpenJnl(rec, page, &currentJnlBatchName)
	if err != nil {
		return err
	}
	return nil
}

// TemplateSelection finds the journal template of a page's type — the template in the page's
// parameters, the only template of the type, a new one when there is none — or asks the user
// to pick one (page 250); the template's name becomes the page's filter and parameter. It
// reports whether a template was selected. The Recurring General Journal takes the recurring
// templates of any type (a new one: RECURRING).
func TemplateSelection(rec *gtables.GenJournalLine, page *pages.Open, pageTemplate gtables.GenJournalTemplateType, recurringJnl bool) (bool, error) {
	genJnlTemplate := gtables.NewGenJournalTemplate(rec)
	if page.Param("template") != "" {
		err := genJnlTemplate.GetOrError(types.Code(page.Param("template")))
		if err != nil {
			return false, err
		}
	} else {
		genJnlTemplate.SetRange("recurring", recurringJnl)
		if !recurringJnl {
			genJnlTemplate.SetRange("type", pageTemplate)
		}
		switch genJnlTemplate.Count() {
		case 0:
			genJnlTemplate.InitRecord()
			genJnlTemplate.Type = pageTemplate
			genJnlTemplate.Recurring = recurringJnl
			language := fcodeunits.CurrentLanguage()
			if !recurringJnl {
				genJnlTemplate.Name = types.Code(system.UpperCase(system.CopyStr(pageTemplate.String(), 1, genJnlTemplate.MaxStrLen("name"))))
				typeCaption := i18n.GetInstance().OptionCaption(gtables.GenJournalTemplateTableName, "type", i18n.Key(pageTemplate.String()), language)
				genJnlTemplate.Description = types.Text(i18n.GetInstance().MessageWithParams("GEN_JNL_TEMPLATE_DESCRIPTION", language, typeCaption))
			} else {
				genJnlTemplate.Name = "RECURRING"
				genJnlTemplate.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_RECURRING_TEMPLATE_DESCRIPTION", language))
			}
			err := genJnlTemplate.Validate("type", genJnlTemplate.Type)
			if err != nil {
				return false, err
			}
			err = genJnlTemplate.InsertOrError(true)
			if err != nil {
				return false, err
			}
		case 1:
			genJnlTemplate.FindFirst()
		default:
			if recurringJnl {
				page.SelectRecord(TemplateListPageID, "template", "name", pages.FixedFilter{Field: "recurring", Value: true})
			} else {
				page.SelectRecord(TemplateListPageID, "template", "name", pages.FixedFilter{Field: "type", Value: int(pageTemplate)}, pages.FixedFilter{Field: "recurring", Value: false})
			}
			return false, nil
		}
	}
	page.SetParam("template", genJnlTemplate.Name.String())
	page.SetFilter("journal_template_name", genJnlTemplate.Name.String())
	return true, nil
}

// OpenJnl opens the journal on a batch of the template: the given one when it exists, else the
// first batch of the template — a new DEFAULT batch when it has none; the batch becomes the
// page's filter and parameter
func OpenJnl(rec *gtables.GenJournalLine, page *pages.Open, currentJnlBatchName *types.Code) error {
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
func checkTemplateName(rec *gtables.GenJournalLine, currentJnlTemplateName types.Code, currentJnlBatchName *types.Code) error {
	genJnlBatch := gtables.NewGenJournalBatch(rec)
	genJnlBatch.SetRange("journal_template_name", currentJnlTemplateName)
	if !genJnlBatch.Get(tables.GenJnlBatchKey(currentJnlTemplateName, *currentJnlBatchName)) {
		if !genJnlBatch.FindFirst() {
			genJnlBatch.InitRecord()
			genJnlBatch.Journal_template_name = currentJnlTemplateName
			err := tables.SetupNewBatch(genJnlBatch)
			if err != nil {
				return err
			}
			genJnlBatch.Name = "DEFAULT"
			genJnlBatch.Description = types.Text(i18n.GetInstance().Message("GEN_JNL_DEFAULT_BATCH", fcodeunits.CurrentLanguage()))
			err = genJnlBatch.InsertOrError(true)
			if err != nil {
				return err
			}
		}
		*currentJnlBatchName = genJnlBatch.Name
	}
	return nil
}

// GetAccounts returns the names of the line's account and balancing account (the journal
// page's Account Name and Bal. Account Name)
func GetAccounts(genJnlLine *gtables.GenJournalLine) (accName, balAccName string) {
	if genJnlLine.Account_no != "" {
		accName = accountName(genJnlLine, int(genJnlLine.Account_type), genJnlLine.Account_no)
	}
	if genJnlLine.Bal_account_no != "" {
		balAccName = accountName(genJnlLine, int(genJnlLine.Bal_account_type), genJnlLine.Bal_account_no)
	}
	return accName, balAccName
}

func accountName(rec *gtables.GenJournalLine, accountType int, no types.Code) string {
	switch accountType {
	case 0:
		glAcc := gtables.NewGLAccount(rec)
		if glAcc.Get(no) {
			return glAcc.Name.String()
		}
	case 1:
		cust := gtables.NewCustomer(rec)
		if cust.Get(no) {
			return cust.Name.String()
		}
	case 2:
		vend := gtables.NewVendor(rec)
		if vend.Get(no) {
			return vend.Name.String()
		}
	case 3:
		bankAcc := gtables.NewBankAccount(rec)
		if bankAcc.Get(no) {
			return bankAcc.Name.String()
		}
	case 4:
		fa := gtables.NewFixedAsset(rec)
		if fa.Get(no) {
			return fa.Description.String()
		}
	}
	return ""
}

// CalcBalance returns the journal's balance up to the line (Balance) and of the whole batch
// (Total Balance), in LCY; for a new line (Line No. 0) the balance up to the line it follows,
// lastGenJnlLine
func CalcBalance(genJnlLine, lastGenJnlLine *gtables.GenJournalLine) (balance, totalBalance types.Decimal) {
	tempGenJnlLine := gtables.NewGenJournalLine(genJnlLine)
	tempGenJnlLine.SetRange("journal_template_name", genJnlLine.Journal_template_name)
	tempGenJnlLine.SetRange("journal_batch_name", genJnlLine.Journal_batch_name)
	err := tempGenJnlLine.CalcSums("balance_lcy")
	if err == nil {
		totalBalance = tempGenJnlLine.Balance_lcy
		if genJnlLine.Line_no == 0 {
			totalBalance = totalBalance.Add(lastGenJnlLine.Balance_lcy)
		}
	}

	if genJnlLine.Line_no != 0 {
		tempGenJnlLine.SetRange("line_no", 0, genJnlLine.Line_no)
		err = tempGenJnlLine.CalcSums("balance_lcy")
		if err == nil {
			balance = tempGenJnlLine.Balance_lcy
		}
	} else {
		tempGenJnlLine.SetRange("line_no", 0, lastGenJnlLine.Line_no)
		err = tempGenJnlLine.CalcSums("balance_lcy")
		if err == nil {
			balance = tempGenJnlLine.Balance_lcy
			tempGenJnlLine.SetRange("line_no")
			tempGenJnlLine.SetFilter("line_no", ">"+strconv.Itoa(lastGenJnlLine.Line_no))
			if tempGenJnlLine.IsEmpty() {
				balance = balance.Add(lastGenJnlLine.Balance_lcy)
			}
		}
	}
	return balance, totalBalance
}
