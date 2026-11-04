package bomjnl

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

const bomJnlLineTable = gtables.BOMJournalLineTableName

// BOMJnlCheckLine is codeunit 31 BOM Jnl.-Check Line: it checks a BOM journal line
// before it is posted. It keeps the user's allowed posting period between lines.
type BOMJnlCheckLine struct {
	allowPostingFrom types.Date
	allowPostingTo   types.Date
}

// RunCheck checks a journal line (OnRun)
func (c *BOMJnlCheckLine) RunCheck(rec *gtables.BOMJournalLine) error {
	if tables.BOMJnlLineEmptyLine(rec) {
		return nil
	}
	err := rec.TestField("item_no")
	if err != nil {
		return err
	}
	err = rec.TestField("posting_date")
	if err != nil {
		return err
	}
	if rec.Posting_date.IsClosing() {
		return apperrors.JnlClosingDate(bomJnlLineTable, "posting_date")
	}

	if c.allowPostingFrom.IsZero() && c.allowPostingTo.IsZero() {
		if system.UserId() != "" {
			userSetup := gtables.NewUserSetup(rec)
			if userSetup.Get(types.Code(system.UserId())) {
				c.allowPostingFrom = userSetup.Allow_posting_from
				c.allowPostingTo = userSetup.Allow_posting_to
			}
		}
		if c.allowPostingFrom.IsZero() && c.allowPostingTo.IsZero() {
			glSetup := gtables.GetGeneralLedgerSetup(rec)
			c.allowPostingFrom = glSetup.Allow_posting_from
			c.allowPostingTo = glSetup.Allow_posting_to
		}
		if c.allowPostingTo.IsZero() {
			c.allowPostingTo, _ = system.DMY2Date(31, 12, 9999)
		}
	}
	if rec.Posting_date.Before(c.allowPostingFrom) || c.allowPostingTo.Before(rec.Posting_date) {
		return apperrors.JnlDateNotAllowed(bomJnlLineTable, "posting_date")
	}

	if !rec.Document_date.IsZero() {
		if rec.Document_date.IsClosing() {
			return apperrors.JnlClosingDate(bomJnlLineTable, "document_date")
		}
	}
	return rec.TestField("quantity")
}
