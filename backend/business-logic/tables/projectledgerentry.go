package tables

import (
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// projectLedgerEntryLogic is the business logic of table 169 Project Ledger Entry (written by
// posting; the user sets only Applies-to ID).
type projectLedgerEntryLogic struct{}

// ProjectLedgerEntriesPageID is page 92 Project Ledger Entries
const ProjectLedgerEntriesPageID = 92

func init() {
	gtables.RegisterProjectLedgerEntryLogic(projectLedgerEntryLogic{})
	// Set Applies-to ID on page 92 opened from a line's Applies-to ID (codeunit 216)
	functions.Register("table", gtables.ProjectLedgerEntryTableName, "SetApplId", functions.For(ProjectLedgEntrySetApplId))
	// the lines' Applies-to ID lookup (SelectJobEntry)
	functions.Register("table", gtables.ProjectJournalLineTableName, "SelectProjectEntry", functions.ForAssistEdit(func(rec *gtables.ProjectJournalLine) (bool, error) {
		err := rec.TestField("applies_to_id")
		if err != nil {
			return false, err
		}
		positive := rec.Total_price.IsPositive()
		if rec.Entry_type == gtables.ProjectJournalLine_Entry_type.Usage {
			positive = rec.Total_price.IsNegative()
		}
		selectProjectEntry(rec, rec.Project_no, rec.Applies_to_id, !rec.Total_price.IsZero(), positive)
		return false, nil
	}), functions.WithAssistEdit())
	functions.Register("table", gtables.SalesLineTableName, "SelectProjectEntry", functions.ForAssistEdit(func(rec *gtables.SalesLine) (bool, error) {
		err := rec.TestField("project_applies_to_id")
		if err != nil {
			return false, err
		}
		amount := rec.Amount.Add(rec.Inv_discount_amount)
		positive := amount.IsPositive()
		if rec.Document_type == gtables.SalesLine_Document_type.CreditMemo {
			positive = amount.IsNegative()
		}
		selectProjectEntry(rec, rec.Project_no, rec.Project_applies_to_id, !amount.IsZero(), positive)
		return false, nil
	}), functions.WithAssistEdit())
}

// selectProjectEntry (SelectJobEntry for Applies-to ID): the project's open entries — of the
// sign the line applies to — in page 92, where Set Applies-to ID marks them with the line's
// Applies-to ID (the page parameter applies_to_id)
func selectProjectEntry(src ftables.Source, projectNo, appliesToID types.Code, bySign, positive bool) {
	projectLedgEntry := gtables.NewProjectLedgerEntry(src)
	projectLedgEntry.SetCurrentKey("project_no", "open", "positive", "posting_date")
	projectLedgEntry.SetRange("project_no", projectNo)
	projectLedgEntry.SetRange("open", true)
	if bySign {
		projectLedgEntry.SetRange("positive", positive)
	}
	page := system.NewPage(ProjectLedgerEntriesPageID)
	page.LookupMode(true)
	page.SetParam("applies_to_id", appliesToID.String())
	page.SetTableView(projectLedgEntry)
	page.RunModal()
}

// ProjectLedgEntrySetApplId (codeunit 216 Job Entry-Set Appl. ID SetApplId): an open entry gets
// the calling line's Applies-to ID (page parameter applies_to_id), or loses the one it has
func ProjectLedgEntrySetApplId(rec *gtables.ProjectLedgerEntry) (fcodeunits.Result, error) {
	projectLedgEntry := gtables.NewProjectLedgerEntry(rec)
	err := projectLedgEntry.GetOrError(rec.Entry_no)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	err = projectLedgEntry.TestField("open", true)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	if projectLedgEntry.Applies_to_id != "" {
		projectLedgEntry.Applies_to_id = ""
	} else {
		lineApplyToID := types.Code(system.PageParam("applies_to_id"))
		if lineApplyToID == "" {
			return fcodeunits.Result{}, projectLedgEntry.TestField("applies_to_id")
		}
		projectLedgEntry.Applies_to_id = lineApplyToID
	}
	err = projectLedgEntry.ModifyOrError(false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}
