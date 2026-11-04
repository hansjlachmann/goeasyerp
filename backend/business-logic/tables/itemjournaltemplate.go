package tables

import (
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// itemJournalTemplateLogic is the business logic of table 82 Item Journal Template: a template
// is a journal type (Item, Transfer, Phys. Inventory) with its page, source code and numbering.
// The type (Recurring) sets the page, the source code and the test and posting reports.
type itemJournalTemplateLogic struct{}

func init() { gtables.RegisterItemJournalTemplateLogic(itemJournalTemplateLogic{}) }

// The journal page of each template type (Form ID): Item Journal 40, Item Transfer Journal 393,
// Phys. Inventory Journal 392
var itemJnlTemplatePages = map[gtables.ItemJournalTemplateType]int{
	gtables.ItemJournalTemplate_Type.Item:          40,
	gtables.ItemJournalTemplate_Type.Transfer:      393,
	gtables.ItemJournalTemplate_Type.PhysInventory: 392,
}

// OnInsert: a template without a page gets its type's page
func (itemJournalTemplateLogic) OnInsert(rec *gtables.ItemJournalTemplate) error {
	return rec.Validate("page_id", rec.Page_id)
}

// OnDelete deletes the template's journal lines and batches
func (itemJournalTemplateLogic) OnDelete(rec *gtables.ItemJournalTemplate) error {
	itemJnlLine := gtables.NewItemJournalLine(rec)
	itemJnlLine.SetRange("journal_template_name", rec.Name)
	itemJnlLine.DeleteAll()
	itemJnlBatch := gtables.NewItemJournalBatch(rec)
	itemJnlBatch.SetRange("journal_template_name", rec.Name)
	itemJnlBatch.DeleteAll()
	return nil
}

// OnValidate_Page_id: no page — the type's page
func (itemJournalTemplateLogic) OnValidate_Page_id(rec *gtables.ItemJournalTemplate) error {
	if rec.Page_id == 0 {
		return rec.Validate("type", rec.Type)
	}
	return nil
}

// OnValidate_Type: the type's source code (Source Code Setup) and page (a recurring template:
// the Recurring Item Jnl.)
func (itemJournalTemplateLogic) OnValidate_Type(rec *gtables.ItemJournalTemplate) error {
	sourceCodeSetup := gtables.GetSourceCodeSetup(rec)
	switch rec.Type {
	case gtables.ItemJournalTemplate_Type.Item:
		rec.Source_code = sourceCodeSetup.Item_journal
	case gtables.ItemJournalTemplate_Type.Transfer:
		rec.Source_code = sourceCodeSetup.Item_transfer_journal
	case gtables.ItemJournalTemplate_Type.PhysInventory:
		rec.Source_code = sourceCodeSetup.Phys_inventory_journal
	}
	rec.Page_id = itemJnlTemplatePages[rec.Type]
	if rec.Recurring {
		rec.Page_id = RecurringItemJnlPageID
	}
	rec.Test_report_id = ItemJnlTestReportID
	rec.Posting_report_id = ItemRegisterReportID
	return nil
}

// The item journals' reports: 702 Inventory Posting - Test, 703 Item Register
const (
	ItemJnlTestReportID  = 702
	ItemRegisterReportID = 703
)

// RecurringItemJnlPageID is page 286 Recurring Item Jnl., the page of every recurring item template
const RecurringItemJnlPageID = 286

// OnValidate_Source_code: the template's journal lines take the new source code
func (itemJournalTemplateLogic) OnValidate_Source_code(rec *gtables.ItemJournalTemplate) error {
	itemJnlLine := gtables.NewItemJournalLine(rec)
	itemJnlLine.SetRange("journal_template_name", rec.Name)
	itemJnlLine.ModifyAll("source_code", rec.Source_code)
	return nil
}

// OnValidate_Recurring: the type's page again; recurring journals have no No. Series
func (itemJournalTemplateLogic) OnValidate_Recurring(rec *gtables.ItemJournalTemplate) error {
	err := rec.Validate("type", rec.Type)
	if err != nil {
		return err
	}
	if rec.Recurring {
		return rec.TestField("no_series", types.Code(""))
	}
	return nil
}

// OnValidate_No_series: not on recurring journals; the posting no. series must differ
func (itemJournalTemplateLogic) OnValidate_No_series(rec *gtables.ItemJournalTemplate) error {
	if rec.No_series != "" {
		if rec.Recurring {
			return apperrors.JnlRecurringOnlyField(gtables.ItemJournalTemplateTableName, "posting_no_series")
		}
		if rec.No_series == rec.Posting_no_series {
			rec.Posting_no_series = ""
		}
	}
	return nil
}

// OnValidate_Posting_no_series: must differ from the No. Series
func (itemJournalTemplateLogic) OnValidate_Posting_no_series(rec *gtables.ItemJournalTemplate) error {
	if rec.Posting_no_series == rec.No_series && rec.Posting_no_series != "" {
		return apperrors.JnlMustNotBe(gtables.ItemJournalTemplateTableName, "posting_no_series", rec.Posting_no_series.String())
	}
	return nil
}
