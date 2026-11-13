package codeunits

import (
	"errors"
	"fmt"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/purchposting"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reminderissue"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/bomjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/demodata"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/projectjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// DemoDataCreate - Codeunit 901: Create Demo Data
//
// Run from the Job Queue (Run action or the scheduler) in the company to fill. The
// job's parameter selects the size: SMALL (default), LARGE or HEAVY, and VAT makes it a VAT
// company instead of a sales tax one ("VAT", "LARGE VAT"). A company that already
// has customers is emptied and filled again (but its job queue and user setup) — run from
// the page, after a question; nothing changes if any part fails. Every run is logged as a
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

	// The demo replaces the company's data: asked when run from the page (a scheduled run
	// replaces it)
	cust := gtables.NewCustomer(ftables.DBSource(c.db, c.company, c.dbType))
	if !cust.IsEmpty() && system.GuiAllowed() {
		if !system.Confirm(ts.Message("DEMO_DATA_REPLACE_CONFIRM", lang), c.company) {
			return fcodeunits.Result{}, nil
		}
	}

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
	size, vat, err := demodata.ParseParameter(jobQueue.Parameter.String())
	if err != nil {
		return demodata.Counts{}, err
	}

	dialog := fcodeunits.GetCurrentDialog() // nil in a scheduled run
	phaseKeys := map[demodata.Phase]string{
		demodata.PhaseSetup:       "DEMO_DATA_SETUP",
		demodata.PhaseCustomers:   "DEMO_DATA_CUSTOMERS",
		demodata.PhaseEntries:     "DEMO_DATA_ENTRIES",
		demodata.PhaseLedgers:     "DEMO_DATA_LEDGERS",
		demodata.PhaseResources:   "DEMO_DATA_RESOURCES",
		demodata.PhaseItems:       "DEMO_DATA_ITEMS",
		demodata.PhaseAssemblies:  "DEMO_DATA_ASSEMBLIES",
		demodata.PhaseSales:       "DEMO_DATA_SALES",
		demodata.PhasePurchases:   "DEMO_DATA_PURCHASES",
		demodata.PhaseFixedAssets: "DEMO_DATA_FIXED_ASSETS",
		demodata.PhaseJournals:    "DEMO_DATA_JOURNALS",
	}
	progress := func(pct int, phase demodata.Phase) {
		if dialog != nil {
			dialog.UpdateWithMessage(1, pct, ts.Message(phaseKeys[phase], lang))
		}
	}

	return demodata.Create(c.db, c.company, c.dbType, demoDataOptions(size, vat, progress))
}

// demoDataOptions are the options of a full demo: every posting the demo goes through
func demoDataOptions(size demodata.Size, vat bool, progress func(int, demodata.Phase)) demodata.Options {
	return demodata.Options{Size: size, VAT: vat, Progress: progress, ItemJnlPostLine: openItemJnlPostLine, BOMJnlPostLine: openBOMJnlPostLine, SalesPost: openSalesPost, PurchPost: openPurchPost, GenJnlPostLine: openGenJnlPostLine, ProjectJnlPostLine: openProjectJnlPostLine, Journals: true,
		ReminderIssue: openReminderIssue, FinChrgMemoIssue: openFinChrgMemoIssue, AdjustExchRates: adjustExchRates}
}

// CreateDemoDataIfEmpty fills a company that has no customers with the demo data of codeunit
// 901 — the startup setting DEMO_DATA (its value is the job parameter: SMALL, LARGE VAT …), so a
// test or trial installation starts from known data. A company with customers is left as it
// is (false). Nothing is changed when it fails.
func CreateDemoDataIfEmpty(db database.Executor, company string, dbType database.DBType, parameter string) (bool, demodata.Counts, error) {
	cust := gtables.NewCustomer(ftables.DBSource(db, company, dbType))
	if !cust.IsEmpty() {
		return false, demodata.Counts{}, nil
	}
	size, vat, err := demodata.ParseParameter(parameter)
	if err != nil {
		return false, demodata.Counts{}, err
	}
	counts, err := demodata.Create(db, company, dbType, demoDataOptions(size, vat, nil))
	if err != nil {
		return false, counts, err
	}
	return true, counts, nil
}

// errorMessage translates the demodata errors users can act on.
func (c *DemoDataCreate) errorMessage(err error, ts *i18n.TranslationService, lang string) string {
	var sizeErr *demodata.SizeError
	var insertErr *demodata.InsertError
	switch {
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

// openProjectJnlPostLine opens the project posting (codeunit 202) for the demo project usage
func openProjectJnlPostLine(src ftables.Source) demodata.ProjectJnlPoster {
	return projectjnl.NewProjectJnlPostLine(src)
}

// openBOMJnlPostLine opens the BOM posting (codeunit 32) for the demo assemblies
func openBOMJnlPostLine(src ftables.Source) demodata.BOMJnlPoster {
	return bomjnl.NewBOMJnlPostLine(src)
}

// openSalesPost opens the sales posting (codeunit 80) for the demo sales documents
func openSalesPost(src ftables.Source) demodata.SalesPoster {
	return salesposting.NewSalesPost(src)
}

// openGenJnlPostLine opens the general journal posting (codeunit 12) for the demo fixed assets
func openGenJnlPostLine(src ftables.Source) demodata.GenJnlPoster {
	return genjnl.NewGenJnlPostLine(src)
}

// openPurchPost opens the purchase posting (codeunit 90) for the demo purchase documents
func openPurchPost(src ftables.Source) demodata.PurchPoster {
	return purchposting.NewPurchPost(src)
}

// openReminderIssue opens codeunit 393 Reminder-Issue for the demo data
func openReminderIssue(src ftables.Source) demodata.ReminderIssuer {
	return reminderissue.New(src)
}

// openFinChrgMemoIssue opens codeunit 395 FinChrgMemo-Issue for the demo data
func openFinChrgMemoIssue(src ftables.Source) demodata.FinChrgMemoIssuer {
	return reminderissue.NewFinChrgMemoIssue(src)
}

// adjustExchRates runs report 595 Adjust Exchange Rates for the demo data (no request page)
func adjustExchRates(src ftables.Source, postingDate types.Date, documentNo types.Code) error {
	_, err := reports.Execute(reports.AdjustExchangeRatesID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Options: map[string]string{"posting_date": postingDate.String(), "ending_date": postingDate.String(), "document_no": documentNo.String()}})
	return err
}
