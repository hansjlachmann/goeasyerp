package reports

import (
	"errors"
	"strings"

	"github.com/hansjlachmann/openerp/backend/business-logic/reminder"
	"github.com/hansjlachmann/openerp/backend/business-logic/reminderissue"
	"github.com/hansjlachmann/openerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/openerp/backend/foundation/errors"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The reminder and finance charge memo reports (main menu Sales → Activities, the
// Reminders / Finance Charge Memos lists and cards):
//   - 188 Create Reminders / 191 Create Finance Charge Memos: for the customers in the filters
//     (with reminder / finance charge terms) the documents of their entries (Document Type … in
//     the entry filters);
//   - 189 Suggest Reminder Lines / 192 Suggest Fin. Charge Memo Lines: the lines of existing
//     documents again;
//   - 190 Issue Reminders / 193 Issue Finance Charge Memos: issued (codeunits 393/395), with the
//     posting date replaced on request;
//   - 187 Update Reminder Text / 186 Update Finance Charge Text: the texts again (187: at another
//     reminder level).
//
// Each customer or document is done in its own transaction (classic COMMIT and RUN per record):
// one that fails stays as it was and is named in the message. The work starts after the data
// item's loop (the keys are collected), so no result set is open while it writes.
const (
	UpdateFinanceChargeTextID  = 186
	UpdateReminderTextID       = 187
	CreateRemindersID          = 188
	SuggestReminderLinesID     = 189
	IssueRemindersID           = 190
	CreateFinanceChargeMemosID = 191
	SuggestFinChrgMemoLinesID  = 192
	IssueFinanceChargeMemosID  = 193
)

func init() {
	Register(CreateRemindersID, "create-reminders", func() Report { return &reminderBatch{id: CreateRemindersID} })
	Register(SuggestReminderLinesID, "suggest-reminder-lines", func() Report { return &reminderBatch{id: SuggestReminderLinesID} })
	Register(IssueRemindersID, "issue-reminders", func() Report { return &reminderBatch{id: IssueRemindersID} })
	Register(UpdateReminderTextID, "update-reminder-text", func() Report { return &reminderBatch{id: UpdateReminderTextID} })
	Register(CreateFinanceChargeMemosID, "create-finance-charge-memos", func() Report { return &reminderBatch{id: CreateFinanceChargeMemosID} })
	Register(SuggestFinChrgMemoLinesID, "suggest-fin-charge-memo-lines", func() Report { return &reminderBatch{id: SuggestFinChrgMemoLinesID} })
	Register(IssueFinanceChargeMemosID, "issue-finance-charge-memos", func() Report { return &reminderBatch{id: IssueFinanceChargeMemosID} })
	Register(UpdateFinanceChargeTextID, "update-finance-charge-text", func() Report { return &reminderBatch{id: UpdateFinanceChargeTextID} })
}

// reminderBatch is one of the reports; keys are the customers or documents the loop found
type reminderBatch struct {
	id            int
	custLedgEntry *gtables.CustomerLedgerEntry // the entry filters (the second data item)
	keys          []types.Code
}

func (r *reminderBatch) reminders() bool {
	return r.id == CreateRemindersID || r.id == SuggestReminderLinesID || r.id == IssueRemindersID || r.id == UpdateReminderTextID
}

func (r *reminderBatch) Definition() *Definition {
	var options []Option
	switch r.id {
	case CreateRemindersID, CreateFinanceChargeMemosID:
		options = []Option{{Name: "posting_date", Type: "date"}, {Name: "document_date", Type: "date"}}
		if r.id == CreateRemindersID {
			options = append(options, Option{Name: "only_overdue_entries", Type: "bool", Default: "false"})
		}
	case SuggestReminderLinesID:
		options = []Option{{Name: "only_overdue_entries", Type: "bool", Default: "false"}}
	case IssueRemindersID, IssueFinanceChargeMemosID:
		options = []Option{{Name: "replace_posting_date", Type: "bool", Default: "false"}, {Name: "posting_date", Type: "date"}}
	case UpdateReminderTextID:
		options = []Option{{Name: "reminder_level", Type: "number", Required: true}}
	}
	def := &Definition{OptionsCaption: "OPTIONS", Options: options, OnInitReport: r.onInitReport, OnPreReport: r.onPreReport, OnPostReport: r.onPostReport}
	entryFilters := &DataItem{
		Name: "CustLedgerEntry", Table: gtables.CustomerLedgerEntryTableName, Key: []string{"customer_no"},
		RequestFilterFields: []string{"document_type"},
		OnPreDataItem:       Trigger(r.entryFiltersOnPreDataItem),
	}
	header := gtables.ReminderHeaderTableName
	if !r.reminders() {
		header = gtables.FinanceChargeMemoHeaderTableName
	}
	switch r.id {
	case CreateRemindersID, CreateFinanceChargeMemosID:
		def.DataItems = []*DataItem{entryFilters, {
			Name: "Customer", Table: gtables.CustomerTableName, Key: []string{"no"}, RequestFilterFields: []string{"no"},
			OnPreDataItem:    Trigger(r.customerOnPreDataItem),
			OnAfterGetRecord: Trigger(func(cust *gtables.Customer, run *Run) error { r.keys = append(r.keys, cust.No); return nil }),
		}}
	case SuggestReminderLinesID, SuggestFinChrgMemoLinesID:
		def.ContextTable = header
		def.DataItems = []*DataItem{entryFilters, r.headerDataItem(header)}
	default:
		def.ContextTable = header
		def.DataItems = []*DataItem{r.headerDataItem(header)}
	}
	return def
}

// headerDataItem: the reminders or finance charge memos in the filters (run from a card: that
// document)
func (r *reminderBatch) headerDataItem(table string) *DataItem {
	if table == gtables.ReminderHeaderTableName {
		return &DataItem{Name: "ReminderHeader", Table: table, Key: []string{"no"}, RequestFilterFields: []string{"no"},
			OnAfterGetRecord: Trigger(func(h *gtables.ReminderHeader, run *Run) error { r.keys = append(r.keys, h.No); return nil })}
	}
	return &DataItem{Name: "FinanceChargeMemoHeader", Table: table, Key: []string{"no"}, RequestFilterFields: []string{"no"},
		OnAfterGetRecord: Trigger(func(h *gtables.FinanceChargeMemoHeader, run *Run) error { r.keys = append(r.keys, h.No); return nil })}
}

// onInitReport: the dates (OnOpenForm: the work date); run from a document, it is the filter
func (r *reminderBatch) onInitReport(run *Run) error {
	today := system.WorkDate().String()
	switch r.id {
	case CreateRemindersID, CreateFinanceChargeMemosID:
		run.SetOption("posting_date", today)
		run.SetOption("document_date", today)
	}
	switch h := run.Context().(type) {
	case *gtables.ReminderHeader:
		if h.No != "" {
			run.SetRequestFilter("ReminderHeader", "no", h.No.String())
		}
	case *gtables.FinanceChargeMemoHeader:
		if h.No != "" {
			run.SetRequestFilter("FinanceChargeMemoHeader", "no", h.No.String())
		}
	}
	return nil
}

func (r *reminderBatch) onPreReport(run *Run) error {
	switch r.id {
	case CreateRemindersID, CreateFinanceChargeMemosID:
		if run.OptionDate("document_date").IsZero() {
			return system.Error(run.Text("SPECIFY_DOCUMENT_DATE"))
		}
	case IssueRemindersID, IssueFinanceChargeMemosID:
		if run.OptionBool("replace_posting_date") && run.OptionDate("posting_date").IsZero() {
			return system.Error(run.Text("ENTER_POSTING_DATE"))
		}
	}
	return nil
}

// entryFiltersOnPreDataItem: the entry filters are kept for the documents' lines (the data item
// itself is not looped)
func (r *reminderBatch) entryFiltersOnPreDataItem(rec *gtables.CustomerLedgerEntry, run *Run) error {
	r.custLedgEntry = gtables.NewCustomerLedgerEntry(run)
	r.custLedgEntry.CopyFilters(rec)
	return Break()
}

// customerOnPreDataItem: only customers with reminder (finance charge) terms
func (r *reminderBatch) customerOnPreDataItem(cust *gtables.Customer, run *Run) error {
	cust.FilterGroup(2)
	if r.id == CreateRemindersID {
		cust.SetFilter("reminder_terms_code", "<>''")
	} else {
		cust.SetFilter("fin_charge_terms_code", "<>''")
	}
	cust.FilterGroup(0)
	return nil
}

// onPostReport: each customer or document in its own transaction; the outcome
func (r *reminderBatch) onPostReport(run *Run) error {
	var failed []string
	done := 0
	window := system.OpenWindow(run.Text("WINDOW"))
	defer window.Close()
	for i, key := range r.keys {
		window.Update(1, system.Round(types.NewDecimalFromInt(int64((i+1)*10000/len(r.keys))), types.NewDecimalFromInt(1)))
		tx, err := ftables.BeginTransaction(run.GetDB())
		if err != nil {
			return err
		}
		src := ftables.DBSource(tx, run.GetCompany(), run.GetDBType())
		err = r.one(run, src, key)
		if err == nil {
			err = ftables.Commit(tx)
		} else {
			_ = ftables.Rollback(tx)
		}
		if err != nil {
			failed = append(failed, key.String()+": "+errorText(err))
			continue
		}
		done++
	}
	if len(failed) > 0 {
		return system.Error(run.Text("NOT_ALL_DONE", done, joinLines(failed)))
	}
	run.Message(run.Text("DONE", done))
	return nil
}

// one: the work for a customer or document
func (r *reminderBatch) one(run *Run, src ftables.Source, key types.Code) error {
	postingDate := run.OptionDate("posting_date")
	switch r.id {
	case CreateRemindersID:
		cust := gtables.NewCustomer(src)
		err := cust.GetOrError(key)
		if err != nil {
			return err
		}
		req := gtables.NewReminderHeader(src)
		req.Posting_date = postingDate
		req.Document_date = run.OptionDate("document_date")
		m := reminder.New(src)
		m.Set(cust, r.entryFilters(src), req, run.OptionBool("only_overdue_entries"))
		return m.Run()
	case CreateFinanceChargeMemosID:
		cust := gtables.NewCustomer(src)
		err := cust.GetOrError(key)
		if err != nil {
			return err
		}
		req := gtables.NewFinanceChargeMemoHeader(src)
		req.Posting_date = postingDate
		req.Document_date = run.OptionDate("document_date")
		m := reminder.NewFinChrgMemoMake(src)
		m.Set(cust, r.entryFilters(src), req)
		return m.Run()
	case SuggestReminderLinesID, IssueRemindersID, UpdateReminderTextID:
		h := gtables.NewReminderHeader(src)
		err := h.GetOrError(key)
		if err != nil {
			return err
		}
		switch r.id {
		case SuggestReminderLinesID:
			m := reminder.New(src)
			m.SuggestLines(h, r.entryFilters(src), run.OptionBool("only_overdue_entries"))
			return m.Run()
		case IssueRemindersID:
			issue := reminderissue.New(src)
			issue.Set(h, run.OptionBool("replace_posting_date"), postingDate)
			return issue.Run()
		default:
			level := run.OptionInt("reminder_level")
			reminderLevel := gtables.NewReminderLevel(src)
			if reminderLevel.Get(map[string]interface{}{"reminder_terms_code": h.Reminder_terms_code, "no": level}) {
				h.Reminder_level = level
				err = h.ModifyOrError(false)
				if err == nil {
					err = tables.UpdateReminderLines(h)
				}
			}
			return err
		}
	default:
		h := gtables.NewFinanceChargeMemoHeader(src)
		err := h.GetOrError(key)
		if err != nil {
			return err
		}
		switch r.id {
		case SuggestFinChrgMemoLinesID:
			m := reminder.NewFinChrgMemoMake(src)
			m.SuggestLines(h, r.entryFilters(src))
			return m.Run()
		case IssueFinanceChargeMemosID:
			issue := reminderissue.NewFinChrgMemoIssue(src)
			issue.Set(h, run.OptionBool("replace_posting_date"), postingDate)
			return issue.Run()
		default:
			return tables.UpdateFinChrgMemoLines(h)
		}
	}
}

// entryFilters: the request's entry filters on a record of the transaction
func (r *reminderBatch) entryFilters(src ftables.Source) *gtables.CustomerLedgerEntry {
	if r.custLedgEntry == nil {
		return nil
	}
	custLedgEntry := gtables.NewCustomerLedgerEntry(src)
	custLedgEntry.CopyFilters(r.custLedgEntry)
	return custLedgEntry
}

// errorText: an error in the user's language
func errorText(err error) string {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		return appErr.Message(fcodeunits.CurrentLanguage())
	}
	return err.Error()
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}
