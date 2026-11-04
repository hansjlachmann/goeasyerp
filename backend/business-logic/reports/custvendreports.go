package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The customer, vendor and bank account list reports (main menu Sales →
// Reports, Purchases → Reports, General Ledger → Reports):
//   - 101 Customer - List, 301 Vendor - List, 1402 Bank Account - List: the accounts with their
//     main data and balance (LCY), the total balance;
//   - 104 Customer - Detail Trial Bal., 304 Vendor - Detail Trial Balance, 1404 Bank Acc. -
//     Detail Trial Bal.: per account the balance before the period, the ledger entries of the
//     period with the running balance (LCY);
//   - 120 Aged Accounts Receivable, 322 Aged Accounts Payable: the open entries' remaining
//     amounts (LCY) by how long they are overdue on the Aged As Of date (Not Due, 1–30, 31–60,
//     61–90, over 90 days), per account and in total; with Print Details each entry.
const (
	CustomerListID          = 101
	CustomerDetailTrialBal  = 104
	AgedAccountsReceivable  = 120
	VendorListID            = 301
	VendorDetailTrialBal    = 304
	AgedAccountsPayable     = 322
	BankAccountListID       = 1402
	BankAccDetailTrialBalID = 1404
)

func init() {
	Register(CustomerListID, "customer-list", func() Report { return &customerList{} })
	Register(CustomerDetailTrialBal, "customer-detail-trial-bal", func() Report { return &customerDetailTrialBal{} })
	Register(AgedAccountsReceivable, "aged-accounts-receivable", func() Report { return &agedCustomer{} })
	Register(VendorListID, "vendor-list", func() Report { return &vendorList{} })
	Register(VendorDetailTrialBal, "vendor-detail-trial-balance", func() Report { return &vendorDetailTrialBal{} })
	Register(AgedAccountsPayable, "aged-accounts-payable", func() Report { return &agedVendor{} })
	Register(BankAccountListID, "bank-account-list", func() Report { return &bankAccountList{} })
	Register(BankAccDetailTrialBalID, "bank-acc-detail-trial-bal", func() Report { return &bankAccDetailTrialBal{} })
}

func reportHead(run *Run) error {
	setReportHeadValues(run)
	return nil
}

// headWithFilters: a data item's OnPreDataItem putting its filters (and period) in the head
func headWithFilters[T interface {
	filteredRecord
}](period func(T) string) TriggerFunc {
	return Trigger(func(rec T, run *Run) error {
		setFilterText(run, rec)
		if period != nil {
			setPeriodText(run, period(rec))
		}
		return nil
	})
}

// Report 101 Customer - List
type customerList struct{ total types.Decimal }

func (r *customerList) Definition() *Definition {
	return &Definition{
		OnPreReport: reportHead,
		DataItems: []*DataItem{{
			Name:                "Customer",
			Table:               gtables.CustomerTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "search_name", "customer_posting_group", "date_filter"},
			OnPreDataItem:       headWithFilters[*gtables.Customer](nil),
			OnAfterGetRecord: Trigger(func(rec *gtables.Customer, run *Run) error {
				rec.CalcFields("balance_lcy")
				r.total = r.total.Add(rec.Balance_lcy)
				run.SetValue("total", r.total)
				return nil
			}),
		}},
	}
}

// Report 301 Vendor - List
type vendorList struct{ total types.Decimal }

func (r *vendorList) Definition() *Definition {
	return &Definition{
		OnPreReport: reportHead,
		DataItems: []*DataItem{{
			Name:                "Vendor",
			Table:               gtables.VendorTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "search_name", "vendor_posting_group", "date_filter"},
			OnPreDataItem:       headWithFilters[*gtables.Vendor](nil),
			OnAfterGetRecord: Trigger(func(rec *gtables.Vendor, run *Run) error {
				rec.CalcFields("balance_lcy")
				r.total = r.total.Add(rec.Balance_lcy)
				run.SetValue("total", r.total)
				return nil
			}),
		}},
	}
}

// Report 1402 Bank Account - List
type bankAccountList struct{ total types.Decimal }

func (r *bankAccountList) Definition() *Definition {
	return &Definition{
		OnPreReport: reportHead,
		DataItems: []*DataItem{{
			Name:                "BankAccount",
			Table:               gtables.BankAccountTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "search_name", "bank_acc_posting_group", "currency_code", "date_filter"},
			OnPreDataItem:       headWithFilters[*gtables.BankAccount](nil),
			OnAfterGetRecord: Trigger(func(rec *gtables.BankAccount, run *Run) error {
				rec.CalcFields("balance", "balance_lcy")
				r.total = r.total.Add(rec.Balance_lcy)
				run.SetValue("total", r.total)
				return nil
			}),
		}},
	}
}

// detailTrialBal is the state of the detail trial balances: the period, the running balance
type detailTrialBal struct {
	dateFilter      string
	balance         types.Decimal
	includeAccounts bool
}

func (d *detailTrialBal) options() []Option {
	return []Option{{Name: "include_accounts_without_entries", Type: "bool", Default: "false"}}
}

func (d *detailTrialBal) onPreReport(run *Run) error {
	setReportHeadValues(run)
	d.includeAccounts = run.OptionBool("include_accounts_without_entries")
	return nil
}

// start: the account's balance before the period (its net change up to the day before) —
// skipped without entries and balance unless the option says otherwise
func (d *detailTrialBal) start(run *Run, startBalance types.Decimal, entries int) error {
	if !d.includeAccounts && startBalance.IsZero() && entries == 0 {
		return Skip()
	}
	d.balance = startBalance
	run.SetValue("start_balance", startBalance)
	run.SetValue("balance", d.balance)
	return nil
}

func (d *detailTrialBal) entry(run *Run, amount types.Decimal) {
	d.balance = d.balance.Add(amount)
	run.SetValue("balance", d.balance)
}

// period: the entries' posting date filter of the period
func (d *detailTrialBal) period(rec interface{ SetFilter(field, expr string) }) {
	if d.dateFilter != "" {
		rec.SetFilter("posting_date", d.dateFilter)
	}
}

// Report 104 Customer - Detail Trial Bal.
type customerDetailTrialBal struct{ detailTrialBal }

func (r *customerDetailTrialBal) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        r.options(),
		OnPreReport:    r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "Customer",
			Table:               gtables.CustomerTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "search_name", "customer_posting_group", "date_filter"},
			OnPreDataItem: Trigger(func(rec *gtables.Customer, run *Run) error {
				setFilterText(run, rec)
				setPeriodText(run, rec.Date_filter)
				r.dateFilter = rec.Date_filter
				return nil
			}),
			OnAfterGetRecord: Trigger(func(rec *gtables.Customer, run *Run) error {
				var startBalance types.Decimal
				if before := beforeFilter(r.dateFilter); before != "" {
					cust := gtables.NewCustomer(run)
					cust.No = rec.No
					if err := cust.SetFlowFilter("date_filter", before); err != nil {
						return err
					}
					cust.CalcFields("net_change_lcy")
					startBalance = cust.Net_change_lcy
				}
				entries := gtables.NewCustomerLedgerEntry(run)
				entries.SetRange("customer_no", rec.No)
				r.period(entries)
				return r.start(run, startBalance, entries.Count())
			}),
			DataItems: []*DataItem{{
				Name:          "CustLedgerEntry",
				Table:         "Customer_ledger_entry",
				Key:           []string{"customer_no", "posting_date"},
				Link:          map[string]string{"customer_no": "no"},
				OnPreDataItem: Trigger(func(rec *gtables.CustomerLedgerEntry, run *Run) error { r.period(rec); return nil }),
				OnAfterGetRecord: Trigger(func(rec *gtables.CustomerLedgerEntry, run *Run) error {
					r.entry(run, rec.Amount_lcy)
					return nil
				}),
			}},
		}},
	}
}

// Report 304 Vendor - Detail Trial Balance
type vendorDetailTrialBal struct{ detailTrialBal }

func (r *vendorDetailTrialBal) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        r.options(),
		OnPreReport:    r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "Vendor",
			Table:               gtables.VendorTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "search_name", "vendor_posting_group", "date_filter"},
			OnPreDataItem: Trigger(func(rec *gtables.Vendor, run *Run) error {
				setFilterText(run, rec)
				setPeriodText(run, rec.Date_filter)
				r.dateFilter = rec.Date_filter
				return nil
			}),
			OnAfterGetRecord: Trigger(func(rec *gtables.Vendor, run *Run) error {
				var startBalance types.Decimal
				if before := beforeFilter(r.dateFilter); before != "" {
					vend := gtables.NewVendor(run)
					vend.No = rec.No
					if err := vend.SetFlowFilter("date_filter", before); err != nil {
						return err
					}
					vend.CalcFields("net_change_lcy")
					startBalance = vend.Net_change_lcy
				}
				entries := gtables.NewVendorLedgerEntry(run)
				entries.SetRange("vendor_no", rec.No)
				r.period(entries)
				return r.start(run, startBalance, entries.Count())
			}),
			DataItems: []*DataItem{{
				Name:          "VendorLedgerEntry",
				Table:         "Vendor_ledger_entry",
				Key:           []string{"vendor_no", "posting_date"},
				Link:          map[string]string{"vendor_no": "no"},
				OnPreDataItem: Trigger(func(rec *gtables.VendorLedgerEntry, run *Run) error { r.period(rec); return nil }),
				OnAfterGetRecord: Trigger(func(rec *gtables.VendorLedgerEntry, run *Run) error {
					r.entry(run, rec.Amount_lcy)
					return nil
				}),
			}},
		}},
	}
}

// Report 1404 Bank Acc. - Detail Trial Bal.
type bankAccDetailTrialBal struct{ detailTrialBal }

func (r *bankAccDetailTrialBal) Definition() *Definition {
	return &Definition{
		OptionsCaption: "OPTIONS",
		Options:        r.options(),
		OnPreReport:    r.onPreReport,
		DataItems: []*DataItem{{
			Name:                "BankAccount",
			Table:               gtables.BankAccountTableName,
			Key:                 []string{"no"},
			RequestFilterFields: []string{"no", "search_name", "bank_acc_posting_group", "date_filter"},
			OnPreDataItem: Trigger(func(rec *gtables.BankAccount, run *Run) error {
				setFilterText(run, rec)
				setPeriodText(run, rec.Date_filter)
				r.dateFilter = rec.Date_filter
				return nil
			}),
			OnAfterGetRecord: Trigger(func(rec *gtables.BankAccount, run *Run) error {
				var startBalance types.Decimal
				if before := beforeFilter(r.dateFilter); before != "" {
					bank := gtables.NewBankAccount(run)
					bank.No = rec.No
					if err := bank.SetFlowFilter("date_filter", before); err != nil {
						return err
					}
					bank.CalcFields("net_change_lcy")
					startBalance = bank.Net_change_lcy
				}
				entries := gtables.NewBankAccountLedgerEntry(run)
				entries.SetRange("bank_account_no", rec.No)
				r.period(entries)
				return r.start(run, startBalance, entries.Count())
			}),
			DataItems: []*DataItem{{
				Name:          "BankAccountLedgerEntry",
				Table:         gtables.BankAccountLedgerEntryTableName,
				Key:           []string{"bank_account_no", "posting_date"},
				Link:          map[string]string{"bank_account_no": "no"},
				OnPreDataItem: Trigger(func(rec *gtables.BankAccountLedgerEntry, run *Run) error { r.period(rec); return nil }),
				OnAfterGetRecord: Trigger(func(rec *gtables.BankAccountLedgerEntry, run *Run) error {
					r.entry(run, rec.Amount_lcy)
					return nil
				}),
			}},
		}},
	}
}

// aging: the remaining amounts (LCY) of open entries by days overdue on the aged-as-of date
type aging struct {
	agedAsOf     types.Date
	printDetails bool
	account      [6]types.Decimal // balance, not due, 1–30, 31–60, 61–90, over 90
	total        [6]types.Decimal
}

var agingValues = []string{"balance", "not_due", "days_1_30", "days_31_60", "days_61_90", "over_90"}

func (a *aging) options() []Option {
	return []Option{
		{Name: "aged_as_of", Type: "date", Required: true},
		{Name: "print_details", Type: "bool", Default: "false"},
	}
}

func (a *aging) onInitReport(run *Run) error {
	run.SetOption("aged_as_of", system.WorkDate().String())
	return nil
}

func (a *aging) onPreReport(run *Run) error {
	setReportHeadValues(run)
	a.agedAsOf = run.OptionDate("aged_as_of")
	if a.agedAsOf.IsZero() {
		return system.Error(run.Text("ENTER_AGED_AS_OF"))
	}
	a.printDetails = run.OptionBool("print_details")
	run.SetValue("aged_as_of", a.agedAsOf)
	run.SetValue("print_details", a.printDetails)
	return nil
}

// bucket: the column of an amount due on the date (0 the balance only)
func (a *aging) bucket(dueDate types.Date) int {
	days := dueDate.DaysBetween(a.agedAsOf)
	switch {
	case dueDate.IsZero() || days <= 0:
		return 1
	case days <= 30:
		return 2
	case days <= 60:
		return 3
	case days <= 90:
		return 4
	}
	return 5
}

// entryValues: an entry's remaining amount in its column and its days overdue
func (a *aging) entryValues(run *Run, remaining types.Decimal, dueDate types.Date) {
	values := [6]types.Decimal{remaining}
	values[a.bucket(dueDate)] = remaining
	for i, name := range agingValues {
		run.SetValue("entry_"+name, values[i])
	}
	run.SetValue("days_overdue", max(0, dueDate.DaysBetween(a.agedAsOf)))
}

// accountValues: an account's buckets from its open entries (none: skipped); the totals
func (a *aging) accountValues(run *Run, add func(func(remaining types.Decimal, dueDate types.Date))) error {
	a.account = [6]types.Decimal{}
	n := 0
	add(func(remaining types.Decimal, dueDate types.Date) {
		n++
		a.account[0] = a.account[0].Add(remaining)
		b := a.bucket(dueDate)
		a.account[b] = a.account[b].Add(remaining)
	})
	if n == 0 {
		return Skip()
	}
	for i, name := range agingValues {
		a.total[i] = a.total[i].Add(a.account[i])
		run.SetValue(name, a.account[i])
		run.SetValue("total_"+name, a.total[i])
	}
	return nil
}

// openUntil: open entries posted up to the aged-as-of date
func (a *aging) openUntil(rec interface {
	SetRange(field string, values ...interface{})
}) {
	rec.SetRange("open", true)
	rec.SetRange("posting_date", types.Date{}, a.agedAsOf)
}

// agingDataItems: the accounts (accountName, accountTable) with their open entries (entryName,
// entryTable) as the nested data item
func agingDataItems(accountName, accountTable string, accountFilters []string, accountTrigger TriggerFunc,
	entryName, entryTable string, entryLink map[string]string, entryKey []string, entryTrigger, entryPre TriggerFunc) []*DataItem {
	return []*DataItem{{
		Name:                accountName,
		Table:               accountTable,
		Key:                 []string{"no"},
		RequestFilterFields: accountFilters,
		OnAfterGetRecord:    accountTrigger,
		DataItems: []*DataItem{{
			Name:             entryName,
			Table:            entryTable,
			Key:              entryKey,
			Link:             entryLink,
			OnPreDataItem:    entryPre,
			OnAfterGetRecord: entryTrigger,
		}},
	}}
}

// Report 120 Aged Accounts Receivable
type agedCustomer struct{ aging }

func (r *agedCustomer) Definition() *Definition {
	items := agingDataItems("Customer", gtables.CustomerTableName, []string{"no", "search_name", "customer_posting_group"},
		Trigger(func(rec *gtables.Customer, run *Run) error {
			entries := gtables.NewCustomerLedgerEntry(run)
			entries.SetRange("customer_no", rec.No)
			r.openUntil(entries)
			return r.accountValues(run, func(add func(types.Decimal, types.Date)) {
				for found := entries.FindSetBuffered(); found; found = entries.Next() {
					add(entries.Remaining_amt_lcy, entries.Due_date)
				}
			})
		}),
		"CustLedgerEntry", "Customer_ledger_entry", map[string]string{"customer_no": "no"}, []string{"customer_no", "due_date"},
		Trigger(func(rec *gtables.CustomerLedgerEntry, run *Run) error {
			r.entryValues(run, rec.Remaining_amt_lcy, rec.Due_date)
			return nil
		}),
		Trigger(func(rec *gtables.CustomerLedgerEntry, run *Run) error {
			if !r.printDetails {
				return Break()
			}
			r.openUntil(rec)
			return nil
		}))
	items[0].OnPreDataItem = headWithFilters[*gtables.Customer](nil)
	return &Definition{OptionsCaption: "OPTIONS", Options: r.options(), OnInitReport: r.onInitReport, OnPreReport: r.onPreReport, DataItems: items}
}

// Report 322 Aged Accounts Payable
type agedVendor struct{ aging }

func (r *agedVendor) Definition() *Definition {
	items := agingDataItems("Vendor", gtables.VendorTableName, []string{"no", "search_name", "vendor_posting_group"},
		Trigger(func(rec *gtables.Vendor, run *Run) error {
			entries := gtables.NewVendorLedgerEntry(run)
			entries.SetRange("vendor_no", rec.No)
			r.openUntil(entries)
			return r.accountValues(run, func(add func(types.Decimal, types.Date)) {
				for found := entries.FindSetBuffered(); found; found = entries.Next() {
					add(entries.Remaining_amt_lcy, entries.Due_date)
				}
			})
		}),
		"VendorLedgerEntry", "Vendor_ledger_entry", map[string]string{"vendor_no": "no"}, []string{"vendor_no", "due_date"},
		Trigger(func(rec *gtables.VendorLedgerEntry, run *Run) error {
			r.entryValues(run, rec.Remaining_amt_lcy, rec.Due_date)
			return nil
		}),
		Trigger(func(rec *gtables.VendorLedgerEntry, run *Run) error {
			if !r.printDetails {
				return Break()
			}
			r.openUntil(rec)
			return nil
		}))
	items[0].OnPreDataItem = headWithFilters[*gtables.Vendor](nil)
	return &Definition{OptionsCaption: "OPTIONS", Options: r.options(), OnInitReport: r.onInitReport, OnPreReport: r.onPreReport, DataItems: items}
}
