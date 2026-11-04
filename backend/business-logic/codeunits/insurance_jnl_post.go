package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/insurancejnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// InsuranceJnlPost - Codeunit 5654 "Insurance Jnl.-Post": the Insurance Journal page's Post
// (F11). After asking, it posts the journal's batch (Insurance Jnl.-Post Batch) in one
// transaction and tells the user the outcome.
const InsuranceJnlPostID = 5654
const InsuranceJnlPostName = "insurance-jnl-post"

func init() {
	Register(InsuranceJnlPostID, InsuranceJnlPostName, NewInsuranceJnlPost)
}

type InsuranceJnlPost struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewInsuranceJnlPost creates a new instance of the codeunit
func NewInsuranceJnlPost(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &InsuranceJnlPost{db: db, company: company, dbType: dbType}
}

func (c *InsuranceJnlPost) ID() int                    { return InsuranceJnlPostID }
func (c *InsuranceJnlPost) Name() string               { return InsuranceJnlPostName }
func (c *InsuranceJnlPost) SourceTable() string        { return gtables.InsuranceJournalLineTableName }
func (c *InsuranceJnlPost) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *InsuranceJnlPost) GetDB() database.Executor   { return c.db }
func (c *InsuranceJnlPost) GetCompany() string         { return c.company }
func (c *InsuranceJnlPost) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
func (c *InsuranceJnlPost) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.InsuranceJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 5654 runs on an insurance journal line")
	}
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = insuranceJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = insuranceJnlFilterValue(rec, "journal_batch_name")
	}
	insuranceJnlTemplate := gtables.NewInsuranceJournalTemplate(c)
	err := insuranceJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = insuranceJnlTemplate.TestField("force_posting_report", false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !system.Confirm(ts.Message("GEN_JNL_POST_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}
	tempJnlBatchName := rec.Journal_batch_name
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	posted, err := insurancejnl.PostBatch(ftables.DBSource(tx, c.company, c.dbType), rec)
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
	case posted.JournalBatchName == tempJnlBatchName:
		return system.Message(ts.Message("GEN_JNL_POSTED", lang)), nil
	default:
		return system.Message(ts.Message("GEN_JNL_POSTED_NEW_BATCH", lang), posted.JournalBatchName), nil
	}
}

// insuranceJnlFilterValue is the exact value an insurance journal line is filtered on (filter
// group 2, the page's filters)
func insuranceJnlFilterValue(rec *gtables.InsuranceJournalLine, field string) types.Code {
	rec.FilterGroup(2)
	value := rec.GetFilter(field)
	rec.FilterGroup(0)
	return types.Code(value)
}
