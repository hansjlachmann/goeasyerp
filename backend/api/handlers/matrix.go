package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/periods"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// matrixMaxRows / matrixMaxColumns: the most rows and periods a matrix returns at once
const (
	matrixMaxRows    = 1000
	matrixMaxColumns = 24
)

// matrixColumn is a period of a matrix
type matrixColumn struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Name  string `json:"name,omitempty"`
}

// matrixRow is a record of a matrix: its key, the row fields, the value its drilldown filters
// on and the cells (one per column, as text like the record's decimals)
type matrixRow struct {
	Key       string                 `json:"key"`
	Fields    map[string]interface{} `json:"fields"`
	Drilldown string                 `json:"drilldown,omitempty"`
	Values    []string               `json:"values"`
}

// matrixRequest is what both matrix endpoints take: the card section, the card's record, the
// user's filters and the view
type matrixRequest struct {
	Section string                 `json:"section"`
	Record  map[string]interface{} `json:"record"`
	Filters []ftables.FieldFilter  `json:"filters"`
	ViewBy  int                    `json:"view_by"`
	ViewAs  int                    `json:"view_as"` // 0 Net Change, 1 Balance at Date
	// Matrix: the first column contains Date (blank: the work date), after moving Step periods;
	// Columns periods
	Date    string `json:"date"`
	Step    int    `json:"step"`
	Columns int    `json:"columns"`
	// Validate: the cell of row Key in the period Start..End takes Value (a number as typed,
	// multiplied by the rounding factor's 1000 / 1000000)
	Key            string `json:"key"`
	Start          string `json:"start"`
	End            string `json:"end"`
	Value          string `json:"value"`
	RoundingFactor int    `json:"rounding_factor"` // 0 None, 1 "1", 2 "1000", 3 "1000000"
	// ColumnFilters: a calculated matrix's column — its own filters (MatrixColumn.DrilldownFilters)
	ColumnFilters map[string]string `json:"column_filters"`
}

// matrixContext is a matrix request checked: the section, the rows' table and a record of it
// with the card record's and the user's FlowFilters set
type matrixContext struct {
	pageID   int
	page     *pages.PageDefinition
	matrix   *pages.Matrix
	req      matrixRequest
	language string
	sess     *session.Session
	src      ftables.Source
}

// openMatrix reads and checks a matrix request (the section exists, the user may read the card's
// and the rows' tables)
func (h *PageTriggersHandler) openMatrix(c *fiber.Ctx) (*matrixContext, error) {
	sess := getSession(c)
	if sess == nil {
		return nil, c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}
	language := sess.GetLanguage()
	pageID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return nil, c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidPageID().Message(language)))
	}
	page, err := pages.GetRegistry().GetPage(pageID)
	if err != nil {
		return nil, c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.PageNotFound(c.Params("id")).Message(language)))
	}
	var req matrixRequest
	if err := c.BodyParser(&req); err != nil {
		return nil, c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}
	var matrix *pages.Matrix
	for _, s := range page.Page.Layout.Sections {
		if s.Name == req.Section && s.Matrix != nil {
			matrix = s.Matrix
		}
	}
	if req.Section == "" {
		matrix = page.Page.Layout.Matrix // a matrix page
	}
	if matrix == nil {
		return nil, c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.InvalidFields().Message(language)))
	}
	for _, table := range []string{page.Page.SourceTable, matrix.Table} {
		if !sess.IsSuper && !sess.HasPermission(table, session.PermRead) {
			return nil, c.Status(fiber.StatusForbidden).JSON(apitypes.NewErrorResponse(apperrors.PermissionDenied(string(session.PermRead), table).Message(language)))
		}
	}
	if _, ok := gtables.GetTableFactory(matrix.Table); !ok {
		return nil, c.Status(fiber.StatusNotFound).JSON(apitypes.NewErrorResponse(apperrors.TableNotFound(matrix.Table).Message(language)))
	}
	return &matrixContext{pageID: pageID, page: page, matrix: matrix, req: req, language: language, sess: sess,
		src: ftables.DBSource(h.db, sess.GetCompany(), h.dbType)}, nil
}

// rowRecord: a record of the rows' table in db with the card record's and the user's FlowFilters
func (m *matrixContext) rowRecord(db ftables.Source) (ftables.Table, error) {
	factory, _ := gtables.GetTableFactory(m.matrix.Table)
	rec := factory()
	rec.InitWithDBType(db.GetDB(), db.GetCompany(), db.GetDBType())
	for rowField, recordField := range m.matrix.RowFilters {
		rec.SetFilter(rowField, fmt.Sprint(m.req.Record[recordField]))
	}
	for rowField, recordField := range m.matrix.RecordFilters {
		value := fmt.Sprint(m.req.Record[recordField])
		if m.req.Record[recordField] == nil || value == "" {
			continue
		}
		if err := rec.SetFlowFilter(rowField, value); err != nil {
			return nil, err
		}
	}
	allowed := map[string]bool{}
	for _, f := range m.matrix.Filters {
		allowed[f.Source] = true
	}
	for _, f := range m.req.Filters {
		if !allowed[f.Field] || f.Expression == "" {
			continue
		}
		if err := rec.SetFlowFilter(f.Field, f.Expression); err != nil {
			return nil, err
		}
	}
	return rec, nil
}

// calculatedDateFilter: a calculated matrix's period as its date filter — Net Change the period,
// Balance at Date (a matrix with view_as) up to its end
func (m *matrixContext) calculatedDateFilter(period periods.Period) string {
	if m.matrix.ViewAs {
		return m.dateFilter(period.Start.String(), period.End.String())
	}
	return period.Start.String() + ".." + period.End.String()
}

// dateFilter: a column's period as the date filter of the view (Balance at Date: up to its end)
func (m *matrixContext) dateFilter(start, end string) string {
	if m.req.ViewAs == 1 {
		return ".." + end
	}
	if start == end {
		return start
	}
	return start + ".." + end
}

// Matrix calculates a card's matrix section (classic matrix forms, G/L Budget by Periods): body
// {"section", "record", "filters": [{field, expression}], "view_by" (0 Day … 5 Accounting
// Period), "view_as" (0 Net Change, 1 Balance at Date), "date" (the first column contains it;
// blank: the work date, by month/quarter/accounting period the start of its year), "step" (periods to move first), "columns"}. It returns {"columns":
// [{start, end, name}], "rows": [{key, fields, drilldown, values}], "date": the first column's
// start}.
// POST /api/pages/:id/matrix
func (h *PageTriggersHandler) Matrix(c *fiber.Ctx) error {
	m, err := h.openMatrix(c)
	if m == nil {
		return err
	}
	fcodeunits.SetCurrentContext(m.sess.GetUserID(), m.sess.GetUserName(), m.sess.GetCompany(), m.language)
	defer fcodeunits.ClearCurrentContext()
	if m.matrix.Calculated {
		return m.calculated(c)
	}

	// the columns: the periods from the date
	viewBy := periods.PeriodType(m.req.ViewBy)
	if viewBy < periods.Day || viewBy > periods.AccountingPeriod {
		viewBy = periods.Month
	}
	columnCount := m.req.Columns
	if columnCount <= 0 || columnCount > matrixMaxColumns {
		columnCount = 12
	}
	// without a date: from the work date — months, quarters and accounting periods from the
	// start of its year (a budget's year)
	date, _ := types.NewDateFromString(m.req.Date)
	if date.IsZero() {
		workDate := system.WorkDate()
		date = workDate
		switch viewBy {
		case periods.Month, periods.Quarter, periods.AccountingPeriod:
			date = types.NewDate(workDate.Year(), time.January, 1)
		}
	}
	columns := []matrixColumn{}
	period, found := periods.FindDate(m.src, "", date, viewBy)
	if found && m.req.Step != 0 {
		period, _ = periods.NextDate(m.src, m.req.Step, period, viewBy)
	}
	for found && len(columns) < columnCount {
		columns = append(columns, matrixColumn{Start: period.Start.String(), End: period.End.String(), Name: period.Name})
		next, steps := periods.NextDate(m.src, 1, period, viewBy)
		if steps == 0 {
			break
		}
		period = next
	}

	// the rows: the records of the table in its key order
	rec, err := m.rowRecord(m.src)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, m.language)))
	}
	var records []map[string]interface{}
	var keys []string
	for found := rec.FindSet(); found && len(records) < matrixMaxRows; found = rec.Next() {
		records = append(records, rec.ToMap())
		keys = append(keys, ftables.KeyString(rec)) // all key fields: rows of a composite key differ
	}
	// option fields are shown by their caption in the user's language
	options := optionCaptions(m.matrix.Table, rec, m.language)
	rows := make([]matrixRow, len(records))
	for i, data := range records {
		fields := map[string]interface{}{}
		public := map[string]interface{}{}
		for k, v := range data {
			public[k] = v
		}
		for _, f := range m.matrix.Fields {
			fields[f.Source] = public[f.Source]
			captions, isOption := options[f.Source]
			if isOption {
				fields[f.Source] = captions[fmt.Sprint(public[f.Source])]
			}
			if f.IndentField != "" {
				fields[f.IndentField] = public[f.IndentField]
			}
			if f.StyleExpr != nil {
				fields[f.StyleExpr.Field] = public[f.StyleExpr.Field]
			}
		}
		drilldown := ""
		if m.matrix.DrilldownRowValue != "" {
			drilldown = fmt.Sprint(public[m.matrix.DrilldownRowValue])
			if public[m.matrix.DrilldownRowValue] == nil {
				drilldown = ""
			}
		}
		if drilldown == "" {
			drilldown = keys[i]
		}
		rows[i] = matrixRow{Key: keys[i], Fields: fields, Drilldown: drilldown, Values: make([]string, len(columns))}
	}
	// the cells: per column one calculation over all rows
	for j, column := range columns {
		if err := rec.SetFlowFilter(m.matrix.DateFilter, m.dateFilter(column.Start, column.End)); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, m.language)))
		}
		values := make([]map[string]interface{}, len(records))
		for i, data := range records {
			values[i] = map[string]interface{}{}
			for k, v := range data {
				values[i][k] = v
			}
		}
		rec.CalcFieldsForRecords(values, m.matrix.Value)
		for i := range values {
			rows[i].Values[j] = fmt.Sprint(values[i][m.matrix.Value])
		}
	}
	first := ""
	if len(columns) > 0 {
		first = columns[0].Start
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"columns": columns, "rows": rows, "date": first}))
}

// MatrixValidate enters a value into a cell of a card's matrix section: the row's record with
// the cell's FlowFilters goes to the page's OnMatrixValidate trigger, in one transaction (body:
// the matrix request with "key", "start", "end", "value", "rounding_factor").
// POST /api/pages/:id/matrix/validate
func (h *PageTriggersHandler) MatrixValidate(c *fiber.Ctx) error {
	m, err := h.openMatrix(c)
	if m == nil {
		return err
	}
	if !m.matrix.Editable || !pages.HasMatrixValidate(m.pageID) {
		return c.Status(fiber.StatusForbidden).JSON(apitypes.NewErrorResponse(apperrors.FieldNotEditable(m.matrix.Table, m.matrix.Value).Message(m.language)))
	}
	if m.matrix.EntryTable != "" && !m.sess.IsSuper && !m.sess.HasPermission(m.matrix.EntryTable, session.PermInsert) {
		return c.Status(fiber.StatusForbidden).JSON(apitypes.NewErrorResponse(apperrors.PermissionDenied(string(session.PermInsert), m.matrix.EntryTable).Message(m.language)))
	}
	value := types.Decimal{}
	if m.req.Value != "" {
		value, err = types.NewDecimalFromString(m.req.Value)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(apperrors.InvalidValue(m.matrix.Value, m.req.Value).Message(m.language)))
		}
	}
	// classic EvaluateAmount: amounts shown in thousands or millions are typed so
	switch m.req.RoundingFactor {
	case 2:
		value = value.Mul(types.NewDecimalFromInt(1000))
	case 3:
		value = value.Mul(types.NewDecimalFromInt(1000000))
	}

	fcodeunits.SetCurrentContext(m.sess.GetUserID(), m.sess.GetUserName(), m.sess.GetCompany(), m.language)
	defer fcodeunits.ClearCurrentContext()

	tx, err := ftables.BeginTransaction(h.db)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(apitypes.NewErrorResponse(errorMessage(err, m.language)))
	}
	src := ftables.DBSource(tx, m.sess.GetCompany(), h.dbType)
	rec, err := m.rowRecord(src)
	if err == nil && m.matrix.DateFilter != "" {
		err = rec.SetFlowFilter(m.matrix.DateFilter, m.dateFilter(m.req.Start, m.req.End))
	}
	if err == nil {
		err = rec.GetOrError(parseRecordKey(m.req.Key, rec))
	}
	if err == nil {
		err = pages.RunMatrixValidate(m.pageID, rec, m.req.ColumnFilters, value)
	}
	if err != nil {
		_ = ftables.Rollback(tx)
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, m.language)))
	}
	if err := ftables.Commit(tx); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, m.language)))
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{}))
}

// calculated answers a calculated matrix (an account schedule's overview): the period of the
// view containing the date (blank: the work date) moved by Step goes into the rows' date filter,
// the rows are the records in the row filters, and the page's OnMatrixCalculate trigger gives
// the columns and the cells. It returns {"columns": [{name, decimal_places, percent}], "rows",
// "period": {start, end, name}, "date"}.
func (m *matrixContext) calculated(c *fiber.Ctx) error {
	viewBy := periods.PeriodType(m.req.ViewBy)
	if viewBy < periods.Day || viewBy > periods.AccountingPeriod {
		viewBy = periods.Month
	}
	date, _ := types.NewDateFromString(m.req.Date)
	if date.IsZero() {
		date = system.WorkDate()
	}
	period, found := periods.FindDate(m.src, "", date, viewBy)
	if found && m.req.Step != 0 {
		period, _ = periods.NextDate(m.src, m.req.Step, period, viewBy)
	}
	empty := map[string]interface{}{"columns": []pages.MatrixColumn{}, "rows": []matrixRow{}, "date": ""}
	if !found {
		return c.JSON(apitypes.NewSuccessResponse(empty))
	}
	rec, err := m.rowRecord(m.src)
	if err == nil {
		// Net Change: the period; Balance at Date (a matrix with view_as): up to its end
		if m.matrix.DateFilter != "" {
			err = rec.SetFlowFilter(m.matrix.DateFilter, m.calculatedDateFilter(period))
		}
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, m.language)))
	}
	factory, _ := gtables.GetTableFactory(m.page.Page.SourceTable)
	card := factory()
	card.InitWithDBType(m.src.GetDB(), m.src.GetCompany(), m.src.GetDBType())
	card.FromMap(m.req.Record)
	if m.matrix.CardDateFilter != "" {
		err = card.SetFlowFilter(m.matrix.CardDateFilter, m.calculatedDateFilter(period))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, m.language)))
		}
	}

	rows := []matrixRow{}
	for found := rec.FindSet(); found && len(rows) < matrixMaxRows; found = rec.Next() {
		data := rec.ToMap()
		fields := map[string]interface{}{}
		for _, f := range m.matrix.Fields {
			fields[f.Source] = data[f.Source]
			if f.IndentField != "" {
				fields[f.IndentField] = data[f.IndentField]
			}
			if f.StyleExpr != nil {
				fields[f.StyleExpr.Field] = data[f.StyleExpr.Field]
			}
		}
		drilldown := ""
		if m.matrix.DrilldownRowValue != "" && data[m.matrix.DrilldownRowValue] != nil {
			drilldown = fmt.Sprint(data[m.matrix.DrilldownRowValue])
		}
		if drilldown == "" {
			drilldown = ftables.KeyString(rec)
		}
		rows = append(rows, matrixRow{Key: ftables.KeyString(rec), Fields: fields, Drilldown: drilldown})
	}
	columns, values, err := pages.RunMatrixCalculate(m.pageID, card, rec)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apitypes.NewErrorResponse(errorMessage(err, m.language)))
	}
	for i := range rows {
		rows[i].Values = values[rows[i].Key]
		if rows[i].Values == nil {
			rows[i].Values = make([]string, len(columns))
		}
	}
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{
		"columns": columns, "rows": rows, "date": period.Start.String(),
		"period": matrixColumn{Start: period.Start.String(), End: period.End.String(), Name: period.Name},
	}))
}
