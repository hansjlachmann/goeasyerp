package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/insurancejnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// InsuranceJnlPostPrint - Codeunit 5672 "Insurance Jnl.-Post+Print": Post and Print (Shift+F11) on the insurance journal page. The
// template must have a Posting Report ID; after asking, it posts the journal's batch in one
// transaction as Post (F11) does, then prints the template's posting report on the
// Insurance Register the posting wrote, without request page, to the report's Printer Selection output.
const InsuranceJnlPostPrintID = 5672
const InsuranceJnlPostPrintName = "insurance-jnl-post-print"

func init() {
	Register(InsuranceJnlPostPrintID, InsuranceJnlPostPrintName, NewInsuranceJnlPostPrint)
}

type InsuranceJnlPostPrint struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewInsuranceJnlPostPrint creates a new instance of the codeunit
func NewInsuranceJnlPostPrint(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &InsuranceJnlPostPrint{db: db, company: company, dbType: dbType}
}

func (c *InsuranceJnlPostPrint) ID() int                    { return InsuranceJnlPostPrintID }
func (c *InsuranceJnlPostPrint) Name() string               { return InsuranceJnlPostPrintName }
func (c *InsuranceJnlPostPrint) SourceTable() string        { return gtables.InsuranceJournalLineTableName }
func (c *InsuranceJnlPostPrint) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *InsuranceJnlPostPrint) GetDB() database.Executor   { return c.db }
func (c *InsuranceJnlPostPrint) GetCompany() string         { return c.company }
func (c *InsuranceJnlPostPrint) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
// and prints its register
func (c *InsuranceJnlPostPrint) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.InsuranceJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 5672 runs on an insurance journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *InsuranceJnlPostPrint) code(rec *gtables.InsuranceJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = insuranceJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = insuranceJnlFilterValue(rec, "journal_batch_name")
	}
	jnlTemplate := gtables.NewInsuranceJournalTemplate(c)
	err := jnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = jnlTemplate.TestField("posting_report_id")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !system.Confirm(ts.Message("JNL_POST_PRINT_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}

	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	src := ftables.DBSource(tx, c.company, c.dbType)
	posted, err := insurancejnl.PostBatch(src, rec)
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
	register := gtables.NewInsuranceRegister(src)
	if register.Get(posted.RegisterNo) {
		register.SetRecFilter()
		printouts, err = printPostingReport(src, jnlTemplate.Posting_report_id, register)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return reports.WithPrintouts(jnlPostedMessage(posted.RegisterNo, posted.JournalBatchName, rec.Journal_batch_name), printouts), nil
}
