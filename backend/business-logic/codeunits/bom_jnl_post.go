package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/bomjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// BOMJnlPost - Codeunit 251 "BOM Jnl.-Post": the BOM journal page's Post (F11). After asking, it
// posts the journal's batch (BOM Jnl.-Post Batch) in one transaction — any error rolls the
// whole posting back — and tells the user the outcome.
const BOMJnlPostID = 251
const BOMJnlPostName = "bom-jnl-post"

func init() {
	Register(BOMJnlPostID, BOMJnlPostName, NewBOMJnlPost)
}

type BOMJnlPost struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewBOMJnlPost creates a new instance of the codeunit
func NewBOMJnlPost(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &BOMJnlPost{db: db, company: company, dbType: dbType}
}

func (c *BOMJnlPost) ID() int                    { return BOMJnlPostID }
func (c *BOMJnlPost) Name() string               { return BOMJnlPostName }
func (c *BOMJnlPost) SourceTable() string        { return gtables.BOMJournalLineTableName }
func (c *BOMJnlPost) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *BOMJnlPost) GetDB() database.Executor   { return c.db }
func (c *BOMJnlPost) GetCompany() string         { return c.company }
func (c *BOMJnlPost) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
func (c *BOMJnlPost) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.BOMJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 251 runs on a BOM journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *BOMJnlPost) code(rec *gtables.BOMJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = bomJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = bomJnlFilterValue(rec, "journal_batch_name")
	}
	bomJnlTemplate := gtables.NewBOMJournalTemplate(c)
	err := bomJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = bomJnlTemplate.TestField("force_posting_report", false)
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
	bomJnlLine := gtables.NewBOMJournalLine(src)
	bomJnlLine.Copy(rec)
	posted, err := bomjnl.NewBOMJnlPostBatch(src).Run(bomJnlLine)
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

// bomJnlFilterValue is the exact value a BOM journal line is filtered on (filter group 2,
// the page's filters)
func bomJnlFilterValue(rec *gtables.BOMJournalLine, field string) types.Code {
	rec.FilterGroup(2)
	value := rec.GetFilter(field)
	rec.FilterGroup(0)
	return types.Code(value)
}
