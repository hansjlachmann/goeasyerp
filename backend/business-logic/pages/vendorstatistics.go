package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Page 152 Vendor Statistics: the vendor's balance and, for this accounting period, this fiscal
// year, last fiscal year and to date, its purchases, discounts and other amounts
// (OnAfterGetRecord; the periods of the work date, DateFilter-Calc)
func init() {
	pages.OnAfterGetRecord(152, func(rec *gtables.Vendor, page *pages.Control) {
		currentDate := system.WorkDate()
		fiscalYearText := i18n.GetInstance().Message("FISCAL_YEAR_NAME", fcodeunits.CurrentLanguage())
		var vendDateFilter, vendDateName [4]string
		vendDateFilter[0], vendDateName[0] = tables.CreateAccountingPeriodFilter(rec, currentDate, 0)
		vendDateFilter[1], vendDateName[1] = tables.CreateFiscalYearFilter(rec, currentDate, 0, fiscalYearText)
		vendDateFilter[2], vendDateName[2] = tables.CreateFiscalYearFilter(rec, currentDate, -1, fiscalYearText)

		vend := gtables.NewVendor(rec)
		vend.No = rec.No
		_ = vend.SetFlowFilter("date_filter", ".."+currentDate.String())
		vend.CalcFields("balance_lcy", "balance_due_lcy")
		page.SetValue("total_lcy", vend.Balance_lcy)
		page.SetValue("balance_due_to_date_lcy", vend.Balance_due_lcy)

		for i, column := range statisticsColumns {
			page.SetValue(column+"_name", vendDateName[i])
			// without accounting periods only To Date has amounts
			vendPeriod := gtables.NewVendor(rec)
			vendPeriod.No = rec.No
			if vendDateFilter[i] != "" || column == "to_date" {
				_ = vendPeriod.SetFlowFilter("date_filter", vendDateFilter[i])
				vendPeriod.CalcFields("purchases_lcy", "inv_discounts_lcy", "pmt_discounts_lcy",
					"fin_charge_memo_amounts_lcy", "cr_memo_amounts_lcy", "payments_lcy", "reminder_amounts_lcy")
			}
			page.SetValue(column+"_purchases_lcy", vendPeriod.Purchases_lcy)
			page.SetValue(column+"_inv_discounts_lcy", vendPeriod.Inv_discounts_lcy)
			page.SetValue(column+"_pmt_discounts_lcy", vendPeriod.Pmt_discounts_lcy)
			page.SetValue(column+"_fin_charge_memo_amounts_lcy", vendPeriod.Fin_charge_memo_amounts_lcy)
			page.SetValue(column+"_cr_memo_amounts_lcy", vendPeriod.Cr_memo_amounts_lcy)
			page.SetValue(column+"_payments_lcy", vendPeriod.Payments_lcy)
			page.SetValue(column+"_reminder_amounts_lcy", vendPeriod.Reminder_amounts_lcy)
		}
	})
}
