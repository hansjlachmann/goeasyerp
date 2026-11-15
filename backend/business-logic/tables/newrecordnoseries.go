package tables

import (
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// NewRecordNoSeries: the number series a new record of an auto-numbered table (auto_key) gets
// its No. from when it is inserted without one — the series its OnInsert passes to InitSeries;
// "" for a table without one
func NewRecordNoSeries(rec ftables.Source) types.Code {
	code, _ := newRecordNoSeries(rec)
	return code
}

// newRecordNoSeries: the series, and whether the table is one of the auto-numbered tables this
// knows (a test checks every auto_key table is)
func newRecordNoSeries(rec ftables.Source) (types.Code, bool) {
	switch r := rec.(type) {
	case *gtables.Customer:
		return gtables.GetSalesReceivablesSetup(r).Customer_nos, true
	case *gtables.Vendor:
		return gtables.GetPurchasesPayablesSetup(r).Vendor_nos, true
	case *gtables.Item:
		return gtables.GetInventorySetup(r).Item_nos, true
	case *gtables.Resource:
		return gtables.GetResourcesSetup(r).Resource_nos, true
	case *gtables.BankAccount:
		return gtables.GetGeneralLedgerSetup(r).Bank_account_nos, true
	case *gtables.FixedAsset:
		return gtables.GetFASetup(r).Fixed_asset_nos, true
	case *gtables.Insurance:
		return gtables.GetFASetup(r).Insurance_nos, true
	case *gtables.Project:
		return gtables.GetProjectsSetup(r).Project_nos, true
	case *gtables.SalesHeader:
		return getSalesNoSeriesCode(r, gtables.GetSalesReceivablesSetup(r)), true
	case *gtables.PurchaseHeader:
		return getPurchNoSeriesCode(r, gtables.GetPurchasesPayablesSetup(r)), true
	}
	return "", false
}

// NewRecordGetsNumber: a new record of the table gets the next number of a series when it is
// inserted without a No. — its series exists and gives numbers by default (Default Nos.); with
// no series, or one for numbers typed by hand only, the user types the number
func NewRecordGetsNumber(rec ftables.Source) bool {
	code := NewRecordNoSeries(rec)
	if code == "" {
		return false
	}
	noSeries := gtables.NewNoSeries(rec)
	if !noSeries.Get(code) {
		return false
	}
	return noSeries.Default_nos
}
