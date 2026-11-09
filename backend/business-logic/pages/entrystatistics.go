package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 302 Customer Entry Statistics and 303 Vendor Entry Statistics (classic forms 302/303):
// the last document of each type (Payment, Invoice, Credit Memo, Finance Charge Memo, Reminder)
// with its amount and remaining amount, the open entries' remaining amount (LCY) per type and,
// for this accounting period, this fiscal year and last fiscal year, the number of documents per
// type — for customers the largest balance and the average collection period, for vendors the
// payment discounts received and missed (OnAfterGetRecord; the periods of the work date).

// entryDocTypes are the document types by their option number (1–5) and the prefix of their
// page fields
var entryDocTypes = []string{"", "payment", "invoice", "credit_memo", "fin_charge_memo", "reminder"}

// entryStatisticsPeriods are the period columns (classic 1 This Period, 2 This Year, 3 Last Year)
var entryStatisticsPeriods = []string{"", "this_period", "this_year", "last_year"}

// entryStatisticsDateFilters is DateFilter-Calc's three periods of the work date (index 1–3)
func entryStatisticsDateFilters(src *gtables.Customer, vend *gtables.Vendor) (dateFilter, dateName [4]string) {
	currentDate := system.WorkDate()
	fiscalYearText := i18n.GetInstance().Message("FISCAL_YEAR_NAME", fcodeunits.CurrentLanguage())
	if src != nil {
		dateFilter[1], dateName[1] = tables.CreateAccountingPeriodFilter(src, currentDate, 0)
		dateFilter[2], dateName[2] = tables.CreateFiscalYearFilter(src, currentDate, 0, fiscalYearText)
		dateFilter[3], dateName[3] = tables.CreateFiscalYearFilter(src, currentDate, -1, fiscalYearText)
		return dateFilter, dateName
	}
	dateFilter[1], dateName[1] = tables.CreateAccountingPeriodFilter(vend, currentDate, 0)
	dateFilter[2], dateName[2] = tables.CreateFiscalYearFilter(vend, currentDate, 0, fiscalYearText)
	dateFilter[3], dateName[3] = tables.CreateFiscalYearFilter(vend, currentDate, -1, fiscalYearText)
	return dateFilter, dateName
}

func init() {
	pages.OnAfterGetRecord(302, customerEntryStatistics)
	pages.OnAfterGetRecord(303, vendorEntryStatistics)
}

// customerEntryStatistics is form 302's OnAfterGetRecord
func customerEntryStatistics(rec *gtables.Customer, page *pages.Control) {
	// the sign each document type is shown with (payments and credit memos are credits)
	sign := []int64{0, -1, 1, -1, 1, 1}
	for j := 1; j <= 5; j++ {
		custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
		custLedgEntry.SetCurrentKey("document_type", "customer_no", "posting_date")
		custLedgEntry.SetRange("document_type", j)
		custLedgEntry.SetRange("customer_no", rec.No)
		found := custLedgEntry.Find("+")
		prefix := "last_" + entryDocTypes[j] + "_"
		if found {
			page.SetValue(prefix+"date", custLedgEntry.Posting_date)
			page.SetValue(prefix+"document_no", custLedgEntry.Document_no)
			page.SetValue(prefix+"currency_code", custLedgEntry.Currency_code)
		}
		page.SetValue(prefix+"amount", custLedgEntry.Amount.Mul(types.NewDecimalFromInt(sign[j])))
		page.SetValue(prefix+"remaining_amount", custLedgEntry.Remaining_amount.Mul(types.NewDecimalFromInt(sign[j])))
	}

	var totalRemainAmountLCY [6]types.Decimal
	custLedgEntry2 := gtables.NewCustomerLedgerEntry(rec)
	custLedgEntry2.SetCurrentKey("customer_no", "open")
	custLedgEntry2.SetRange("customer_no", rec.No)
	custLedgEntry2.SetRange("open", true)
	for found := custLedgEntry2.Find("+"); found; found = custLedgEntry2.Next(-1) {
		j := int(custLedgEntry2.Document_type)
		if j > 0 {
			totalRemainAmountLCY[j] = totalRemainAmountLCY[j].Add(custLedgEntry2.Remaining_amt_lcy)
		}
	}
	for j := 1; j <= 5; j++ {
		page.SetValue("remaining_amt_lcy_"+entryDocTypes[j], totalRemainAmountLCY[j].Mul(types.NewDecimalFromInt(sign[j])))
	}

	custDateFilter, custDateName := entryStatisticsDateFilters(rec, nil)
	for i := 1; i <= 3; i++ {
		var noOfDoc [6]int
		highestBalanceLCY := types.ZeroDecimal()
		daysToPay, noOfInv := 0, 0
		// without accounting periods there are no periods to count in
		if custDateFilter[i] != "" {
			custLedgEntry2 = gtables.NewCustomerLedgerEntry(rec)
			custLedgEntry2.SetCurrentKey("customer_no", "posting_date")
			custLedgEntry2.SetRange("customer_no", rec.No)
			custLedgEntry2.SetFilter("posting_date", custDateFilter[i])
			periodEnd, _ := custLedgEntry2.GetRangeMax("posting_date").(types.Date)
			custLedgEntry2.SetRange("posting_date", types.Date{}, periodEnd)
			custBalanceLCY := types.ZeroDecimal()
			err := custLedgEntry2.CalcSums("amount_lcy")
			if err == nil {
				custBalanceLCY = custLedgEntry2.Amount_lcy
			}
			highestBalanceLCY = custBalanceLCY
			custLedgEntry2.SetFilter("posting_date", custDateFilter[i])
			for found := custLedgEntry2.Find("+"); found; found = custLedgEntry2.Next(-1) {
				j := int(custLedgEntry2.Document_type)
				if j > 0 {
					noOfDoc[j]++
				}
				custBalanceLCY = custBalanceLCY.Sub(custLedgEntry2.Amount_lcy)
				if custBalanceLCY.GreaterThan(highestBalanceLCY) {
					highestBalanceLCY = custBalanceLCY
				}
				// Optimized Approximation
				if custLedgEntry2.Document_type == gtables.CustomerLedgerEntry_Document_type.Invoice && !custLedgEntry2.Open {
					if custLedgEntry2.Closed_at_date.After(custLedgEntry2.Posting_date) {
						daysToPay += custLedgEntry2.Posting_date.DaysBetween(custLedgEntry2.Closed_at_date)
						noOfInv++
					} else {
						if custLedgEntry2.Closed_by_entry_no != 0 {
							custLedgEntry3 := gtables.NewCustomerLedgerEntry(rec)
							if custLedgEntry3.Get(custLedgEntry2.Closed_by_entry_no) {
								daysToPay += custLedgEntry2.Posting_date.DaysBetween(custLedgEntry3.Posting_date)
								noOfInv++
							}
						} else {
							custLedgEntry3 := gtables.NewCustomerLedgerEntry(rec)
							custLedgEntry3.SetRange("closed_by_entry_no", custLedgEntry2.Entry_no)
							if custLedgEntry3.Find("+") {
								daysToPay += custLedgEntry2.Posting_date.DaysBetween(custLedgEntry3.Posting_date)
								noOfInv++
							}
						}
					}
				}
			}
		}
		avgDaysToPay := types.ZeroDecimal()
		if noOfInv != 0 {
			avgDaysToPay = system.Round(types.NewDecimalFromInt(int64(daysToPay)).Div(types.NewDecimalFromInt(int64(noOfInv))), types.MustDecimal("0.1"))
		}
		prefix := entryStatisticsPeriods[i] + "_"
		page.SetValue(prefix+"name", custDateName[i])
		for j := 1; j <= 5; j++ {
			page.SetValue(prefix+"no_of_"+entryDocTypes[j]+"s", noOfDoc[j])
		}
		page.SetValue(prefix+"largest_balance_lcy", highestBalanceLCY)
		page.SetValue(prefix+"avg_collection_period", avgDaysToPay)
	}
}

// vendorEntryStatistics is form 303's OnAfterGetRecord
func vendorEntryStatistics(rec *gtables.Vendor, page *pages.Control) {
	// the sign each document type is shown with (invoices are credits to the vendor)
	sign := []int64{0, 1, -1, 1, -1, -1}
	for j := 1; j <= 5; j++ {
		vendLedgEntry := gtables.NewVendorLedgerEntry(rec)
		vendLedgEntry.SetCurrentKey("document_type", "vendor_no", "posting_date")
		vendLedgEntry.SetRange("document_type", j)
		vendLedgEntry.SetRange("vendor_no", rec.No)
		found := vendLedgEntry.Find("+")
		prefix := "last_" + entryDocTypes[j] + "_"
		if found {
			page.SetValue(prefix+"date", vendLedgEntry.Posting_date)
			page.SetValue(prefix+"document_no", vendLedgEntry.Document_no)
			page.SetValue(prefix+"currency_code", vendLedgEntry.Currency_code)
		}
		page.SetValue(prefix+"amount", vendLedgEntry.Amount.Mul(types.NewDecimalFromInt(sign[j])))
		page.SetValue(prefix+"remaining_amount", vendLedgEntry.Remaining_amount.Mul(types.NewDecimalFromInt(sign[j])))
	}

	var totalRemainAmountLCY [6]types.Decimal
	vendLedgEntry2 := gtables.NewVendorLedgerEntry(rec)
	vendLedgEntry2.SetCurrentKey("vendor_no", "open")
	vendLedgEntry2.SetRange("vendor_no", rec.No)
	vendLedgEntry2.SetRange("open", true)
	for found := vendLedgEntry2.Find("+"); found; found = vendLedgEntry2.Next(-1) {
		j := int(vendLedgEntry2.Document_type)
		if j > 0 {
			totalRemainAmountLCY[j] = totalRemainAmountLCY[j].Add(vendLedgEntry2.Remaining_amt_lcy)
		}
	}
	for j := 1; j <= 5; j++ {
		page.SetValue("remaining_amt_lcy_"+entryDocTypes[j], totalRemainAmountLCY[j].Mul(types.NewDecimalFromInt(sign[j])))
	}

	vendDateFilter, vendDateName := entryStatisticsDateFilters(nil, rec)
	for i := 1; i <= 3; i++ {
		var noOfDoc [6]int
		totalPaymentDiscLCY := types.ZeroDecimal()
		paymentDiscMissedLCY := types.ZeroDecimal()
		// without accounting periods there are no periods to count in
		if vendDateFilter[i] != "" {
			vendLedgEntry2 = gtables.NewVendorLedgerEntry(rec)
			vendLedgEntry2.SetCurrentKey("vendor_no", "posting_date")
			vendLedgEntry2.SetRange("vendor_no", rec.No)
			vendLedgEntry2.SetFilter("posting_date", vendDateFilter[i])
			for found := vendLedgEntry2.Find("+"); found; found = vendLedgEntry2.Next(-1) {
				j := int(vendLedgEntry2.Document_type)
				if j > 0 {
					noOfDoc[j]++
				}
				totalPaymentDiscLCY = totalPaymentDiscLCY.Add(vendLedgEntry2.Pmt_disc_rcd_lcy)
				if vendLedgEntry2.Document_type == gtables.VendorLedgerEntry_Document_type.Invoice &&
					!vendLedgEntry2.Open && !vendLedgEntry2.Amount.IsZero() {
					paymentDiscMissedLCY = paymentDiscMissedLCY.
						Add(vendLedgEntry2.Pmt_disc_possible.Mul(vendLedgEntry2.Amount_lcy.Div(vendLedgEntry2.Amount))).
						Sub(vendLedgEntry2.Pmt_disc_rcd_lcy)
				}
			}
		}
		prefix := entryStatisticsPeriods[i] + "_"
		page.SetValue(prefix+"name", vendDateName[i])
		for j := 1; j <= 5; j++ {
			page.SetValue(prefix+"no_of_"+entryDocTypes[j]+"s", noOfDoc[j])
		}
		page.SetValue(prefix+"pmt_disc_received_lcy", totalPaymentDiscLCY.Neg())
		page.SetValue(prefix+"pmt_disc_missed_lcy", paymentDiscMissedLCY.Neg())
	}
}
