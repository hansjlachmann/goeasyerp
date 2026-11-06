package codeunits

import (
	"errors"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/format"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Codeunits 226 CustEntry-Apply Posted Entries and 227 VendEntry-Apply Posted Entries: Post
// Application (F11 on Apply Customer/Vendor Entries) asks, then posts the application of the
// entry to the open entries marked with its Applies-to ID (codeunit 12) in one transaction.
// Unapply Entries (on the ledger entries) undoes an entry's latest application: the entries get
// their application fields back and the application's G/L postings are posted the other way
// (the application register, tables 905/906).
const (
	CustEntryApplyPostedEntriesID   = 226
	CustEntryApplyPostedEntriesName = "cust-entry-apply-posted-entries"
	VendEntryApplyPostedEntriesID   = 227
	VendEntryApplyPostedEntriesName = "vend-entry-apply-posted-entries"
)

func init() {
	Register(CustEntryApplyPostedEntriesID, CustEntryApplyPostedEntriesName, NewCustEntryApplyPostedEntries)
	Register(VendEntryApplyPostedEntriesID, VendEntryApplyPostedEntriesName, NewVendEntryApplyPostedEntries)
	// (the ledger entry tables by their API names)
	functions.Register("table", "Customer_ledger_entry", "UnapplyEntries", functions.For(UnapplyCustEntries), functions.WithDialog())
	functions.Register("table", "Vendor_ledger_entry", "UnapplyEntries", functions.For(UnapplyVendEntries), functions.WithDialog())
}

type CustEntryApplyPostedEntries struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewCustEntryApplyPostedEntries creates codeunit 226
func NewCustEntryApplyPostedEntries(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &CustEntryApplyPostedEntries{db: db, company: company, dbType: dbType}
}

func (c *CustEntryApplyPostedEntries) ID() int                    { return CustEntryApplyPostedEntriesID }
func (c *CustEntryApplyPostedEntries) Name() string               { return CustEntryApplyPostedEntriesName }
func (c *CustEntryApplyPostedEntries) SourceTable() string        { return "Customer_ledger_entry" }
func (c *CustEntryApplyPostedEntries) UsesProgress() bool         { return true } // it asks (Confirm)
func (c *CustEntryApplyPostedEntries) GetDB() database.Executor   { return c.db }
func (c *CustEntryApplyPostedEntries) GetCompany() string         { return c.company }
func (c *CustEntryApplyPostedEntries) GetDBType() database.DBType { return c.dbType }

// Run posts the application of the customer entry (the page's current record)
func (c *CustEntryApplyPostedEntries) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.CustomerLedgerEntry)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 226 runs on a customer ledger entry")
	}
	err := rec.TestField("applies_to_id")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if !system.Confirm(ts.Message("APPLY_POST_CONFIRM", lang), rec.GetOptionCaption("document_type", optionKey(rec.Document_type.String()), lang), rec.Document_no) {
		return fcodeunits.Result{Success: true}, nil
	}
	err = inTransaction(c.db, c.company, c.dbType, func(src ftables.Source) error {
		custLedgEntry := gtables.NewCustomerLedgerEntry(src)
		err := custLedgEntry.GetOrError(rec.Entry_no)
		if err != nil {
			return err
		}
		return genjnl.PostCustApplication(src, custLedgEntry)
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	result := system.Message(ts.Message("APPLY_POSTED", lang))
	result.Refresh = true
	return result, nil
}

type VendEntryApplyPostedEntries struct {
	db      database.Executor
	company string
	dbType  database.DBType
}

// NewVendEntryApplyPostedEntries creates codeunit 227
func NewVendEntryApplyPostedEntries(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &VendEntryApplyPostedEntries{db: db, company: company, dbType: dbType}
}

func (c *VendEntryApplyPostedEntries) ID() int                    { return VendEntryApplyPostedEntriesID }
func (c *VendEntryApplyPostedEntries) Name() string               { return VendEntryApplyPostedEntriesName }
func (c *VendEntryApplyPostedEntries) SourceTable() string        { return "Vendor_ledger_entry" }
func (c *VendEntryApplyPostedEntries) UsesProgress() bool         { return true }
func (c *VendEntryApplyPostedEntries) GetDB() database.Executor   { return c.db }
func (c *VendEntryApplyPostedEntries) GetCompany() string         { return c.company }
func (c *VendEntryApplyPostedEntries) GetDBType() database.DBType { return c.dbType }

// Run posts the application of the vendor entry (the page's current record)
func (c *VendEntryApplyPostedEntries) Run(record interface{}) (fcodeunits.Result, error) {
	rec, ok := record.(*gtables.VendorLedgerEntry)
	if !ok {
		return fcodeunits.Result{}, errors.New("codeunit 227 runs on a vendor ledger entry")
	}
	err := rec.TestField("applies_to_id")
	if err != nil {
		return fcodeunits.Result{}, err
	}
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if !system.Confirm(ts.Message("APPLY_POST_CONFIRM", lang), rec.GetOptionCaption("document_type", optionKey(rec.Document_type.String()), lang), rec.Document_no) {
		return fcodeunits.Result{Success: true}, nil
	}
	err = inTransaction(c.db, c.company, c.dbType, func(src ftables.Source) error {
		vendLedgEntry := gtables.NewVendorLedgerEntry(src)
		err := vendLedgEntry.GetOrError(rec.Entry_no)
		if err != nil {
			return err
		}
		return genjnl.PostVendApplication(src, vendLedgEntry)
	})
	if err != nil {
		return fcodeunits.Result{}, err
	}
	result := system.Message(ts.Message("APPLY_POSTED", lang))
	result.Refresh = true
	return result, nil
}

// UnapplyCustEntries (Unapply Entries on Customer Ledger Entries) asks, then unapplies the
// entry's latest application in one transaction
func UnapplyCustEntries(rec *gtables.CustomerLedgerEntry) (fcodeunits.Result, error) {
	src := ftables.DBSource(rec.GetDB(), rec.GetCompany(), rec.GetDBType())
	applicationNo, postingDate := genjnl.LastApplication(src, gtables.EntryApplication_Account_type.Customer, rec.Entry_no)
	return unapply(rec.GetDB(), rec.GetCompany(), rec.GetDBType(), rec.Entry_no, applicationNo, postingDate, func(src ftables.Source) error {
		custLedgEntry := gtables.NewCustomerLedgerEntry(src)
		err := custLedgEntry.GetOrError(rec.Entry_no)
		if err != nil {
			return err
		}
		_, err = genjnl.UnapplyCustEntry(src, custLedgEntry)
		return err
	})
}

// UnapplyVendEntries (Unapply Entries on Vendor Ledger Entries)
func UnapplyVendEntries(rec *gtables.VendorLedgerEntry) (fcodeunits.Result, error) {
	src := ftables.DBSource(rec.GetDB(), rec.GetCompany(), rec.GetDBType())
	applicationNo, postingDate := genjnl.LastApplication(src, gtables.EntryApplication_Account_type.Vendor, rec.Entry_no)
	return unapply(rec.GetDB(), rec.GetCompany(), rec.GetDBType(), rec.Entry_no, applicationNo, postingDate, func(src ftables.Source) error {
		vendLedgEntry := gtables.NewVendorLedgerEntry(src)
		err := vendLedgEntry.GetOrError(rec.Entry_no)
		if err != nil {
			return err
		}
		_, err = genjnl.UnapplyVendEntry(src, vendLedgEntry)
		return err
	})
}

// unapply asks about the entry's latest application, then unapplies it in one transaction
func unapply(db database.Executor, company string, dbType database.DBType, entryNo, applicationNo int, postingDate types.Date, run func(src ftables.Source) error) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	if applicationNo == 0 {
		return system.Message(ts.Message("UNAPPLY_NOTHING", lang), entryNo), nil
	}
	if !system.Confirm(ts.Message("UNAPPLY_CONFIRM", lang), applicationNo, format.Date(postingDate, lang), entryNo) {
		return fcodeunits.Result{Success: true}, nil
	}
	err := inTransaction(db, company, dbType, run)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	result := system.Message(ts.Message("UNAPPLY_DONE", lang), applicationNo)
	result.Refresh = true
	return result, nil
}

// inTransaction runs fn in one transaction (committed with the balance check, rolled back on an
// error)
func inTransaction(db database.Executor, company string, dbType database.DBType, fn func(src ftables.Source) error) error {
	tx, err := ftables.BeginTransaction(db)
	if err != nil {
		return err
	}
	err = fn(ftables.DBSource(tx, company, dbType))
	if err != nil {
		_ = ftables.Rollback(tx)
		return err
	}
	return ftables.Commit(tx)
}

// optionKey is the caption key of an option value ("Credit Memo" → credit_memo)
func optionKey(value string) string {
	return i18n.Key(value)
}
