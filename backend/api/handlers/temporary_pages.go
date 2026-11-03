package handlers

import (
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/hansjlachmann/openerp/backend/business-logic/postingpreview"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"

	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
)

// temporaryRecord is a generated record that can be made temporary (InitTemporary)
type temporaryRecord interface {
	InitTemporary(db database.Executor, company string, dbType database.DBType) error
}

// fillTemporaryPage: for a list page on a temporary source table (page_id of a page with
// source_table_temporary), the table becomes temporary and the page's OnFindRecords trigger
// fills it from the page's parameters (page_params) — the list's filters, search, sort and
// windows then read the temporary rows. Other requests leave the table as it is.
func (h *TablesHandler) fillTemporaryPage(c *fiber.Ctx, table ftables.Table, tableName string, sess *session.Session) error {
	pageID := c.QueryInt("page_id", 0)
	if pageID <= 0 {
		return nil
	}
	page, err := pages.GetRegistry().GetPage(pageID)
	if err != nil || !page.Page.SourceTableTemporary || !strings.EqualFold(page.Page.SourceTable, tableName) {
		return nil
	}
	temp, ok := table.(temporaryRecord)
	if !ok {
		return nil
	}
	if err := temp.InitTemporary(h.db, sess.GetCompany(), h.dbType); err != nil {
		return err
	}
	var params map[string]string
	if raw := c.Query("page_params", ""); raw != "" {
		_ = json.Unmarshal([]byte(raw), &params)
	}
	// The trigger runs as the user (middleware.UserContext): it sees what the user may read
	return pages.RunFindRecords(pageID, table, params)
}

// fillPreview: a list opened from a posting preview (the page parameter preview, Posting
// Preview 115 → an entries page) shows the entries the previewed posting wrote — the table
// becomes temporary with the preview's rows, the list's filters, search, sort and windows then
// read them. A preview belongs to the user and company that made it and expires (Lifetime).
func (h *TablesHandler) fillPreview(c *fiber.Ctx, table ftables.Table, tableName string, sess *session.Session) error {
	raw := c.Query("page_params", "")
	if raw == "" {
		return nil
	}
	var params map[string]string
	_ = json.Unmarshal([]byte(raw), &params)
	id := params["preview"]
	if id == "" || !postingpreview.IsEntryTable(tableName) {
		return nil // Posting Preview's own lines come from its page trigger
	}
	preview, ok := postingpreview.Get(id, sess.GetUserID(), sess.GetCompany())
	if !ok {
		return apperrors.PreviewExpired()
	}
	temp, ok := table.(temporaryRecord)
	if !ok {
		return nil
	}
	if err := temp.InitTemporary(h.db, sess.GetCompany(), h.dbType); err != nil {
		return err
	}
	for _, row := range preview.Rows(tableName) {
		table.FromMap(row)
		if err := table.InsertOrError(false); err != nil {
			return err
		}
	}
	return nil
}

// fillTemporaryFunctionPage: a function run from a page on a temporary source table works on one
// of the page's rows — the table becomes temporary and the page's OnFindRecords fills it from
// the page parameters again (as the list did), so the row is read by its key from there
func fillTemporaryFunctionPage(db database.Executor, company string, dbType database.DBType, pageID int, tableName string, params map[string]string, table ftables.Table) error {
	if pageID <= 0 {
		return nil
	}
	page, err := pages.GetRegistry().GetPage(pageID)
	if err != nil || !page.Page.SourceTableTemporary || !strings.EqualFold(page.Page.SourceTable, tableName) {
		return nil
	}
	temp, ok := table.(temporaryRecord)
	if !ok {
		return nil
	}
	err = temp.InitTemporary(db, company, dbType)
	if err != nil {
		return err
	}
	return pages.RunFindRecords(pageID, table, params)
}
