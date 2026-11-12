package codeunits

import (
	"bytes"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
)

// currentDialogs holds the dialog for each goroutine
var currentDialogs sync.Map

// goroutineContext holds per-goroutine session context
type goroutineContext struct {
	UserID   string
	UserName string
	Company  string
	Language string
	// Access: the user's permissions (nil: no user — internal code, tests — is not checked)
	Access *session.Session
	// PageParams: the parameters of the page the code runs for (a page opened from code with
	// parameters, e.g. Apply Customer Entries with the journal line's Applies-to ID)
	PageParams map[string]string
	// Regional: the user's work date and date and number formats (My Settings)
	Regional Regional
	// DimensionLabels: the company's names of dimension 1 and 2 (General Ledger Setup)
	DimensionLabels [2]string
}

// currentContexts holds the context for each goroutine
var currentContexts sync.Map

// SetCurrentContext sets the session context for the current goroutine
// This should be called at the start of a goroutine that runs codeunits
func SetCurrentContext(userID, userName, company, language string) {
	ctx := &goroutineContext{
		UserID:   userID,
		UserName: userName,
		Company:  company,
		Language: language,
	}
	if RegionalLoader != nil && userID != "" {
		ctx.Regional = RegionalLoader(userID)
	}
	ctx.DimensionLabels = loadDimensionLabels(company)
	currentContexts.Store(getGoroutineID(), ctx)
}

// SetCurrentAccess sets the permissions of the current goroutine's user (after
// SetCurrentContext): CanExecute checks them
func SetCurrentAccess(access *session.Session) {
	ctx := GetCurrentContext()
	if ctx == nil {
		ctx = &goroutineContext{}
		currentContexts.Store(getGoroutineID(), ctx)
	}
	ctx.Access = access
}

// CanExecute reports whether the current user may run a codeunit or report (session
// CanExecute); without a user (internal code, a run without permissions loaded) it is allowed
func CanExecute(objectType string, id int) bool {
	ctx := GetCurrentContext()
	if ctx == nil || ctx.Access == nil {
		return true
	}
	return ctx.Access.CanExecute(objectType, id)
}

// CanRead reports whether the current user may read a table (classic READPERMISSION; session
// HasPermission); without a user (internal code, a run without permissions loaded) it is allowed
func CanRead(tableName string) bool {
	ctx := GetCurrentContext()
	if ctx == nil || ctx.Access == nil {
		return true
	}
	return ctx.Access.HasPermission(tableName, session.PermRead)
}

// HasPermission reports whether the current user has a permission on a table of the company
// (session HasPermission); without a user (internal code, a run without permissions loaded) it
// is allowed
func HasPermission(tableName string, op session.PermissionOperation) bool {
	ctx := GetCurrentContext()
	if ctx == nil || ctx.Access == nil {
		return true
	}
	return ctx.Access.HasPermission(tableName, op)
}

// CurrentAccess returns the current user's permissions in the company (nil: not loaded —
// internal code)
func CurrentAccess() *session.Session {
	ctx := GetCurrentContext()
	if ctx == nil {
		return nil
	}
	return ctx.Access
}

// CanReadInCompany reports whether the current user may read a table of another company (code
// that reads it with ChangeCompany): the user's roles there — without roles in it a user with
// roles has no access to the company; without a user (internal code) or without roles at all
// (permission system not configured for the user) it is allowed
func CanReadInCompany(db database.Executor, dbType database.DBType, company, tableName string) bool {
	access := CurrentAccess()
	if access == nil || !access.HasRoles {
		return true
	}
	hasRoles, isSuper, perms := session.LoadPermissions(db, dbType, CurrentUserID(), company)
	if !hasRoles {
		return false
	}
	other := &session.Session{}
	other.SetPermissions(hasRoles, isSuper, perms)
	return other.HasPermission(tableName, session.PermRead)
}

// GetCurrentContext returns the context for the current goroutine
func GetCurrentContext() *goroutineContext {
	if ctx, ok := currentContexts.Load(getGoroutineID()); ok {
		return ctx.(*goroutineContext)
	}
	return nil
}

// ClearCurrentContext clears the context for the current goroutine
func ClearCurrentContext() {
	currentContexts.Delete(getGoroutineID())
}

// SetCurrentDialog sets the dialog for the current goroutine
func SetCurrentDialog(d *Dialog) {
	currentDialogs.Store(getGoroutineID(), d)
}

// GetCurrentDialog returns the dialog for the current goroutine
func GetCurrentDialog() *Dialog {
	if d, ok := currentDialogs.Load(getGoroutineID()); ok {
		return d.(*Dialog)
	}
	return nil
}

// ClearCurrentDialog clears the dialog for the current goroutine
func ClearCurrentDialog() {
	currentDialogs.Delete(getGoroutineID())
}

// getGoroutineID returns the ID of the current goroutine
func getGoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	// Stack trace format: "goroutine 123 [running]:"
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

// CurrentUserID returns the current user ID
// First checks goroutine-specific context, then falls back to global session
func CurrentUserID() string {
	// Check goroutine-specific context first
	if ctx := GetCurrentContext(); ctx != nil {
		return ctx.UserID
	}
	// Fall back to global session
	sess := session.GetCurrent()
	if sess != nil {
		return sess.GetUserID()
	}
	return ""
}

// CurrentCompany returns the current company name
// First checks goroutine-specific context, then falls back to global session
func CurrentCompany() string {
	// Check goroutine-specific context first
	if ctx := GetCurrentContext(); ctx != nil {
		return ctx.Company
	}
	// Fall back to global session
	sess := session.GetCurrent()
	if sess != nil {
		return sess.GetCompany()
	}
	return ""
}

// CurrentLanguage returns the current user's language
// First checks goroutine-specific context, then falls back to global session
func CurrentLanguage() string {
	if ctx := GetCurrentContext(); ctx != nil {
		return ctx.Language
	}
	sess := session.GetCurrent()
	if sess != nil {
		return sess.GetLanguage()
	}
	return "en-US"
}

// CurrentDateTime returns the current date and time
func CurrentDateTime() time.Time {
	return time.Now()
}

// CurrentDate returns the current date (time set to midnight)
func CurrentDate() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// CurrentTime returns the current time (date set to zero)
func CurrentTime() time.Time {
	now := time.Now()
	return time.Date(0, 1, 1, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location())
}

// Message returns a result that displays an info dialog
func Message(text string) Result {
	ts := i18n.GetInstance()
	return Result{
		Success: true,
		Dialog: &DialogResult{
			Title:   ts.Message("DLG_MESSAGE_TITLE", CurrentLanguage()),
			Message: text,
			Type:    "info",
		},
	}
}

// Error returns a result that displays an error dialog
func Error(text string) Result {
	ts := i18n.GetInstance()
	return Result{
		Success: false,
		Dialog: &DialogResult{
			Title:   ts.Message("DLG_ERROR_TITLE", CurrentLanguage()),
			Message: text,
			Type:    "error",
		},
	}
}

// Confirm shows a confirmation dialog and waits for user response
// Returns true if user clicks Yes, false if No
// Like Confirm('Are you sure?')
// In an API request (a table trigger) the question is answered in rounds (questions.go).
func Confirm(message string) bool {
	dialog := GetCurrentDialog()
	if dialog == nil {
		answer, asked := requestConfirm(message)
		if asked {
			return answer
		}
		// No dialog context - can't show confirm
		// Return false as safe default
		return false
	}
	return dialog.Confirm(message)
}

// RequestInput shows an input dialog with arbitrary fields and waits for user response
// Returns a map of field name → value, or nil on cancel/timeout
func RequestInput(title string, fields []InputField) map[string]string {
	dialog := GetCurrentDialog()
	if dialog == nil {
		return nil
	}
	return dialog.RequestInput(title, fields)
}

// Hyperlink opens a URL in the user's browser; without a dialog (a scheduled run) nothing
// happens. Note: This only works in async codeunits (UsesProgress() = true)
func Hyperlink(url string) {
	if dialog := GetCurrentDialog(); dialog != nil {
		dialog.Hyperlink(url)
	}
}

// RunPage opens a page from a codeunit and waits until the user closes it (see
// Dialog.RunPage); nil without a dialog (a scheduled run).
func RunPage(pageID int, filters, flowFilters interface{}, selectKey string, lookupMode bool, params map[string]string) map[string]string {
	return RunPageFixed(pageID, filters, nil, flowFilters, selectKey, lookupMode, params)
}

// RunPageFixed is RunPage with fixed filters: field and expression, the page keeps them in
// its filter group 2 (the user's filters narrow them and cannot remove them)
func RunPageFixed(pageID int, filters interface{}, fixedFilters []ftables.FieldFilter, flowFilters interface{}, selectKey string, lookupMode bool, params map[string]string) map[string]string {
	dialog := GetCurrentDialog()
	if dialog == nil {
		return nil
	}
	return dialog.runPage(pageID, filters, fixedFilters, flowFilters, selectKey, lookupMode, params)
}

// RequestPage shows a report's request page (options and the data items' filters) and waits
// for the answer (see Dialog.RequestPage); nil without a dialog or on cancel. A printed
// report's request page (output: the output chosen first — "preview", "print" or "pdf"; ""
// for a report that does not print) has Preview, Print and PDF instead of OK: the answer's
// OutputKey says which was chosen
func RequestPage(title string, fields []InputField, dataItems []RequestDataItem, optionsCaption string, output string) map[string]string {
	dialog := GetCurrentDialog()
	if dialog == nil {
		return nil
	}
	return dialog.RequestPage(title, fields, dataItems, optionsCaption, output)
}

// SetCurrentPageParams sets the parameters of the page the current goroutine's code runs for
// (its functions and triggers read them with system.PageParam)
func SetCurrentPageParams(params map[string]string) {
	ctx := GetCurrentContext()
	if ctx == nil {
		ctx = &goroutineContext{}
		currentContexts.Store(getGoroutineID(), ctx)
	}
	ctx.PageParams = params
}

// CurrentPageParam returns a parameter of the page the code runs for ("" when none)
func CurrentPageParam(name string) string {
	ctx := GetCurrentContext()
	if ctx == nil {
		return ""
	}
	return ctx.PageParams[name]
}
