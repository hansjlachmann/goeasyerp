package codeunits

import (
	"errors"

	"github.com/hansjlachmann/openerp/backend/business-logic/projectjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// ProjectJnlPostPrint - Codeunit 282 "Project Jnl.-Post+Print": Post and Print (Shift+F11) on the project journal pages. The
// template must have a Posting Report ID; after asking, it posts the journal's batch in one
// transaction as Post (F11) does, then prints the template's posting report on the
// Project Register the posting wrote, without request page, to the report's Printer Selection output.
const ProjectJnlPostPrintID = 282
const ProjectJnlPostPrintName = "project-jnl-post-print"

func init() {
	Register(ProjectJnlPostPrintID, ProjectJnlPostPrintName, NewProjectJnlPostPrint)
}

type ProjectJnlPostPrint struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewProjectJnlPostPrint creates a new instance of the codeunit
func NewProjectJnlPostPrint(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &ProjectJnlPostPrint{db: db, company: company, dbType: dbType}
}

func (c *ProjectJnlPostPrint) ID() int                    { return ProjectJnlPostPrintID }
func (c *ProjectJnlPostPrint) Name() string               { return ProjectJnlPostPrintName }
func (c *ProjectJnlPostPrint) SourceTable() string        { return gtables.ProjectJournalLineTableName }
func (c *ProjectJnlPostPrint) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *ProjectJnlPostPrint) GetDB() database.Executor   { return c.db }
func (c *ProjectJnlPostPrint) GetCompany() string         { return c.company }
func (c *ProjectJnlPostPrint) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
// and prints its register
func (c *ProjectJnlPostPrint) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.ProjectJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 282 runs on a journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *ProjectJnlPostPrint) code(rec *gtables.ProjectJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = projectJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = projectJnlFilterValue(rec, "journal_batch_name")
	}
	jnlTemplate := gtables.NewProjectJournalTemplate(c)
	err := jnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = jnlTemplate.TestField("posting_report_id")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if jnlTemplate.Recurring && rec.GetFilter("posting_date") != "" {
		return fcodeunits.Result{}, apperrors.JnlRecurringDateFiltered(gtables.ProjectJournalLineTableName)
	}
	if !system.Confirm(ts.Message("JNL_POST_PRINT_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}

	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	jnlLine := gtables.NewProjectJournalLine(src)
	jnlLine.Copy(rec)
	posted, err := projectjnl.NewProjectJnlPostBatch(src).Run(jnlLine)
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
	register := gtables.NewProjectRegister(src)
	if register.Get(posted.RegisterNo) {
		register.SetRecFilter()
		printouts, err = printPostingReport(src, jnlTemplate.Posting_report_id, register)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return reports.WithPrintouts(jnlPostedMessage(posted.RegisterNo, posted.JournalBatchName, rec.Journal_batch_name), printouts), nil
}
