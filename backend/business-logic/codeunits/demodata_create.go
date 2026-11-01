package codeunits

import (
	"errors"
	"fmt"
	"github.com/hansjlachmann/openerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/openerp/backend/business-logic/salesposting"

	"github.com/hansjlachmann/openerp/backend/business-logic/bomjnl"
	"github.com/hansjlachmann/openerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/openerp/backend/business-logic/itemjnl"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/database"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// DemoDataCreate - Codeunit 901: Create Demo Data
//
// Run from the Job Queue (Run action or the scheduler) in the company to fill. The
// job's parameter selects the size: SMALL (default), LARGE or HEAVY. The company must not
// have customers yet; nothing is written if any part fails. Every run is logged as a
// Job Queue Entry.
const DemoDataCreateID = 901
const DemoDataCreateName = "create-demo-data"

func init() {
	Register(DemoDataCreateID, DemoDataCreateName, NewDemoDataCreate)
}

type DemoDataCreate struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewDemoDataCreate creates a new instance of the codeunit
func NewDemoDataCreate(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &DemoDataCreate{db: db, company: company, dbType: dbType}
}

// ID returns the codeunit ID
func (c *DemoDataCreate) ID() int { return DemoDataCreateID }

// Name returns the codeunit name
func (c *DemoDataCreate) Name() string { return DemoDataCreateName }

// SourceTable returns the table this codeunit operates on
func (c *DemoDataCreate) SourceTable() string { return "Job_Queue" }

// UsesProgress returns true - a manual run shows a progress dialog
func (c *DemoDataCreate) UsesProgress() bool { return true }

// Run creates the demo data
func (c *DemoDataCreate) Run(record interface{}) (fcodeunits.Result, error) {
	jobQueue, ok := record.(*gtables.JobQueue)
	if !ok {
		return fcodeunits.Result{}, fmt.Errorf("expected *JobQueue record, got %T", record)
	}
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	start := types.Now()

	counts, err := c.create(jobQueue, ts, lang)
	if err != nil {
		msg := c.errorMessage(err, ts, lang)
		logErr := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, msg, start)
		if logErr != nil {
			msg += " (" + logErr.Error() + ")"
		}
		// An error fails the run: the scheduler sets the job to Error and the
		// progress dialog shows the message
		return fcodeunits.Result{}, errors.New(msg)
	}

	err = CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Success, "", start)
	if err != nil {
		return fcodeunits.Message(err.Error()), nil
	}
	return fcodeunits.Message(ts.MessageWithParams("DEMO_DATA_DONE", lang,
		fmt.Sprint(counts.Customers), fmt.Sprint(counts.LedgerEntries), fmt.Sprint(counts.Vendors), fmt.Sprint(counts.VendorLedgerEntries))), nil
}

func (c *DemoDataCreate) create(jobQueue *gtables.JobQueue, ts *i18n.TranslationService, lang string) (demodata.Counts, error) {
	size, err := demodata.ParseSize(jobQueue.Parameter.String())
	if err != nil {
		return demodata.Counts{}, err
	}

	dialog := fcodeunits.GetCurrentDialog() // nil in a scheduled run
	phaseKeys := map[demodata.Phase]string{
		demodata.PhaseSetup:      "DEMO_DATA_SETUP",
		demodata.PhaseCustomers:  "DEMO_DATA_CUSTOMERS",
		demodata.PhaseEntries:    "DEMO_DATA_ENTRIES",
		demodata.PhaseLedgers:    "DEMO_DATA_LEDGERS",
		demodata.PhaseResources:  "DEMO_DATA_RESOURCES",
		demodata.PhaseItems:      "DEMO_DATA_ITEMS",
		demodata.PhaseAssemblies: "DEMO_DATA_ASSEMBLIES",
		demodata.PhaseSales:      "DEMO_DATA_SALES",
		demodata.PhasePurchases:  "DEMO_DATA_PURCHASES",
	}
	progress := func(pct int, phase demodata.Phase) {
		if dialog != nil {
			dialog.UpdateWithMessage(1, pct, ts.Message(phaseKeys[phase], lang))
		}
	}

	return demodata.Create(c.db, c.company, c.dbType, demodata.Options{Size: size, Progress: progress, ItemJnlPostLine: openItemJnlPostLine, BOMJnlPostLine: openBOMJnlPostLine, SalesPost: openSalesPost, PurchPost: openPurchPost})
}

// errorMessage translates the demodata errors users can act on.
func (c *DemoDataCreate) errorMessage(err error, ts *i18n.TranslationService, lang string) string {
	var sizeErr *demodata.SizeError
	var insertErr *demodata.InsertError
	switch {
	case errors.Is(err, demodata.ErrCompanyNotEmpty):
		return ts.MessageWithParams("DEMO_DATA_NOT_EMPTY", lang, c.company)
	case errors.As(err, &sizeErr):
		return ts.MessageWithParams("DEMO_DATA_BAD_SIZE", lang, sizeErr.Parameter)
	case errors.As(err, &insertErr):
		return ts.MessageWithParams("DEMO_DATA_INSERT_FAILED", lang, insertErr.Table, insertErr.Key, insertErr.Err.Error())
	}
	return err.Error()
}

// openItemJnlPostLine opens the item posting (codeunit 22) for the demo items' postings
func openItemJnlPostLine(src ftables.Source) demodata.ItemJnlPoster {
	return itemjnl.NewItemJnlPostLine(src)
}

// openBOMJnlPostLine opens the BOM posting (codeunit 32) for the demo assemblies
func openBOMJnlPostLine(src ftables.Source) demodata.BOMJnlPoster {
	return bomjnl.NewBOMJnlPostLine(src)
}

// openSalesPost opens the sales posting (codeunit 80) for the demo sales documents
func openSalesPost(src ftables.Source) demodata.SalesPoster {
	return salesposting.NewSalesPost(src)
}

// openPurchPost opens the purchase posting (codeunit 90) for the demo purchase documents
func openPurchPost(src ftables.Source) demodata.PurchPoster {
	return purchposting.NewPurchPost(src)
}
