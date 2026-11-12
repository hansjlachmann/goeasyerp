package codeunits

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"

	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
)

// ProgressEvent represents a progress update or dialog event
type ProgressEvent struct {
	JobID     string `json:"job_id"`
	Field     int    `json:"field"`     // Field number (1-based)
	Value     int    `json:"value"`     // Current value (0-100 for progress)
	Message   string `json:"message"`   // Optional message
	Completed bool   `json:"completed"` // Job completed
	Error     string `json:"error"`     // Error message if failed
	// ErrorDetails: what lies behind the error (an AppError's Details — e.g. the G/L entries of
	// a posting that does not balance), shown with Show Entries
	ErrorDetails *apperrors.Details     `json:"error_details,omitempty"`
	Timestamp    int64                  `json:"timestamp"`
	EventType    string                 `json:"event_type,omitempty"` // "progress", "confirm", etc.
	ConfirmID    string                 `json:"confirm_id,omitempty"` // Unique ID for confirm dialogs
	Data         map[string]interface{} `json:"data,omitempty"`       // Result data (e.g., PDF base64)
}

// OutputKey: the answer of a printed report's request page holds the output chosen under this
// key — "preview", "print" or "pdf"
const OutputKey = "_output"

// InputField describes a single input field for RequestInput dialogs
type InputField struct {
	Name     string `json:"name"`               // Field key (e.g. "date")
	Label    string `json:"label"`              // Display label
	Type     string `json:"type"`               // "date", "text", "number", "option", "bool", "file" (the contents as base64)
	Required bool   `json:"required,omitempty"` // Whether the field is required
	Default  string `json:"default,omitempty"`  // Default value ("option": the number, 1-based)
	// Options: the choices of an "option" field, shown as a list to pick one from; the value
	// is the number (1-based) of the chosen option
	Options []string `json:"options,omitempty"`
	// Lookup: a code field picked from the rows of a table (a report option with a table
	// relation, e.g. a currency) — shown as the dropdown of a relation field
	Lookup *InputLookup `json:"lookup,omitempty"`
}

// InputLookup is the dropdown of an input field: its columns (the key first) and rows, each
// with "_key" (the value picked)
type InputLookup struct {
	Columns  []InputLookupColumn      `json:"columns"`
	Rows     []map[string]interface{} `json:"rows"`
	Captions map[string]string        `json:"captions,omitempty"` // column captions
}

// InputLookupColumn is a column of an input field's dropdown
type InputLookupColumn struct {
	Source string `json:"source"`
	Width  int    `json:"width"`
}

// RequestDataItem is a data item of a report's request page: the user filters its records on
// any of its fields, as on a list page (the filter pane)
type RequestDataItem struct {
	Name    string          `json:"name"`    // the data item's name (the answer's key: "filters:<name>")
	Caption string          `json:"caption"` // the table's caption
	Indent  int             `json:"indent"`  // 0 top level, 1 under its parent, …
	Fields  []RequestField  `json:"fields"`  // the fields that can be filtered
	Filters []RequestFilter `json:"filters"` // the filters to start with (request filter fields)
}

// RequestField is a field a request page can filter on
type RequestField struct {
	Name    string `json:"name"`
	Caption string `json:"caption"`
	Type    string `json:"type,omitempty"`
}

// RequestFilter is a filter of a request page: a field and an expression in the filter syntax
type RequestFilter struct {
	Field      string `json:"field"`
	Expression string `json:"expression"`
}

// Dialog represents a dialog with progress bar support
// Usage:
//
//	dialog := OpenDialog("Processing @1@@@@@@@@@@@")
//	defer dialog.Close()
//	for i := 1; i <= 100; i++ {
//	    dialog.Update(1, i)
//	}
type Dialog struct {
	jobID          string
	template       string
	events         chan ProgressEvent
	ctx            context.Context
	cancel         context.CancelFunc
	closed         bool
	mu             sync.Mutex
	pendingConfirm map[string]chan bool // Map of confirm ID to response channel
	confirmMu      sync.Mutex
	pendingInput   map[string]chan map[string]string // Map of confirm ID to input response channel
	inputMu        sync.Mutex
	// activity is closed by the first event that is not the end (progress, a question, a
	// request page, a page, a note): the job API then lets the browser follow the job
	activity     chan struct{}
	activityOnce sync.Once
}

// signalActivity marks that the run has shown something to the user (Activity)
func (d *Dialog) signalActivity() {
	d.activityOnce.Do(func() {
		if d.activity != nil {
			close(d.activity)
		}
	})
}

// Activity is closed once the run sends the browser an event before it ends (progress, a
// question, a request page, a page it opens, a note): a run that ends without one can answer
// in the request itself
func (d *Dialog) Activity() <-chan struct{} {
	return d.activity
}

// Discard removes a dialog nobody follows (its run ended without showing anything; the
// result went back in the request)
func (d *Dialog) Discard() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	d.cancel()
	jobRegistry.Remove(d.jobID)
}

// Update sends a progress update for a field
// field is 1-based (@1@, @2@, etc.)
// value is typically 0-100 for percentage
func (d *Dialog) Update(field int, value int) {
	d.signalActivity()
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}

	select {
	case d.events <- ProgressEvent{
		JobID:     d.jobID,
		Field:     field,
		Value:     value,
		Timestamp: time.Now().UnixMilli(),
	}:
	case <-d.ctx.Done():
	default:
		// Non-blocking, drop if buffer full
	}
}

// UpdateWithMessage sends a progress update with a message
func (d *Dialog) UpdateWithMessage(field int, value int, message string) {
	d.signalActivity()
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}

	select {
	case d.events <- ProgressEvent{
		JobID:     d.jobID,
		Field:     field,
		Value:     value,
		Message:   message,
		Timestamp: time.Now().UnixMilli(),
	}:
	case <-d.ctx.Done():
	default:
	}
}

// Close closes the dialog and signals completion
func (d *Dialog) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}
	d.closed = true

	select {
	case d.events <- ProgressEvent{
		JobID:     d.jobID,
		Completed: true,
		Timestamp: time.Now().UnixMilli(),
	}:
	case <-d.ctx.Done():
	default:
	}

	// Give SSE clients time to receive the completion event
	time.AfterFunc(500*time.Millisecond, func() {
		d.cancel()
		jobRegistry.Remove(d.jobID)
	})
}

// CloseWithError closes the dialog with an error
func (d *Dialog) CloseWithError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}
	d.closed = true

	// an AppError in the user's language, with its details
	message := err.Error()
	var details *apperrors.Details
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		message = appErr.Message(CurrentLanguage())
		details = appErr.Details
	}
	select {
	case d.events <- ProgressEvent{
		JobID:        d.jobID,
		Completed:    true,
		Error:        message,
		ErrorDetails: details,
		Timestamp:    time.Now().UnixMilli(),
	}:
	case <-d.ctx.Done():
	default:
	}

	time.AfterFunc(500*time.Millisecond, func() {
		d.cancel()
		jobRegistry.Remove(d.jobID)
	})
}

// CloseWithData closes the dialog with result data (e.g., PDF base64)
func (d *Dialog) CloseWithData(data map[string]interface{}, message string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}
	d.closed = true

	select {
	case d.events <- ProgressEvent{
		JobID:     d.jobID,
		Completed: true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}:
	case <-d.ctx.Done():
	default:
	}

	time.AfterFunc(500*time.Millisecond, func() {
		d.cancel()
		jobRegistry.Remove(d.jobID)
	})
}

// JobID returns the job ID for this dialog
func (d *Dialog) JobID() string {
	return d.jobID
}

// Cancel cancels the running job
func (d *Dialog) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}
	d.closed = true

	select {
	case d.events <- ProgressEvent{
		JobID:     d.jobID,
		Completed: true,
		Error:     "Cancelled by user",
		EventType: "cancelled",
		Timestamp: time.Now().UnixMilli(),
	}:
	case <-d.ctx.Done():
	default:
	}

	// Cancel the context immediately to stop the codeunit
	d.cancel()
	jobRegistry.Remove(d.jobID)
}

// Events returns the channel for progress events (used by SSE handler)
func (d *Dialog) Events() <-chan ProgressEvent {
	return d.events
}

// Context returns the context for this dialog
func (d *Dialog) Context() context.Context {
	return d.ctx
}

// Confirm shows a confirmation dialog and waits for user response
// Returns true if user clicks Yes, false if No
// Like Confirm('Are you sure?')
func (d *Dialog) Confirm(message string) bool {
	d.signalActivity()
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return false
	}
	d.mu.Unlock()

	// Create unique confirm ID and response channel
	confirmID := uuid.New().String()
	responseChan := make(chan bool, 1)

	// Register the pending confirm
	d.confirmMu.Lock()
	if d.pendingConfirm == nil {
		d.pendingConfirm = make(map[string]chan bool)
	}
	d.pendingConfirm[confirmID] = responseChan
	d.confirmMu.Unlock()

	// Clean up on completion
	defer func() {
		d.confirmMu.Lock()
		delete(d.pendingConfirm, confirmID)
		d.confirmMu.Unlock()
	}()

	// Send confirm event to frontend
	select {
	case d.events <- ProgressEvent{
		JobID:     d.jobID,
		EventType: "confirm",
		ConfirmID: confirmID,
		Message:   message,
		Timestamp: time.Now().UnixMilli(),
	}:
	case <-d.ctx.Done():
		return false
	}

	// Wait for response
	select {
	case response := <-responseChan:
		return response
	case <-d.ctx.Done():
		return false
	case <-time.After(5 * time.Minute): // Timeout after 5 minutes
		return false
	}
}

// Hyperlink asks the frontend to open a URL in the user's browser (a new tab); it does not
// wait for an answer
func (d *Dialog) Hyperlink(url string) {
	d.signalActivity()
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}

	select {
	case d.events <- ProgressEvent{
		JobID:     d.jobID,
		EventType: "hyperlink",
		Message:   url,
		Timestamp: time.Now().UnixMilli(),
	}:
	case <-d.ctx.Done():
	}
}

// RespondToConfirm handles a confirm response from the frontend
func (d *Dialog) RespondToConfirm(confirmID string, response bool) bool {
	d.confirmMu.Lock()
	defer d.confirmMu.Unlock()

	if ch, ok := d.pendingConfirm[confirmID]; ok {
		select {
		case ch <- response:
			return true
		default:
			return false
		}
	}
	return false
}

// RequestInput shows an input dialog with arbitrary fields and waits for user response
// Returns a map of field name → value, or nil on cancel/timeout
func (d *Dialog) RequestInput(title string, fields []InputField) map[string]string {
	return d.requestInput(title, fields, nil, "", "")
}

// RequestPage is RequestInput with the data items of a report: the answer has the options
// ("option" fields as in RequestInput) and, per data item, "filters:<name>" — a JSON list of
// {field, expression}. The options come below the filters, under optionsCaption ("": the
// standard "Options"). A printed report's (output: the output chosen first, its printer —
// "preview", "print" or "pdf"; "" for a report that does not print) has Preview, Print and PDF
// instead of OK, and the answer's OutputKey holds the choice. nil on cancel/timeout.
func (d *Dialog) RequestPage(title string, fields []InputField, dataItems []RequestDataItem, optionsCaption string, output string) map[string]string {
	return d.requestInput(title, fields, dataItems, optionsCaption, output)
}

func (d *Dialog) requestInput(title string, fields []InputField, dataItems []RequestDataItem, optionsCaption string, output string) map[string]string {
	d.signalActivity()
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.mu.Unlock()

	// Create unique confirm ID and response channel
	confirmID := uuid.New().String()
	responseChan := make(chan map[string]string, 1)

	// Register the pending input
	d.inputMu.Lock()
	if d.pendingInput == nil {
		d.pendingInput = make(map[string]chan map[string]string)
	}
	d.pendingInput[confirmID] = responseChan
	d.inputMu.Unlock()

	// Clean up on completion
	defer func() {
		d.inputMu.Lock()
		delete(d.pendingInput, confirmID)
		d.inputMu.Unlock()
	}()

	// Send request_input event to frontend
	select {
	case d.events <- ProgressEvent{
		JobID:     d.jobID,
		EventType: "request_input",
		ConfirmID: confirmID,
		Message:   title,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"fields":          fields,
			"data_items":      dataItems,
			"options_caption": optionsCaption,
			"printed":         output != "",
			"default_output":  output,
		},
	}:
	case <-d.ctx.Done():
		return nil
	}

	// Wait for response
	select {
	case values := <-responseChan:
		return values
	case <-d.ctx.Done():
		return nil
	case <-time.After(5 * time.Minute):
		return nil
	}
}

// RunPage asks the frontend to open a page (a list, filtered, with a record selected) and
// waits until the user closes it; in lookup mode the answer holds the key of the record the
// user picked: {"action": "LookupOK", "key": "C01"}. nil on cancel/timeout.
func (d *Dialog) RunPage(pageID int, filters, flowFilters interface{}, selectKey string, lookupMode bool, params map[string]string) map[string]string {
	return d.runPage(pageID, filters, nil, flowFilters, selectKey, lookupMode, params)
}

func (d *Dialog) runPage(pageID int, filters interface{}, fixedFilters []ftables.FieldFilter, flowFilters interface{}, selectKey string, lookupMode bool, params map[string]string) map[string]string {
	d.signalActivity()
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.mu.Unlock()

	confirmID := uuid.New().String()
	responseChan := make(chan map[string]string, 1)
	d.inputMu.Lock()
	if d.pendingInput == nil {
		d.pendingInput = make(map[string]chan map[string]string)
	}
	d.pendingInput[confirmID] = responseChan
	d.inputMu.Unlock()
	defer func() {
		d.inputMu.Lock()
		delete(d.pendingInput, confirmID)
		d.inputMu.Unlock()
	}()

	select {
	case d.events <- ProgressEvent{
		JobID:     d.jobID,
		EventType: "run_page",
		ConfirmID: confirmID,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"page_id":      pageID,
			"filters":      filters,
			"flow_filters": flowFilters,
			// the fixed filters as the list's fixed filters: expressions in filter group 2
			"fixed_filters": fixedPageFilters(fixedFilters),
			"select":        selectKey,
			"lookup_mode":   lookupMode,
			"params":        params,
		},
	}:
	case <-d.ctx.Done():
		return nil
	}

	// The user works in the page: wait longer than for a question
	select {
	case values := <-responseChan:
		return values
	case <-d.ctx.Done():
		return nil
	case <-time.After(30 * time.Minute):
		return nil
	}
}

// RespondToInput handles an input dialog response from the frontend
func (d *Dialog) RespondToInput(confirmID string, values map[string]string) bool {
	d.inputMu.Lock()
	defer d.inputMu.Unlock()

	if ch, ok := d.pendingInput[confirmID]; ok {
		select {
		case ch <- values:
			return true
		default:
			return false
		}
	}
	return false
}

// JobRegistry tracks running jobs with dialogs
type JobRegistry struct {
	jobs map[string]*Dialog
	mu   sync.RWMutex
}

var jobRegistry = &JobRegistry{
	jobs: make(map[string]*Dialog),
}

// GetJobRegistry returns the global job registry
func GetJobRegistry() *JobRegistry {
	return jobRegistry
}

// Register adds a job to the registry
func (r *JobRegistry) Register(d *Dialog) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[d.jobID] = d
}

// Get retrieves a job by ID
func (r *JobRegistry) Get(jobID string) (*Dialog, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.jobs[jobID]
	return d, ok
}

// Remove removes a job from the registry
func (r *JobRegistry) Remove(jobID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.jobs, jobID)
}

// OpenDialog creates a new dialog with progress bar support
// template follows the usual convention: @1@@@@@@@@@@@ for field 1 progress bar
// Returns the dialog and its job ID
func OpenDialog(template string) *Dialog {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Dialog{
		jobID:    uuid.New().String(),
		template: template,
		events:   make(chan ProgressEvent, 100), // Buffered channel
		ctx:      ctx,
		cancel:   cancel,
		activity: make(chan struct{}),
	}
	jobRegistry.Register(d)
	return d
}

// OpenDialogWithID creates a dialog with a specific job ID (for testing)
func OpenDialogWithID(template string, jobID string) *Dialog {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Dialog{
		jobID:    jobID,
		template: template,
		events:   make(chan ProgressEvent, 100),
		ctx:      ctx,
		cancel:   cancel,
		activity: make(chan struct{}),
	}
	jobRegistry.Register(d)
	return d
}

// fixedPageFilters are fixed filters as a list page takes them ({field, value, expression})
func fixedPageFilters(filters []ftables.FieldFilter) []map[string]interface{} {
	result := []map[string]interface{}{}
	for _, f := range filters {
		result = append(result, map[string]interface{}{"field": f.Field, "value": f.Expression, "expression": true})
	}
	return result
}
