package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fareclass"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// FAReclassJnlTransfer - Codeunit 5644 "FA Reclass. Jnl.-Transfer": the FA Reclass. Journal
// page's Reclassify (F11). After asking, it turns the journal's batch into FA/general journal
// lines (FA Reclass. Transfer Batch) in one transaction and tells the user the outcome.
const FAReclassJnlTransferID = 5644
const FAReclassJnlTransferName = "fa-reclass-jnl-transfer"

// FAReclassJnlBTransfer - Codeunit 5645 "FA Reclass. Jnl.-B.Transfer": Reclassify on the FA
// Reclass. Journal Batches page — every batch in the list, each in its own transaction; the
// message says whether all could be reclassified.
const FAReclassJnlBTransferID = 5645
const FAReclassJnlBTransferName = "fa-reclass-jnl-b-transfer"

func init() {
	Register(FAReclassJnlTransferID, FAReclassJnlTransferName, NewFAReclassJnlTransfer)
	Register(FAReclassJnlBTransferID, FAReclassJnlBTransferName, NewFAReclassJnlBTransfer)
}

type FAReclassJnlTransfer struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewFAReclassJnlTransfer creates a new instance of the codeunit
func NewFAReclassJnlTransfer(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &FAReclassJnlTransfer{db: db, company: company, dbType: dbType}
}

func (c *FAReclassJnlTransfer) ID() int                    { return FAReclassJnlTransferID }
func (c *FAReclassJnlTransfer) Name() string               { return FAReclassJnlTransferName }
func (c *FAReclassJnlTransfer) SourceTable() string        { return gtables.FAReclassJournalLineTableName }
func (c *FAReclassJnlTransfer) UsesProgress() bool         { return true } // it asks (Confirm) and shows the progress
func (c *FAReclassJnlTransfer) GetDB() database.Executor   { return c.db }
func (c *FAReclassJnlTransfer) GetCompany() string         { return c.company }
func (c *FAReclassJnlTransfer) GetDBType() database.DBType { return c.dbType }

// Run reclassifies the batch of the journal line (the page's current line, with its filters)
func (c *FAReclassJnlTransfer) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.FAReclassJournalLine)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 5644 runs on an FA reclass. journal line")
	}
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if rec.Journal_template_name == "" {
		rec.Journal_template_name = faReclassJnlFilterValue(rec, "journal_template_name")
	}
	if rec.Journal_batch_name == "" {
		rec.Journal_batch_name = faReclassJnlFilterValue(rec, "journal_batch_name")
	}
	faReclassJnlTemplate := gtables.NewFAReclassJournalTemplate(c)
	err := faReclassJnlTemplate.GetOrError(rec.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !system.Confirm(ts.Message("FA_RECLASS_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}
	jnlBatchName2 := rec.Journal_batch_name
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	result, err := fareclass.TransferBatch(ftables.DBSource(tx, c.company, c.dbType), rec)
	if err != nil {
		_ = ftables.Rollback(tx)
		return fcodeunits.Result{}, err
	}
	err = ftables.Commit(tx)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	switch {
	case !result.Done:
		return system.Message(ts.Message("FA_RECLASS_NOTHING", lang)), nil
	case jnlBatchName2 == result.JournalBatchName:
		return system.Message(ts.Message("FA_RECLASS_DONE", lang)), nil
	default:
		return system.Message(ts.Message("FA_RECLASS_DONE_NEW_BATCH", lang), result.JournalBatchName), nil
	}
}

// faReclassJnlFilterValue is the exact value an FA reclass. journal line is filtered on (filter
// group 2, the page's filters)
func faReclassJnlFilterValue(rec *gtables.FAReclassJournalLine, field string) types.Code {
	rec.FilterGroup(2)
	value := rec.GetFilter(field)
	rec.FilterGroup(0)
	return types.Code(value)
}

type FAReclassJnlBTransfer struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewFAReclassJnlBTransfer creates a new instance of the codeunit
func NewFAReclassJnlBTransfer(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &FAReclassJnlBTransfer{db: db, company: company, dbType: dbType}
}

func (c *FAReclassJnlBTransfer) ID() int                    { return FAReclassJnlBTransferID }
func (c *FAReclassJnlBTransfer) Name() string               { return FAReclassJnlBTransferName }
func (c *FAReclassJnlBTransfer) SourceTable() string        { return gtables.FAReclassJournalBatchTableName }
func (c *FAReclassJnlBTransfer) UsesProgress() bool         { return true }
func (c *FAReclassJnlBTransfer) GetDB() database.Executor   { return c.db }
func (c *FAReclassJnlBTransfer) GetCompany() string         { return c.company }
func (c *FAReclassJnlBTransfer) GetDBType() database.DBType { return c.dbType }

// Run reclassifies every batch in the list's filters
func (c *FAReclassJnlBTransfer) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.FAReclassJournalBatch)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 5645 runs on an FA reclass. journal batch")
	}
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	faReclassJnlBatch := rec
	if faReclassJnlBatch.Journal_template_name == "" {
		faReclassJnlBatch.FilterGroup(2)
		faReclassJnlBatch.Journal_template_name = types.Code(faReclassJnlBatch.GetFilter("journal_template_name"))
		faReclassJnlBatch.FilterGroup(0)
	}
	faReclassJnlTemplate := gtables.NewFAReclassJournalTemplate(c)
	err := faReclassJnlTemplate.GetOrError(faReclassJnlBatch.Journal_template_name)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if !system.Confirm(ts.Message("FA_RECLASS_BATCHES_CONFIRM", lang)) {
		return fcodeunits.Result{Success: true}, nil
	}
	journalWithErrors := false
	for found := faReclassJnlBatch.FindSetBuffered(); found; found = faReclassJnlBatch.Next() {
		faReclassJnlLine := gtables.NewFAReclassJournalLine(c)
		faReclassJnlLine.Journal_template_name = faReclassJnlBatch.Journal_template_name
		faReclassJnlLine.Journal_batch_name = faReclassJnlBatch.Name
		faReclassJnlLine.Line_no = 1
		if !c.transferBatch(faReclassJnlLine) {
			journalWithErrors = true
		}
	}
	if !journalWithErrors {
		result := system.Message(ts.Message("FA_RECLASS_BATCHES_DONE", lang))
		result.Refresh = true
		return result, nil
	}
	result := system.Message(ts.Message("FA_RECLASS_BATCHES_ERRORS", lang))
	result.Refresh = true
	return result, nil
}

// transferBatch: one batch in its own transaction (classic IF FAReclassTransferBatch.RUN)
func (c *FAReclassJnlBTransfer) transferBatch(faReclassJnlLine *gtables.FAReclassJournalLine) bool {
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return false
	}
	_, err = fareclass.TransferBatch(ftables.DBSource(tx, c.company, c.dbType), faReclassJnlLine)
	if err != nil {
		_ = ftables.Rollback(tx)
		return false
	}
	return ftables.Commit(tx) == nil
}
