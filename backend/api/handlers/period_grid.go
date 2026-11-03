package handlers

import (
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"

	apitypes "github.com/hansjlachmann/openerp/backend/api/types"
	"github.com/hansjlachmann/openerp/backend/business-logic/periods"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/pages"
	"github.com/hansjlachmann/openerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// periodGridMaxRows is the most rows a period grid returns at once
const periodGridMaxRows = 100

// periodGridRow is a row of a period grid: its period (or record) and the columns' values
type periodGridRow struct {
	PeriodStart string                 `json:"period_start"`
	PeriodEnd   string                 `json:"period_end"`
	PeriodName  string                 `json:"period_name,omitempty"`
	Key         string                 `json:"key,omitempty"`     // rows from a table: the record's key
	Caption     string                 `json:"caption,omitempty"` // and its name
	Values      map[string]interface{} `json:"values"`
}

// PeriodGrid calculates a card's period grid section for the card's record (classic matrix-by-
// periods subforms, codeunit 359): body {"section", "record", "filters": [{field, expression}],
// "view_by" (0 Day … 5 Accounting Period), "view_as" (0 Net Change, 1 Balance at Date), "date"
// (the first period contains it; blank: the work date), "step" (periods to move first),
// "rows"}. It returns {"rows": [{period_start, period_end, period_name, key, caption, values}],
// "date": the first period's start}.
// POST /api/pages/:id/period-grid
func (h *PageTriggersHandler) PeriodGrid(c *fiber.Ctx) error {
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
		Section string                 `json:"section"`
		Record  map[string]interface{} `json:"record"`
		Filters []ftables.FieldFilter  `json:"filters"`
		ViewBy  int                    `json:"view_by"`
		ViewAs  int                    `json:"view_as"`
		Date    string                 `json:"date"`
		Step    int                    `json:"step"`
		Rows    int                    `json:"rows"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	var grid *pages.PeriodGrid
	for _, s := range page.Page.Layout.Sections {
		if s.Name == req.Section && s.PeriodGrid != nil {
			grid = s.PeriodGrid
		}
	}
	if grid == nil {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.InvalidFields().Message(language)))
	}
	if !sess.IsSuper && !sess.HasPermission(page.Page.SourceTable, session.PermRead) {
		return c.Status(fiber.StatusForbidden).JSON(apitypes.NewErrorResponse(apperrors.PermissionDenied(string(session.PermRead), page.Page.SourceTable).Message(language)))
	}
	factory, ok := gtables.GetTableFactory(page.Page.SourceTable)
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(page.Page.SourceTable).Message(language)))
	}
	src := ftables.DBSource(h.db, sess.GetCompany(), h.dbType)
	rec := factory()
	rec.InitWithDBType(h.db, sess.GetCompany(), h.dbType)
	rec.FromMap(req.Record)
	// the user's filters: only the grid's filter fields
	allowed := map[string]bool{}
	for _, f := range grid.Filters {
		allowed[f.Source] = true
	}
	flowFilterValues := map[string]string{} // the FlowFilters set, for the rows' filters
	for _, f := range req.Filters {
		if !allowed[f.Field] {
			continue
		}
		flowFilterValues[f.Field] = f.Expression
		if err := rec.SetFlowFilter(f.Field, f.Expression); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
		}
	}

	fcodeunits.SetCurrentContext(sess.GetUserID(), sess.GetUserName(), sess.GetCompany(), language)
	defer fcodeunits.ClearCurrentContext()

	viewBy := periods.PeriodType(req.ViewBy)
	if viewBy < periods.Day || viewBy > periods.AccountingPeriod {
		viewBy = periods.Day
	}
	date, _ := types.NewDateFromString(req.Date)
	period, found := periods.FindDate(src, "", date, viewBy)
	if !found {
		return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"rows": []periodGridRow{}, "date": ""}))
	}
	if req.Step != 0 {
		period, _ = periods.NextDate(src, req.Step, period, viewBy)
	}
	rowCount := req.Rows
	if rowCount <= 0 || rowCount > periodGridMaxRows {
		rowCount = periodGridMaxRows
	}

	var columns []string
	for _, col := range grid.Columns {
		if !col.Calculated {
			columns = append(columns, col.Source)
		}
	}
	// calc fills the columns for the record as its FlowFilters are set
	calc := func() (map[string]interface{}, error) {
		rec.CalcFields(columns...)
		values := map[string]interface{}{}
		data := ftables.PublicMap(rec)
		for _, col := range columns {
			values[col] = data[col]
		}
		control, err := pages.RunAfterGetRecordControl(pageID, rec)
		if err != nil {
			return nil, err
		}
		for field, value := range control.Values() {
			values[field] = value
		}
		return values, nil
	}
	dateFilter := func(p periods.Period) string {
		if req.ViewAs == 1 { // Balance at Date
			return ".." + p.End.String()
		}
		return p.Start.String() + ".." + p.End.String()
	}

	rows := []periodGridRow{}
	if grid.Rows == nil {
		for i := 0; i < rowCount; i++ {
			if err := rec.SetFlowFilter(grid.DateFilter, dateFilter(period)); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
			}
			values, err := calc()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
			}
			rows = append(rows, periodGridRow{PeriodStart: period.Start.String(), PeriodEnd: period.End.String(), PeriodName: period.Name, Values: values})
			next, steps := periods.NextDate(src, 1, period, viewBy)
			if steps == 0 {
				break
			}
			period = next
		}
	} else {
		// rows from a table, all for the one period
		if err := rec.SetFlowFilter(grid.DateFilter, dateFilter(period)); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
		}
		rowFactory, ok := gtables.GetTableFactory(grid.Rows.Table)
		if !ok {
			return c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(grid.Rows.Table).Message(language)))
		}
		rowRec := rowFactory()
		rowRec.InitWithDBType(h.db, sess.GetCompany(), h.dbType)
		recValues := rec.ToMap()
		ranger, ok := rowRec.(interface {
			SetRange(fieldName string, values ...interface{})
		})
		if ok {
			for rowField, recField := range grid.Rows.Filters {
				// a FlowFilter of the grid (the bins of the Location Filter): its expression, none when blank
				if allowed[recField] {
					if expr := flowFilterValues[recField]; expr != "" {
						if err := ftables.ApplyFilter(rowRec, ftables.FieldFilter{Field: rowField, Expression: expr}); err != nil {
							return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
						}
					}
					continue
				}
				ranger.SetRange(rowField, recValues[recField])
			}
		}
		rowRec.SetCurrentKey(grid.Rows.Field)
		// the rows' records first: each row then reads the record's FlowFields
		var rowRecords []map[string]interface{}
		for found := rowRec.FindSet(); found && len(rowRecords) < rowCount; found = rowRec.Next() {
			rowRecords = append(rowRecords, rowRec.ToMap())
		}
		for _, row := range rowRecords {
			key := fmt.Sprint(row[grid.Rows.Field])
			if err := rec.SetFlowFilter(grid.Rows.Filter, key); err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
			}
			values, err := calc()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
			}
			rows = append(rows, periodGridRow{PeriodStart: period.Start.String(), PeriodEnd: period.End.String(), PeriodName: period.Name,
				Key: key, Caption: fmt.Sprint(row[grid.Rows.CaptionField]), Values: values})
		}
	}
	first := ""
	if len(rows) > 0 {
		first = rows[0].PeriodStart
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"rows": rows, "date": first}))
}
