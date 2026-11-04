package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ResJnlPost - Codeunit 271 "Res. Jnl.-Post": the resource journal page's Post (F11). After asking,
// it posts the journal's batch (Res. Jnl.-Post Batch) in one transaction — any error rolls the
// whole posting back — and tells the user the outcome.
const ResJnlPostID = 271
const ResJnlPostName = "res-jnl-post"

func init() {
	Register(ResJnlPostID, ResJnlPostName, NewResJnlPost)
}

type ResJnlPost struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewResJnlPost creates a new instance of the codeunit
func NewResJnlPost(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &ResJnlPost{db: db, company: company, dbType: dbType}
}

func (c *ResJnlPost) ID() int                    { return ResJnlPostID }
func (c *ResJnlPost) Name() string               { return ResJnlPostName }
func (c *ResJnlPost) SourceTable() string        { return gtables.ResJournalLineTableName }
func (c *ResJnlPost) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *ResJnlPost) GetDB() database.Executor   { return c.db }
func (c *ResJnlPost) GetCompany() string         { return c.company }
func (c *ResJnlPost) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
func (c *ResJnlPost) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.ResJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 271 runs on a resource journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *ResJnlPost) code(rec *gtables.ResJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = resJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = resJnlFilterValue(rec, "journal_batch_name")
	}
	resJnlTemplate := gtables.NewResJournalTemplate(c)
	err := resJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = resJnlTemplate.TestField("force_posting_report", false)
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
	resJnlLine := gtables.NewResJournalLine(src)
	resJnlLine.Copy(rec)
	posted, err := resjnl.NewResJnlPostBatch(src).Run(resJnlLine)
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

// resJnlFilterValue is the exact value a resource journal line is filtered on (filter group 2,
// the page's filters)
func resJnlFilterValue(rec *gtables.ResJournalLine, field string) types.Code {
	rec.FilterGroup(2)
	value := rec.GetFilter(field)
	rec.FilterGroup(0)
	return types.Code(value)
}
