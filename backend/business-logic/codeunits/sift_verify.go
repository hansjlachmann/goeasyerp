package codeunits

import (
	"errors"
	"fmt"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/sift"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"sort"
	"strings"
)

// SIFTVerify - Codeunit 902: Verify SIFT
//
// Run from the Job Queue (Run action or the scheduler) in a company. Compares every SIFT
// totals table of the company with a fresh sum over its entries ("verify SIFT").
// Parameter VERIFY (default) only reports; REPAIR rebuilds the keys whose totals differ.
// Unrepaired differences fail the run, so a scheduled job is set to Error and can notify
// by e-mail. Every run is logged as a Job Queue Entry.
const SIFTVerifyID = 902
const SIFTVerifyName = "verify-sift"

func init() {
	Register(SIFTVerifyID, SIFTVerifyName, NewSIFTVerify)
}

type SIFTVerify struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewSIFTVerify creates a new instance of the codeunit
func NewSIFTVerify(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &SIFTVerify{db: db, company: company, dbType: dbType}
}

// ID returns the codeunit ID
func (c *SIFTVerify) ID() int { return SIFTVerifyID }

// Name returns the codeunit name
func (c *SIFTVerify) Name() string { return SIFTVerifyName }

// SourceTable returns the table this codeunit operates on
func (c *SIFTVerify) SourceTable() string { return "Job_Queue" }

// UsesProgress returns true - a manual run shows a progress dialog
func (c *SIFTVerify) UsesProgress() bool { return true }

// siftVerifier is implemented by generated tables
type siftVerifier interface {
	VerifySIFT(repair bool) ([]sift.VerifyResult, error)
}

// Run checks (and with REPAIR rebuilds) the company's SIFT totals
func (c *SIFTVerify) Run(record interface{}) (fcodeunits.Result, error) {
	jobQueue, ok := record.(*gtables.JobQueue)
	if !ok {
		return fcodeunits.Result{}, fmt.Errorf("expected *JobQueue record, got %T", record)
	}
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	start := types.Now()

	fail := func(msg string) (fcodeunits.Result, error) {
		err := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, msg, start)
		if err != nil {
			msg += " (" + err.Error() + ")"
		}
		return fcodeunits.Result{}, errors.New(msg)
	}

	var repair bool
	switch parameter := strings.ToUpper(strings.TrimSpace(jobQueue.Parameter.String())); parameter {
	case "", "VERIFY":
	case "REPAIR":
		repair = true
	default:
		return fail(ts.MessageWithParams("SIFT_VERIFY_BAD_PARAMETER", lang, jobQueue.Parameter.String()))
	}

	results, err := c.verify(repair, ts, lang)
	if err != nil {
		return fail(err.Error())
	}

	var differing, rebuilt []string
	for _, r := range results {
		name := fmt.Sprintf("%s.%s (%d)", r.Table, r.Key, r.Differences)
		if r.Rebuilt {
			rebuilt = append(rebuilt, name)
		} else if r.Differences > 0 {
			differing = append(differing, name)
		}
	}
	if len(differing) > 0 {
		return fail(ts.MessageWithParams("SIFT_VERIFY_DIFFERENCES", lang, fmt.Sprint(len(differing)), strings.Join(differing, ", ")))
	}

	msg := ts.MessageWithParams("SIFT_VERIFY_OK", lang, fmt.Sprint(len(results)))
	if len(rebuilt) > 0 {
		msg = ts.MessageWithParams("SIFT_VERIFY_REPAIRED", lang, fmt.Sprint(len(rebuilt)), strings.Join(rebuilt, ", "))
	}
	err = CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Success, "", start)
	if err != nil {
		return fcodeunits.Message(err.Error()), nil
	}
	return fcodeunits.Message(msg), nil
}

// verify runs VerifySIFT on every registered table, reporting progress per table
func (c *SIFTVerify) verify(repair bool, ts *i18n.TranslationService, lang string) ([]sift.VerifyResult, error) {
	names := gtables.ListTableNames()
	sort.Strings(names)
	dialog := fcodeunits.GetCurrentDialog() // nil in a scheduled run

	var results []sift.VerifyResult
	for i, name := range names {
		factory, ok := gtables.GetTableFactory(name)
		if !ok {
			continue
		}
		table := factory()
		table.InitWithDBType(c.db, c.company, c.dbType)
		verifier, ok := table.(siftVerifier)
		if !ok {
			continue
		}
		if dialog != nil {
			dialog.UpdateWithMessage(1, i*100/len(names), ts.MessageWithParams("SIFT_VERIFY_CHECKING", lang, name))
		}
		r, err := verifier.VerifySIFT(repair)
		if err != nil {
			return results, fmt.Errorf("%s: %w", name, err)
		}
		results = append(results, r...)
	}
	return results, nil
}
