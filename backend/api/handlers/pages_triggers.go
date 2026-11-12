package handlers

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// PageTriggersHandler runs page triggers (business-logic/pages) for the records pages show
type PageTriggersHandler struct {
	db     *sql.DB
	dbType database.DBType
}

// NewPageTriggersHandler creates the page triggers handler
func NewPageTriggersHandler(db *sql.DB, dbType database.DBType) *PageTriggersHandler {
	return &PageTriggersHandler{db: db, dbType: dbType}
}

// FieldStates runs the page's OnAfterGetRecord triggers for the record a card shows (body:
// the record's fields) and returns the fields they switched: {"field_states": {field:
// {"editable": false, "visible": true}}}
// POST /api/pages/:id/field-states
func (h *PageTriggersHandler) FieldStates(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	language := sess.GetLanguage()
	pageID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidPageID().Message(language)))
	}
	page, err := pages.GetRegistry().GetPage(pageID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.PageNotFound(c.Params("id")).Message(language)))
	}
	var record map[string]interface{}
	if err := c.BodyParser(&record); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	factory, ok := gtables.GetTableFactory(page.Page.SourceTable)
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(page.Page.SourceTable).Message(language)))
	}
	rec := factory()
	rec.InitWithDBType(h.db, sess.GetCompany(), h.dbType)
	rec.FromMap(record)
	// the triggers' texts in the user's language (fcodeunits.CurrentLanguage)
	fcodeunits.SetCurrentContext(sess.GetUserID(), sess.GetUserName(), sess.GetCompany(), language)
	defer fcodeunits.ClearCurrentContext()
	control, err := pages.RunAfterGetRecordControl(pageID, rec)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(apitypes.NewErrorResponse(err.Error()))
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"field_states": control.States(), "values": control.Values()}))
}

// calculatedValues fills the calculated fields of a list's rows: the page's OnAfterGetRecord
// triggers run for each row (records are the rows' maps) and their SetValue values join the row;
// the fields they switched for the row (SetEditable/SetVisible) go with it as RowFieldStatesKey
// — always, also empty, so a row saved again drops the states it no longer has
func calculatedValues(pageID int, table string, records []map[string]interface{}, newRecord func() ftables.Table) error {
	page, err := pages.GetRegistry().GetPage(pageID)
	if err != nil || !pages.HasRecordTriggers(pageID) || !strings.EqualFold(strings.ReplaceAll(page.Page.SourceTable, " ", "_"), strings.ReplaceAll(table, " ", "_")) {
		return nil // not this table's page, or nothing to calculate
	}
	for _, row := range records {
		rec := newRecord()
		rec.FromMap(row)
		control, err := pages.RunAfterGetRecordControl(pageID, rec)
		if err != nil {
			return err
		}
		for field, value := range control.Values() {
			row[field] = value
		}
		row[RowFieldStatesKey] = control.States()
	}
	return nil
}

// RowFieldStatesKey is the key of a list row's field states (the page's OnAfterGetRecord
// SetEditable/SetVisible for the row); the leading underscore keeps the frontend from sending it
// back with the row
const RowFieldStatesKey = "_field_states"

// OpenPage runs the page's OnOpenPage triggers with the page's parameters (body: {"params":
// {"template": "GENERAL", "batch": "DEFAULT"}}) and returns the parameters as the triggers
// left them, the filters the page's list applies (filter group 2) and, when a trigger asks the
// user to pick a record first, the page to pick from: {"params", "fixed_filters", "select"}
// POST /api/pages/:id/open
func (h *PageTriggersHandler) OpenPage(c *fiber.Ctx) error {
	sess := getSession(c)
	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	language := sess.GetLanguage()
	pageID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidPageID().Message(language)))
	}
	page, err := pages.GetRegistry().GetPage(pageID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.PageNotFound(c.Params("id")).Message(language)))
	}
	var req struct {
		Params map[string]string `json:"params"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	factory, ok := gtables.GetTableFactory(page.Page.SourceTable)
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(page.Page.SourceTable).Message(language)))
	}
	if !sess.HasPermission(page.Page.SourceTable, session.PermRead) {
		return c.Status(fiber.StatusForbidden).JSON(apitypes.NewErrorResponse(apperrors.PermissionDenied(string(session.PermRead), page.Page.SourceTable).Message(language)))
	}
	rec := factory()
	rec.InitWithDBType(h.db, sess.GetCompany(), h.dbType)
	fcodeunits.SetCurrentContext(sess.GetUserID(), sess.GetUserName(), sess.GetCompany(), language)
	fcodeunits.SetCurrentAccess(sess)
	defer fcodeunits.ClearCurrentContext()
	open, err := pages.RunOpenPage(pageID, rec, req.Params)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
	}
	filters := open.Filters()
	if filters == nil {
		filters = []pages.FixedFilter{}
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{
		"params":        open.Params(),
		"fixed_filters": filters,
		"select":        open.Selection(),
	}))
}

// dropRowReadOnlyFields leaves out of a row's changes the fields the page's OnAfterGetRecord
// triggers made read-only (or hid) for the record as it is stored — as the page shows them;
// rec is that stored record. Nothing when the page is not the table's or has no such trigger.
func dropRowReadOnlyFields(pageID int, table string, rec ftables.Table, data map[string]interface{}) error {
	if pageID <= 0 || !pages.HasRecordTriggers(pageID) {
		return nil
	}
	page, err := pages.GetRegistry().GetPage(pageID)
	if err != nil || !strings.EqualFold(strings.ReplaceAll(page.Page.SourceTable, " ", "_"), strings.ReplaceAll(table, " ", "_")) {
		return nil
	}
	control, err := pages.RunAfterGetRecordControl(pageID, rec)
	if err != nil {
		return err
	}
	for field, state := range control.States() {
		if (state.Editable != nil && !*state.Editable) || (state.Visible != nil && !*state.Visible) {
			delete(data, field)
		}
	}
	return nil
}
