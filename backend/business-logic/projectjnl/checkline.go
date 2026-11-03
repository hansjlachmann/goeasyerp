package projectjnl

import (
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

const projectJnlLineTable = gtables.ProjectJournalLineTableName

// ProjectJnlCheckLine is codeunit 201 Project Jnl.-Check Line: it checks a project journal line
// before it is posted. It keeps the user's allowed posting period between lines.
type ProjectJnlCheckLine struct {
	allowPostingFrom types.Date
	allowPostingTo   types.Date
}

// RunCheck checks a journal line (OnRun)
func (c *ProjectJnlCheckLine) RunCheck(rec *gtables.ProjectJournalLine) error {
	if tables.ProjectJnlLineEmptyLine(rec) {
		return nil
	}
	err := rec.TestField("project_no")
	if err != nil {
		return err
	}
	err = rec.TestField("no")
	if err != nil {
		return err
	}
	err = rec.TestField("posting_date")
	if err != nil {
		return err
	}
	if rec.Type != gtables.ProjectJournalLine_Type.AccountGL {
		err = rec.TestField("gen_prod_posting_group")
		if err != nil {
			return err
		}
		err = rec.TestField("balancing_acc_no", types.Code(""))
		if err != nil {
			return err
		}
	}
	if rec.Posting_date.IsClosing() {
		return apperrors.JnlClosingDate(projectJnlLineTable, "posting_date")
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
		return apperrors.JnlDateNotAllowed(projectJnlLineTable, "posting_date")
	}

	if !rec.Document_date.IsZero() {
		if rec.Document_date.IsClosing() {
			return apperrors.JnlClosingDate(projectJnlLineTable, "document_date")
		}
	}
	return nil
}
