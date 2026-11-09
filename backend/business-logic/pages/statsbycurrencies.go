package pages

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/pages"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Pages 481 Customer Stats. by Currencies and 482 Vendor Stats. by Currencies (classic forms
// 481/482 with their lines 486/487): the customer's or vendor's balance, outstanding orders,
// shipped/received not invoiced and balance due per currency — the blank currency (LCY) and
// every currency, those with ledger entries of the customer or vendor (Cust./Vendor Ledg.
// Entries in Filter). The page is a temporary list of currencies filled from the page
// parameter customer_no / vendor_no, which also is the currencies' Customer/Vendor Filter.
const (
	custStatsByCurrenciesPageID = 481
	vendStatsByCurrenciesPageID = 482
)

func init() {
	pages.OnFindRecords(custStatsByCurrenciesPageID, func(rec *gtables.Currency, page *pages.Open) error {
		customerNo := page.Param("customer_no")
		err := rec.SetFlowFilter("customer_filter", customerNo)
		if err != nil {
			return err
		}
		return insertCurrencies(rec, func(currency2 *gtables.Currency) (bool, error) {
			err := currency2.SetFlowFilter("customer_filter", customerNo)
			if err != nil {
				return false, err
			}
			currency2.CalcFields("cust_ledg_entries_in_filter")
			return currency2.Cust_ledg_entries_in_filter, nil
		})
	})
	pages.OnAfterGetRecord(custStatsByCurrenciesPageID, func(rec *gtables.Currency, page *pages.Control) {
		currency := gtables.NewCurrency(rec)
		currency.Code = rec.Code
		_ = currency.SetFlowFilter("customer_filter", system.PageParam("customer_no"))
		currency.CalcFields("customer_balance", "customer_outstanding_orders", "customer_shipped_not_invoiced")
		totalAmount := currency.Customer_balance.Add(currency.Customer_outstanding_orders).Add(currency.Customer_shipped_not_invoiced)
		page.SetValue("total_amount", totalAmount)
	})

	pages.OnFindRecords(vendStatsByCurrenciesPageID, func(rec *gtables.Currency, page *pages.Open) error {
		vendorNo := page.Param("vendor_no")
		err := rec.SetFlowFilter("vendor_filter", vendorNo)
		if err != nil {
			return err
		}
		return insertCurrencies(rec, func(currency2 *gtables.Currency) (bool, error) {
			err := currency2.SetFlowFilter("vendor_filter", vendorNo)
			if err != nil {
				return false, err
			}
			currency2.CalcFields("vendor_ledg_entries_in_filter")
			return currency2.Vendor_ledg_entries_in_filter, nil
		})
	})
	pages.OnAfterGetRecord(vendStatsByCurrenciesPageID, func(rec *gtables.Currency, page *pages.Control) {
		currency := gtables.NewCurrency(rec)
		currency.Code = rec.Code
		_ = currency.SetFlowFilter("vendor_filter", system.PageParam("vendor_no"))
		currency.CalcFields("vendor_balance", "vendor_outstanding_orders", "vendor_amt_rcd_not_invoiced")
		totalAmount := currency.Vendor_balance.Add(currency.Vendor_outstanding_orders).Add(currency.Vendor_amt_rcd_not_invoiced)
		page.SetValue("total_amount", totalAmount)
	})
}

// insertCurrencies fills the temporary currencies (form 486/487 OnOpenForm): the blank currency
// and every currency, each only when inFilter says the customer or vendor has entries in it
func insertCurrencies(rec *gtables.Currency, inFilter func(currency2 *gtables.Currency) (bool, error)) error {
	rec.DeleteAll()
	blankCurrency := gtables.NewCurrency(rec)
	blankCurrency.Code = types.Code("")
	include, err := inFilter(blankCurrency)
	if err != nil {
		return err
	}
	if include {
		rec.Init()
		rec.Code = ""
		err = rec.InsertOrError(false)
		if err != nil {
			return err
		}
	}
	currency := gtables.NewCurrency(rec)
	for found := currency.FindSetBuffered(); found; found = currency.Next() {
		include, err = inFilter(currency)
		if err != nil {
			return err
		}
		if include {
			rec.Init()
			rec.Code = currency.Code
			rec.Description = currency.Description
			err = rec.InsertOrError(false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
