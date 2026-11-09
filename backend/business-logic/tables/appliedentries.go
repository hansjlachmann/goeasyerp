package tables

import (
	"strconv"
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Applied Entries on the customer and vendor ledger entries (forms 61 Applied Customer Entries,
// 62 Applied Vendor Entries): the entry that closed the entry and the entries it closed, of the
// same customer or vendor.

// The pages of the applied entries
const (
	AppliedCustomerEntriesPageID = 61
	AppliedVendorEntriesPageID   = 62
)

func init() {
	functions.Register("table", "Customer_ledger_entry", "AppliedEntries", functions.For(CustLedgEntryAppliedEntries), functions.WithDialog())
	functions.Register("table", "Vendor_ledger_entry", "AppliedEntries", functions.For(VendLedgEntryAppliedEntries), functions.WithDialog())
}

// CustLedgEntryAppliedEntries (Applied Entries, form 61 OnOpenForm): page 61 on the entries
// applied to the entry
func CustLedgEntryAppliedEntries(rec *gtables.CustomerLedgerEntry) (fcodeunits.Result, error) {
	applied, err := AppliedCustEntries(rec)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	system.RunModal(AppliedCustomerEntriesPageID, applied, false)
	return fcodeunits.Result{Success: true}, nil
}

// AppliedCustEntries: the entries applied to an entry, as a record filtered on them — the one
// whose entry closed it and those it closed, of its customer (MARK/MARKEDONLY in the classic form)
func AppliedCustEntries(rec *gtables.CustomerLedgerEntry) (*gtables.CustomerLedgerEntry, error) {
	createCustLedgEntry := gtables.NewCustomerLedgerEntry(rec)
	err := createCustLedgEntry.GetOrError(rec.Entry_no)
	if err != nil {
		return nil, err
	}
	var entryNos []string
	if createCustLedgEntry.Closed_by_entry_no != 0 {
		entryNos = append(entryNos, strconv.Itoa(createCustLedgEntry.Closed_by_entry_no))
	}
	custLedgEntry := gtables.NewCustomerLedgerEntry(rec)
	custLedgEntry.SetRange("closed_by_entry_no", createCustLedgEntry.Entry_no)
	for found := custLedgEntry.FindSetBuffered(); found; found = custLedgEntry.Next() {
		entryNos = append(entryNos, strconv.Itoa(custLedgEntry.Entry_no))
	}
	if len(entryNos) == 0 {
		entryNos = []string{"0"} // none (MARKEDONLY without marks)
	}
	applied := gtables.NewCustomerLedgerEntry(rec)
	applied.SetFilter("entry_no", strings.Join(entryNos, "|"))
	applied.SetRange("customer_no", createCustLedgEntry.Customer_no)
	return applied, nil
}

// VendLedgEntryAppliedEntries (Applied Entries, form 62 OnOpenForm): page 62 on the entries
// applied to the entry
func VendLedgEntryAppliedEntries(rec *gtables.VendorLedgerEntry) (fcodeunits.Result, error) {
	applied, err := AppliedVendEntries(rec)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	system.RunModal(AppliedVendorEntriesPageID, applied, false)
	return fcodeunits.Result{Success: true}, nil
}

// AppliedVendEntries: the entries applied to an entry, as a record filtered on them — the one
// whose entry closed it and those it closed, of its vendor
func AppliedVendEntries(rec *gtables.VendorLedgerEntry) (*gtables.VendorLedgerEntry, error) {
	createVendLedgEntry := gtables.NewVendorLedgerEntry(rec)
	err := createVendLedgEntry.GetOrError(rec.Entry_no)
	if err != nil {
		return nil, err
	}
	var entryNos []string
	if createVendLedgEntry.Closed_by_entry_no != 0 {
		entryNos = append(entryNos, strconv.Itoa(createVendLedgEntry.Closed_by_entry_no))
	}
	vendLedgEntry := gtables.NewVendorLedgerEntry(rec)
	vendLedgEntry.SetRange("closed_by_entry_no", createVendLedgEntry.Entry_no)
	for found := vendLedgEntry.FindSetBuffered(); found; found = vendLedgEntry.Next() {
		entryNos = append(entryNos, strconv.Itoa(vendLedgEntry.Entry_no))
	}
	if len(entryNos) == 0 {
		entryNos = []string{"0"} // none (MARKEDONLY without marks)
	}
	applied := gtables.NewVendorLedgerEntry(rec)
	applied.SetFilter("entry_no", strings.Join(entryNos, "|"))
	applied.SetRange("vendor_no", createVendLedgEntry.Vendor_no)
	return applied, nil
}
