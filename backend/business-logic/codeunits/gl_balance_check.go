package codeunits

import (
	"strconv"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/glcheck"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// GLBalanceCheck - Codeunit 904: G/L Balance Check
//
// Checks the whole general ledger of the company (glcheck): every posting date and transaction
// of the G/L entries adds up to 0, no entry numbers are missing, the G/L registers cover the
// entries once each. Run from the menu (General Ledger → Activities) or the Job Queue (e.g.
// nightly): problems fail the run with the findings as the error's details (Show Entries), so
// a scheduled job goes to Error and can notify by e-mail.
const GLBalanceCheckID = 904
const GLBalanceCheckName = "gl-balance-check"

// glCheckMaxRows: the rows shown per section of the findings
const glCheckMaxRows = 1000

func init() {
	Register(GLBalanceCheckID, GLBalanceCheckName, NewGLBalanceCheck)
}

type GLBalanceCheck struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewGLBalanceCheck creates a new instance of the codeunit
func NewGLBalanceCheck(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &GLBalanceCheck{db: db, company: company, dbType: dbType}
}

func (c *GLBalanceCheck) ID() int             { return GLBalanceCheckID }
func (c *GLBalanceCheck) Name() string        { return GLBalanceCheckName }
func (c *GLBalanceCheck) SourceTable() string { return gtables.GLEntryTableName }
func (c *GLBalanceCheck) UsesProgress() bool  { return true } // a manual run shows its progress

// Run checks the G/L; run by the Job Queue (a Job Queue record) it logs the run
func (c *GLBalanceCheck) Run(record interface{}) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	start := types.Now()
	jobQueue, fromJobQueue := record.(*gtables.JobQueue)

	dialog := fcodeunits.GetCurrentDialog() // nil in a scheduled run
	var progress func(percent int)
	if dialog != nil {
		progress = func(percent int) {
			dialog.UpdateWithMessage(1, percent, ts.Message("GL_CHECK_CHECKING", lang))
		}
	}
	result := glcheck.Check(ftables.DBSource(c.db, c.company, c.dbType), progress)

	if !result.Balanced() {
		err := c.findings(result, ts, lang)
		if fromJobQueue {
			logErr := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, err.Message(lang), start)
			if logErr != nil {
				return fcodeunits.Result{}, logErr
			}
		}
		return fcodeunits.Result{}, err
	}
	if fromJobQueue {
		err := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Success, "", start)
		if err != nil {
			return fcodeunits.Result{}, err
		}
	}
	return fcodeunits.Message(ts.MessageWithParams("GL_CHECK_OK", lang, strconv.Itoa(result.Entries), strconv.Itoa(result.Registers))), nil
}

// findings is the error naming what the check found, with the findings as its details
func (c *GLBalanceCheck) findings(result *glcheck.Result, ts *i18n.TranslationService, lang string) *apperrors.AppError {
	entryCaption := func(field string) string {
		return ts.FieldCaption(gtables.GLEntryTableName, field, lang)
	}
	registerCaption := func(field string) string {
		return ts.FieldCaption(gtables.GLRegisterTableName, field, lang)
	}
	amount := func(d types.Decimal) string {
		return format.Decimal(d, "2:2", lang)
	}
	details := &apperrors.Details{Title: ts.Message("GL_CHECK_TITLE", lang)}
	add := func(section apperrors.DetailsSection) {
		if len(section.Rows) > glCheckMaxRows {
			section.Rows = section.Rows[:glCheckMaxRows]
		}
		if len(section.Rows) > 0 {
			details.Sections = append(details.Sections, section)
		}
	}

	dates := apperrors.DetailsSection{
		Caption: ts.Message("GL_CHECK_DATES", lang),
		Columns: []apperrors.DetailsColumn{{Caption: entryCaption("posting_date")}, {Caption: entryCaption("amount"), Numeric: true},
			{Caption: entryCaption("additional_currency_amount"), Numeric: true}},
	}
	for _, date := range result.Dates {
		dates.Rows = append(dates.Rows, []string{format.Date(date.PostingDate, lang), amount(date.Amount), amount(date.AddCurrAmount)})
	}
	add(dates)

	transactions := apperrors.DetailsSection{
		Caption: ts.Message("GL_CHECK_TRANSACTIONS", lang),
		Columns: []apperrors.DetailsColumn{
			{Caption: entryCaption("transaction_no"), Numeric: true},
			{Caption: entryCaption("posting_date")},
			{Caption: entryCaption("document_no")},
			{Caption: entryCaption("amount"), Numeric: true},
			{Caption: entryCaption("additional_currency_amount"), Numeric: true},
		},
	}
	for _, transaction := range result.Transactions {
		transactions.Rows = append(transactions.Rows, []string{strconv.Itoa(transaction.TransactionNo),
			format.Date(transaction.PostingDate, lang), transaction.DocumentNo.String(), amount(transaction.Amount), amount(transaction.AddCurrAmount)})
	}
	add(transactions)

	gaps := apperrors.DetailsSection{
		Caption: ts.Message("GL_CHECK_GAPS", lang),
		Columns: []apperrors.DetailsColumn{{Caption: registerCaption("from_entry_no"), Numeric: true}, {Caption: registerCaption("to_entry_no"), Numeric: true}},
	}
	for _, gap := range result.Gaps {
		gaps.Rows = append(gaps.Rows, []string{strconv.Itoa(gap.From), strconv.Itoa(gap.To)})
	}
	add(gaps)

	registers := apperrors.DetailsSection{
		Caption: ts.Message("GL_CHECK_REGISTERS", lang),
		Columns: []apperrors.DetailsColumn{
			{Caption: registerCaption("no"), Numeric: true},
			{Caption: registerCaption("from_entry_no"), Numeric: true},
			{Caption: registerCaption("to_entry_no"), Numeric: true},
			{Caption: ts.Message("GL_CHECK_PROBLEM", lang)},
		},
	}
	for _, problem := range result.RegisterList {
		registerNo := ""
		if problem.RegisterNo != 0 {
			registerNo = strconv.Itoa(problem.RegisterNo)
		}
		registers.Rows = append(registers.Rows, []string{registerNo, strconv.Itoa(problem.From), strconv.Itoa(problem.To), registerProblem(problem, ts, lang)})
	}
	add(registers)

	return apperrors.GLCheckFailed(strconv.Itoa(len(result.Dates)), strconv.Itoa(len(result.Transactions)),
		strconv.Itoa(len(result.Gaps)), strconv.Itoa(len(result.RegisterList))).WithDetails(details)
}

// registerProblem: what is wrong with a register's range, in the language
func registerProblem(problem glcheck.RegisterProblem, ts *i18n.TranslationService, lang string) string {
	switch problem.Problem {
	case glcheck.Overlaps:
		return ts.MessageWithParams("GL_CHECK_OVERLAPS", lang, strconv.Itoa(problem.OtherNo))
	case glcheck.Reversed:
		return ts.Message("GL_CHECK_REVERSED", lang)
	case glcheck.EntriesMissing:
		return ts.Message("GL_CHECK_ENTRIES_MISSING", lang)
	case glcheck.NotRegistered:
		return ts.Message("GL_CHECK_NOT_REGISTERED", lang)
	}
	return ""
}
