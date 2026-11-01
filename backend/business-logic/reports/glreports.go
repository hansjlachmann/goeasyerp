package reports

import (
	"strings"

	ftables "github.com/hansjlachmann/openerp/backend/foundation/tables"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// The general ledger list reports (main menu General Ledger → Reports): 1 Chart of Accounts,
// 3 G/L Register, 4 Detail Trial Balance and 6 Trial Balance. Like the other list printouts
// they start each page with the report head (setReportHeadValues) and the first page with the
// filters (filter_text) and the period (period_text).
const (
	ChartOfAccountsID    = 1
	GLRegisterID         = 3
	DetailTrialBalanceID = 4
	TrialBalanceID       = 6
)

func init() {
	Register(ChartOfAccountsID, "chart-of-accounts", func() Report { return &chartOfAccounts{} })
	Register(GLRegisterID, "g-l-register", func() Report { return &glRegisterReport{} })
	Register(DetailTrialBalanceID, "detail-trial-balance", func() Report { return &detailTrialBalance{} })
	Register(TrialBalanceID, "trial-balance", func() Report { return &trialBalance{} })
}

// dateFilterRange: the first and last date of a date filter ("a..b", "..b", "a..", "a"); a
// blank end is open
func dateFilterRange(expr string) (from, to types.Date) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return
	}
	fromText, toText, isRange := strings.Cut(expr, "..")
	if !isRange {
		toText = fromText
	}
	from, _ = types.NewDateFromString(strings.TrimSpace(fromText))
	to, _ = types.NewDateFromString(strings.TrimSpace(toText))
	return from, to
}

// beforeFilter: the date filter of everything before the period of a date filter ("..from-1");
// blank when the period has no start
func beforeFilter(expr string) string {
	from, _ := dateFilterRange(expr)
	if from.IsZero() {
		return ""
	}
	return ".." + from.AddDays(-1).String()
}

// setPeriodText: the period of a date filter in the head ("Period: …"), blank without one
func setPeriodText(run *Run, dateFilter string) {
	if dateFilter == "" {
		run.SetValue("period_text", "")
		return
	}
	run.SetValue("period_text", run.Text("PERIOD", dateFilter))
}

// indentedName: a G/L account's name indented by its level (PADSTR(”, Indentation * 2))
func indentedName(acc *gtables.GLAccount) string {
	return strings.Repeat("  ", acc.Indentation) + acc.Name.String()
}

// isTotalAccount: a heading or total account prints in bold
func isTotalAccount(acc *gtables.GLAccount) bool {
	return acc.Account_type != gtables.GLAccount_Account_type.Posting
}

// debitCredit: an amount as its debit and credit column (a positive amount is a debit)
func debitCredit(amount types.Decimal) (debit, credit types.Decimal) {
	if amount.IsPositive() {
		return amount, types.Decimal{}
	}
	return types.Decimal{}, amount.Neg()
}

// Report 1 Chart of Accounts: the G/L accounts in the filters with their account type,
// income/balance, totaling and posting groups, indented by level
type chartOfAccounts struct{}

func (r *chartOfAccounts) Definition() *Definition {
	return &Definition{
		OnPreReport: func(run *Run) error { setReportHeadValues(run); return nil },
		DataItems: []*DataItem{{
			Name:                "GLAccount",
			Table:               gtables.GLAccountTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "account_type", "income_balance"},
			OnPreDataItem: Trigger(func(rec *gtables.GLAccount, run *Run) error {
				setFilterText(run, rec)
				return nil
			}),
			OnAfterGetRecord: Trigger(func(rec *gtables.GLAccount, run *Run) error {
				run.SetValue("name", indentedName(rec))
				run.SetValue("is_total", isTotalAccount(rec))
				return nil
			}),
		}},
	}
}

// Report 6 Trial Balance: each G/L account's net change in the period and its balance at the
// period's end, as debit and credit columns (heading and total accounts in bold); the totals
// of the posting accounts
type trialBalance struct {
	totals [4]types.Decimal
}

func (r *trialBalance) Definition() *Definition {
	return &Definition{
		OnPreReport: func(run *Run) error { setReportHeadValues(run); return nil },
		DataItems: []*DataItem{{
			Name:                "GLAccount",
			Table:               gtables.GLAccountTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "account_type", "date_filter", "department_filter", "project_filter"},
			OnPreDataItem: Trigger(func(rec *gtables.GLAccount, run *Run) error {
				setFilterText(run, rec)
				setPeriodText(run, rec.Date_filter)
				return nil
			}),
			OnAfterGetRecord: Trigger(r.glAccOnAfterGetRecord),
		}},
	}
}

func (r *trialBalance) glAccOnAfterGetRecord(rec *gtables.GLAccount, run *Run) error {
	rec.CalcFields("net_change", "balance_at_date")
	netDebit, netCredit := debitCredit(rec.Net_change)
	balDebit, balCredit := debitCredit(rec.Balance_at_date)
	run.SetValue("name", indentedName(rec))
	run.SetValue("is_total", isTotalAccount(rec))
	run.SetValue("net_change_debit", netDebit)
	run.SetValue("net_change_credit", netCredit)
	run.SetValue("balance_debit", balDebit)
	run.SetValue("balance_credit", balCredit)
	if !isTotalAccount(rec) {
		for i, v := range []types.Decimal{netDebit, netCredit, balDebit, balCredit} {
			r.totals[i] = r.totals[i].Add(v)
		}
	}
	run.SetValue("total_net_change_debit", r.totals[0])
	run.SetValue("total_net_change_credit", r.totals[1])
	run.SetValue("total_balance_debit", r.totals[2])
	run.SetValue("total_balance_credit", r.totals[3])
	return nil
}

// Report 4 Detail Trial Balance: for each posting account its balance before the period, its
// G/L entries of the period with the running balance, and its balance at the end; accounts
// without entries and balance are left out unless Include Accounts Without Entries
type detailTrialBalance struct {
	dateFilter      string
	startBalance    types.Decimal
	balance         types.Decimal
	includeAccounts bool
}

func (r *detailTrialBalance) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        []Option{{Name: "include_accounts_without_entries", Type: "bool", Default: "false"}},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			r.includeAccounts = run.OptionBool("include_accounts_without_entries")
			return nil
		},
		DataItems: []*DataItem{{
			Name:                "GLAccount",
			Table:               gtables.GLAccountTableName,
			Key:                 []string{"no"},
			TableView:           []ftables.FieldFilter{{Field: "account_type", Expression: "0"}},
			RequestFilterFields: []string{"no", "search_name", "income_balance", "debit_credit", "date_filter"},
			OnPreDataItem: Trigger(func(rec *gtables.GLAccount, run *Run) error {
				setFilterText(run, rec)
				setPeriodText(run, rec.Date_filter)
				r.dateFilter = rec.Date_filter
				return nil
			}),
			OnAfterGetRecord: Trigger(r.glAccOnAfterGetRecord),
			DataItems: []*DataItem{{
				Name:          "GLEntry",
				Table:         gtables.GLEntryTableName,
				Key:           []string{"gl_account_no", "posting_date"},
				Link:          map[string]string{"gl_account_no": "no"},
				OnPreDataItem: Trigger(r.glEntryOnPreDataItem),
				OnAfterGetRecord: Trigger(func(rec *gtables.GLEntry, run *Run) error {
					r.balance = r.balance.Add(rec.Amount)
					run.SetValue("balance", r.balance)
					return nil
				}),
			}},
		}},
	}
}

func (r *detailTrialBalance) glAccOnAfterGetRecord(rec *gtables.GLAccount, run *Run) error {
	r.startBalance = types.Decimal{}
	if before := beforeFilter(r.dateFilter); before != "" {
		acc := gtables.NewGLAccount(run)
		acc.No = rec.No
		err := acc.SetFlowFilter("date_filter", before)
		if err != nil {
			return err
		}
		acc.Department_filter, acc.Project_filter = rec.Department_filter, rec.Project_filter
		acc.CalcFields("balance_at_date")
		r.startBalance = acc.Balance_at_date
	}
	if !r.includeAccounts && r.startBalance.IsZero() && r.entries(rec).Count() == 0 {
		return Skip()
	}
	r.balance = r.startBalance
	run.SetValue("start_balance", r.startBalance)
	run.SetValue("balance", r.balance)
	return nil
}

// entries: the account's G/L entries in the period
func (r *detailTrialBalance) entries(acc *gtables.GLAccount) *gtables.GLEntry {
	glEntry := gtables.NewGLEntry(acc)
	glEntry.SetRange("gl_account_no", acc.No)
	r.setPeriod(glEntry)
	return glEntry
}

func (r *detailTrialBalance) setPeriod(glEntry *gtables.GLEntry) {
	if r.dateFilter != "" {
		glEntry.SetFilter("posting_date", r.dateFilter)
	}
}

func (r *detailTrialBalance) glEntryOnPreDataItem(rec *gtables.GLEntry, run *Run) error {
	r.setPeriod(rec)
	return nil
}

// Report 3 G/L Register: the G/L registers in the filters, each with the G/L entries it wrote
// and their total
type glRegisterReport struct {
	glRegister *gtables.GLRegister
	total      types.Decimal
}

func (r *glRegisterReport) Definition() *Definition {
	return &Definition{
		OnPreReport: func(run *Run) error { setReportHeadValues(run); return nil },
		DataItems: []*DataItem{{
			Name:                "GLRegister",
			Table:               gtables.GLRegisterTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "creation_date", "source_code"},
			OnPreDataItem: Trigger(func(rec *gtables.GLRegister, run *Run) error {
				setFilterText(run, rec)
				return nil
			}),
			OnAfterGetRecord: Trigger(func(rec *gtables.GLRegister, run *Run) error {
				r.glRegister = rec
				return nil
			}),
			DataItems: []*DataItem{{
				Name:  "GLEntry",
				Table: gtables.GLEntryTableName,
				Key:   []string{"entry_no"},
				OnPreDataItem: Trigger(func(rec *gtables.GLEntry, run *Run) error {
					rec.SetRange("entry_no", r.glRegister.From_entry_no, r.glRegister.To_entry_no)
					return nil
				}),
				OnAfterGetRecord: Trigger(func(rec *gtables.GLEntry, run *Run) error {
					acc := gtables.NewGLAccount(run)
					if !acc.Get(rec.Gl_account_no) {
						acc.Init()
					}
					run.SetValue("account_name", acc.Name.String())
					r.total = r.total.Add(rec.Amount)
					run.SetValue("total", r.total)
					return nil
				}),
			}},
		}},
	}
}
