package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 375 Bank Account Statistics: the bank account's balance and its net change in this
// accounting period, this fiscal year, last fiscal year and to date (OnAfterGetRecord; the
// periods of the work date, DateFilter-Calc)
func init() {
	pages.OnAfterGetRecord(375, func(rec *gtables.BankAccount, page *pages.Control) {
		currentDate := system.WorkDate()
		fiscalYearText := i18n.GetInstance().Message("FISCAL_YEAR_NAME", fcodeunits.CurrentLanguage())
		var bankAccDateFilter, bankAccDateName [4]string
		bankAccDateFilter[0], bankAccDateName[0] = tables.CreateAccountingPeriodFilter(rec, currentDate, 0)
		bankAccDateFilter[1], bankAccDateName[1] = tables.CreateFiscalYearFilter(rec, currentDate, 0, fiscalYearText)
		bankAccDateFilter[2], bankAccDateName[2] = tables.CreateFiscalYearFilter(rec, currentDate, -1, fiscalYearText)

		for i, column := range statisticsColumns {
			page.SetValue(column+"_name", bankAccDateName[i])
			// without accounting periods only To Date has amounts
			bankAcc := gtables.NewBankAccount(rec)
			bankAcc.No = rec.No
			if bankAccDateFilter[i] != "" || column == "to_date" {
				_ = bankAcc.SetFlowFilter("date_filter", bankAccDateFilter[i])
				bankAcc.CalcFields("net_change", "net_change_lcy")
			}
			page.SetValue(column+"_net_change", bankAcc.Net_change)
			page.SetValue(column+"_net_change_lcy", bankAcc.Net_change_lcy)
		}
	})
}
