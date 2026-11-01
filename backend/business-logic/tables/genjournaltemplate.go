package tables

import (
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// genJournalTemplateLogic is the business logic of table 80 Gen. Journal Template: a template
// is a journal type (General, Sales, …) with its page, source code and numbering.
//
// Not migrated yet: the test and posting reports (report prints), recurring journals (their
// page), fixed asset G/L and payroll accrual journals (their pages).
type genJournalTemplateLogic struct{}

func init() { gtables.RegisterGenJournalTemplateLogic(genJournalTemplateLogic{}) }

// The journal page of each template type (Form ID): General Journal 39, Sales Journal 253,
// Purchase Journal 254, Cash Receipt Journal 255, Payment Journal 256, Fixed Asset G/L Journal
// 5628
var genJnlTemplatePages = map[gtables.GenJournalTemplateType]int{
	gtables.GenJournalTemplate_Type.General:      39,
	gtables.GenJournalTemplate_Type.Sales:        253,
	gtables.GenJournalTemplate_Type.Purchases:    254,
	gtables.GenJournalTemplate_Type.CashReceipts: 255,
	gtables.GenJournalTemplate_Type.Payments:     256,
	gtables.GenJournalTemplate_Type.Assets:       5628,
}

// OnInsert: a template without a page gets its type's page
func (genJournalTemplateLogic) OnInsert(rec *gtables.GenJournalTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnDelete deletes the template's journal lines (with their triggers) and batches
func (genJournalTemplateLogic) OnDelete(rec *gtables.GenJournalTemplate) error {
	genJnlLine := gtables.NewGenJournalLine(rec)
	genJnlLine.SetRange("journal_template_name", rec.Name)
	err := deleteGenJnlLines(genJnlLine)
	if err != nil {
		return err
	}
	genJnlBatch := gtables.NewGenJournalBatch(rec)
	genJnlBatch.SetRange("journal_template_name", rec.Name)
	genJnlBatch.DeleteAll()
	return nil
}

// deleteGenJnlLines deletes the journal lines in the record's filters, each with its OnDelete
// trigger (DELETEALL(TRUE))
func deleteGenJnlLines(genJnlLine *gtables.GenJournalLine) error {
	for genJnlLine.FindFirst() {
		err := genJnlLine.DeleteOrError(true)
		if err != nil {
			return err
		}
	}
	return nil
}

// OnValidate_Page_id: no page — the type's page
func (genJournalTemplateLogic) OnValidate_Page_id(rec *gtables.GenJournalTemplate) error {
	if rec.Page_id == 0 {
		return rec.Validate("type", rec.Type)
	}
	return nil
}

// OnValidate_Type: the type's source code (Source Code Setup) and page
func (genJournalTemplateLogic) OnValidate_Type(rec *gtables.GenJournalTemplate) error {
	sourceCodeSetup := gtables.GetSourceCodeSetup(rec)
	switch rec.Type {
	case gtables.GenJournalTemplate_Type.General:
		rec.Source_code = sourceCodeSetup.General_journal
	case gtables.GenJournalTemplate_Type.Sales:
		rec.Source_code = sourceCodeSetup.Sales_journal
	case gtables.GenJournalTemplate_Type.Purchases:
		rec.Source_code = sourceCodeSetup.Purchase_journal
	case gtables.GenJournalTemplate_Type.CashReceipts:
		rec.Source_code = sourceCodeSetup.Cash_receipt_journal
	case gtables.GenJournalTemplate_Type.Payments:
		rec.Source_code = sourceCodeSetup.Payment_journal
	case gtables.GenJournalTemplate_Type.Assets:
		rec.Source_code = sourceCodeSetup.Fixed_asset_g_l_journal
	}
	rec.Page_id = genJnlTemplatePages[rec.Type]
	return nil
}

// OnValidate_Source_code: the template's journal lines take the new source code
func (genJournalTemplateLogic) OnValidate_Source_code(rec *gtables.GenJournalTemplate) error {
	genJnlLine := gtables.NewGenJournalLine(rec)
	genJnlLine.SetRange("journal_template_name", rec.Name)
	genJnlLine.ModifyAll("source_code", rec.Source_code)
	return nil
}

// OnValidate_Recurring: the type's page again; recurring journals have no No. Series
func (genJournalTemplateLogic) OnValidate_Recurring(rec *gtables.GenJournalTemplate) error {
	err := rec.Validate("type", rec.Type)
	if err != nil {
		return err
	}
	if rec.Recurring {
		return rec.TestField("no_series", types.Code(""))
	}
	return nil
}

// OnValidate_Bal_account_type: Bal. Account No. starts again
func (genJournalTemplateLogic) OnValidate_Bal_account_type(rec *gtables.GenJournalTemplate) error {
	rec.Bal_account_no = ""
	return nil
}

// OnValidate_Bal_account_no: a G/L account must allow direct posting
func (genJournalTemplateLogic) OnValidate_Bal_account_no(rec *gtables.GenJournalTemplate) error {
	if rec.Bal_account_type == gtables.GenJournalTemplate_Bal_account_type.GLAccount {
		return checkJnlBalGLAcc(rec, rec.Bal_account_no)
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (genJournalTemplateLogic) OnValidate_No_series(rec *gtables.GenJournalTemplate) error {
	if rec.No_series != "" {
		if rec.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.GenJournalTemplateTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			rec.Posting_no_series = ""
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series
func (genJournalTemplateLogic) OnValidate_Posting_no_series(rec *gtables.GenJournalTemplate) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.GenJournalTemplateTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	return nil
}

// checkJnlBalGLAcc: a balancing G/L account of a template or batch is a posting account that
// is not blocked and allows direct posting — CheckGLAcc
func checkJnlBalGLAcc(rec ftables.Source, accNo types.Code) error {
	if accNo != "" {
		glAcc := gtables.NewGLAccount(rec)
		err := glAcc.GetOrError(accNo)
		if err != nil {
			return err
		}
		err = CheckGLAcc(glAcc)
		if err != nil {
			return err
		}
		return glAcc.TestField("direct_posting", true)
	}
	return nil
}
