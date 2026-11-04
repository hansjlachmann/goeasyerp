package codeunits

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/session"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// What the journals' Post and Print codeunits (232, 242, 252, 272, 282) share: the posting
// report printed on the register the posting wrote, and the message about the posting.

// printPostingReport (REPORT.RUN(Template."Posting Report ID", FALSE, FALSE, Reg)): the journal
// template's posting report on the register (filtered to it, SETRECFILTER), without request
// page, to the report's Printer Selection output (the browser's print dialog by default); the
// user needs Execute on the report
func printPostingReport(src ftables.Source, reportID int, register ftables.Table) ([]reports.Result, error) {
	if !fcodeunits.CanExecute(session.ObjectReport, reportID) {
		return nil, apperrors.NoPermissionReport(reportID, reports.Caption(reportID, fcodeunits.CurrentLanguage()))
	}
	result, err := reports.Execute(reportID, src.GetDB(), src.GetCompany(), src.GetDBType(), reports.RunOptions{
		Record: register,
		Output: reports.PrinterOutput(src, reportID, reports.OutputPrint),
	})
	if err != nil {
		return nil, err
	}
	if result.Cancelled {
		return nil, nil
	}
	return []reports.Result{result}, nil
}

// jnlPostedMessage: there was nothing to post (no register), the lines were posted, or posted
// and the journal moved on to another batch (a batch with a number series)
func jnlPostedMessage(registerNo int, postedBatchName, batchName types.Code) fcodeunits.Result {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	switch {
	case registerNo == 0:
		return system.Message(ts.Message("GEN_JNL_NOTHING_TO_POST", lang))
	case postedBatchName == batchName:
		return system.Message(ts.Message("GEN_JNL_POSTED", lang))
	default:
		return system.Message(ts.Message("GEN_JNL_POSTED_NEW_BATCH", lang), postedBatchName)
	}
}
