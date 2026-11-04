package codeunits

import (
	"errors"

	"github.com/hansjlachmann/openerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// GenJnlPostPrint - Codeunit 232 "Gen. Jnl.-Post+Print": Post and Print (Shift+F11) on the general journal pages. The
// template must have a Posting Report ID; after asking, it posts the journal's batch in one
// transaction as Post (F11) does, then prints the template's posting report on the
// G/L Register the posting wrote, without request page, to the report's Printer Selection output.
const GenJnlPostPrintID = 232
const GenJnlPostPrintName = "gen-jnl-post-print"

func init() {
	Register(GenJnlPostPrintID, GenJnlPostPrintName, NewGenJnlPostPrint)
}

type GenJnlPostPrint struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewGenJnlPostPrint creates a new instance of the codeunit
func NewGenJnlPostPrint(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &GenJnlPostPrint{db: db, company: company, dbType: dbType}
}

func (c *GenJnlPostPrint) ID() int                    { return GenJnlPostPrintID }
func (c *GenJnlPostPrint) Name() string               { return GenJnlPostPrintName }
func (c *GenJnlPostPrint) SourceTable() string        { return gtables.GenJournalLineTableName }
func (c *GenJnlPostPrint) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *GenJnlPostPrint) GetDB() database.Executor   { return c.db }
func (c *GenJnlPostPrint) GetCompany() string         { return c.company }
func (c *GenJnlPostPrint) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
// and prints its register
func (c *GenJnlPostPrint) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.GenJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 232 runs on a journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *GenJnlPostPrint) code(rec *gtables.GenJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = filterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = filterValue(rec, "journal_batch_name")
	}
	jnlTemplate := gtables.NewGenJournalTemplate(c)
	err := jnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = jnlTemplate.TestField("posting_report_id")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if jnlTemplate.Recurring && rec.GetFilter("posting_date") != "" {
		return fcodeunits.Result{}, apperrors.JnlRecurringDateFiltered(gtables.GenJournalLineTableName)
	}
	if !system.Confirm(ts.Message("JNL_POST_PRINT_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}

	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	jnlLine := gtables.NewGenJournalLine(src)
	jnlLine.Copy(rec)
	posted, err := genjnl.NewGenJnlPostBatch(src).Run(jnlLine)
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
	register := gtables.NewGLRegister(src)
	if register.Get(posted.RegisterNo) {
		register.SetRecFilter()
		printouts, err = printPostingReport(src, jnlTemplate.Posting_report_id, register)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return reports.WithPrintouts(jnlPostedMessage(posted.RegisterNo, posted.JournalBatchName, rec.Journal_batch_name), printouts), nil
}
