package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ResJnlPostPrint - Codeunit 272 "Res. Jnl.-Post+Print": Post and Print (Shift+F11) on the resource journal pages. The
// template must have a Posting Report ID; after asking, it posts the journal's batch in one
// transaction as Post (F11) does, then prints the template's posting report on the
// Resource Register the posting wrote, without request page, to the report's Printer Selection output.
const ResJnlPostPrintID = 272
const ResJnlPostPrintName = "res-jnl-post-print"

func init() {
	Register(ResJnlPostPrintID, ResJnlPostPrintName, NewResJnlPostPrint)
}

type ResJnlPostPrint struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewResJnlPostPrint creates a new instance of the codeunit
func NewResJnlPostPrint(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &ResJnlPostPrint{db: db, company: company, dbType: dbType}
}

func (c *ResJnlPostPrint) ID() int                    { return ResJnlPostPrintID }
func (c *ResJnlPostPrint) Name() string               { return ResJnlPostPrintName }
func (c *ResJnlPostPrint) SourceTable() string        { return gtables.ResJournalLineTableName }
func (c *ResJnlPostPrint) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *ResJnlPostPrint) GetDB() database.Executor   { return c.db }
func (c *ResJnlPostPrint) GetCompany() string         { return c.company }
func (c *ResJnlPostPrint) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
// and prints its register
func (c *ResJnlPostPrint) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.ResJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 272 runs on a journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *ResJnlPostPrint) code(rec *gtables.ResJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = resJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = resJnlFilterValue(rec, "journal_batch_name")
	}
	jnlTemplate := gtables.NewResJournalTemplate(c)
	err := jnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = jnlTemplate.TestField("posting_report_id")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if jnlTemplate.Recurring && rec.GetFilter("posting_date") != "" {
		return fcodeunits.Result{}, apperrors.JnlRecurringDateFiltered(gtables.ResJournalLineTableName)
	}
	if !system.Confirm(ts.Message("JNL_POST_PRINT_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}

	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	jnlLine := gtables.NewResJournalLine(src)
	jnlLine.Copy(rec)
	posted, err := resjnl.NewResJnlPostBatch(src).Run(jnlLine)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}

	var printouts []reports.Result
	src = ftables.DBSource(c.db, c.company, c.dbType)
	register := gtables.NewResourceRegister(src)
	if register.Get(posted.RegisterNo) {
		register.SetRecFilter()
		printouts, err = printPostingReport(src, jnlTemplate.Posting_report_id, register)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return reports.WithPrintouts(jnlPostedMessage(posted.RegisterNo, posted.JournalBatchName, rec.Journal_batch_name), printouts), nil
}
