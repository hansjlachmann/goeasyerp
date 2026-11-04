package demodata

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Demo vendors (data/vendors.yaml) and their ledger entries: purchase invoices over the last
// year, most of them paid. The vendors' random numbers come from a generator of their own,
// so the customers' data stays the same.

type vendorData struct {
	No           string `yaml:"no"`
	Name         string `yaml:"name"`
	Address      string `yaml:"address"`
	PostCode     string `yaml:"post_code"`
	City         string `yaml:"city"`
	Country      string `yaml:"country"`
	Phone        string `yaml:"phone"`
	PaymentTerms string `yaml:"payment_terms"`
	Currency     string `yaml:"currency"`
}

type vendorsFile struct {
	Vendors []vendorData `yaml:"vendors"`
}

type vendorEntry struct {
	vendorNo    string
	date        time.Time
	docType     gtables.VendorLedgerEntryDocument_type
	docNo       string
	extDocNo    string
	description string
	amount      float64 // negative for invoices (what is owed), positive for payments
	remaining   float64
	purchase    float64
	open        bool
	due         time.Time
	closedBy    *vendorEntry
	closedAt    time.Time
	entryNo     int
	bankAccount string // the bank account of a payment
}

func insertVendors(exec database.Executor, company string, dbType database.DBType, vendors []vendorData, counts *Counts) error {
	purchasers := []string{"AH", "JB", "PL"}
	for i, v := range vendors {
		rec := gtables.NewVendor(ftables.DBSource(exec, company, dbType))
		rec.InitRecord()
		rec.No = types.NewCode(v.No)
		rec.Name = types.NewText(v.Name)
		rec.Search_name = types.NewCode(strings.ToUpper(v.Name))
		rec.Address = types.NewText(v.Address)
		rec.Post_code = types.NewCode(v.PostCode)
		rec.City = types.NewText(v.City)
		rec.Country_code = types.NewCode(v.Country)
		rec.Phone_no = types.NewText(v.Phone)
		rec.Payment_terms_code = types.NewCode(v.PaymentTerms)
		rec.Currency_code = types.NewCode(v.Currency)
		rec.Invoice_disc_code = rec.No
		rec.Purchaser_code = types.NewCode(purchasers[i%len(purchasers)])
		rec.Payment_method_code = "BANK"
		if v.Country == "NO" {
			rec.Vendor_posting_group, rec.Gen_bus_posting_group, rec.Tax_bus_posting_group = "DOMESTIC", "DOMESTIC", "DOMESTIC"
			rec.Tax_area_code, rec.Tax_liable = "DOMESTIC", true
			rec.Shipment_method_code, rec.Shipping_agent_code = "TRUCK", "OWN"
		} else {
			rec.Vendor_posting_group, rec.Gen_bus_posting_group, rec.Tax_bus_posting_group = "FOREIGN", "EXPORT", "EXPORT"
			rec.Tax_area_code = "ABROAD"
			rec.Shipment_method_code, rec.Shipping_agent_code = "TRUCK", "DHL"
		}
		err := rec.InsertOrError(true)
		if err != nil {
			return &InsertError{Table: gtables.VendorTableName, Key: v.No, Err: err}
		}
		counts.Vendors++
	}
	return nil
}

// generateVendorEntries: one to two purchase invoices a month per vendor over the last year;
// those due more than five days ago are paid (most of them)
func generateVendorEntries(vendors []vendorData, dueDays map[string]int, today time.Time) []*vendorEntry {
	// payments in pounds go through the GBP account, the others through the operating account
	bankAccount := func(v vendorData) string {
		if v.Currency == "GBP" {
			return "B030"
		}
		return string(demoBankAccount)
	}
	rng := rand.New(rand.NewSource(seed + 23))
	day := func(t time.Time, days int) time.Time { return t.AddDate(0, 0, days) }
	var entries []*vendorEntry
	for _, v := range vendors {
		n := 12 + rng.Intn(12)
		for i := 0; i < n; i++ {
			date := day(today, -rng.Intn(365))
			amount := -round2(1000 + rng.Float64()*39000)
			inv := &vendorEntry{
				vendorNo:  v.No,
				date:      date,
				docType:   gtables.VendorLedgerEntry_Document_type.Invoice,
				extDocNo:  fmt.Sprintf("%s-%04d", strings.ToUpper(v.No[len(v.No)-2:]), 1000+rng.Intn(9000)),
				amount:    amount,
				remaining: amount,
				purchase:  amount, // with the sign of the amount: Purchases (LCY) is its -Sum
				open:      true,
				due:       day(date, dueDays[strings.ToUpper(v.PaymentTerms)]),
			}
			entries = append(entries, inv)
			if inv.due.Before(day(today, -5)) && rng.Float64() < 0.92 {
				payDate := inv.due
				if payDate.After(today) {
					payDate = today
				}
				pay := &vendorEntry{
					vendorNo:    v.No,
					date:        payDate,
					docType:     gtables.VendorLedgerEntry_Document_type.Payment,
					amount:      -inv.remaining,
					closedBy:    inv,
					closedAt:    payDate,
					bankAccount: bankAccount(v),
				}
				entries = append(entries, pay)
				inv.remaining, inv.open = 0, false
				inv.closedBy, inv.closedAt = pay, payDate
			}
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].date.Before(entries[j].date) })
	var invoices, payments int
	for _, e := range entries {
		if e.docType == gtables.VendorLedgerEntry_Document_type.Invoice {
			invoices++
			e.docNo = fmt.Sprintf("PI%06d", invoices)
			e.description = "Invoice " + e.extDocNo
		} else {
			payments++
			e.docNo = fmt.Sprintf("VPAY%06d", payments)
		}
	}
	for _, e := range entries {
		if e.docType == gtables.VendorLedgerEntry_Document_type.Payment {
			e.description = "Payment " + e.closedBy.docNo
		}
	}
	return entries
}

func insertVendorEntries(exec database.Executor, company string, dbType database.DBType, entries []*vendorEntry, counts *Counts) error {
	last := gtables.NewVendorLedgerEntry(ftables.DBSource(exec, company, dbType))
	firstNo := 1
	if last.FindLast() {
		firstNo = last.Entry_no + 1
	}
	for i, e := range entries {
		e.entryNo = firstNo + i
	}
	batch := make([]*gtables.VendorLedgerEntry, 0, len(entries))
	for _, e := range entries {
		rec := gtables.NewVendorLedgerEntry(last)
		rec.Entry_no = e.entryNo
		rec.Vendor_no = types.NewCode(e.vendorNo)
		rec.Buy_from_vendor_no = types.NewCode(e.vendorNo)
		rec.Posting_date = types.NewDateFromTime(e.date)
		rec.Document_date = types.NewDateFromTime(e.date)
		rec.Document_type = e.docType
		rec.Document_no = types.NewCode(e.docNo)
		rec.External_document_no = types.NewCode(e.extDocNo)
		rec.Description = types.NewText(e.description)
		rec.Amount = types.NewDecimal(e.amount)
		rec.Amount_lcy = types.NewDecimal(e.amount)
		rec.Original_amount_lcy = types.NewDecimal(e.amount)
		tables.SetVendDebitCredit(rec)
		rec.Remaining_amount = types.NewDecimal(e.remaining)
		rec.Remaining_amt_lcy = types.NewDecimal(e.remaining)
		rec.Purchase_lcy = types.NewDecimal(e.purchase)
		rec.Open = e.open
		rec.Positive = e.amount > 0
		if !e.due.IsZero() {
			rec.Due_date = types.NewDateFromTime(e.due)
		}
		if e.closedBy != nil {
			rec.Closed_by_entry_no = e.closedBy.entryNo
			rec.Closed_at_date = types.NewDateFromTime(e.closedAt)
		}
		if e.docType == gtables.VendorLedgerEntry_Document_type.Payment {
			rec.Bal_account_type = gtables.VendorLedgerEntry_Bal_account_type.BankAccount
			rec.Bal_account_no = types.NewCode(e.bankAccount)
		}
		batch = append(batch, rec)
	}
	if len(batch) == 0 {
		return nil
	}
	err := last.InsertAll(batch, true)
	if err != nil {
		var batchErr *ftables.BatchInsertError
		key, cause := "", err
		if errors.As(err, &batchErr) {
			key, cause = fmt.Sprint(entries[batchErr.Index].entryNo), batchErr.Err
		}
		return &InsertError{Table: gtables.VendorLedgerEntryTableName, Key: key, Err: cause}
	}
	counts.VendorLedgerEntries += len(batch)
	return nil
}

// vendorSeriesUse: the last numbers the demo vendors and posted purchase invoices took (VEND,
// P-INV+), so the series continue after them
func vendorSeriesUse(vendors []vendorData, entries []*vendorEntry) map[string]ledgerUse {
	last := map[string]ledgerUse{}
	for _, v := range vendors {
		if v.No > last["VEND"].no {
			last["VEND"] = ledgerUse{v.No, time.Time{}}
		}
	}
	for _, e := range entries {
		if e.docType == gtables.VendorLedgerEntry_Document_type.Invoice && e.docNo > last["P-INV+"].no {
			last["P-INV+"] = ledgerUse{e.docNo, e.date}
		}
	}
	return last
}
