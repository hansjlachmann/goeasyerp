package codeunits

import (
	"errors"

	"github.com/hansjlachmann/openerp/backend/business-logic/fajnl"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// FAJnlPost - Codeunit 5636 "FA. Jnl.-Post": the Fixed Asset Journal page's Post (F11). After asking, it
// posts the journal's batch (FA Jnl.-Post Batch) in one transaction — any error rolls the
// whole posting back — and tells the user the outcome.
const FAJnlPostID = 5636
const FAJnlPostName = "fa-jnl-post"

func init() {
	Register(FAJnlPostID, FAJnlPostName, NewFAJnlPost)
}

type FAJnlPost struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewFAJnlPost creates a new instance of the codeunit
func NewFAJnlPost(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &FAJnlPost{db: db, company: company, dbType: dbType}
}

func (c *FAJnlPost) ID() int                    { return FAJnlPostID }
func (c *FAJnlPost) Name() string               { return FAJnlPostName }
func (c *FAJnlPost) SourceTable() string        { return gtables.FAJournalLineTableName }
func (c *FAJnlPost) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *FAJnlPost) GetDB() database.Executor   { return c.db }
func (c *FAJnlPost) GetCompany() string         { return c.company }
func (c *FAJnlPost) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
func (c *FAJnlPost) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.FAJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 5636 runs on an FA journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *FAJnlPost) code(rec *gtables.FAJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = faJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = faJnlFilterValue(rec, "journal_batch_name")
	}
	faJnlTemplate := gtables.NewFAJournalTemplate(c)
	err := faJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = faJnlTemplate.TestField("force_posting_report", false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !system.Confirm(ts.Message("GEN_JNL_POST_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}

	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	faJnlLine := gtables.NewFAJournalLine(src)
	faJnlLine.Copy(rec)
	posted, err := fajnl.NewFAJnlPostBatch(src).Run(faJnlLine)
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

// faJnlFilterValue is the exact value an FA journal line is filtered on (filter group 2,
// the page's filters)
func faJnlFilterValue(rec *gtables.FAJournalLine, field string) types.Code {
	rec.FilterGroup(2)
	value := rec.GetFilter(field)
	rec.FilterGroup(0)
	return types.Code(value)
}
