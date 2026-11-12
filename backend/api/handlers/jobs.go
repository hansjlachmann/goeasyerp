package handlers

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	apitypes "github.com/hansjlachmann/goeasyerp/backend/api/types"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// JobsHandler handles job-related API requests (codeunits with progress)
type JobsHandler struct {
	db     *sql.DB
	dbType database.DBType
}

// NewJobsHandler creates a new jobs handler
func NewJobsHandler(db *sql.DB) *JobsHandler {
	return NewJobsHandlerWithDBType(db, database.DBTypeSQLite)
}

// NewJobsHandlerWithDBType creates a new jobs handler with explicit database type
func NewJobsHandlerWithDBType(db *sql.DB, dbType database.DBType) *JobsHandler {
	return &JobsHandler{db: db, dbType: dbType}
}

// StartJob starts a codeunit as a background job with progress tracking
// POST /api/jobs/start
func (h *JobsHandler) StartJob(c *fiber.Ctx) error {
	sess := getSession(c)

	if sess == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apitypes.NewErrorResponse(apperrors.NoActiveSession().Message("en-US")))
	}

	// Capture session context for use in codeunit
	company := sess.GetCompany()
	language := sess.GetLanguage()
	userID := sess.GetUserID()
	userName := sess.GetUserName()

	// Parse request body
	var req struct {
		CodeunitID int `json:"codeunit_id"`
		ReportID   int `json:"report_id"` // a processing-only report instead of a codeunit
		// Function: a registered page function (run_object "table:Job_Queue_Entry.ShowLineNo")
		// called with the page's current record, read again from the database by its key
		Function string                 `json:"function"`
		Record   map[string]interface{} `json:"record"`
		// The records selected on the list (rec.SetSelectionFilter in the codeunit)
		Selection *listSelection `json:"selection"`
		// The list's filters: the record carries them (fixed filters in filter group 2, the
		// user's in group 0), so the codeunit works on what the list shows (a journal's batch)
		FixedFilters []fixedFilterParam `json:"fixed_filters"`
		Filters      []filterParam      `json:"filters"`
		// The page's parameters (a page opened from code with parameters): its functions read
		// them with system.PageParam
		PageParams map[string]string `json:"page_params"`
		// PageID: the page the function runs from — a page on a temporary source table
		// (Reservation 498) fills its rows again, the function's record is one of them
		PageID int `json:"page_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse(apperrors.InvalidRequestBody().Message(language)))
	}

	if req.CodeunitID == 0 && req.ReportID == 0 && req.Function == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse("Codeunit ID is required"))
	}

	// Get codeunit factory from registry (a report runs like a codeunit with a dialog)
	factory, ok := codeunits.Get(req.CodeunitID)
	if req.ReportID != 0 {
		factory, ok = reports.CodeunitFactory(req.ReportID)
	}
	if req.Function != "" {
		fn, args, err := functions.Resolve(req.Function)
		if err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(err.Error()))
		}
		ok = true
		factory = func(database.Executor, string, database.DBType) fcodeunits.Codeunit { return fn.Codeunit(args) }
	}
	if !ok {
		return c.Status(404).JSON(apitypes.NewErrorResponse("Codeunit not found"))
	}
	// Permission to run it: Execute on the codeunit or report, Modify on a function's table
	if denied := runPermissionError(sess, req.CodeunitID, req.ReportID, req.Function, language); denied != nil {
		return c.Status(fiber.StatusForbidden).JSON(apitypes.NewErrorResponse(denied.Message(language)))
	}

	// Create codeunit instance
	codeunit := factory(h.db, company, h.dbType)

	// The source table's record from the request (typed); an object without a source table
	// (a report without data items) runs without a record
	tableName := codeunit.SourceTable()
	var table ftables.Table
	if tableName != "" {
		tableFactory, ok := gtables.GetTableFactory(tableName)
		if !ok {
			return c.Status(500).JSON(apitypes.NewErrorResponse("Source table not found: " + tableName))
		}

		// Create table instance and populate from request
		table = tableFactory()
		table.InitWithDBType(h.db, company, h.dbType)
		table.FromMap(req.Record)
		// A function works on the stored record (not on what the browser sent) — an AssistEdit
		// function on the record as the page has it (also a new one), with the stored values as
		// its old values
		if req.Function != "" {
			fn, _ := functions.Lookup(req.Function)
			err := fillTemporaryFunctionPage(h.db, company, h.dbType, req.PageID, tableName, req.PageParams, table)
			if err != nil {
				return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
			}
			table.FromMap(req.Record) // the row the function runs on (the fill moved the record)
			key, id := fullPrimaryKey(table)
			switch {
			case fn != nil && fn.AssistEdit:
				stored := tableFactory()
				stored.InitWithDBType(h.db, company, h.dbType)
				if stored.Get(key) {
					stored.UpdateFromMap(req.Record)
					table = stored
				}
			case !table.Get(key):
				return c.Status(404).JSON(apitypes.NewErrorResponse(apperrors.RecordNotFound(tableName, id).Message(language)))
			}
		}
		newTable := func() ftables.Table {
			t := tableFactory()
			t.InitWithDBType(h.db, company, h.dbType)
			return t
		}
		if err := applySelection(c, table, req.Selection, newTable, language); err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
		}
		if err := applyFixedFilters(table, req.FixedFilters, language); err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
		}
		if err := applyFieldFilters(table, req.Filters, tableName, language); err != nil {
			return c.Status(400).JSON(apitypes.NewErrorResponse(errorMessage(err, language)))
		}
	}

	// Every run has a dialog, so whatever the code does — a question, StrMenu, a request page,
	// a page it opens with RunModal — reaches the user (also from a function or codeunit that
	// does not show progress). A run that does not use progress may still answer in the request
	// itself: it ends within syncWait without sending the browser anything. Otherwise (it asks
	// something, opens a page, shows progress or a note, or takes longer) the browser follows
	// the job by its ID, as for a run with progress.
	dialog := fcodeunits.OpenDialog("Processing...")
	jobID := dialog.JobID()
	done := make(chan jobOutcome, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- jobOutcome{err: fmt.Errorf("panic: %v", r)}
			}
		}()
		fcodeunits.SetCurrentContext(userID, userName, company, language)
		fcodeunits.SetCurrentAccess(sess)
		fcodeunits.SetCurrentPageParams(req.PageParams)
		defer fcodeunits.ClearCurrentContext()
		fcodeunits.SetCurrentDialog(dialog)
		defer fcodeunits.ClearCurrentDialog()

		result, err := codeunit.Run(table)
		done <- jobOutcome{result: result, err: err}
	}()

	if !codeunit.UsesProgress() {
		select {
		case outcome := <-done:
			if !hasActivity(dialog) {
				dialog.Discard()
				return syncJobResponse(c, outcome, language)
			}
			finishJob(dialog, outcome)
			return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{"job_id": jobID}))
		case <-dialog.Activity():
		case <-time.After(syncWait):
		}
	}
	go func() {
		finishJob(dialog, <-done)
	}()
	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{
		"job_id": jobID,
	}))
}

// syncWait: how long a run without progress may take to answer in the request itself
const syncWait = 2 * time.Second

// jobOutcome is how a job's run ended
type jobOutcome struct {
	result fcodeunits.Result
	err    error
}

// hasActivity: the run sent the browser an event before it ended
func hasActivity(dialog *fcodeunits.Dialog) bool {
	select {
	case <-dialog.Activity():
		return true
	default:
		return false
	}
}

// syncJobResponse answers a run that ended without showing anything in the request: the
// result, its message box and data, or the error
func syncJobResponse(c *fiber.Ctx, outcome jobOutcome, language string) error {
	if outcome.err != nil {
		return c.Status(500).JSON(apitypes.NewErrorResponse(errorMessage(outcome.err, language)))
	}
	result := outcome.result
	withOpenPage(&result)
	response := map[string]interface{}{
		"success": result.Success,
		"message": result.Message,
	}
	if result.Data != nil {
		response["data"] = result.Data
	}
	if result.Dialog != nil {
		response["dialog"] = result.Dialog
	}
	if result.Refresh {
		response["refresh"] = true
	}
	return c.JSON(apitypes.NewSuccessResponse(response))
}

// finishJob ends a job the browser follows: its result, message box and data, or the error
func finishJob(dialog *fcodeunits.Dialog, outcome jobOutcome) {
	if outcome.err != nil {
		dialog.CloseWithError(outcome.err)
		return
	}
	result := outcome.result
	withOpenPage(&result)
	// The page reloads its data afterwards: sent with the result data
	if result.Refresh {
		if result.Data == nil {
			result.Data = map[string]interface{}{}
		}
		result.Data["refresh"] = true
	}
	switch {
	case !result.Success:
		// Codeunit reported an error — send it as an error event
		errMsg := result.Message
		if result.Dialog != nil {
			errMsg = result.Dialog.Message
		}
		dialog.CloseWithError(fmt.Errorf("%s", errMsg))
	case result.Data != nil:
		// Close with data (e.g., PDF base64), and the closing message box with it
		message := result.Message
		if result.Dialog != nil {
			message = result.Dialog.Message
			result.Data["dialog"] = result.Dialog
		}
		dialog.CloseWithData(result.Data, message)
	case result.Dialog != nil:
		// The codeunit's closing message box (Message): the page shows it
		dialog.CloseWithData(map[string]interface{}{"dialog": result.Dialog}, result.Dialog.Message)
	case result.Message != "":
		// a plain result message (a run without progress shows it as it would in the request)
		dialog.CloseWithData(nil, result.Message)
	default:
		dialog.Close()
	}
}

// GetJobEvents streams job progress events via SSE
// GET /api/jobs/:id/events
func (h *JobsHandler) GetJobEvents(c *fiber.Ctx) error {
	jobID := c.Params("id")
	if jobID == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse("Job ID is required"))
	}

	// Get the job from registry
	dialog, ok := fcodeunits.GetJobRegistry().Get(jobID)
	if !ok {
		return c.Status(404).JSON(apitypes.NewErrorResponse("Job not found"))
	}

	// Set SSE headers
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	// Stream events
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		events := dialog.Events()
		ctx := dialog.Context()

		// Send initial connected event
		fmt.Fprintf(w, "event: connected\ndata: {\"job_id\":\"%s\"}\n\n", jobID)
		w.Flush()

		// Keep-alive ticker
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case event, ok := <-events:
				if !ok {
					// Channel closed
					fmt.Fprintf(w, "event: close\ndata: {}\n\n")
					w.Flush()
					return
				}

				// Send progress event - use JSON marshal to include Data field
				eventData := map[string]interface{}{
					"job_id":     event.JobID,
					"field":      event.Field,
					"value":      event.Value,
					"message":    event.Message,
					"completed":  event.Completed,
					"error":      event.Error,
					"timestamp":  event.Timestamp,
					"event_type": event.EventType,
					"confirm_id": event.ConfirmID,
				}
				if event.Data != nil {
					eventData["data"] = event.Data
				}
				if event.ErrorDetails != nil {
					eventData["error_details"] = event.ErrorDetails
				}
				jsonData, _ := json.Marshal(eventData)
				fmt.Fprintf(w, "event: progress\ndata: %s\n\n", string(jsonData))
				w.Flush()

				if event.Completed {
					return
				}

			case <-ticker.C:
				// Send keep-alive
				fmt.Fprintf(w, ": keepalive\n\n")
				w.Flush()

			case <-ctx.Done():
				// Job cancelled or completed
				fmt.Fprintf(w, "event: close\ndata: {}\n\n")
				w.Flush()
				return
			}
		}
	})

	return nil
}

// RespondToConfirm handles confirm dialog responses from the frontend
// POST /api/jobs/:id/confirm
func (h *JobsHandler) RespondToConfirm(c *fiber.Ctx) error {
	jobID := c.Params("id")
	if jobID == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse("Job ID is required"))
	}

	var req struct {
		ConfirmID string `json:"confirm_id"`
		Response  bool   `json:"response"`
		// Input values: text; numbers and booleans are taken as their text
		Values map[string]interface{} `json:"values,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(apitypes.NewErrorResponse("Invalid request body"))
	}

	if req.ConfirmID == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse("Confirm ID is required"))
	}

	// Get the job from registry
	dialog, ok := fcodeunits.GetJobRegistry().Get(jobID)
	if !ok {
		return c.Status(404).JSON(apitypes.NewErrorResponse("Job not found"))
	}

	// Route to input response or confirm response
	if req.Values != nil {
		values := make(map[string]string, len(req.Values))
		for name, v := range req.Values {
			switch x := v.(type) {
			case nil:
				values[name] = ""
			case string:
				values[name] = x
			case float64:
				values[name] = strconv.FormatFloat(x, 'f', -1, 64)
			default:
				values[name] = fmt.Sprint(x)
			}
		}
		if !dialog.RespondToInput(req.ConfirmID, values) {
			return c.Status(400).JSON(apitypes.NewErrorResponse("No pending input request with that ID"))
		}
	} else {
		if !dialog.RespondToConfirm(req.ConfirmID, req.Response) {
			return c.Status(400).JSON(apitypes.NewErrorResponse("No pending confirm with that ID"))
		}
	}

	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{
		"acknowledged": true,
	}))
}

// CancelJob cancels a running job
// POST /api/jobs/:id/cancel
func (h *JobsHandler) CancelJob(c *fiber.Ctx) error {
	jobID := c.Params("id")
	if jobID == "" {
		return c.Status(400).JSON(apitypes.NewErrorResponse("Job ID is required"))
	}

	// Get the job from registry
	dialog, ok := fcodeunits.GetJobRegistry().Get(jobID)
	if !ok {
		return c.Status(404).JSON(apitypes.NewErrorResponse("Job not found"))
	}

	// Cancel the job
	dialog.Cancel()

	return c.JSON(apitypes.NewSuccessResponse(map[string]interface{}{
		"cancelled": true,
	}))
}

// runPermissionError is the reason the session's user may not start an object (nil when it may):
// Execute permission on a codeunit or report (classic object permissions); a page function
// ("table:Customer.AssistEdit") needs Modify on its table, a codeunit function
// ("codeunit:900.PostLine") Execute on the codeunit
func runPermissionError(sess *session.Session, codeunitID, reportID int, function, language string) *apperrors.AppError {
	switch {
	case function != "":
		if fn, ok := functions.Lookup(function); ok && fn.Print {
			return nil // it checks Execute on the reports it prints
		}
		kind, rest, _ := strings.Cut(function, ":")
		object, _, _ := strings.Cut(rest, ".")
		switch kind {
		case "table":
			if !sess.HasPermission(object, session.PermModify) {
				return apperrors.PermissionDenied(string(session.PermModify), object)
			}
		case "codeunit":
			id, err := strconv.Atoi(object)
			if err == nil && !sess.CanExecute(session.ObjectCodeunit, id) {
				return apperrors.NoPermissionCodeunit(id, codeunits.List()[id])
			}
		}
	case reportID != 0:
		if !sess.CanExecute(session.ObjectReport, reportID) {
			return apperrors.NoPermissionReport(reportID, reports.Caption(reportID, language))
		}
	default:
		if !sess.CanExecute(session.ObjectCodeunit, codeunitID) {
			return apperrors.NoPermissionCodeunit(codeunitID, codeunits.List()[codeunitID])
		}
	}
	return nil
}

// withOpenPage puts the page a result opens afterwards into its data (open_page), and the
// records it marked (mark)
func withOpenPage(result *fcodeunits.Result) {
	if result.OpenPage == nil && len(result.Mark) == 0 {
		return
	}
	if result.Data == nil {
		result.Data = map[string]interface{}{}
	}
	if result.OpenPage != nil {
		result.Data["open_page"] = result.OpenPage
	}
	if len(result.Mark) > 0 {
		result.Data["mark"] = result.Mark
	}
}
