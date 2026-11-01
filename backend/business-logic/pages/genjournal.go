package pages

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The journal pages 39 General Journal, 253 Sales Journal, 254 Purchase Journal, 255 Cash
// Receipt Journal, 256 Payment Journal and 5628 Fixed Asset G/L Journal: OnOpenPage opens a template of the page's type and
// one of its batches (GenJnlManagement); a new line gets its values from the line above
// (SetUpNewLine); the footer shows the names of the line's accounts and the journal's balance
// up to the line and in total.
var genJournalPages = map[int]gtables.GenJournalTemplateType{
	39:   gtables.GenJournalTemplate_Type.General,
	253:  gtables.GenJournalTemplate_Type.Sales,
	254:  gtables.GenJournalTemplate_Type.Purchases,
	255:  gtables.GenJournalTemplate_Type.CashReceipts,
	256:  gtables.GenJournalTemplate_Type.Payments,
	5628: gtables.GenJournalTemplate_Type.Assets,
}

func init() {
	for pageID, templateType := range genJournalPages {
		pages.OnOpenPage(pageID, func(rec *gtables.GenJournalLine, page *pages.Open) error {
			return genjnl.OpenJournalPage(rec, page, templateType)
		})
		pages.OnNewRecord(pageID, func(rec, xRec *gtables.GenJournalLine, belowXRec bool) {
			balance, _ := genjnl.CalcBalance(rec, xRec)
			_ = tables.SetUpNewGenJnlLine(rec, xRec, balance, belowXRec)
		})
		pages.OnAfterGetRecord(pageID, func(rec *gtables.GenJournalLine, page *pages.Control) {
			accName, balAccName := genjnl.GetAccounts(rec)
			page.SetValue("account_name", accName)
			page.SetValue("bal_account_name", balAccName)
			balance, totalBalance := genjnl.CalcBalance(rec, rec)
			page.SetValue("balance", balance)
			page.SetValue("total_balance", totalBalance)
		})
	}

	// Page 251 General Journal Batches: a new batch takes the template's balancing account,
	// numbering and reason code (SetupNewBatch)
	pages.OnNewRecord(251, func(rec, xRec *gtables.GenJournalBatch, belowXRec bool) {
		_ = tables.SetupNewBatch(rec)
	})
}
