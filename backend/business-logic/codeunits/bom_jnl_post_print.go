package codeunits

import (
	"errors"

	"github.com/hansjlachmann/openerp/backend/business-logic/bomjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// BOMJnlPostPrint - Codeunit 252 "BOM Jnl.-Post+Print": Post and Print (Shift+F11) on the BOM journal pages. The
// template must have a Posting Report ID; after asking, it posts the journal's batch in one
// transaction as Post (F11) does, then prints the template's posting report on the
// BOM Register the posting wrote, without request page, to the report's Printer Selection output.
const BOMJnlPostPrintID = 252
const BOMJnlPostPrintName = "bom-jnl-post-print"

func init() {
	Register(BOMJnlPostPrintID, BOMJnlPostPrintName, NewBOMJnlPostPrint)
}

type BOMJnlPostPrint struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewBOMJnlPostPrint creates a new instance of the codeunit
func NewBOMJnlPostPrint(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &BOMJnlPostPrint{db: db, company: company, dbType: dbType}
}

func (c *BOMJnlPostPrint) ID() int                    { return BOMJnlPostPrintID }
func (c *BOMJnlPostPrint) Name() string               { return BOMJnlPostPrintName }
func (c *BOMJnlPostPrint) SourceTable() string        { return gtables.BOMJournalLineTableName }
func (c *BOMJnlPostPrint) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *BOMJnlPostPrint) GetDB() database.Executor   { return c.db }
func (c *BOMJnlPostPrint) GetCompany() string         { return c.company }
func (c *BOMJnlPostPrint) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
// and prints its register
func (c *BOMJnlPostPrint) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.BOMJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 252 runs on a journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *BOMJnlPostPrint) code(rec *gtables.BOMJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = bomJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = bomJnlFilterValue(rec, "journal_batch_name")
	}
	jnlTemplate := gtables.NewBOMJournalTemplate(c)
	err := jnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = jnlTemplate.TestField("posting_report_id")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if jnlTemplate.Recurring && rec.GetFilter("posting_date") != "" {
		return fcodeunits.Result{}, apperrors.JnlRecurringDateFiltered(gtables.BOMJournalLineTableName)
	}
	if !system.Confirm(ts.Message("JNL_POST_PRINT_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}

	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	jnlLine := gtables.NewBOMJournalLine(src)
	jnlLine.Copy(rec)
	posted, err := bomjnl.NewBOMJnlPostBatch(src).Run(jnlLine)
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
	register := gtables.NewBOMRegister(src)
	if register.Get(posted.RegisterNo) {
		register.SetRecFilter()
		printouts, err = printPostingReport(src, jnlTemplate.Posting_report_id, register)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return reports.WithPrintouts(jnlPostedMessage(posted.RegisterNo, posted.JournalBatchName, rec.Journal_batch_name), printouts), nil
}
