package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/projectjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// ProjectJnlPost - Codeunit 281 "Project Jnl.-Post": the project journal page's Post (F11). After asking,
// it posts the journal's batch (Project Jnl.-Post Batch) in one transaction — any error rolls the
// whole posting back — and tells the user the outcome.
const ProjectJnlPostID = 281
const ProjectJnlPostName = "project-jnl-post"

func init() {
	Register(ProjectJnlPostID, ProjectJnlPostName, NewProjectJnlPost)
}

type ProjectJnlPost struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewProjectJnlPost creates a new instance of the codeunit
func NewProjectJnlPost(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &ProjectJnlPost{db: db, company: company, dbType: dbType}
}

func (c *ProjectJnlPost) ID() int                    { return ProjectJnlPostID }
func (c *ProjectJnlPost) Name() string               { return ProjectJnlPostName }
func (c *ProjectJnlPost) SourceTable() string        { return gtables.ProjectJournalLineTableName }
func (c *ProjectJnlPost) UsesProgress() bool         { return true } // it asks (Confirm) and shows the posting's progress
func (c *ProjectJnlPost) GetDB() database.Executor   { return c.db }
func (c *ProjectJnlPost) GetCompany() string         { return c.company }
func (c *ProjectJnlPost) GetDBType() database.DBType { return c.dbType }

// Run posts the batch of the journal line (the page's current line, with the page's filters)
func (c *ProjectJnlPost) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.ProjectJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 281 runs on a project journal line")
	}
	result, err := c.code(rec)
	return result, err
}

func (c *ProjectJnlPost) code(rec *gtables.ProjectJournalLine) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	// The journal: the line's template and batch, or the page's filters when it has no lines
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = projectJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = projectJnlFilterValue(rec, "journal_batch_name")
	}
	projectJnlTemplate := gtables.NewProjectJournalTemplate(c)
	err := projectJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = projectJnlTemplate.TestField("force_posting_report", false)
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
	projectJnlLine := gtables.NewProjectJournalLine(src)
	projectJnlLine.Copy(rec)
	posted, err := projectjnl.NewProjectJnlPostBatch(src).Run(projectJnlLine)
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

// projectJnlFilterValue is the exact value a project journal line is filtered on (filter group 2,
// the page's filters)
func projectJnlFilterValue(rec *gtables.ProjectJournalLine, field string) types.Code {
	rec.FilterGroup(2)
	value := rec.GetFilter(field)
	rec.FilterGroup(0)
	return types.Code(value)
}
