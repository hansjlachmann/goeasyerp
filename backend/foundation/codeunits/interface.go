package codeunits

import "github.com/hansjlachmann/goeasyerp/backend/foundation/database"

// Codeunit is the interface that all codeunits must implement
type Codeunit interface {
	// ID returns the unique codeunit ID (e.g., 900)
	ID() int

	// Name returns the codeunit name (e.g., "generate-cust-ledger-entries")
	Name() string

	// SourceTable returns the name of the table this codeunit operates on (e.g., "Customer")
	SourceTable() string

	// Run executes the codeunit with the given record
	// The record should be a pointer to the appropriate table struct
	Run(record interface{}) (Result, error)

	// UsesProgress returns true if this codeunit uses progress dialog updates
	// If true, the codeunit will be run async with SSE progress streaming
	// If false, the codeunit runs synchronously and returns result directly
	UsesProgress() bool
}

// Result represents the result of a codeunit execution
type Result struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Dialog  *DialogResult          `json:"dialog,omitempty"` // Optional dialog to show
	// Refresh: the page reloads its data afterwards — a card its record and its subpages'
	// lines (e.g. a function on the header that changed the lines)
	Refresh bool `json:"refresh,omitempty"`
	// OpenPage: the page opens a page afterwards (after the message box) — a card on a record
	// the codeunit made, e.g. the order a quote became (classic PAGE.RUN after the codeunit)
	OpenPage *PageLink `json:"open_page,omitempty"`
}

// PageLink is a page on a record: its number and the record's key (comma-joined for a
// composite key, as in page URLs), the page's parameters (Posting Preview: preview) and
// whether Esc/close returns to the page the codeunit ran from (Return)
type PageLink struct {
	PageID int               `json:"page_id"`
	Key    string            `json:"key"`
	Params map[string]string `json:"params,omitempty"`
	Return bool              `json:"return,omitempty"`
}

// DialogResult represents a dialog/message box to display
type DialogResult struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Type    string `json:"type"` // "info", "success", "warning", "error"
}

// Factory is a function that creates a new codeunit instance
type Factory func(db database.Executor, company string, dbType database.DBType) Codeunit
