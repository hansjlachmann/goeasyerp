package tables

import (
	"strconv"
	"strings"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/format"
	"github.com/hansjlachmann/openerp/backend/foundation/functions"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// projectLedgerEntryLogic is the business logic of table 169 Project Ledger Entry (written by
// posting; the user sets only Applies-to ID).
type projectLedgerEntryLogic struct{}

// ProjectLedgerEntriesPageID is page 92 Project Ledger Entries, AppliedProjectEntriesPageID
// page 229 Applied Project Entries
const (
	ProjectLedgerEntriesPageID  = 92
	AppliedProjectEntriesPageID = 229
)

// The page parameters of the entries pages opened from a line's Applies-to ID (classic
// SetJobJnlLine / SetSalesLine: CallingLineType and the line): its Applies-to ID, entry type
// (usage, sale), total price and the heading of the reconciliation (Document … / Invoice …)
var callingLineParams = []string{"applies_to_id", "line_entry_type", "line_total_price", "line_heading"}

func init() {
	gtables.RegisterProjectLedgerEntryLogic(projectLedgerEntryLogic{})
	// Set Applies-to ID on page 92 opened from a line's Applies-to ID (codeunit 216)
	functions.Register("table", gtables.ProjectLedgerEntryTableName, "SetApplId", functions.For(ProjectLedgEntrySetApplId))
	// Applied Entries (page 229) and Reconcile (page 365) on pages 92 and 229
	functions.Register("table", gtables.ProjectLedgerEntryTableName, "AppliedEntries", functions.For(ProjectLedgEntryAppliedEntries), functions.WithDialog())
	functions.Register("table", gtables.ProjectLedgerEntryTableName, "Reconcile", functions.For(ProjectLedgEntryReconcile))
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
		entryType := "usage"
		if rec.Entry_type == gtables.ProjectJournalLine_Entry_type.Sale {
			entryType = "sale"
		}
		heading := system.StrSubstNo(i18n.GetInstance().Message("PROJECT_RECONCILE_DOCUMENT", fcodeunits.CurrentLanguage()), rec.Document_no)
		selectProjectEntry(rec, rec.Project_no, rec.Applies_to_id, !rec.Total_price.IsZero(), positive, entryType, rec.Total_price, heading)
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
		entryType := "sale"
		if rec.Document_type == gtables.SalesLine_Document_type.CreditMemo {
			entryType = "usage"
		}
		language := fcodeunits.CurrentLanguage()
		heading := rec.GetOptionCaption("document_type", i18n.Key(rec.GetOptionFields()["document_type"][rec.Document_type]), language) + " " + rec.Document_no.String()
		selectProjectEntry(rec, rec.Project_no, rec.Project_applies_to_id, !amount.IsZero(), positive, entryType, amount, heading)
		return false, nil
	}), functions.WithAssistEdit())
}

// selectProjectEntry (SelectJobEntry for Applies-to ID): the project's open entries — of the
// sign the line applies to — in page 92, where Set Applies-to ID marks them with the line's
// Applies-to ID and Reconcile compares them with the line (the calling line's page parameters)
func selectProjectEntry(src ftables.Source, projectNo, appliesToID types.Code, bySign, positive bool, entryType string, totalPrice types.Decimal, heading string) {
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
	page.SetParam("line_entry_type", entryType)
	page.SetParam("line_total_price", totalPrice.String())
	page.SetParam("line_heading", heading)
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

// ProjectLedgEntryAppliedEntries (Applied Entries, form 229 OnOpenForm): the entries applied to
// an entry — the one that closed it and those it closed — in page 229, which keeps the calling
// line's parameters
func ProjectLedgEntryAppliedEntries(rec *gtables.ProjectLedgerEntry) (fcodeunits.Result, error) {
	applied, err := AppliedProjectEntries(rec)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	page := system.NewPage(AppliedProjectEntriesPageID)
	page.SetTableView(applied)
	for _, name := range callingLineParams {
		page.SetParam(name, system.PageParam(name))
	}
	page.RunModal()
	return fcodeunits.Result{Success: true, Refresh: true}, nil
}

// AppliedProjectEntries: the entries applied to an entry, as a record filtered on them (form
// 229 OnOpenForm: the entry that closed it and those it closed, of its project)
func AppliedProjectEntries(rec *gtables.ProjectLedgerEntry) (*gtables.ProjectLedgerEntry, error) {
	originalEntry := gtables.NewProjectLedgerEntry(rec)
	err := originalEntry.GetOrError(rec.Entry_no)
	if err != nil {
		return nil, err
	}
	var entryNos []string
	if originalEntry.Closed_by_entry_no != 0 {
		entryNos = append(entryNos, strconv.Itoa(originalEntry.Closed_by_entry_no))
	}
	projectLedgEntry := gtables.NewProjectLedgerEntry(rec)
	projectLedgEntry.SetCurrentKey("closed_by_entry_no")
	projectLedgEntry.SetRange("closed_by_entry_no", originalEntry.Entry_no)
	for found := projectLedgEntry.FindSetBuffered(); found; found = projectLedgEntry.Next() {
		entryNos = append(entryNos, strconv.Itoa(projectLedgEntry.Entry_no))
	}
	if len(entryNos) == 0 {
		entryNos = []string{"0"} // none (MARKEDONLY without marks)
	}
	applied := gtables.NewProjectLedgerEntry(rec)
	applied.SetFilter("entry_no", strings.Join(entryNos, "|"))
	applied.SetRange("project_no", originalEntry.Project_no)
	return applied, nil
}

// ProjectLedgEntryReconcile (Reconcile, form 365 Job Reconciliation): the calling line's amount,
// the remaining amount of the entries with its Applies-to ID and the balance — only on the
// entries pages opened from a project journal line or a sales line
func ProjectLedgEntryReconcile(rec *gtables.ProjectLedgerEntry) (fcodeunits.Result, error) {
	language := fcodeunits.CurrentLanguage()
	message := func(key string) string { return i18n.GetInstance().Message(key, language) }
	lineEntryType := system.PageParam("line_entry_type")
	lineApplyToID := types.Code(system.PageParam("applies_to_id"))
	if lineEntryType == "" || lineApplyToID == "" {
		return fcodeunits.Result{}, system.Error(message("PROJECT_RECONCILE_FROM_LINE_ONLY"))
	}
	lineTotalPrice, _ := types.NewDecimalFromString(system.PageParam("line_total_price"))
	usage := lineEntryType == "usage"
	entryMustBePositive := lineTotalPrice.IsPositive()
	if usage {
		entryMustBePositive = lineTotalPrice.IsNegative()
	}
	projectLedgEntry := gtables.NewProjectLedgerEntry(rec)
	projectLedgEntry.SetCurrentKey("applies_to_id")
	projectLedgEntry.SetRange("applies_to_id", lineApplyToID)
	var appliedAmount types.Decimal
	for found := projectLedgEntry.FindSetBuffered(); found; found = projectLedgEntry.Next() {
		err := projectLedgEntry.TestField("positive", entryMustBePositive)
		if err == nil {
			err = projectLedgEntry.TestField("open", true)
		}
		if err != nil {
			return fcodeunits.Result{}, err
		}
		appliedAmount = appliedAmount.Sub(projectLedgEntry.Remaining_amount)
	}
	if !usage {
		appliedAmount = appliedAmount.Neg()
	}
	amount := func(d types.Decimal) string { return format.Decimal(d, "2:2", language) }
	return system.Message(message("PROJECT_RECONCILIATION"), system.PageParam("line_heading"),
		amount(lineTotalPrice), amount(appliedAmount), amount(lineTotalPrice.Sub(appliedAmount))), nil
}
