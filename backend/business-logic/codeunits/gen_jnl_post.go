package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// GenJnlPost - Codeunit 231 "Gen. Jnl.-Post": the journal pages' Post (F11). After asking, it
// posts the journal's batch (Gen. Jnl.-Post Batch) in one transaction — any error rolls the
// whole posting back — and tells the user the outcome.
const GenJnlPostID = 231
const GenJnlPostName = "gen-jnl-post"

func init() {
	Register(GenJnlPostID, GenJnlPostName, NewGenJnlPost)
}

type GenJnlPost struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewGenJnlPost creates a new instance of the codeunit
func NewGenJnlPost(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &GenJnlPost{db: db, company: company, dbType: dbType}
}

func (c *GenJnlPost) ID() int                    { return GenJnlPostID }
func (c *GenJnlPost) Name() string               { return GenJnlPostName }
func (c *GenJnlPost) SourceTable() string        { return gtables.GenJournalLineTableName }
func (c *GenJnlPost) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *GenJnlPost) GetDB() database.Executor   { return c.db }
func (c *GenJnlPost) GetCompany() string         { return c.company }
func (c *GenJnlPost) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
func (c *GenJnlPost) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.GenJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 231 runs on a journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *GenJnlPost) code(rec *gtables.GenJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = filterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = filterValue(rec, "journal_batch_name")
	}
	genJnlTemplate := gtables.NewGenJournalTemplate(c)
	err := genJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = genJnlTemplate.TestField("force_posting_report", false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if genJnlTemplate.Recurring && rec.GetFilter("posting_date") != "" {
		return fcodeunits.Result{}, apperrors.JnlRecurringDateFiltered(gtables.GenJournalLineTableName)
	}
	if !system.Confirm(ts.Message("GEN_JNL_POST_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}

	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	genJnlLine := gtables.NewGenJournalLine(src)
	genJnlLine.Copy(rec)
	posted, err := genjnl.NewGenJnlPostBatch(src).Run(genJnlLine)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}

	switch {
	case posted.RegisterNo == 0:
		return system.Message(ts.Message("GEN_JNL_NOTHING_TO_POST", lang)), nil
	case posted.JournalBatchName == rec.Journal_batch_name:
		return system.Message(ts.Message("GEN_JNL_POSTED", lang)), nil
	default:
		return system.Message(ts.Message("GEN_JNL_POSTED_NEW_BATCH", lang), posted.JournalBatchName), nil
	}
}

// filterValue is the exact value a record is filtered on (filter group 2, the page's filters)
func filterValue(rec *gtables.GenJournalLine, field string) types.Code {
	rec.FilterGroup(2)
	value := rec.GetFilter(field)
	rec.FilterGroup(0)
	return types.Code(value)
}
