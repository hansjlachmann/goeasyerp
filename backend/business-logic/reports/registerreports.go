package reports

import (
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/openerp/backend/generated/tables"
)

// More list reports (main menu Reports entries):
//   - 7 Trial Balance/Previous Year: each G/L account's net change in the period and its balance
//     at the period's end, next to the same period a year earlier and the change in percent
//     (without a Date Filter: up to the work date);
//   - 1403 Bank Account Register: the G/L registers with the bank account ledger entries they
//     wrote (a bank entry has the number of its G/L entry) and their total;
//   - 703 Item Register (from the classic US Item Register): the item registers with the item
//     ledger entries they wrote.
const (
	TrialBalancePrevYearID = 7
	BankAccountRegisterID  = 1403
	ItemRegisterID         = 703
)

func init() {
	Register(TrialBalancePrevYearID, "trial-balance-previous-year", func() Report { return &trialBalancePrevYear{} })
	Register(BankAccountRegisterID, "bank-account-register", func() Report { return &bankAccountRegister{} })
	Register(ItemRegisterID, "item-register", func() Report { return &itemRegister{} })
}

// Report 7 Trial Balance/Previous Year
type trialBalancePrevYear struct {
	prevYearFilter string
	useAddRptCurr  bool
}

func (r *trialBalancePrevYear) Definition() *Definition {
	return &Definition{
		Options: []Option{{Name: "use_add_rpt_curr", Type: "bool", Default: "false"}},
		OnPreReport: func(run *Run) error {
			err := reportHead(run)
			if err != nil {
				return err
			}
			r.useAddRptCurr, err = useAddRptCurrOption(run)
			return err
		},
		DataItems: []*DataItem{{
			Name:                "GLAccount",
			Table:               gtables.GLAccountTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "account_type", "date_filter", "department_filter", "project_filter"},
			OnPreDataItem: Trigger(func(rec *gtables.GLAccount, run *Run) error {
				setFilterText(run, rec)
				// without a period: up to the work date, compared with up to a year earlier
				if rec.Date_filter == "" {
					err := rec.SetFlowFilter("date_filter", ".."+system.WorkDate().String())
					if err != nil {
						return err
					}
				}
				setPeriodText(run, rec.Date_filter)
				r.prevYearFilter = previousYearFilter(rec.Date_filter)
				return nil
			}),
			OnAfterGetRecord: Trigger(r.glAccOnAfterGetRecord),
		}},
	}
}

// previousYearFilter: the date filter a year earlier ("" without one)
func previousYearFilter(expr string) string {
	if expr == "" {
		return ""
	}
	from, to := dateFilterRange(expr)
	filter := ""
	if !from.IsZero() {
		filter = from.AddYears(-1).String()
	}
	filter += ".."
	if !to.IsZero() {
		filter += to.AddYears(-1).String()
	}
	return filter
}

func (r *trialBalancePrevYear) glAccOnAfterGetRecord(rec *gtables.GLAccount, run *Run) error {
	netChange, balanceAtDate := netChangeAndBalance(rec, r.useAddRptCurr)
	prev := gtables.NewGLAccount(run)
	prev.No = rec.No
	prev.Account_type, prev.Totaling = rec.Account_type, rec.Totaling
	prev.Department_filter, prev.Project_filter = rec.Department_filter, rec.Project_filter
	if r.prevYearFilter != "" {
		err := prev.SetFlowFilter("date_filter", r.prevYearFilter)
		if err != nil {
			return err
		}
	}
	netChangePrev, balanceAtDatePrev := netChangeAndBalance(prev, r.useAddRptCurr)
	run.SetValue("name", indentedName(rec))
	run.SetValue("is_total", isTotalAccount(rec))
	run.SetValue("net_change", netChange)
	run.SetValue("net_change_prev", netChangePrev)
	run.SetValue("net_change_pct", changePercent(netChange, netChangePrev))
	run.SetValue("balance", balanceAtDate)
	run.SetValue("balance_prev", balanceAtDatePrev)
	run.SetValue("balance_pct", changePercent(balanceAtDate, balanceAtDatePrev))
	return nil
}

// changePercent: how much an amount changed from the previous one, in percent of the previous
// (blank without a previous amount)
func changePercent(amount, previous types.Decimal) types.Decimal {
	if previous.IsZero() {
		return types.Decimal{}
	}
	return amount.Sub(previous).Div(previous.Abs()).Mul(types.NewDecimalFromInt(100)).Round(1)
}

// Report 1403 Bank Account Register
type bankAccountRegister struct {
	glRegister *gtables.GLRegister
	total      types.Decimal
}

func (r *bankAccountRegister) Definition() *Definition {
	return &Definition{
		OnPreReport: reportHead,
		DataItems: []*DataItem{{
			Name:                "GLRegister",
			Table:               gtables.GLRegisterTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "creation_date", "source_code"},
			OnPreDataItem:       headWithFilters[*gtables.GLRegister](nil),
			OnAfterGetRecord: Trigger(func(rec *gtables.GLRegister, run *Run) error {
				r.glRegister = rec
				// registers without bank entries are left out
				entries := gtables.NewBankAccountLedgerEntry(run)
				entries.SetRange("entry_no", rec.From_entry_no, rec.To_entry_no)
				if entries.Count() == 0 {
					return Skip()
				}
				return nil
			}),
			DataItems: []*DataItem{{
				Name:  "BankAccountLedgerEntry",
				Table: gtables.BankAccountLedgerEntryTableName,
				Key:   []string{"entry_no"},
				OnPreDataItem: Trigger(func(rec *gtables.BankAccountLedgerEntry, run *Run) error {
					rec.SetRange("entry_no", r.glRegister.From_entry_no, r.glRegister.To_entry_no)
					return nil
				}),
				OnAfterGetRecord: Trigger(func(rec *gtables.BankAccountLedgerEntry, run *Run) error {
					bankAcc := gtables.NewBankAccount(run)
					if !bankAcc.Get(rec.Bank_account_no) {
						bankAcc.Init()
					}
					run.SetValue("bank_account_name", bankAcc.Name.String())
					r.total = r.total.Add(rec.Amount_lcy)
					run.SetValue("total", r.total)
					return nil
				}),
			}},
		}},
	}
}

// Report 703 Item Register
type itemRegister struct {
	itemReg *gtables.ItemRegister
}

func (r *itemRegister) Definition() *Definition {
	return &Definition{
		OnPreReport: reportHead,
		DataItems: []*DataItem{{
			Name:                "ItemRegister",
			Table:               gtables.ItemRegisterTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "creation_date", "source_code"},
			OnPreDataItem:       headWithFilters[*gtables.ItemRegister](nil),
			OnAfterGetRecord: Trigger(func(rec *gtables.ItemRegister, run *Run) error {
				r.itemReg = rec
				return nil
			}),
			DataItems: []*DataItem{{
				Name:  "ItemLedgerEntry",
				Table: gtables.ItemLedgerEntryTableName,
				Key:   []string{"entry_no"},
				OnPreDataItem: Trigger(func(rec *gtables.ItemLedgerEntry, run *Run) error {
					rec.SetRange("entry_no", r.itemReg.From_entry_no, r.itemReg.To_entry_no)
					return nil
				}),
				OnAfterGetRecord: Trigger(func(rec *gtables.ItemLedgerEntry, run *Run) error {
					item := gtables.NewItem(run)
					if !item.Get(rec.Item_no) {
						item.Init()
					}
					run.SetValue("item_description", item.Description.String())
					return nil
				}),
			}},
		}},
	}
}
