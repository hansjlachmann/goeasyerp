package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ItemJnlPost - Codeunit 241 "Item Jnl.-Post": the item journal pages' Post (F11). After asking, it
// posts the journal's batch (Item Jnl.-Post Batch) in one transaction — any error rolls the
// whole posting back — and tells the user the outcome.
const ItemJnlPostID = 241
const ItemJnlPostName = "item-jnl-post"

func init() {
	Register(ItemJnlPostID, ItemJnlPostName, NewItemJnlPost)
}

type ItemJnlPost struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewItemJnlPost creates a new instance of the codeunit
func NewItemJnlPost(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &ItemJnlPost{db: db, company: company, dbType: dbType}
}

func (c *ItemJnlPost) ID() int                    { return ItemJnlPostID }
func (c *ItemJnlPost) Name() string               { return ItemJnlPostName }
func (c *ItemJnlPost) SourceTable() string        { return gtables.ItemJournalLineTableName }
func (c *ItemJnlPost) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *ItemJnlPost) GetDB() database.Executor   { return c.db }
func (c *ItemJnlPost) GetCompany() string         { return c.company }
func (c *ItemJnlPost) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
func (c *ItemJnlPost) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.ItemJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 241 runs on an item journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *ItemJnlPost) code(rec *gtables.ItemJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = itemJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = itemJnlFilterValue(rec, "journal_batch_name")
	}
	itemJnlTemplate := gtables.NewItemJournalTemplate(c)
	err := itemJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = itemJnlTemplate.TestField("force_posting_report", false)
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
	itemJnlLine := gtables.NewItemJournalLine(src)
	itemJnlLine.Copy(rec)
	posted, err := itemjnl.NewItemJnlPostBatch(src).Run(itemJnlLine)
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

// itemJnlFilterValue is the exact value an item journal line is filtered on (filter group 2,
// the page's filters)
func itemJnlFilterValue(rec *gtables.ItemJournalLine, field string) types.Code {
	rec.FilterGroup(2)
	value := rec.GetFilter(field)
	rec.FilterGroup(0)
	return types.Code(value)
}
